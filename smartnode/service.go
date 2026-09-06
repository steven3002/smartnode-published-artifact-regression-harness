package smartnode

import (
	"context"
	"os/exec"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
)

// StartServiceParams holds the parameters to start the Smartnode service.
type StartServiceParams struct {
	IgnoreSlashTimer bool
	Yes              bool
}

// StartDeadline bounds stack startup.
//
// It is generous because the first run on a host pulls several gigabytes of
// client images before anything starts, and a pull that is merely slow must not
// be recorded as a product defect.
const StartDeadline = 20 * time.Minute

// Start brings up the generated stack.
//
// --ignore-slash-timer is required in addition to --yes: the anti-slashing
// prompt is raised without a --yes guard, so it fires on a fresh install even
// under --yes and blocks an unattended run.
func Start(ctx context.Context, rc *runctx.RunContext, binPath string, params StartServiceParams, redactFn func([]byte) []byte) (runctx.StepResult, error) {
	args := []string{"service", "start"}
	if params.Yes {
		args = append(args, "--yes")
	}
	if params.IgnoreSlashTimer {
		args = append(args, "--ignore-slash-timer")
	}

	cmd := exec.Command(binPath, args...)
	return rc.RunStep(ctx, "start", cmd, StartDeadline, 16*1024*1024, redactFn)
}
