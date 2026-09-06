package smartnode

import (
	"context"
	"os/exec"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/runctx"
)

// StartServiceParams holds the parameters to start the Smartnode service.
type StartServiceParams struct {
	IgnoreSlashTimer bool
	Yes              bool
}

// Start runs `rocketpool service start` with the given parameters.
func Start(ctx context.Context, rc *runctx.RunContext, binPath string, params StartServiceParams) (runctx.StepResult, error) {
	args := []string{"service", "start"}
	if params.Yes {
		args = append(args, "--yes")
	}
	if params.IgnoreSlashTimer {
		args = append(args, "--ignore-slash-timer")
	}

	cmd := exec.Command(binPath, args...)
	// Starting containers can take some time as it pulls images, so we use a 5-minute deadline.
	// We allow up to 10MB of output.
	return rc.RunStep(ctx, cmd, 5*time.Minute, 10*1024*1024, nil)
}
