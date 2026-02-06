package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ReadLine reads a single line from the reader and trims whitespace.
func ReadLine(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ReadLineWithDefault prompts with a label and optional default value.
// If the user enters empty input, the default value is returned.
func ReadLineWithDefault(w io.Writer, r io.Reader, label, hint, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, defaultVal)
	} else if hint != "" {
		fmt.Fprintf(w, "%s \033[90m(%s)\033[0m: ", label, hint)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}
	input, err := ReadLine(r)
	if err != nil {
		return "", err
	}
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}

// ReadOptionalField prompts for an optional field. Returns "" if the user skips.
func ReadOptionalField(w io.Writer, r io.Reader, label, hint string) (string, error) {
	return ReadLineWithDefault(w, r, label, hint, "")
}
