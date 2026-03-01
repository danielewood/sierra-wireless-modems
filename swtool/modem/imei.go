package modem

import (
	"fmt"
	"strings"
)

// ValidateIMEI checks that an IMEI string is exactly 15 digits and passes
// the Luhn check digit algorithm (used by GSM/LTE for equipment identity).
func ValidateIMEI(imei string) error {
	if len(imei) != 15 {
		return fmt.Errorf("IMEI must be 15 digits, got %d", len(imei))
	}
	for _, r := range imei {
		if r < '0' || r > '9' {
			return fmt.Errorf("IMEI contains non-digit character %q", r)
		}
	}
	if !luhnValid(imei) {
		return fmt.Errorf("IMEI fails Luhn check digit validation")
	}
	return nil
}

// LuhnCheckDigit computes the Luhn check digit for the first 14 digits of an IMEI.
func LuhnCheckDigit(first14 string) (byte, error) {
	if len(first14) != 14 {
		return 0, fmt.Errorf("expected 14 digits, got %d", len(first14))
	}
	for _, r := range first14 {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-digit character %q", r)
		}
	}
	sum := 0
	for i, r := range first14 {
		d := int(r - '0')
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	check := (10 - sum%10) % 10
	return byte('0') + byte(check), nil
}

// EncodeIMEIBCD encodes a 15-digit IMEI as 8 BCD bytes for AT!NVENCRYPTIMEI.
// Each byte contains two consecutive digits. The 15th digit is padded with 0
// to produce the 8th byte. Returns comma-separated decimal values.
//
// Example: "354480082434669" → "35,44,80,08,24,34,66,90"
func EncodeIMEIBCD(imei string) (string, error) {
	if err := ValidateIMEI(imei); err != nil {
		return "", err
	}
	// Pad to 16 digits (last byte gets trailing 0).
	padded := imei + "0"
	parts := make([]string, 8)
	for i := range 8 {
		hi := padded[2*i] - '0'
		lo := padded[2*i+1] - '0'
		parts[i] = fmt.Sprintf("%02X", hi<<4|lo)
	}
	return strings.Join(parts, ","), nil
}

// luhnValid verifies the Luhn check digit of a 15-digit IMEI.
func luhnValid(digits string) bool {
	sum := 0
	for i, r := range digits {
		d := int(r - '0')
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}
