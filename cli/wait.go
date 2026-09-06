package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/ui"
)

// heartbeat is how often an unchanged wait is restated on a non-interactive
// stream. Long enough that a CI log stays readable, short enough that a reader
// can see the run is alive.
const heartbeat = 30 * time.Second

// waitReporter collapses a repeated progress message into a single line.
//
// The readiness poll reports every attempt, and a client can take minutes to
// open its port, so printing each attempt produces a wall of identical lines.
// That is not merely untidy: a reader cannot tell a slow start from a hang, and
// the reasonable response to a screen of repeating text is to interrupt a run
// that was working.
//
// On a terminal the line is rewritten in place with elapsed time. Elsewhere it
// is stated once and then restated on a heartbeat, because rewriting a line
// with escape codes corrupts a log file.
type waitReporter struct {
	out io.Writer
	cap ui.Capability

	message  string
	since    time.Time
	lastEmit time.Time
	inPlace  bool
}

func newWaitReporter(out io.Writer, cap ui.Capability) *waitReporter {
	return &waitReporter{out: out, cap: cap}
}

// report shows a progress message, suppressing consecutive repeats.
func (w *waitReporter) report(message string) {
	now := time.Now()

	if message != w.message {
		w.clear()
		w.message = message
		w.since = now
		w.lastEmit = now
		w.write(message, 0)
		return
	}

	elapsed := now.Sub(w.since)
	if w.cap.IsTTY {
		w.write(message, elapsed)
		return
	}
	if now.Sub(w.lastEmit) >= heartbeat {
		w.lastEmit = now
		w.write(message, elapsed)
	}
}

// done ends any in-place line so the next output starts cleanly.
func (w *waitReporter) done() {
	w.clear()
	w.message = ""
}

func (w *waitReporter) write(message string, elapsed time.Duration) {
	text := message
	if elapsed >= time.Second {
		text = fmt.Sprintf("%s [%s]", message, round(elapsed))
	}
	line := ui.FormatStatus(w.cap, ui.StatusInfo, text)

	if !w.cap.IsTTY {
		fmt.Fprintln(w.out, line)
		return
	}

	// \r returns to the start of the line and \033[K clears to its end, so a
	// shorter message cannot leave the tail of a longer one behind.
	fmt.Fprintf(w.out, "\r\033[K%s", line)
	w.inPlace = true
}

// clear terminates an in-place line with a newline so it is preserved rather
// than overwritten by whatever prints next.
func (w *waitReporter) clear() {
	if w.inPlace {
		fmt.Fprintln(w.out)
		w.inPlace = false
	}
}

// round trims a duration to whole seconds, which is the only precision that
// matters when a reader is watching a client start.
func round(d time.Duration) time.Duration {
	return d.Round(time.Second)
}
