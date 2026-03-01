// Package keygen implements the Sierra Wireless AT!OPENLOCK challenge-response
// key derivation algorithm. It is a pure-Go port of sierrakeygen.py by B.Kerler.
//
// The algorithm is an RC4-variant stream cipher with a 256-byte permutation
// table (S-box) and a 5-register state array. Each modem generation uses
// different keys and register initialization indices.
package keygen

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// KeyType selects which challenge-response key to compute.
type KeyType int

const (
	KeyOpenLock KeyType = iota // AT!OPENLOCK
	KeyOpenMEP                 // AT!OPENMEP
	KeyOpenCND                 // AT!OPENCND
)

// AlgoParams holds the SierraAlgo register mapping for a modem generation.
// MDM8200 uses one set of parameters; all other generations share another.
type AlgoParams struct {
	A, B, C, D, E int
	Ret, Ret2     int
	Flag          int
}

// Generation describes a modem chipset generation and its keygen parameters.
type Generation struct {
	Name     string
	OpenLock int        // keytable index for AT!OPENLOCK
	OpenMEP  int        // keytable index for AT!OPENMEP
	OpenCND  int        // keytable index for AT!OPENCND
	CLen     int        // challenge length in bytes (always 8)
	Init     [5]int     // register initialization indices
	Algo     AlgoParams // cipher step parameters
}

// algoStandard is used by all generations except MDM8200.
var algoStandard = AlgoParams{A: 4, B: 2, C: 1, D: 0, E: 3, Ret: 2, Ret2: 0, Flag: 0}

// algoMDM8200 is used only by MDM8200.
var algoMDM8200 = AlgoParams{A: 2, B: 4, C: 1, D: 3, E: 0, Ret: 3, Ret2: 4, Flag: 0}

