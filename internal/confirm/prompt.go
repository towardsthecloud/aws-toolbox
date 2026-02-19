package confirm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Prompter handles interactive yes/no confirmation prompts.
type Prompter struct {
	In  io.Reader
	Out io.Writer
}

// NewPrompter creates a prompter that reads from stdin and writes to stdout when nil is passed.
func NewPrompter(in io.Reader, out io.Writer) Prompter {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}

	return Prompter{In: in, Out: out}
}

// Confirm asks for explicit user confirmation unless noConfirm is true.
func (p Prompter) Confirm(action string, noConfirm bool) (bool, error) {
	if noConfirm {
		return true, nil
	}

	scanner := bufio.NewScanner(p.In)
	for {
		if _, err := fmt.Fprintf(p.Out, "%s [y/N]: ", action); err != nil {
			return false, err
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, err
			}
			return false, nil
		}

		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		switch response {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		default:
			if _, err := fmt.Fprintln(p.Out, "Please answer yes or no."); err != nil {
				return false, err
			}
		}
	}
}
