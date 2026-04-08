package main

import "testing"

// Test_isValidPort verifies the scenario described by its name.
func Test_isValidPort(t *testing.T) {
	tests := []struct {
		input string
		want  bool
		label string
	}{
		{"8989", true, "default port"},
		{"2525", true, "custom port"},
		{"1", true, "minimum valid port"},
		{"65535", true, "maximum valid port"},
		{"80", true, "HTTP port"},

		{"", false, "empty string"},
		{"0", false, "port zero"},
		{"65536", false, "above max"},
		{"99999", false, "way above max"},
		{"abc", false, "non-numeric"},
		{"-1", false, "negative (has dash)"},
		{"12.5", false, "decimal"},
		{"8080a", false, "trailing alpha"},
		{"a8080", false, "leading alpha"},
		{" 80", false, "leading space"},
		{"80 ", false, "trailing space"},
		{"00001", true, "leading zeros still valid (parses to 1)"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := isValidPort(tt.input)
			if got != tt.want {
				t.Errorf("isValidPort(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
