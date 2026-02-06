package menu

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/GainForest/hypercerts-cli/internal/style"
)

// Confirm prompts for yes/no confirmation using a huh confirm widget.
// Falls back to a plain text prompt when stdin is not a terminal (e.g. in tests).
func Confirm(w io.Writer, r io.Reader, message string) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return confirmText(w, r, message)
	}
	var yes bool
	err := huh.NewConfirm().
		Title(message).
		Value(&yes).
		WithTheme(style.Theme()).
		Run()
	if err != nil {
		return false
	}
	return yes
}

// ConfirmBulkDelete prompts user to confirm deletion of multiple items.
// Auto-confirms for single items.
func ConfirmBulkDelete(w io.Writer, r io.Reader, count int, itemType string) bool {
	if count <= 1 {
		return true
	}
	message := fmt.Sprintf("Delete %d %ss?", count, itemType)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return confirmText(w, r, message)
	}
	var yes bool
	err := huh.NewConfirm().
		Title(message).
		Description("This action cannot be undone").
		Value(&yes).
		WithTheme(style.Theme()).
		Run()
	if err != nil {
		return false
	}
	return yes
}

// confirmText is the plain-text fallback for non-TTY environments and tests.
func confirmText(w io.Writer, r io.Reader, message string) bool {
	fmt.Fprintf(w, "%s [y/N]: ", message)
	reader := bufio.NewReader(r)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}
