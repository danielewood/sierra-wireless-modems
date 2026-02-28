package cmd

import (
	"strconv"
	"strings"
	"sync"

	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
	"github.com/danielewood/sierra-wireless-modems/swtool/qmi"
)

// qmiCache holds cached QMI query results so we don't re-poll slow queries
// on every streaming callback. Signal info is always fresh; the rest is cached.
type qmiCache struct {
	mu      sync.Mutex
	serving *qmi.ServingSystem
	sysInfo *qmi.SystemInfo
	cells   *qmi.CellLocationInfo
}

// createQMIClient returns a QueryClient if a control device exists.
// Native QMI is tried first; qmicli is used as fallback if available.
// Returns nil if no control device is present (graceful AT-only fallback).
func createQMIClient(dev *modem.Device) *qmi.QueryClient {
	if dev.CDCDevice != "" {
		return qmi.NewQueryClient(dev.CDCDevice, true, logger)
	}
	if dev.QCQMIDevice != "" {
		return qmi.NewQueryClient(dev.QCQMIDevice, false, logger)
	}
	return nil
}

// overlayQMIData runs QMI queries and merges results into the Info struct.
// For one-shot use (no cache). Each query is soft-fail.
func overlayQMIData(qc *qmi.QueryClient, info *modem.Info) {
	if sig, err := qc.GetSignalInfo(); err == nil {
		overlaySignalInfo(sig, info)
	} else {
		logger.Debugf("QMI signal info: %v", err)
	}
	if srv, err := qc.GetServingSystem(); err == nil {
		overlayServingSystem(srv, info)
	} else {
		logger.Debugf("QMI serving system: %v", err)
	}
	if sys, err := qc.GetSystemInfo(); err == nil {
		overlaySystemInfo(sys, info)
	} else {
		logger.Debugf("QMI system info: %v", err)
	}
	if cells, err := qc.GetCellLocationInfo(); err == nil {
		overlayCellLocationInfo(cells, info)
	} else {
		logger.Debugf("QMI cell location: %v", err)
	}
}

// overlayQMICached runs QMI queries with caching for the TUI polling loop.
// Signal info is always fresh (fast, changes often). Serving system, system
// info, and cell location are cached and only re-queried once per poll cycle
// (when freshPoll is true — i.e., first callback of a new cycle).
func overlayQMICached(qc *qmi.QueryClient, cache *qmiCache, freshPoll bool, info *modem.Info) {
	// Signal — always fresh (fast query, changes frequently)
	if sig, err := qc.GetSignalInfo(); err == nil {
		overlaySignalInfo(sig, info)
	} else {
		logger.Debugf("QMI signal info: %v", err)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if freshPoll {
		// Re-query slow/stable data once per poll cycle
		if srv, err := qc.GetServingSystem(); err == nil {
			cache.serving = srv
		} else {
			logger.Debugf("QMI serving system: %v", err)
		}
		if sys, err := qc.GetSystemInfo(); err == nil {
			cache.sysInfo = sys
		} else {
			logger.Debugf("QMI system info: %v", err)
		}
		if cells, err := qc.GetCellLocationInfo(); err == nil {
			cache.cells = cells
		} else {
			logger.Debugf("QMI cell location: %v", err)
		}
	}

	// Apply cached data
	if cache.serving != nil {
		overlayServingSystem(cache.serving, info)
	}
	if cache.sysInfo != nil {
		overlaySystemInfo(cache.sysInfo, info)
	}
	if cache.cells != nil {
		overlayCellLocationInfo(cache.cells, info)
	}
}

func overlaySignalInfo(sig *qmi.SignalInfo, info *modem.Info) {
	if info.Diagnostics.GStatus == nil {
		info.Diagnostics.GStatus = make(map[string]string)
	}

	if sig.LTE != nil {
		if sig.LTE.RSRP != "" {
			info.Diagnostics.GStatus["RSRP (dBm)"] = stripUnit(sig.LTE.RSRP)
		}
		if sig.LTE.RSRQ != "" {
			info.Diagnostics.GStatus["RSRQ (dB)"] = stripUnit(sig.LTE.RSRQ)
		}
		if sig.LTE.SNR != "" {
			info.Diagnostics.GStatus["SINR (dB)"] = stripUnit(sig.LTE.SNR)
		}
		if sig.LTE.RSSI != "" {
			if rssi := parseDBM(sig.LTE.RSSI); rssi != 0 {
				info.Diagnostics.RSSI = rssi
				info.Diagnostics.GStatus["RSSI (dBm)"] = strconv.Itoa(rssi)
			}
		}
	}
}

func overlayServingSystem(srv *qmi.ServingSystem, info *modem.Info) {
	if srv.MCC != "" && srv.MNC != "" {
		info.Operator = srv.MCC + srv.MNC
	}

	if srv.Registered {
		status := "registered"
		if srv.Roaming {
			status = "registered, roaming"
		}
		if srv.Domain != "" {
			switch {
			case strings.Contains(srv.Domain, "cs") && strings.Contains(srv.Domain, "ps"):
				info.Registration.CS = modem.RegStatus{Status: status}
				info.Registration.EPS = modem.RegStatus{Status: status}
			case strings.Contains(srv.Domain, "ps"):
				info.Registration.EPS = modem.RegStatus{Status: status}
			case strings.Contains(srv.Domain, "cs"):
				info.Registration.CS = modem.RegStatus{Status: status}
			}
		}
	}

	if srv.RAT != "" {
		if info.Diagnostics.GStatus == nil {
			info.Diagnostics.GStatus = make(map[string]string)
		}
		info.Diagnostics.GStatus["System mode"] = strings.ToUpper(srv.RAT)
	}
}

func overlaySystemInfo(sys *qmi.SystemInfo, info *modem.Info) {
	if info.Diagnostics.GStatus == nil {
		info.Diagnostics.GStatus = make(map[string]string)
	}

	if sys.TAC != "" {
		info.Diagnostics.GStatus["TAC"] = sys.TAC
	}
	if sys.CellID != "" {
		info.Diagnostics.GStatus["Cell ID"] = sys.CellID
	}
	if sys.ServiceStatus != "" {
		info.Diagnostics.GStatus["PS state"] = sys.ServiceStatus
	}
}

func overlayCellLocationInfo(cells *qmi.CellLocationInfo, info *modem.Info) {
	if len(cells.LTEIntra) > 0 {
		info.LTEDetail.IntraFreq = cells.LTEIntra
	}
	if len(cells.LTEInter) > 0 {
		info.LTEDetail.InterFreq = cells.LTEInter
	}
}

// stripUnit removes common unit suffixes from QMI values.
// e.g. "-89 dBm" -> "-89", "7.8 dB" -> "7.8"
func stripUnit(s string) string {
	s = strings.TrimSpace(s)
	for _, suffix := range []string{" dBm", " dB"} {
		s = strings.TrimSuffix(s, suffix)
	}
	return s
}

// parseDBM extracts an integer dBm value from a string like "-55 dBm" or "-55".
func parseDBM(s string) int {
	s = stripUnit(s)
	v, _ := strconv.Atoi(s)
	return v
}