// generations maps generation names to their parameters.
// Matches prodtable from sierrakeygen.py.
var generations = map[string]*Generation{
	"MDM8200":    {Name: "MDM8200", OpenLock: 0, OpenMEP: 1, OpenCND: 0, CLen: 8, Init: [5]int{1, 3, 5, 7, 0}, Algo: algoMDM8200},
	"MDM9200":    {Name: "MDM9200", OpenLock: 0, OpenMEP: 1, OpenCND: 0, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9200_V1": {Name: "MDM9200_V1", OpenLock: 2, OpenMEP: 1, OpenCND: 0, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9200_V2": {Name: "MDM9200_V2", OpenLock: 3, OpenMEP: 1, OpenCND: 0, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9200_V3": {Name: "MDM9200_V3", OpenLock: 8, OpenMEP: 1, OpenCND: 8, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9x15":    {Name: "MDM9x15", OpenLock: 0, OpenMEP: 1, OpenCND: 0, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9x07":    {Name: "MDM9x07", OpenLock: 9, OpenMEP: 10, OpenCND: 9, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9x06":    {Name: "MDM9x06", OpenLock: 20, OpenMEP: 19, OpenCND: 20, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9x30":    {Name: "MDM9x30", OpenLock: 5, OpenMEP: 4, OpenCND: 5, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9x30_V1": {Name: "MDM9x30_V1", OpenLock: 17, OpenMEP: 15, OpenCND: 17, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9x40":    {Name: "MDM9x40", OpenLock: 11, OpenMEP: 12, OpenCND: 11, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
	"MDM9x50":    {Name: "MDM9x50", OpenLock: 7, OpenMEP: 6, OpenCND: 7, CLen: 8, Init: [5]int{7, 3, 0, 1, 5}, Algo: algoStandard},
}

// keyTable contains 21 cipher keys, each 16 bytes.
// Matches keytable from sierrakeygen.py.
var keyTable = [21][16]byte{
	{0xF0, 0x14, 0x55, 0x0D, 0x5E, 0xDA, 0x92, 0xB3, 0xA7, 0x6C, 0xCE, 0x84, 0x90, 0xBC, 0x7F, 0xED}, // 0
	{0x61, 0x94, 0xCE, 0xA7, 0xB0, 0xEA, 0x4F, 0x0A, 0x73, 0xC5, 0xC3, 0xA6, 0x5E, 0xEC, 0x1C, 0xE2}, // 1
	{0x39, 0xC6, 0x7B, 0x04, 0xCA, 0x50, 0x82, 0x1F, 0x19, 0x63, 0x36, 0xDE, 0x81, 0x49, 0xF0, 0xD7}, // 2
	{0xDE, 0xA5, 0xAD, 0x2E, 0xBE, 0xE1, 0xC9, 0xEF, 0xCA, 0xF9, 0xFE, 0x1F, 0x17, 0xFE, 0xED, 0x3B}, // 3
	{0xFE, 0xD4, 0x40, 0x52, 0x2D, 0x4B, 0x12, 0x5C, 0xE7, 0x0D, 0xF8, 0x79, 0xF8, 0xC0, 0xDD, 0x37}, // 4
	{0x3B, 0x18, 0x99, 0x6B, 0x57, 0x24, 0x0A, 0xD8, 0x94, 0x6F, 0x8E, 0xD9, 0x90, 0xBC, 0x67, 0x56}, // 5
	{0x47, 0x4F, 0x4F, 0x44, 0x4A, 0x4F, 0x42, 0x44, 0x45, 0x43, 0x4F, 0x44, 0x49, 0x4E, 0x47, 0x2E}, // 6
	{0x4F, 0x4D, 0x41, 0x52, 0x20, 0x44, 0x49, 0x44, 0x20, 0x54, 0x48, 0x49, 0x53, 0x2E, 0x2E, 0x2E}, // 7
	{0x8F, 0xA5, 0x85, 0x05, 0x5E, 0xCF, 0x44, 0xA0, 0x98, 0x8B, 0x09, 0xE8, 0xBB, 0xC6, 0xF7, 0x65}, // 8
	{0x4D, 0x42, 0xD8, 0xC1, 0x25, 0x44, 0xD8, 0xA0, 0x1D, 0x80, 0xC4, 0x52, 0x8E, 0xEC, 0x8B, 0xE3}, // 9
	{0xED, 0xA9, 0xB7, 0x0A, 0xDB, 0x85, 0x3D, 0xC0, 0x92, 0x49, 0x7D, 0x41, 0x9A, 0x91, 0x09, 0xEE}, // 10
	{0x8A, 0x56, 0x03, 0xF0, 0xBB, 0x9C, 0x13, 0xD2, 0x4E, 0xB2, 0x45, 0xAD, 0xC4, 0x0A, 0xE7, 0x52}, // 11
	{0x2A, 0xEF, 0x07, 0x2B, 0x19, 0x60, 0xC9, 0x01, 0x8B, 0x87, 0xF2, 0x6E, 0xC1, 0x42, 0xA8, 0x3A}, // 12
	{0x28, 0x55, 0x48, 0x52, 0x24, 0x72, 0x63, 0x37, 0x14, 0x26, 0x37, 0x50, 0xBE, 0xFE, 0x00, 0x00}, // 13
	{0x22, 0x63, 0x48, 0x02, 0x24, 0x72, 0x27, 0x37, 0x19, 0x26, 0x37, 0x50, 0xBE, 0xEF, 0xCA, 0xFE}, // 14
	{0x98, 0xE1, 0xC1, 0x93, 0xC3, 0xBF, 0xC3, 0x50, 0x8D, 0xA1, 0x35, 0xFE, 0x50, 0x47, 0xB3, 0xC4}, // 15
	{0x61, 0x94, 0xCE, 0xA7, 0xB0, 0xEA, 0x4F, 0x0A, 0x73, 0xC5, 0xC3, 0xA6, 0x5E, 0xEC, 0x1C, 0xE2}, // 16
	{0xC5, 0x50, 0x40, 0xDA, 0x23, 0xE8, 0xF4, 0x4C, 0x29, 0xE9, 0x07, 0xDE, 0x24, 0xE5, 0x2C, 0x1D}, // 17
	{0xF0, 0x14, 0x55, 0x0D, 0x5E, 0xDA, 0x92, 0xB3, 0xA7, 0x6C, 0xCE, 0x84, 0x90, 0xBC, 0x7F, 0xED}, // 18
	{0x78, 0x19, 0xC5, 0x6D, 0xC3, 0xD8, 0x25, 0x3E, 0x51, 0x60, 0x8C, 0xA7, 0x32, 0x83, 0x37, 0x9D}, // 19
	{0x12, 0xF0, 0x79, 0x6B, 0x19, 0xC7, 0xF4, 0xEC, 0x50, 0xF3, 0x8C, 0x40, 0x02, 0xC9, 0x43, 0xC8}, // 20
}

// generator holds the mutable cipher state for a single keygen operation.
type generator struct {
	tbl  [256]byte // permutation table (RC4-like S-box)
	rtbl [20]byte  // register table (5 active registers at indices 0..4)
	gen  *Generation
}

// sierraPreInit performs key schedule mixing. Returns (counter, challengelen, mcount).
// Port of SierraPreInit from sierrakeygen.py lines 226-246.
func (g *generator) sierraPreInit(counter int, key []byte, keylen, challengelen, mcount int) (int, int, int) {
	if counter == 0 {
		return counter, challengelen, mcount
	}

	tmp2 := 0
	i := 1
	for i < counter {
		i = 2*i + 1
	}

	for {
		tmp := mcount
		mcount = tmp + 1
		challengelen = (int(key[tmp&0xFF]) + int(g.tbl[challengelen&0xFF])) & 0xFF
		if mcount >= keylen {
			mcount = 0
			challengelen = (challengelen + keylen) & 0xFF
		}
		tmp2++
		tmp3 := (challengelen & i) & 0xFF
		if tmp2 >= 0xB {
			if tmp3 == 0 {
				// Guard against division by zero (never happens with valid keys).
				tmp3 = 1
			}
			tmp3 = counter % tmp3
		}
		if tmp3 <= counter {
			counter = tmp3 & 0xFF
			break
		}
	}

	return counter, challengelen, mcount
}

// sierraInit initializes the S-box from a key and sets up the register table.
// Port of SierraInit from sierrakeygen.py lines 248-274.
func (g *generator) sierraInit(key []byte, keylen int) bool {
	if keylen == 0 || keylen > 0x20 {
		return false
	}

	// Initialize identity permutation.
	for i := range 256 {
		g.tbl[i] = byte(i)
	}

	mcount := 0
	cl := keylen & 0xFFFFFF00 // mask off low byte
	// Iterate from 255 down to 0 inclusive.
	for i := 0xFF; i >= 0; i-- {
		t, newCL, newMcount := g.sierraPreInit(i, key, keylen, cl, mcount)
		cl = newCL
		mcount = newMcount
		m := g.tbl[i]
		g.tbl[i] = g.tbl[t&0xFF]
		g.tbl[t&0xFF] = m
	}

	// Initialize 5 registers from tbl using generation-specific indices.
	// When init index is 0, use cl (accumulated state) as index instead.
	for r := range 5 {
		idx := g.gen.Init[r]
		if idx != 0 {
			g.rtbl[r] = g.tbl[idx]
		} else {
			g.rtbl[r] = g.tbl[cl&0xFF]
		}
	}

	return true
}

// sierraAlgo performs a single cipher step, consuming one challenge byte and
// producing one response byte. Port of SierraAlgo from sierrakeygen.py lines 296-316.
//
// IMPORTANT: Python and Go have different operator precedence for & vs +.
// In Python, + binds tighter than & (so "a + b & c" = "(a+b) & c").
// In Go, & binds tighter than + (so "a + b & c" = "a + (b&c)").
// All expressions below use explicit parentheses to match Python semantics.
func (g *generator) sierraAlgo(challenge byte, p AlgoParams) byte {
	v6 := int(g.rtbl[p.E])
	v0 := (v6 + 1) & 0xFF
	g.rtbl[p.E] = byte(v0)

	// Python: self.tbl[v6 + flag & 0xFF] = self.tbl[(v6 + flag) & 0xFF]
	// (In Python, + binds tighter than &)
	g.rtbl[p.C] = byte((int(g.tbl[(v6+p.Flag)&0xFF]) + int(g.rtbl[p.C])) & 0xFF)

	v4 := int(g.rtbl[p.C])
	v2 := int(g.rtbl[p.B])
	v1 := int(g.tbl[v2])
	g.tbl[v2] = g.tbl[v4]

	v5 := int(g.rtbl[p.D])
	g.tbl[v4] = g.tbl[v5]
	g.tbl[v5] = g.tbl[v0]
	g.tbl[v0] = byte(v1)

	// Compute u: three nested tbl lookups.
	// Python: self.tbl[((self.rtbl[a] + self.tbl[(v1 & 0xFF)]) & 0xFF)]
	t1 := int(g.tbl[(int(g.rtbl[p.A])+int(g.tbl[v1&0xFF]))&0xFF])
	// Python: t1 + self.tbl[(v5 & 0xFF)] + self.tbl[(v2 & 0xFF)] & 0xff
	// = (t1 + tbl[v5] + tbl[v2]) & 0xFF  (Python: + binds tighter than &)
	t2 := int(g.tbl[(t1+int(g.tbl[v5])+int(g.tbl[v2]))&0xFF])
	u := int(g.tbl[t2&0xFF])

	// v = tbl[((tbl[v4] + v1) & 0xFF)]
	v := int(g.tbl[(int(g.tbl[v4])+v1)&0xFF])

	ch := int(challenge)
	g.rtbl[p.Ret] = byte(u ^ v ^ ch)
	g.rtbl[p.A] = byte((int(g.tbl[v1&0xFF]) + int(g.rtbl[p.A])) & 0xFF)
	g.rtbl[p.Ret2] = byte(ch & 0xFF)

	return g.rtbl[p.Ret]
}

// sierraFinish zeros the cipher state.
func (g *generator) sierraFinish() {
	g.tbl = [256]byte{}
	g.rtbl = [20]byte{}
}

// sierraKeygen runs the full keygen: init S-box, run cipher over challenge bytes,
// return result. Port of SierraKeygen from sierrakeygen.py lines 328-337.
func (g *generator) sierraKeygen(challenge, key []byte, challengeLen, keyLen int) []byte {
	result := make([]byte, challengeLen)

	ok := g.sierraInit(key, keyLen)
	if !ok {
		return result
	}

	for i := range challengeLen {
		result[i] = g.sierraAlgo(challenge[i], g.gen.Algo)
	}

	g.sierraFinish()
	return result
}

// Solve computes the challenge-response for a given hex challenge string.
// Returns the uppercase hex response string.
func Solve(challenge string, gen *Generation, keyType KeyType) (string, error) {
	challengeBytes, err := hex.DecodeString(challenge)
	if err != nil {
		return "", fmt.Errorf("decoding challenge hex: %w", err)
	}

	// Pad challenge to CLen if shorter.
	for len(challengeBytes) < gen.CLen {
		challengeBytes = append(challengeBytes, 0)
	}
	challengeLen := len(challengeBytes)

	// Select key index based on type.
	var keyIdx int
	switch keyType {
	case KeyOpenLock:
		keyIdx = gen.OpenLock
	case KeyOpenMEP:
		keyIdx = gen.OpenMEP
	case KeyOpenCND:
		keyIdx = gen.OpenCND
	default:
		return "", fmt.Errorf("unknown key type %d", keyType)
	}

	if keyIdx < 0 || keyIdx >= len(keyTable) {
		return "", fmt.Errorf("key index %d out of range", keyIdx)
	}
	key := keyTable[keyIdx][:]

	g := &generator{gen: gen}
	resp := g.sierraKeygen(challengeBytes, key, challengeLen, 16)

	// Truncate response to challenge length and hex-encode.
	return strings.ToUpper(hex.EncodeToString(resp[:challengeLen])), nil
}

// GenerationByName returns the Generation for a known name like "MDM9x30".
func GenerationByName(name string) (*Generation, error) {
	gen, ok := generations[name]
	if !ok {
		return nil, fmt.Errorf("unknown generation %q", name)
	}
	return gen, nil
}

// DetectGeneration determines the modem generation from ATI revision and model strings.
// Matches the detection logic from sierrakeygen.py lines 492-509.
func DetectGeneration(revision, model string) (*Generation, error) {
	switch {
	case strings.Contains(revision, "9X07"):
		return generations["MDM9x07"], nil
	case strings.Contains(revision, "NTG9X35C"):
		return generations["MDM9x30_V1"], nil
	case strings.Contains(revision, "9X15"):
		return generations["MDM9x15"], nil
	case strings.Contains(revision, "9X30"):
		return generations["MDM9x30"], nil
	case strings.Contains(revision, "9X40"):
		return generations["MDM9x40"], nil
	case strings.Contains(revision, "9X50"):
		// MR1100 is an MDM9x50 device but uses MDM9x40 keys.
		if strings.Contains(model, "MR1100") {
			return generations["MDM9x40"], nil
		}
		return generations["MDM9x50"], nil
	case strings.Contains(revision, "9X06"):
		return generations["MDM9x06"], nil
	}
	return nil, fmt.Errorf("unknown modem generation for revision %q", revision)
}

// Generations returns a sorted list of all supported generation names.
func Generations() []string {
	names := make([]string, 0, len(generations))
	for name := range generations {
		names = append(names, name)
	}
	return names
}
