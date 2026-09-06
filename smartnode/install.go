package smartnode

import (
	"context"
	"os/exec"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/runctx"
)

// Install runs the non-interactive installation to unpack templates and scripts.
func Install(ctx context.Context, rc *runctx.RunContext, binPath string) (runctx.StepResult, error) {
	args := []string{"service", "install", "-d", "-y"}

	cmd := exec.Command(binPath, args...)
	// Installation unpacks files locally, should be fast.
	return rc.RunStep(ctx, cmd, 30*time.Second, 1024*1024, nil)
}
