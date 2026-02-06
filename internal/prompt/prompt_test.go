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
