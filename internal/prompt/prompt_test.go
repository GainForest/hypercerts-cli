package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadLine(t *testing.T) {
	input := "hello world\n"
	got, err := ReadLine(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestReadLine_trimSpace(t *testing.T) {
	input := "  padded  \n"
	got, err := ReadLine(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadLine: %v", err)
	}
	if got != "padded" {
		t.Errorf("got %q, want %q", got, "padded")
	}
}

func TestReadLine_eof(t *testing.T) {
	_, err := ReadLine(strings.NewReader(""))
	if err != ErrCancelled {
		t.Errorf("got %v, want ErrCancelled", err)
	}
}

func TestReadLine_eof_partial(t *testing.T) {
	got, err := ReadLine(strings.NewReader("partial"))
	if err != ErrCancelled {
		t.Errorf("got err %v, want ErrCancelled", err)
	}
	if got != "partial" {
		t.Errorf("got %q, want %q", got, "partial")
	}
}

func TestReadLineWithDefault_empty(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadLineWithDefault(&buf, strings.NewReader("\n"), "Name", "", "Alice")
	if err != nil {
		t.Fatalf("ReadLineWithDefault: %v", err)
	}
	if got != "Alice" {
		t.Errorf("got %q, want %q", got, "Alice")
	}
	// Should show [Alice] in prompt
	if !strings.Contains(buf.String(), "[Alice]") {
		t.Errorf("prompt should contain default: %q", buf.String())
	}
}

func TestReadLineWithDefault_override(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadLineWithDefault(&buf, strings.NewReader("Bob\n"), "Name", "", "Alice")
	if err != nil {
		t.Fatalf("ReadLineWithDefault: %v", err)
	}
	if got != "Bob" {
		t.Errorf("got %q, want %q", got, "Bob")
	}
}

func TestReadLineWithDefault_hint(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadLineWithDefault(&buf, strings.NewReader("\n"), "Email", "optional", "")
	if err != nil {
		t.Fatalf("ReadLineWithDefault: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if !strings.Contains(buf.String(), "optional") {
		t.Errorf("prompt should contain hint: %q", buf.String())
	}
}

func TestReadLineWithDefault_eof(t *testing.T) {
	var buf bytes.Buffer
	_, err := ReadLineWithDefault(&buf, strings.NewReader(""), "Name", "", "Alice")
	if err != ErrCancelled {
		t.Errorf("got %v, want ErrCancelled", err)
	}
}

func TestReadOptionalField_empty(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadOptionalField(&buf, strings.NewReader("\n"), "Notes", "optional")
	if err != nil {
		t.Fatalf("ReadOptionalField: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestReadOptionalField_value(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadOptionalField(&buf, strings.NewReader("some text\n"), "Notes", "optional")
	if err != nil {
		t.Fatalf("ReadOptionalField: %v", err)
	}
	if got != "some text" {
		t.Errorf("got %q, want %q", got, "some text")
	}
}

func TestReadRequired_firstTry(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadRequired(&buf, strings.NewReader("hello\n"), "Title", "required")
	if err != nil {
		t.Fatalf("ReadRequired: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	// Should not contain warning
	if strings.Contains(buf.String(), "⚠") {
		t.Errorf("should not show warning on first valid input: %q", buf.String())
	}
}

func TestReadRequired_retryThenSucceed(t *testing.T) {
	// First press enter (empty), then enter value
	var buf bytes.Buffer
	got, err := ReadRequired(&buf, strings.NewReader("\n\nworld\n"), "Title", "required")
	if err != nil {
		t.Fatalf("ReadRequired: %v", err)
	}
	if got != "world" {
		t.Errorf("got %q, want %q", got, "world")
	}
	// Should contain warning(s) for empty attempts
	if !strings.Contains(buf.String(), "Title is required") {
		t.Errorf("should show retry warning: %q", buf.String())
	}
}

func TestReadRequired_eof(t *testing.T) {
	var buf bytes.Buffer
	_, err := ReadRequired(&buf, strings.NewReader(""), "Title", "required")
	if err != ErrCancelled {
		t.Errorf("got %v, want ErrCancelled", err)
	}
}

func TestReadRequired_eofAfterRetry(t *testing.T) {
	// Press enter once (empty), then EOF
	var buf bytes.Buffer
	_, err := ReadRequired(&buf, strings.NewReader("\n"), "Title", "required")
	if err != ErrCancelled {
		t.Errorf("got %v, want ErrCancelled", err)
	}
	if !strings.Contains(buf.String(), "Title is required") {
		t.Errorf("should show warning before EOF: %q", buf.String())
	}
}

func TestReadRequired_noHint(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadRequired(&buf, strings.NewReader("val\n"), "Name", "")
	if err != nil {
		t.Fatalf("ReadRequired: %v", err)
	}
	if got != "val" {
		t.Errorf("got %q, want %q", got, "val")
	}
	// Should show plain prompt without hint
	if strings.Contains(buf.String(), "(") {
		t.Errorf("should not show hint parens: %q", buf.String())
	}
}

func TestReadRequiredWithDefault_usesDefault(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadRequiredWithDefault(&buf, strings.NewReader("\n"), "From", "required", "did:plc:abc")
	if err != nil {
		t.Fatalf("ReadRequiredWithDefault: %v", err)
	}
	if got != "did:plc:abc" {
		t.Errorf("got %q, want %q", got, "did:plc:abc")
	}
}

func TestReadRequiredWithDefault_override(t *testing.T) {
	var buf bytes.Buffer
	got, err := ReadRequiredWithDefault(&buf, strings.NewReader("did:plc:xyz\n"), "From", "required", "did:plc:abc")
	if err != nil {
		t.Fatalf("ReadRequiredWithDefault: %v", err)
	}
	if got != "did:plc:xyz" {
		t.Errorf("got %q, want %q", got, "did:plc:xyz")
	}
}

func TestReadRequiredWithDefault_noDefault_retries(t *testing.T) {
	// No default, empty input should retry
	var buf bytes.Buffer
	got, err := ReadRequiredWithDefault(&buf, strings.NewReader("\nvalue\n"), "Field", "required", "")
	if err != nil {
		t.Fatalf("ReadRequiredWithDefault: %v", err)
	}
	if got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
	if !strings.Contains(buf.String(), "Field is required") {
		t.Errorf("should show warning: %q", buf.String())
	}
}

func TestReadRequiredWithDefault_eof(t *testing.T) {
	var buf bytes.Buffer
	_, err := ReadRequiredWithDefault(&buf, strings.NewReader(""), "From", "required", "did:plc:abc")
	if err != ErrCancelled {
		t.Errorf("got %v, want ErrCancelled", err)
	}
}
