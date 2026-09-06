package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/ui"
)

// TestRepeatedWaitPrintsOnceOffTTY is the behaviour that made a working run
// look stuck: the readiness poll reports every attempt, and printing each one
// produced a wall of identical lines.
func TestRepeatedWaitPrintsOnceOffTTY(t *testing.T) {
	var buf bytes.Buffer
	w := newWaitReporter(&buf, ui.Capability{IsTTY: false})

	const msg = "waiting for consensus client: connection refused"
	for i := 0; i < 20; i++ {
		w.report(msg)
	}
	w.done()

	if got := strings.Count(buf.String(), "waiting for consensus client"); got != 1 {
		t.Errorf("printed the same wait %d times; want 1", got)
	}
}

// TestChangedWaitPrintsAgain guards against collapsing distinct messages, which
// would hide a transition from one client to the other.
func TestChangedWaitPrintsAgain(t *testing.T) {
	var buf bytes.Buffer
	w := newWaitReporter(&buf, ui.Capability{IsTTY: false})

	w.report("waiting for execution client: connection refused")
	w.report("waiting for execution client: connection refused")
	w.report("waiting for consensus client: connection refused")
	w.done()

	out := buf.String()
	if strings.Count(out, "execution client") != 1 {
		t.Errorf("execution client line printed %d times; want 1", strings.Count(out, "execution client"))
	}
	if strings.Count(out, "consensus client") != 1 {
		t.Errorf("consensus client line printed %d times; want 1", strings.Count(out, "consensus client"))
	}
}

// TestHeartbeatRestatesWithElapsed checks a long wait is restated periodically,
// so a non-interactive log shows the run is alive rather than silent.
func TestHeartbeatRestatesWithElapsed(t *testing.T) {
	var buf bytes.Buffer
	w := newWaitReporter(&buf, ui.Capability{IsTTY: false})

	const msg = "waiting for consensus client: connection refused"
	w.report(msg)

	// Backdate the reporter so the next report crosses the heartbeat.
	w.since = w.since.Add(-2 * heartbeat)
	w.lastEmit = w.lastEmit.Add(-2 * heartbeat)
	w.report(msg)
	w.done()

	out := buf.String()
	if strings.Count(out, "waiting for consensus client") != 2 {
		t.Fatalf("heartbeat did not restate the wait:\n%s", out)
	}
	if !strings.Contains(out, "[1m0s]") {
		t.Errorf("restated wait carries no elapsed time:\n%s", out)
	}
}

// TestTTYRewritesInPlace checks a terminal gets one updating line rather than a
// new line per attempt, and that it is terminated before other output follows.
func TestTTYRewritesInPlace(t *testing.T) {
	var buf bytes.Buffer
	w := newWaitReporter(&buf, ui.Capability{IsTTY: true})

	const msg = "waiting for consensus client: connection refused"
	w.report(msg)
	w.since = w.since.Add(-5 * time.Second)
	w.report(msg)
	w.done()

	out := buf.String()
	if !strings.Contains(out, "\r\033[K") {
		t.Error("terminal output does not rewrite the line in place")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("in-place line was not terminated before following output")
	}
}

// TestNoEscapeCodesOffTTY keeps cursor control out of files and CI logs.
func TestNoEscapeCodesOffTTY(t *testing.T) {
	var buf bytes.Buffer
	w := newWaitReporter(&buf, ui.Capability{IsTTY: false})
	w.report("waiting for consensus client: connection refused")
	w.done()

	if strings.ContainsAny(buf.String(), "\r\033") {
		t.Errorf("escape codes written to a non-terminal stream: %q", buf.String())
	}
}
