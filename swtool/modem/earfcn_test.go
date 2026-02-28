package modem

import "testing"

func TestFormatEARFCN(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		earfcn int
		want   string
	}{
		{name: "band 5 mid", earfcn: 2585, want: "2585 (887.5 MHz, B5)"},
		{name: "band 5 low", earfcn: 2400, want: "2400 (869 MHz, B5)"},
		{name: "band 2 mid", earfcn: 900, want: "900 (1960 MHz, B2)"},
		{name: "band 4", earfcn: 2175, want: "2175 (2132.5 MHz, B4)"},
		{name: "band 12", earfcn: 5095, want: "5095 (737.5 MHz, B12)"},
		{name: "band 66", earfcn: 66886, want: "66886 (2155 MHz, B66)"},
		{name: "band 41 tdd", earfcn: 40620, want: "40620 (2593 MHz, B41)"},
		{name: "no channel", earfcn: 0xFFFFFFFF, want: "—"},
		{name: "unknown", earfcn: 99999, want: "99999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatEARFCN(tt.earfcn)
			if got != tt.want {
				t.Errorf("FormatEARFCN(%d) = %q, want %q", tt.earfcn, got, tt.want)
			}
		})
	}
}

func TestFormatULEARFCN(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		earfcn int
		want   string
	}{
		{name: "band 2 ul", earfcn: 18900, want: "18900 (1880 MHz, B2)"},
		{name: "band 5 ul", earfcn: 20525, want: "20525 (836.5 MHz, B5)"},
		{name: "tdd band 41 ul", earfcn: 40620, want: "40620 (2593 MHz, B41)"},
		{name: "no tx", earfcn: 0xFFFFFFFF, want: "—"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatULEARFCN(tt.earfcn)
			if got != tt.want {
				t.Errorf("FormatULEARFCN(%d) = %q, want %q", tt.earfcn, got, tt.want)
			}
		})
	}
}
