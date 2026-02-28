package modem

import "fmt"

// earfcnBand defines the EARFCN-to-frequency mapping for one E-UTRA band.
// From 3GPP TS 36.101 Table 5.7.3-1.
type earfcnBand struct {
	Band   int
	Name   string  // e.g. "850", "AWS-1"
	FDLLow float64 // lowest DL frequency (MHz)
	NDLMin int     // lowest DL EARFCN
	NDLMax int     // highest DL EARFCN
	FULLow float64 // lowest UL frequency (MHz), 0 for TDD/SDL
	NULMin int     // lowest UL EARFCN, 0 for TDD/SDL
	NULMax int     // highest UL EARFCN, 0 for TDD/SDL
}

// Common E-UTRA bands for North America and Europe.
var earfcnBands = []earfcnBand{
	// FDD bands
	{Band: 1, Name: "2100", FDLLow: 2110, NDLMin: 0, NDLMax: 599, FULLow: 1920, NULMin: 18000, NULMax: 18599},
	{Band: 2, Name: "1900 PCS", FDLLow: 1930, NDLMin: 600, NDLMax: 1199, FULLow: 1850, NULMin: 18600, NULMax: 19199},
	{Band: 3, Name: "1800+", FDLLow: 1805, NDLMin: 1200, NDLMax: 1949, FULLow: 1710, NULMin: 19200, NULMax: 19949},
	{Band: 4, Name: "AWS-1", FDLLow: 2110, NDLMin: 1950, NDLMax: 2399, FULLow: 1710, NULMin: 19950, NULMax: 20399},
	{Band: 5, Name: "850", FDLLow: 869, NDLMin: 2400, NDLMax: 2649, FULLow: 824, NULMin: 20400, NULMax: 20649},
	{Band: 7, Name: "2600", FDLLow: 2620, NDLMin: 2750, NDLMax: 3449, FULLow: 2500, NULMin: 20750, NULMax: 21449},
	{Band: 8, Name: "900 GSM", FDLLow: 925, NDLMin: 3450, NDLMax: 3799, FULLow: 880, NULMin: 21450, NULMax: 21799},
	{Band: 12, Name: "700 a", FDLLow: 729, NDLMin: 5010, NDLMax: 5179, FULLow: 699, NULMin: 23010, NULMax: 23179},
	{Band: 13, Name: "700 c", FDLLow: 746, NDLMin: 5180, NDLMax: 5279, FULLow: 777, NULMin: 23180, NULMax: 23279},
	{Band: 14, Name: "700 PS", FDLLow: 758, NDLMin: 5280, NDLMax: 5379, FULLow: 788, NULMin: 23280, NULMax: 23379},
	{Band: 17, Name: "700 b/c", FDLLow: 734, NDLMin: 5730, NDLMax: 5849, FULLow: 704, NULMin: 23730, NULMax: 23849},
	{Band: 20, Name: "800 DD", FDLLow: 791, NDLMin: 6150, NDLMax: 6449, FULLow: 832, NULMin: 24150, NULMax: 24449},
	{Band: 25, Name: "1900+", FDLLow: 1930, NDLMin: 8040, NDLMax: 8689, FULLow: 1850, NULMin: 26040, NULMax: 26689},
	{Band: 26, Name: "850+", FDLLow: 859, NDLMin: 8690, NDLMax: 9039, FULLow: 814, NULMin: 26690, NULMax: 27039},
	{Band: 28, Name: "700 APT", FDLLow: 758, NDLMin: 9210, NDLMax: 9659, FULLow: 703, NULMin: 27210, NULMax: 27659},
	{Band: 29, Name: "700 SDL", FDLLow: 717, NDLMin: 9660, NDLMax: 9769},
	{Band: 30, Name: "2300 WCS", FDLLow: 2350, NDLMin: 9770, NDLMax: 9869, FULLow: 2305, NULMin: 27660, NULMax: 27759},
	{Band: 66, Name: "AWS-3", FDLLow: 2110, NDLMin: 66436, NDLMax: 67335, FULLow: 1710, NULMin: 131972, NULMax: 132671},
	{Band: 71, Name: "600", FDLLow: 617, NDLMin: 68586, NDLMax: 68935, FULLow: 663, NULMin: 133122, NULMax: 133471},

	// TDD bands
	{Band: 38, Name: "2600 TDD", FDLLow: 2570, NDLMin: 37750, NDLMax: 38249},
	{Band: 39, Name: "1900 TDD", FDLLow: 1880, NDLMin: 38250, NDLMax: 38649},
	{Band: 40, Name: "2300 TDD", FDLLow: 2300, NDLMin: 38650, NDLMax: 39649},
	{Band: 41, Name: "2500 TDD", FDLLow: 2496, NDLMin: 39650, NDLMax: 41589},
	{Band: 42, Name: "3500 TDD", FDLLow: 3400, NDLMin: 41590, NDLMax: 43589},
	{Band: 43, Name: "3700 TDD", FDLLow: 3600, NDLMin: 43590, NDLMax: 45589},
	{Band: 48, Name: "3600 CBRS", FDLLow: 3550, NDLMin: 55240, NDLMax: 56739},
}

