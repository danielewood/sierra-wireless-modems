package keygen

import "testing"

func TestSolve(t *testing.T) {
	t.Parallel()

	// All test vectors from sierrakeygen.py selftest() — type 0 (OpenLock).
	tests := []struct {
		generation string
		challenge  string
		want       string
	}{
		{"MDM9x15", "8101A18AB3C3E66A", "D1E128FCA8A963ED"},
		{"MDM9x40", "BE96CBBEE0829BCA", "1033773720F6EE66"},
		{"MDM9x30", "BE96CBBEE0829BCA", "1E02CE6A98B7DD2A"},
		{"MDM9x50", "BE96CBBEE0829BCA", "32AB617DB4B1C205"},
		{"MDM9x06", "BE96CBBEE0829BCA", "28D718CCD669DEDE"},
		{"MDM9x07", "BE96CBBEE0829BCA", "F5A4C9A0D402E34E"},
		{"MDM8200", "BE96CBBEE0829BCA", "EE702212D9C12FAB"},
		{"MDM9200_V1", "BE96CBBEE0829BCA", "A9A4E76E2653F753"},
		{"MDM9200_V2", "BE96CBBEE0829BCA", "8B0FAB4B6F81B080"},
		{"MDM9200_V3", "BE96CBBEE0829BCA", "4A69AD8A69F390E0"},
		{"MDM9x30_V1", "BE96CBBEE0829BCA", "6A5E4C9CBCBDA7DC"},
		{"MDM9200", "BE96CBBEE0829BCA", "EEDBF8BFF8DAE346"},
	}

	for _, tt := range tests {
		t.Run(tt.generation, func(t *testing.T) {
			t.Parallel()
			gen, err := GenerationByName(tt.generation)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Solve(tt.challenge, gen, KeyOpenLock)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Solve(%q, %s, OpenLock) = %q, want %q", tt.challenge, tt.generation, got, tt.want)
			}
		})
	}
}

func TestSolve_LowercaseInput(t *testing.T) {
	t.Parallel()
	gen, err := GenerationByName("MDM9x30")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Solve("be96cbbee0829bca", gen, KeyOpenLock)
	if err != nil {
		t.Fatal(err)
	}
	want := "1E02CE6A98B7DD2A"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSolve_BadHex(t *testing.T) {
	t.Parallel()
	gen, err := GenerationByName("MDM9x30")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Solve("ZZZZ", gen, KeyOpenLock)
	if err == nil {
		t.Error("expected error for bad hex input")
	}
}

func TestGenerationByName_Unknown(t *testing.T) {
	t.Parallel()
	_, err := GenerationByName("MDM9999")
	if err == nil {
		t.Error("expected error for unknown generation")
	}
}

func TestDetectGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		revision string
		model    string
		wantGen  string
	}{
		{"EM7455", "SWI9X30C_02.33.03.00 r8209 CARMD-EV-FRMWR2 2019/08/28 20:59:30", "", "MDM9x30"},
		{"NTG9X35C", "NTG9X35C_02.08.29.00", "", "MDM9x30_V1"},
		{"EM7565", "SWI9X50C_01.14.04.00", "", "MDM9x50"},
		{"MR1100", "SWI9X50C_01.14.04.00", "MR1100", "MDM9x40"},
		{"AC815s", "NTG9X40C_11.14.08.11", "", "MDM9x40"},
		{"EM7305", "SWI9X15C_05.05.58.00", "", "MDM9x15"},
		{"SWI9X07Y", "SWI9X07Y_02.25.02.01", "", "MDM9x07"},
		{"WP77xx", "SWI9X06Y_02.14.04.00", "", "MDM9x06"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gen, err := DetectGeneration(tt.revision, tt.model)
			if err != nil {
				t.Fatal(err)
			}
			if gen.Name != tt.wantGen {
				t.Errorf("DetectGeneration(%q, %q) = %q, want %q", tt.revision, tt.model, gen.Name, tt.wantGen)
			}
		})
	}
}

func TestDetectGeneration_Unknown(t *testing.T) {
	t.Parallel()
	_, err := DetectGeneration("UNKNOWN_FIRMWARE_1.0", "")
	if err == nil {
		t.Error("expected error for unknown revision")
	}
}
