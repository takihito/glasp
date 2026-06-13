package transform

import "testing"

func TestModeLabel(t *testing.T) {
	cases := []struct {
		mode Mode
		want string
	}{
		{ModeGasToTS, "gas-to-ts"},
		{ModeTSToGas, "ts-to-gas"},
		{Mode(""), "none"},
		{Mode("unknown"), "none"},
	}
	for _, tc := range cases {
		if got := tc.mode.Label(); got != tc.want {
			t.Errorf("Mode(%q).Label() = %q, want %q", string(tc.mode), got, tc.want)
		}
	}
}

func TestModeFromLabel(t *testing.T) {
	cases := []struct {
		label string
		want  Mode
	}{
		{"gas-to-ts", ModeGasToTS},
		{"ts-to-gas", ModeTSToGas},
		{" gas-to-ts ", ModeGasToTS},
		{"none", Mode("")},
		{"", Mode("")},
		{"unknown", Mode("")},
	}
	for _, tc := range cases {
		if got := ModeFromLabel(tc.label); got != tc.want {
			t.Errorf("ModeFromLabel(%q) = %q, want %q", tc.label, string(got), string(tc.want))
		}
	}
}
