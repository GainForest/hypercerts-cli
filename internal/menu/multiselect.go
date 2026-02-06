package menu

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/GainForest/hypercerts-cli/internal/style"
)

// MultiSelect displays an interactive multi-select menu using huh.
// Space/x toggles selection, ctrl+a toggles all. Returns the selected items.
func MultiSelect[T comparable](w io.Writer, items []T, itemType string, getName func(T) string, getInfo func(T) string) ([]T, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no %ss found", itemType)
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, ErrNonInteractive
	}

	opts := make([]huh.Option[int], len(items))
	for i, item := range items {
		label := getName(item)
		if info := getInfo(item); info != "" {
			label = fmt.Sprintf("%s  %s", label, info)
		}
		opts[i] = huh.NewOption(label, i)
	}

	var selectedIndices []int
	ms := huh.NewMultiSelect[int]().
		Title("Select " + itemType + "s").
		Options(opts...).
		Height(selectHeight(len(opts))).
		Filterable(len(items) > 5).
		Value(&selectedIndices)

	form := huh.NewForm(huh.NewGroup(ms)).
		WithTheme(style.Theme())

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return nil, ErrCancelled
		}
		return nil, err
	}

	var result []T
	for _, idx := range selectedIndices {
		result = append(result, items[idx])
	}

	if len(result) == 0 {
		return nil, ErrCancelled
	}

	if len(result) == 1 {
		fmt.Fprintf(w, "Selected: %s\n\n", getName(result[0]))
	} else {
		fmt.Fprintf(w, "Selected %d %ss\n\n", len(result), itemType)
	}
	return result, nil
}
