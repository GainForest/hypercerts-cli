package menu

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// MultiSelect displays an interactive multi-select menu with checkboxes.
// Space toggles selection, 'a' selects all, 'n' selects none.
func MultiSelect[T any](w io.Writer, items []T, itemType string, getName func(T) string, getInfo func(T) string) ([]T, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no %ss found", itemType)
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, ErrNonInteractive
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("terminal error: %w", err)
	}

	fmt.Fprint(os.Stdout, "\033[?1049h\033[H")

	cursor := 0
	selected := make(map[int]bool)
	buf := make([]byte, 3)
	for {
		RenderMultiSelect(os.Stdout, items, itemType, cursor, selected, getName, getInfo)

		n, err := os.Stdin.Read(buf)
		if err != nil {
			fmt.Fprint(os.Stdout, "\033[?1049l")
			_ = term.Restore(fd, oldState)
			return nil, err
		}

		if n == 1 {
			switch buf[0] {
			case 13, 10: // Enter
				fmt.Fprint(os.Stdout, "\033[?1049l")
				_ = term.Restore(fd, oldState)
				if len(selected) == 0 {
					selected[cursor] = true
				}
				var result []T
				for i, item := range items {
					if selected[i] {
						result = append(result, item)
					}
				}
				if len(result) == 1 {
					fmt.Fprintf(w, "Selected: %s\n\n", getName(result[0]))
				} else {
					fmt.Fprintf(w, "Selected %d %ss\n\n", len(result), itemType)
				}
				return result, nil
			case 'q', 3:
				fmt.Fprint(os.Stdout, "\033[?1049l")
				_ = term.Restore(fd, oldState)
				return nil, ErrCancelled
			case ' ':
				selected[cursor] = !selected[cursor]
				if !selected[cursor] {
					delete(selected, cursor)
				}
			case 'a':
				for i := range items {
					selected[i] = true
				}
			case 'n':
				selected = make(map[int]bool)
			case 'k':
				if cursor > 0 {
					cursor--
				}
			case 'j':
				if cursor < len(items)-1 {
					cursor++
				}
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65:
				if cursor > 0 {
					cursor--
				}
			case 66:
				if cursor < len(items)-1 {
					cursor++
				}
			}
		}
	}
}
