package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrCancelled is returned when the user cancels input (EOF/Ctrl+D).
var ErrCancelled = errors.New("cancelled")

// readLineFrom reads a single line from a buffered reader and trims whitespace.
func readLineFrom(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return strings.TrimSpace(line), ErrCancelled
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// ReadLine reads a single line from the reader and trims whitespace.
func ReadLine(r io.Reader) (string, error) {
	return readLineFrom(bufio.NewReader(r))
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

// ReadRequired prompts for a required field and retries until non-empty input
// is provided. Returns ErrCancelled if the user sends EOF (Ctrl+D).
func ReadRequired(w io.Writer, r io.Reader, label, hint string) (string, error) {
	br := bufio.NewReader(r)
	for {
		if hint != "" {
			fmt.Fprintf(w, "%s \033[90m(%s)\033[0m: ", label, hint)
		} else {
			fmt.Fprintf(w, "%s: ", label)
		}
		input, err := readLineFrom(br)
		if err != nil {
			return "", err
		}
		if input != "" {
			return input, nil
		}
		fmt.Fprintf(w, "  \033[33m⚠ %s is required, try again\033[0m\n", label)
	}
}

// ReadRequiredWithDefault prompts for a required field with a default value.
// If the user enters empty input, the default is used. Retries until non-empty.
// Returns ErrCancelled if the user sends EOF (Ctrl+D).
func ReadRequiredWithDefault(w io.Writer, r io.Reader, label, hint, defaultVal string) (string, error) {
	br := bufio.NewReader(r)
	for {
		if defaultVal != "" {
			fmt.Fprintf(w, "%s [%s]: ", label, defaultVal)
		} else if hint != "" {
			fmt.Fprintf(w, "%s \033[90m(%s)\033[0m: ", label, hint)
		} else {
			fmt.Fprintf(w, "%s: ", label)
		}
		input, err := readLineFrom(br)
		if err != nil {
			return "", err
		}
		if input != "" {
			return input, nil
		}
		if defaultVal != "" {
			return defaultVal, nil
		}
		fmt.Fprintf(w, "  \033[33m⚠ %s is required, try again\033[0m\n", label)
	}
}
