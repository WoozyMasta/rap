package rap

import "testing"

func TestFormatFloat32RawNormalized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float32
		want  string
	}{
		{
			name:  "compact decimal",
			value: float32(0.0099999998),
			want:  "0.01",
		},
		{
			name:  "integer keeps float marker",
			value: float32(1.0),
			want:  "1.0",
		},
		{
			name:  "simple fractional",
			value: float32(0.55),
			want:  "0.55",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := formatFloat32RawNormalized(test.value)
			if got != test.want {
				t.Fatalf("formatFloat32RawNormalized(%v) = %q; want %q", test.value, got, test.want)
			}
		})
	}
}

func TestFormatFloat32RawVerbose(t *testing.T) {
	t.Parallel()

	got := formatFloat32RawVerbose(float32(1.0))
	if got != "1.0" {
		t.Fatalf("formatFloat32RawVerbose(1.0) = %q; want %q", got, "1.0")
	}
}

func TestQuoteRVCfgString_BIStyleEscaping(t *testing.T) {
	t.Parallel()

	input := `this animationPhase "shutter1" < 0.5 && damage this < 1`
	want := `"this animationPhase ""shutter1"" < 0.5 && damage this < 1"`
	got := quoteRVCfgString(input)
	if got != want {
		t.Fatalf("quoteRVCfgString(%q) = %q; want %q", input, got, want)
	}
}

func TestUnquoteRVCfgString_BIStyleEscaping(t *testing.T) {
	t.Parallel()

	raw := `"this animationPhase ""shutter1"" < 0.5 && damage this < 1"`
	want := `this animationPhase "shutter1" < 0.5 && damage this < 1`
	got, ok := unquoteRVCfgString(raw)
	if !ok {
		t.Fatalf("unquoteRVCfgString(%q) unexpectedly failed", raw)
	}

	if got != want {
		t.Fatalf("unquoteRVCfgString(%q) = %q; want %q", raw, got, want)
	}
}

func TestUnquoteRVCfgString_LegacyBackslashEscaping(t *testing.T) {
	t.Parallel()

	raw := `"this animationPhase \"shutter1\" < 0.5 && damage this < 1"`
	want := `this animationPhase "shutter1" < 0.5 && damage this < 1`
	got, ok := unquoteRVCfgString(raw)
	if !ok {
		t.Fatalf("unquoteRVCfgString(%q) unexpectedly failed", raw)
	}

	if got != want {
		t.Fatalf("unquoteRVCfgString(%q) = %q; want %q", raw, got, want)
	}
}

func TestQuoteUnquoteRVCfgString_ConsumablesPattern(t *testing.T) {
	t.Parallel()

	raw := `"DZ\gear\consumables\data\"""".rvmat"`
	unquoted, ok := unquoteRVCfgString(raw)
	if !ok {
		t.Fatalf("unquoteRVCfgString(%q) unexpectedly failed", raw)
	}

	quoted := quoteRVCfgString(unquoted)
	if quoted != raw {
		t.Fatalf("quoteRVCfgString(unquoteRVCfgString(%q)) = %q; want %q", raw, quoted, raw)
	}
}
