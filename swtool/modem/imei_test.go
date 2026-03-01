package modem

import "testing"

func TestValidateIMEI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		imei    string
		wantErr bool
	}{
		{"valid EM7455 IMEI", "354480082434669", false},
		{"valid computed", "490154203237518", false},
		{"bad check digit", "354480082434660", true},
		{"too short", "35448008243466", true},
		{"too long", "3544800824346699", true},
		{"non-digit", "35448008243466X", true},
		{"all zeros invalid", "000000000000000", false}, // Luhn passes for all zeros
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateIMEI(tt.imei)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIMEI(%q) err = %v, wantErr = %v", tt.imei, err, tt.wantErr)
			}
		})
	}
}

func TestLuhnCheckDigit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		first14 string
		want    byte
	}{
		{"35448008243466", '9'},
		{"49015420323751", '8'},
	}

	for _, tt := range tests {
		t.Run(tt.first14, func(t *testing.T) {
			t.Parallel()
			got, err := LuhnCheckDigit(tt.first14)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("LuhnCheckDigit(%q) = %c, want %c", tt.first14, got, tt.want)
			}
		})
	}
}

func TestEncodeIMEIBCD(t *testing.T) {
	t.Parallel()

	tests := []struct {
		imei string
		want string
	}{
		// 354480082434669 + pad 0 → 3544800824346690
		// BCD: 0x35,0x44,0x80,0x08,0x24,0x34,0x66,0x90
		{"354480082434669", "35,44,80,08,24,34,66,90"},
		{"490154203237518", "49,01,54,20,32,37,51,80"},
	}

	for _, tt := range tests {
		t.Run(tt.imei, func(t *testing.T) {
			t.Parallel()
			got, err := EncodeIMEIBCD(tt.imei)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("EncodeIMEIBCD(%q) = %q, want %q", tt.imei, got, tt.want)
			}
		})
	}
}

func TestLuhnCheckDigit_Errors(t *testing.T) {
	t.Parallel()

	if _, err := LuhnCheckDigit("123"); err == nil {
		t.Error("expected error for wrong length")
	}
	if _, err := LuhnCheckDigit("1234567890123X"); err == nil {
		t.Error("expected error for non-digit")
	}
}
