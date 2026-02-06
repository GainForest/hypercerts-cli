package menu

import (
	"fmt"
	"io"
)

// RenderSingleSelect draws a single-select menu.
func RenderSingleSelect[T any](w io.Writer, items []T, itemType string, selected int, getName func(T) string, getInfo func(T) string) {
	fmt.Fprint(w, "\033[H\033[J")
	fmt.Fprintf(w, "Select a %s:\r\n\r\n", itemType)

	for i, item := range items {
		name := getName(item)
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		info := getInfo(item)
		infoStr := ""
		if info != "" {
			infoStr = fmt.Sprintf(" \033[90m(%s)\033[0m", info)
		}
		fmt.Fprint(w, "\033[K")
		if i == selected {
			fmt.Fprintf(w, "  \033[36m>\033[0m \033[1m%s\033[0m%s\r\n", name, infoStr)
		} else {
			fmt.Fprintf(w, "    \033[90m%s\033[0m%s\r\n", name, infoStr)
		}
	}

	fmt.Fprint(w, "\r\n\033[K")
	fmt.Fprint(w, "\033[90m\u2191/\u2193 navigate \u00b7 enter select \u00b7 q cancel\033[0m\r\n")
}

// RenderSingleSelectWithCreate draws a single-select menu with a "Create new..." option.
func RenderSingleSelectWithCreate[T any](w io.Writer, items []T, itemType string, selected, createNewIndex int, getName func(T) string, getInfo func(T) string, createLabel string) {
	fmt.Fprint(w, "\033[H\033[J")
	fmt.Fprintf(w, "Select a %s:\r\n\r\n", itemType)

	for i, item := range items {
		name := getName(item)
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		info := getInfo(item)
		infoStr := ""
		if info != "" {
			infoStr = fmt.Sprintf(" \033[90m(%s)\033[0m", info)
		}
		fmt.Fprint(w, "\033[K")
		if i == selected {
			fmt.Fprintf(w, "  \033[36m>\033[0m \033[1m%s\033[0m%s\r\n", name, infoStr)
		} else {
			fmt.Fprintf(w, "    \033[90m%s\033[0m%s\r\n", name, infoStr)
		}
	}

	// Separator and "Create new..." option
	fmt.Fprint(w, "\r\n\033[K")
	if selected == createNewIndex {
		fmt.Fprintf(w, "  \033[36m>\033[0m \033[1;32m+ %s\033[0m\r\n", createLabel)
	} else {
		fmt.Fprintf(w, "    \033[32m+ %s\033[0m\r\n", createLabel)
	}

	fmt.Fprint(w, "\r\n\033[K")
	fmt.Fprint(w, "\033[90m\u2191/\u2193 navigate \u00b7 enter select \u00b7 q cancel\033[0m\r\n")
}

// RenderMultiSelect draws a multi-select menu with checkboxes.
func RenderMultiSelect[T any](w io.Writer, items []T, itemType string, cursor int, selected map[int]bool, getName func(T) string, getInfo func(T) string) {
	fmt.Fprint(w, "\033[H\033[J")

	count := len(selected)
	if count > 0 {
		fmt.Fprintf(w, "Select %ss: \033[33m%d selected\033[0m\r\n\r\n", itemType, count)
	} else {
		fmt.Fprintf(w, "Select %ss:\r\n\r\n", itemType)
	}

	for i, item := range items {
		name := getName(item)
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		info := getInfo(item)
		infoStr := ""
		if info != "" {
			infoStr = fmt.Sprintf(" \033[90m%s\033[0m", info)
		}
		checkbox := "[ ]"
		if selected[i] {
			checkbox = "\033[32m[x]\033[0m"
		}
		fmt.Fprint(w, "\033[K")
		if i == cursor {
			fmt.Fprintf(w, "  \033[36m>\033[0m %s \033[1m%s\033[0m%s\r\n", checkbox, name, infoStr)
		} else {
			fmt.Fprintf(w, "    %s \033[90m%s\033[0m%s\r\n", checkbox, name, infoStr)
		}
	}

	fmt.Fprint(w, "\r\n\033[K")
	fmt.Fprint(w, "\033[90m\u2191/\u2193 navigate \u00b7 space toggle \u00b7 a all \u00b7 enter confirm \u00b7 q cancel\033[0m\r\n")
}
