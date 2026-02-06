package menu

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/GainForest/hypercerts-cli/internal/style"
)

// ErrCancelled is returned when the user cancels an interactive menu.
var ErrCancelled = fmt.Errorf("cancelled")

// ErrNonInteractive is returned when a terminal is required but unavailable.
var ErrNonInteractive = fmt.Errorf("non-interactive mode (use CLI flags instead)")

// defaultHeight is the number of visible options before scrolling kicks in.
const defaultHeight = 10

// SingleSelect displays an interactive single-select menu using huh.
// The getName callback provides the display label; getInfo provides
// supplementary text shown in parentheses beside the label.
func SingleSelect[T comparable](w io.Writer, items []T, itemType string, getName func(T) string, getInfo func(T) string) (*T, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no %ss found", itemType)
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, ErrNonInteractive
	}

	opts := buildOptions(items, getName, getInfo)

	var selected int
	sel := huh.NewSelect[int]().
		Title("Select a " + itemType).
		Options(opts...).
		Height(selectHeight(len(opts))).
		Value(&selected)

	if len(items) > 5 {
		sel = sel.Filtering(true)
	}

	form := huh.NewForm(huh.NewGroup(sel)).
		WithTheme(style.Theme())

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return nil, ErrCancelled
		}
		return nil, err
	}

	fmt.Fprintf(w, "Selected: %s\n\n", getName(items[selected]))
	return &items[selected], nil
}

// SingleSelectWithCreate is like SingleSelect but adds a "Create new..." option
// at the bottom. Returns (item, isCreate, error). If isCreate is true, the item
// pointer is nil.
func SingleSelectWithCreate[T comparable](w io.Writer, items []T, itemType string, getName func(T) string, getInfo func(T) string, createLabel string) (*T, bool, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, false, ErrNonInteractive
	}

	createIndex := len(items)
	opts := buildOptions(items, getName, getInfo)
	opts = append(opts, huh.NewOption("+ "+createLabel, createIndex))

	var selected int
	sel := huh.NewSelect[int]().
		Title("Select a " + itemType).
		Options(opts...).
		Height(selectHeight(len(opts))).
		Value(&selected)

	if len(items) > 5 {
		sel = sel.Filtering(true)
	}

	form := huh.NewForm(huh.NewGroup(sel)).
		WithTheme(style.Theme())

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return nil, false, ErrCancelled
		}
		return nil, false, err
	}

	if selected == createIndex {
		return nil, true, nil
	}

	fmt.Fprintf(w, "Selected: %s\n", getName(items[selected]))
	return &items[selected], false, nil
}

// buildOptions converts a slice of items into huh.Option values keyed by index.
// The display label is formed from getName, with getInfo appended as dim
// supplementary text when non-empty.
func buildOptions[T any](items []T, getName func(T) string, getInfo func(T) string) []huh.Option[int] {
	opts := make([]huh.Option[int], len(items))
	for i, item := range items {
		label := getName(item)
		if info := getInfo(item); info != "" {
			label = fmt.Sprintf("%s  %s", label, info)
		}
		opts[i] = huh.NewOption(label, i)
	}
	return opts
}

// selectHeight returns a sensible height for the options viewport based on the
// number of options, capping at defaultHeight.
func selectHeight(n int) int {
	h := n + 1 // +1 for breathing room
	if h > defaultHeight {
		return defaultHeight
	}
	if h < 3 {
		return 3
	}
	return h
}
