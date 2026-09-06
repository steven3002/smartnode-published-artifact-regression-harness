package smartnode

import (
	"context"
	"os/exec"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
)

// installDeadline bounds template unpacking, which is a local file operation.
const installDeadline = 2 * time.Minute

// Install unpacks the templates and scripts the generated stack is built from.
func Install(ctx context.Context, rc *runctx.RunContext, binPath string, redactFn func([]byte) []byte) (runctx.StepResult, error) {
	cmd := exec.Command(binPath, "service", "install", "-d", "-y")
	return rc.RunStep(ctx, "install", cmd, installDeadline, 4*1024*1024, redactFn)
}
