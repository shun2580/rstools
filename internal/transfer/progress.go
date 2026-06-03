package transfer

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// Progress tracks and displays file transfer progress to stderr.
type Progress struct {
	total   int
	current int
	enabled bool
}

// NewProgress creates a Progress tracker. Display is suppressed in non-TTY environments.
func NewProgress(total int) *Progress {
	return &Progress{
		total:   total,
		enabled: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

// Inc increments the counter and prints the current progress.
func (p *Progress) Inc(name string) {
	p.current++
	if p.enabled {
		fmt.Fprintf(os.Stderr, "\r[%d/%d] %s", p.current, p.total, name)
	}
}

// Done prints a newline to finalize the progress output.
func (p *Progress) Done() {
	if p.enabled && p.current > 0 {
		fmt.Fprintln(os.Stderr)
	}
}
