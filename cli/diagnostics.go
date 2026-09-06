package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/docker"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
)

// logsDirName is where captured output is staged inside the run directory, to be
// collected into the report's diagnostics bundle.
const logsDirName = "logs"

// containerLogTail is how many lines are kept per container. Enough to show why
// a service failed, bounded so a chatty client cannot dominate the bundle.
const containerLogTail = "200"

// containerLogTimeout bounds log collection during teardown.
const containerLogTimeout = 60 * time.Second

// captureStepOutput stages one step's captured output for the diagnostics
// bundle.
//
// Without this the bundle is empty on a failing run: step output is held in
// memory to enforce the byte cap and is otherwise discarded, so nothing reaches
// the report that explains what went wrong.
func captureStepOutput(rc *runctx.RunContext, step runctx.StepResult) {
	if len(step.Output) == 0 && step.TruncatedBytes == 0 {
		return
	}

	dir := filepath.Join(rc.DataDir, logsDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}

	body := step.Output
	if step.TruncatedBytes > 0 {
		body = append(body,
			[]byte(fmt.Sprintf("\n[output truncated: %d bytes dropped]\n", step.TruncatedBytes))...)
	}
	os.WriteFile(filepath.Join(dir, step.Name+".log"), body, 0o600)
}

// captureContainerLogs stages the tail of each container's log.
//
// It runs before teardown, because teardown removes the containers these logs
// belong to, and a bundle collected afterwards would be empty for exactly the
// runs that need it.
func captureContainerLogs(ctx context.Context, rc *runctx.RunContext) {
	ctx, cancel := context.WithTimeout(ctx, containerLogTimeout)
	defer cancel()

	states, err := docker.GetStackStates(ctx, composeProject)
	if err != nil || len(states) == 0 {
		return
	}

	dir := filepath.Join(rc.DataDir, logsDirName, "containers")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}

	for name := range states {
		out, err := exec.CommandContext(ctx, "docker", "logs", "--tail", containerLogTail, name).CombinedOutput()
		if err != nil && len(out) == 0 {
			continue
		}
		os.WriteFile(filepath.Join(dir, name+".log"), out, 0o600)
	}
}
