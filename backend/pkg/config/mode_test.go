package config

import "testing"

func TestParseMode(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"enforce", ModeEnforce, false},
		{"shadow", ModeShadow, false},
		{"", ModeEnforce, true},
		{"Shadow", ModeEnforce, true},
		{"observe", ModeEnforce, true},
	} {
		got, err := ParseMode(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseMode(%q) = %v, want an error — an unrecognised mode must never quietly become a default", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMode(%q) returned error %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// A client reading a mode must never see a blank or ambiguous label —
// the whole risk of shadow mode is someone not realising it's on.
func TestModeString(t *testing.T) {
	if got := ModeEnforce.String(); got != "enforce" {
		t.Errorf("ModeEnforce.String() = %q, want enforce", got)
	}
	if got := ModeShadow.String(); got != "shadow" {
		t.Errorf("ModeShadow.String() = %q, want shadow", got)
	}
}
