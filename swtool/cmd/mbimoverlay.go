package cmd

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/danielewood/sierra-wireless-modems/swtool/mbim"
	"github.com/danielewood/sierra-wireless-modems/swtool/modem"
)

// mbimCache holds cached MBIM query results for the TUI polling loop.
type mbimCache struct {
	mu       sync.Mutex
	devCaps  *mbim.DeviceCaps
	subReady *mbim.SubscriberReadyStatus
	regState *mbim.RegisterState
}

// createMBIMClient returns an MBIM client for the modem's CDC device.
// Returns nil if no CDC device is present.
func createMBIMClient(dev *modem.Device) *mbim.Client {
	if dev.CDCDevice == "" {
		return nil
	}
	mc, err := mbim.NewClient(dev.CDCDevice, logger)
	if err != nil {
		logger.Debugf("MBIM client: %v", err)
		return nil
	}
	return mc
}

// overlayMBIMData runs MBIM queries and merges results into the Info struct.
// For one-shot use (no cache). Each query is soft-fail.
func overlayMBIMData(mc *mbim.Client, info *modem.Info) {
	if caps, err := mc.GetDeviceCaps(); err == nil {
		overlayDeviceCaps(caps, info)
	} else {
		logger.Debugf("MBIM device caps: %v", err)
	}
	if sub, err := mc.GetSubscriberReadyStatus(); err == nil {
		overlaySubscriberReady(sub, info)
	} else {
		logger.Debugf("MBIM subscriber status: %v", err)
	}
	if reg, err := mc.GetRegisterState(); err == nil {
		overlayRegisterState(reg, info)
	} else {
		logger.Debugf("MBIM register state: %v", err)
	}
	if sig, err := mc.GetSignalState(); err == nil {
		overlaySignalState(sig, info)
	} else {
		logger.Debugf("MBIM signal state: %v", err)
	}
}

// overlayMBIMCached runs MBIM queries with caching for the TUI polling loop.
// Signal is always fresh; device caps, subscriber status, and registration
// are cached and only re-queried when freshPoll is true.
func overlayMBIMCached(mc *mbim.Client, cache *mbimCache, freshPoll bool, info *modem.Info) {
	// Signal — always fresh
	if sig, err := mc.GetSignalState(); err == nil {
		overlaySignalState(sig, info)
	} else {
		logger.Debugf("MBIM signal state: %v", err)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if freshPoll {
		if caps, err := mc.GetDeviceCaps(); err == nil {
			cache.devCaps = caps
		} else {
			logger.Debugf("MBIM device caps: %v", err)
		}
		if sub, err := mc.GetSubscriberReadyStatus(); err == nil {
			cache.subReady = sub
		} else {
			logger.Debugf("MBIM subscriber status: %v", err)
		}
		if reg, err := mc.GetRegisterState(); err == nil {
			cache.regState = reg
		} else {
			logger.Debugf("MBIM register state: %v", err)
		}
	}

	if cache.devCaps != nil {
		overlayDeviceCaps(cache.devCaps, info)
	}
	if cache.subReady != nil {
		overlaySubscriberReady(cache.subReady, info)
	}
	if cache.regState != nil {
		overlayRegisterState(cache.regState, info)
	}
}

func overlayDeviceCaps(caps *mbim.DeviceCaps, info *modem.Info) {
	if caps.DeviceID != "" && info.Identity.IMEI == "" {
		info.Identity.IMEI = caps.DeviceID
	}
	if caps.FirmwareInfo != "" {
		// MBIM firmware info is often more detailed than AT+CGMR
		if info.Identity.Revision == "" || len(caps.FirmwareInfo) > len(info.Identity.Revision) {
			info.Identity.Revision = caps.FirmwareInfo
		}
	}
	if caps.HardwareInfo != "" {
		info.Identity.HardwareRev = caps.HardwareInfo
	}
	// Populate data class capabilities
	if caps.DataClass != 0 {
		if info.Diagnostics.GStatus == nil {
			info.Diagnostics.GStatus = make(map[string]string)
		}
		info.Diagnostics.GStatus["Data class"] = mbim.DataClassDescription(caps.DataClass)
	}
}

func overlaySubscriberReady(sub *mbim.SubscriberReadyStatus, info *modem.Info) {
	// SIM status — MBIM is authoritative since AT+CPIN? returns ERROR on Intel
	simStatus := mbim.ReadyStateName(sub.ReadyState)
	if info.SIM.Status == "" || info.SIM.Status == "ERROR" {
		info.SIM.Status = simStatus
	}
	if sub.SubscriberID != "" && info.SIM.IMSI == "" {
		info.SIM.IMSI = sub.SubscriberID
	}
	if sub.SimICCID != "" && info.SIM.ICCID == "" {
		info.SIM.ICCID = sub.SimICCID
	}
}

func overlayRegisterState(reg *mbim.RegisterState, info *modem.Info) {
	if info.Diagnostics.GStatus == nil {
		info.Diagnostics.GStatus = make(map[string]string)
	}

	// Always show MBIM registration state
	info.Diagnostics.GStatus["MBIM state"] = mbim.RegStateName(reg.State)

	// Registration status
	if reg.State == mbim.RegStateHome || reg.State == mbim.RegStateRoaming || reg.State == mbim.RegStatePartner {
		regStatus := mbim.RegStateName(reg.State)
		if info.Registration.EPS.Status == "" || info.Registration.EPS.Status == "Not registered" {
			info.Registration.EPS = modem.RegStatus{Status: regStatus}
		}
	}

	// Operator
	if reg.ProviderName != "" && (info.Operator == "" || info.Operator == "0") {
		info.Operator = reg.ProviderName
	} else if reg.ProviderID != "" && (info.Operator == "" || info.Operator == "0") {
		info.Operator = reg.ProviderID
	}

	// RAT from available data class
	if rat := mbim.DataClassName(reg.AvailableDataClass); rat != "" {
		info.Diagnostics.GStatus["System mode"] = rat
	}
}

func overlaySignalState(sig *mbim.SignalState, info *modem.Info) {
	if info.Diagnostics.GStatus == nil {
		info.Diagnostics.GStatus = make(map[string]string)
	}

	if sig.RSSI < 99 {
		rssiDBM := sig.RSSIdBm()
		info.Diagnostics.GStatus["RSSI (dBm)"] = strconv.Itoa(rssiDBM)
		if info.Diagnostics.RSSI == 0 {
			info.Diagnostics.RSSI = rssiDBM
			info.Diagnostics.SignalBars = rssiToBars(rssiDBM)
		}
	}
	if sig.ErrorRate < 99 {
		info.Diagnostics.BER = int(sig.ErrorRate)
		info.Diagnostics.GStatus["BER"] = fmt.Sprintf("%d", sig.ErrorRate)
	}
}

// rssiToBars is duplicated here since it's in modem package.
// TODO: move to a shared location.
func rssiToBars(rssi int) int {
	switch {
	case rssi >= -65:
		return 5
	case rssi >= -75:
		return 4
	case rssi >= -85:
		return 3
	case rssi >= -95:
		return 2
	case rssi >= -105:
		return 1
	default:
		return 0
	}
}
