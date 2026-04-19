package deploy

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

// StyleStep renders step headings in bold.
var StyleStep = lipgloss.NewStyle().Bold(true)

// StyleOK renders the success marker in green.
var StyleOK = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)

// StyleFail renders the failure marker in red.
var StyleFail = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)

// StyleMuted renders ancillary output in a dim gray.
var StyleMuted = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

// OKMark is the glyph printed when a step succeeds.
const OKMark = "✓"

// FailMark is the glyph printed when a step fails.
const FailMark = "✗"

// Progress tracks step counts and prints structured, coloured progress lines
// to an io.Writer. It is intentionally tiny — just enough to keep wizard
// output legible without dragging in a TUI dependency.
type Progress struct {
	w         io.Writer
	stepsDone int
	total     int
	current   string
}

// NewProgress returns a Progress that writes to w with the given step total.
func NewProgress(w io.Writer, total int) *Progress {
	return &Progress{w: w, total: total}
}

// Step starts a new step, incrementing the counter and printing a header.
func (p *Progress) Step(title string) {
	if p == nil {
		return
	}
	p.stepsDone++
	p.current = title
	fmt.Fprintf(p.w, "%s %s\n",
		StyleStep.Render(fmt.Sprintf("▸ Step %d/%d:", p.stepsDone, p.total)),
		title,
	)
}

// OK closes the current step with a green check mark and optional summary.
func (p *Progress) OK(summary string) {
	if p == nil {
		return
	}
	if summary == "" {
		fmt.Fprintf(p.w, "  %s\n", StyleOK.Render(OKMark))
		return
	}
	fmt.Fprintf(p.w, "  %s %s\n", StyleOK.Render(OKMark), StyleMuted.Render(summary))
}

// Fail closes the current step with a red cross and the error cause.
func (p *Progress) Fail(err error) {
	if p == nil {
		return
	}
	msg := "failed"
	if err != nil {
		msg = err.Error()
	}
	fmt.Fprintf(p.w, "  %s %s\n", StyleFail.Render(FailMark), msg)
}

// Info writes an informational line indented under the current step.
func (p *Progress) Info(line string) {
	if p == nil {
		return
	}
	fmt.Fprintf(p.w, "  %s\n", StyleMuted.Render(line))
}
