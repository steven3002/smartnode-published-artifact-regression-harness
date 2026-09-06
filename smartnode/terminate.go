package smartnode

import (
	"context"
	"os/exec"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
)

// terminateDeadline bounds teardown. Teardown is a step like any other and must
// not be able to hang a run that has already produced its verdict.
const terminateDeadline = 3 * time.Minute

// Terminate stops the stack and removes its containers, volumes and networks.
func Terminate(ctx context.Context, rc *runctx.RunContext, binPath string, redactFn func([]byte) []byte) (runctx.StepResult, error) {
	cmd := exec.Command(binPath, "service", "terminate", "--yes")
	return rc.RunStep(ctx, "terminate", cmd, terminateDeadline, 4*1024*1024, redactFn)
}
