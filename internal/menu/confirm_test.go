package menu

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes_lowercase", "y\n", true},
		{"yes_full", "yes\n", true},
		{"yes_uppercase", "YES\n", true},
		{"yes_mixed", "Yes\n", true},
		{"no", "n\n", false},
		{"no_full", "no\n", false},
		{"empty", "\n", false},
		{"garbage", "maybe\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := Confirm(&buf, strings.NewReader(tt.input), "Continue?")
			if got != tt.want {
				t.Errorf("Confirm(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestConfirmBulkDelete_single(t *testing.T) {
	var buf bytes.Buffer
	// count <= 1 auto-confirms without reading input
	got := ConfirmBulkDelete(&buf, strings.NewReader(""), 1, "item")
	if !got {
		t.Error("ConfirmBulkDelete(count=1) should auto-confirm")
	}
}

func TestConfirmBulkDelete_multi(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"confirm", "y\n", true},
		{"reject", "n\n", false},
		{"empty", "\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := ConfirmBulkDelete(&buf, strings.NewReader(tt.input), 3, "item")
			if got != tt.want {
				t.Errorf("ConfirmBulkDelete(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
