package cmd

import (
	"testing"
)

func TestKeyPattern(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		// Valid keys
		{"simple", "climate", true},
		{"with_underscore", "climate_action", true},
		{"multi_underscore", "mangrove_restoration", true},
		{"with_numbers", "sdg13", true},
		{"numbers_only", "123", true},
		{"number_underscore", "sdg_13", true},
		{"long_key", "this_is_a_very_long_key_name", true},

		// Invalid keys
		{"uppercase", "Climate", false},
		{"mixed_case", "climateAction", false},
		{"hyphen", "climate-action", false},
		{"space", "climate action", false},
		{"leading_underscore", "_climate", false},
		{"trailing_underscore", "climate_", false},
		{"double_underscore", "climate__action", false},
		{"special_chars", "climate@action", false},
		{"unicode", "climat\u00e9", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyPattern.MatchString(tt.key)
			if got != tt.valid {
				t.Errorf("keyPattern.MatchString(%q) = %v, want %v", tt.key, got, tt.valid)
			}
		})
	}
}
