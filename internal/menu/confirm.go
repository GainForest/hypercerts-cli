package menu

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Confirm prompts for yes/no confirmation. Returns true if user enters "y" or "yes".
func Confirm(w io.Writer, r io.Reader, message string) bool {
	fmt.Fprintf(w, "%s [y/N]: ", message)
	reader := bufio.NewReader(r)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// ConfirmBulkDelete prompts user to confirm deletion of multiple items.
// Auto-confirms for single items.
func ConfirmBulkDelete(w io.Writer, r io.Reader, count int, itemType string) bool {
	if count <= 1 {
		return true
	}
	fmt.Fprintf(w, "\033[1;33mDelete %d %ss?\033[0m [y/N]: ", count, itemType)
	reader := bufio.NewReader(r)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}
