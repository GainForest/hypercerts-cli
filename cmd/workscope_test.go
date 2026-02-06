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
		{"with_hyphen", "climate-action", true},
		{"multi_hyphen", "climate-action-2025", true},
		{"with_numbers", "sdg13", true},
		{"numbers_only", "123", true},
		{"number_hyphen", "sdg-13", true},
		{"long_key", "this-is-a-very-long-key-name", true},

		// Invalid keys
		{"uppercase", "Climate", false},
		{"mixed_case", "climateAction", false},
		{"underscore", "climate_action", false},
		{"space", "climate action", false},
		{"leading_hyphen", "-climate", false},
		{"trailing_hyphen", "climate-", false},
		{"double_hyphen", "climate--action", false},
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

func TestWorkScopeKinds(t *testing.T) {
	expected := []string{"topic", "language", "domain", "method", "tag"}
	if len(workScopeKinds) != len(expected) {
		t.Errorf("workScopeKinds length = %d, want %d", len(workScopeKinds), len(expected))
	}
	for i, kind := range expected {
		if workScopeKinds[i] != kind {
			t.Errorf("workScopeKinds[%d] = %q, want %q", i, workScopeKinds[i], kind)
		}
	}
}
