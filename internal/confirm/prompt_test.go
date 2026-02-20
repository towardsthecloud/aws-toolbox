package confirm

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestConfirmNoConfirmBypassesPrompt(t *testing.T) {
	prompter := NewPrompter(strings.NewReader("n\n"), &bytes.Buffer{})
	ok, err := prompter.Confirm("delete resources", true)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if !ok {
		t.Fatal("expected true when noConfirm is set")
	}
}

func TestConfirmAcceptsYesAndNo(t *testing.T) {
	var out bytes.Buffer
	prompter := NewPrompter(strings.NewReader("yes\n"), &out)
	ok, err := prompter.Confirm("run action", false)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if !ok {
		t.Fatal("expected yes response to return true")
	}

	out.Reset()
	prompter = NewPrompter(strings.NewReader("\n"), &out)
	ok, err = prompter.Confirm("run action", false)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if ok {
		t.Fatal("expected empty response to default to no")
	}
}

func TestConfirmRepromptsOnInvalidInput(t *testing.T) {
	var out bytes.Buffer
	prompter := NewPrompter(strings.NewReader("maybe\ny\n"), &out)

	ok, err := prompter.Confirm("delete resources", false)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if !ok {
		t.Fatal("expected final yes response to return true")
	}

	output := out.String()
	if !strings.Contains(output, "Please answer yes or no.") {
		t.Fatalf("expected validation message in output, got %q", output)
	}
}

func TestConfirmDefaultsToNoOnEOF(t *testing.T) {
	prompter := NewPrompter(strings.NewReader(""), &bytes.Buffer{})
	ok, err := prompter.Confirm("run action", false)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if ok {
		t.Fatal("expected EOF to default to no")
	}
}

func TestNewPrompterDefaultsToStandardStreams(t *testing.T) {
	prompter := NewPrompter(nil, nil)
	if prompter.In == nil || prompter.Out == nil {
		t.Fatal("expected default stdio to be set")
	}
}

func TestConfirmReturnsWriteError(t *testing.T) {
	prompter := NewPrompter(strings.NewReader("y\n"), &alwaysErrorWriter{})
	_, err := prompter.Confirm("run action", false)
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestConfirmReturnsScannerError(t *testing.T) {
	prompter := NewPrompter(&alwaysErrorReader{}, &bytes.Buffer{})
	_, err := prompter.Confirm("run action", false)
	if err == nil {
		t.Fatal("expected scanner read error")
	}
}

func TestConfirmReturnsValidationWriteError(t *testing.T) {
	prompter := NewPrompter(strings.NewReader("maybe\n"), &nthErrorWriter{FailAt: 2})
	_, err := prompter.Confirm("run action", false)
	if err == nil {
		t.Fatal("expected validation write error")
	}
}

type alwaysErrorWriter struct{}

func (*alwaysErrorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

type alwaysErrorReader struct{}

func (*alwaysErrorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}

type nthErrorWriter struct {
	Count  int
	FailAt int
}

func (w *nthErrorWriter) Write(p []byte) (int, error) {
	w.Count++
	if w.Count >= w.FailAt {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}
