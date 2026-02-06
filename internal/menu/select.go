package menu

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// ErrCancelled is returned when the user cancels an interactive menu.
var ErrCancelled = fmt.Errorf("cancelled")

// ErrNonInteractive is returned when a terminal is required but unavailable.
var ErrNonInteractive = fmt.Errorf("non-interactive mode (use CLI flags instead)")

// SingleSelect displays an interactive single-select menu in an alternate screen buffer.
func SingleSelect[T any](w io.Writer, items []T, itemType string, getName func(T) string, getInfo func(T) string) (*T, error) {
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

	selected := 0
	buf := make([]byte, 3)
	for {
		RenderSingleSelect(os.Stdout, items, itemType, selected, getName, getInfo)

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
				fmt.Fprintf(w, "Selected: %s\n\n", getName(items[selected]))
				return &items[selected], nil
			case 'q', 3: // q or Ctrl+C
				fmt.Fprint(os.Stdout, "\033[?1049l")
				_ = term.Restore(fd, oldState)
				return nil, ErrCancelled
			case 'k':
				if selected > 0 {
					selected--
				}
			case 'j':
				if selected < len(items)-1 {
					selected++
				}
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65: // Up
				if selected > 0 {
					selected--
				}
			case 66: // Down
				if selected < len(items)-1 {
					selected++
				}
			}
		}
	}
}

// SingleSelectWithCreate is like SingleSelect but adds a "Create new..." option at the bottom.
// Returns (item, isCreate, error). If isCreate is true, the item pointer is nil.
func SingleSelectWithCreate[T any](w io.Writer, items []T, itemType string, getName func(T) string, getInfo func(T) string, createLabel string) (*T, bool, error) {
	totalOptions := len(items) + 1
	createNewIndex := len(items)

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, false, ErrNonInteractive
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, false, fmt.Errorf("terminal error: %w", err)
	}

	fmt.Fprint(os.Stdout, "\033[?1049h\033[H")

	selected := 0
	buf := make([]byte, 3)
	for {
		RenderSingleSelectWithCreate(os.Stdout, items, itemType, selected, createNewIndex, getName, getInfo, createLabel)

		n, err := os.Stdin.Read(buf)
		if err != nil {
			fmt.Fprint(os.Stdout, "\033[?1049l")
			_ = term.Restore(fd, oldState)
			return nil, false, err
		}

		if n == 1 {
			switch buf[0] {
			case 13, 10: // Enter
				fmt.Fprint(os.Stdout, "\033[?1049l")
				_ = term.Restore(fd, oldState)
				if selected == createNewIndex {
					return nil, true, nil
				}
				fmt.Fprintf(w, "Selected: %s\n", getName(items[selected]))
				return &items[selected], false, nil
			case 'q', 3:
				fmt.Fprint(os.Stdout, "\033[?1049l")
				_ = term.Restore(fd, oldState)
				return nil, false, ErrCancelled
			case 'k':
				if selected > 0 {
					selected--
				}
			case 'j':
				if selected < totalOptions-1 {
					selected++
				}
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65:
				if selected > 0 {
					selected--
				}
			case 66:
				if selected < totalOptions-1 {
					selected++
				}
			}
		}
	}
}