// EARFCNInfo holds the decoded frequency and band for an EARFCN value.
type EARFCNInfo struct {
	FreqMHz float64
	Band    int
	Name    string
}

// FormatEARFCN converts a DL EARFCN to a human-readable string like
// "2585 (887.5 MHz, B5)". Returns the raw value if the EARFCN is unknown
// or 0xFFFFFFFF (no channel).
func FormatEARFCN(earfcn int) string {
	if earfcn == 0xFFFFFFFF || earfcn < 0 {
		return "—"
	}

	info, err := lookupDLEARFCN(earfcn)
	if err != nil {
		return fmt.Sprintf("%d", earfcn)
	}

	freq := info.FreqMHz
	if freq == float64(int(freq)) {
		return fmt.Sprintf("%d (%g MHz, B%d)", earfcn, freq, info.Band)
	}
	return fmt.Sprintf("%d (%.1f MHz, B%d)", earfcn, freq, info.Band)
}

// FormatULEARFCN converts a UL EARFCN to a human-readable string.
func FormatULEARFCN(earfcn int) string {
	if earfcn == 0xFFFFFFFF || earfcn < 0 {
		return "—"
	}

	info, err := lookupULEARFCN(earfcn)
	if err != nil {
		return fmt.Sprintf("%d", earfcn)
	}

	freq := info.FreqMHz
	if freq == float64(int(freq)) {
		return fmt.Sprintf("%d (%g MHz, B%d)", earfcn, freq, info.Band)
	}
	return fmt.Sprintf("%d (%.1f MHz, B%d)", earfcn, freq, info.Band)
}

func lookupDLEARFCN(earfcn int) (EARFCNInfo, error) {
	for _, b := range earfcnBands {
		if earfcn >= b.NDLMin && earfcn <= b.NDLMax {
			freq := b.FDLLow + 0.1*float64(earfcn-b.NDLMin)
			return EARFCNInfo{FreqMHz: freq, Band: b.Band, Name: b.Name}, nil
		}
	}
	return EARFCNInfo{}, fmt.Errorf("unknown DL EARFCN %d", earfcn)
}

func lookupULEARFCN(earfcn int) (EARFCNInfo, error) {
	for _, b := range earfcnBands {
		if b.NULMin == 0 && b.NULMax == 0 {
			continue
		}
		if earfcn >= b.NULMin && earfcn <= b.NULMax {
			freq := b.FULLow + 0.1*float64(earfcn-b.NULMin)
			return EARFCNInfo{FreqMHz: freq, Band: b.Band, Name: b.Name}, nil
		}
	}
	// TDD bands: UL EARFCN same as DL EARFCN
	for _, b := range earfcnBands {
		if b.NULMin == 0 && b.NULMax == 0 && earfcn >= b.NDLMin && earfcn <= b.NDLMax {
			freq := b.FDLLow + 0.1*float64(earfcn-b.NDLMin)
			return EARFCNInfo{FreqMHz: freq, Band: b.Band, Name: b.Name}, nil
		}
	}
	return EARFCNInfo{}, fmt.Errorf("unknown UL EARFCN %d", earfcn)
}
