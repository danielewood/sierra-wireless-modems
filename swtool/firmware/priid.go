// Package firmware provides firmware downloading, extraction, and PRI ID parsing.
package firmware

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// PRIID holds the PRI identification extracted from an .nvu firmware file.
type PRIID struct {
	PartNumber string // e.g. "9904609"
	Revision   string // e.g. "002.026"
	Carrier    string // e.g. "GENERIC" — exact case from the NVU binary
	FullString string // e.g. "9999999_9904609_SWI9X30C_02.24.05.06_00_GENERIC_002.026_000"
}

// cweHeaderSize is the size of a Sierra Wireless CWE file header.
const cweHeaderSize = 0x190

// cweVersionOffset is the offset of the version string within a CWE header.
// The version field is 84 bytes of null-terminated ASCII at offset 0x11C.
const cweVersionOffset = 0x11C

// cweVersionLen is the length of the version field in a CWE header.
const cweVersionLen = 84

// cweImgSizeOffset is the offset of the image size field (big-endian uint32).
const cweImgSizeOffset = 0x114

// ExtractPRIID reads an .nvu binary file and extracts the PRI identification
// from the CWE header version fields.
//
// NVU files are nested CWE containers. The version string is stored at a fixed
// offset (0x11C) in each 0x190-byte header. We read the outermost header's
// version field, which contains the full PRI descriptor:
//
//	9999999_9904609_SWI9X30C_02.39.00.00_00_GENERIC_002.085_000
//	  0       1        2          3        4    5       6     7
func ExtractPRIID(nvuPath string) (*PRIID, error) {
	data, err := os.ReadFile(nvuPath)
	if err != nil {
		return nil, fmt.Errorf("reading nvu file: %w", err)
	}

	if len(data) < cweHeaderSize {
		return nil, fmt.Errorf("file too small for CWE header: %d bytes", len(data))
	}

	// Validate that this looks like a CWE container by checking imgsize
	imgSize := binary.BigEndian.Uint32(data[cweImgSizeOffset : cweImgSizeOffset+4])
	if imgSize == 0 || int(imgSize) > len(data) {
		return nil, fmt.Errorf("invalid CWE image size: %d", imgSize)
	}

	// Read the version string from the first CWE header
	version := readCString(data[cweVersionOffset : cweVersionOffset+cweVersionLen])
	if version == "" {
		return nil, fmt.Errorf("empty version string in CWE header")
	}

	return parsePRIVersion(version, nvuPath)
}

// parsePRIVersion parses a PRI version string into a PRIID.
// Expected format: 9999999_<PN>_SWI9X30C_<ver>_<xx>_<CARRIER>_<rev>_<seq>
func parsePRIVersion(version, source string) (*PRIID, error) {
	if !strings.HasPrefix(version, "9999999_") {
		return nil, fmt.Errorf("version string does not start with 9999999_: %q in %s", version, source)
	}

	parts := strings.Split(version, "_")
	if len(parts) < 7 {
		return nil, fmt.Errorf("version string has too few fields (%d): %q in %s", len(parts), version, source)
	}

	pri := &PRIID{
		FullString: version,
		PartNumber: parts[1],
		Carrier:    parts[5],
		Revision:   parts[6],
	}

	if pri.Carrier == "" {
		return nil, fmt.Errorf("empty carrier in version string: %q in %s", version, source)
	}

	return pri, nil
}

// readCString reads a null-terminated string from a byte slice.
func readCString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
