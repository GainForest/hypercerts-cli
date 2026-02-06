package menu

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderSingleSelect(t *testing.T) {
	items := []string{"Alpha", "Beta", "Gamma"}
	var buf bytes.Buffer
	RenderSingleSelect(&buf, items, "item", 0, func(s string) string { return s }, func(s string) string { return "" })

	output := buf.String()
	if !strings.Contains(output, "Select a item:") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "Alpha") {
		t.Error("missing first item")
	}
	if !strings.Contains(output, "Beta") {
		t.Error("missing second item")
	}
	if !strings.Contains(output, "Gamma") {
		t.Error("missing third item")
	}
	// First item should have cursor arrow (cyan >)
	if !strings.Contains(output, "\033[36m>\033[0m") {
		t.Error("missing cursor arrow")
	}
}

func TestRenderSingleSelectWithCreate(t *testing.T) {
	items := []string{"Alice", "Bob"}
	var buf bytes.Buffer
	RenderSingleSelectWithCreate(&buf, items, "person", 2, 2,
		func(s string) string { return s },
		func(s string) string { return "" },
		"Create new person...",
	)

	output := buf.String()
	if !strings.Contains(output, "Create new person...") {
		t.Error("missing create option")
	}
	// Create option is selected (index 2 == createNewIndex)
	if !strings.Contains(output, "\033[1;32m+") {
		t.Error("create option should be highlighted when selected")
	}
}

func TestRenderMultiSelect(t *testing.T) {
	items := []string{"X", "Y", "Z"}
	selected := map[int]bool{1: true}
	var buf bytes.Buffer
	RenderMultiSelect(&buf, items, "option", 0, selected,
		func(s string) string { return s },
		func(s string) string { return "" },
	)

	output := buf.String()
	if !strings.Contains(output, "1 selected") {
		t.Error("missing selection count")
	}
	if !strings.Contains(output, "[x]") {
		t.Error("missing checked checkbox")
	}
	if !strings.Contains(output, "[ ]") {
		t.Error("missing unchecked checkbox")
	}
}

func TestRenderMultiSelect_noSelection(t *testing.T) {
	items := []string{"A", "B"}
	selected := map[int]bool{}
	var buf bytes.Buffer
	RenderMultiSelect(&buf, items, "item", 0, selected,
		func(s string) string { return s },
		func(s string) string { return "" },
	)

	output := buf.String()
	// Should not show "0 selected"
	if strings.Contains(output, "selected") {
		t.Error("should not show selection count when none selected")
	}
}
