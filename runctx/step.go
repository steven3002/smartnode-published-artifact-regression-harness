package runctx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// StepFailure names why a step did not complete normally.
//
// It is deliberately not the reporting layer's failure class: runctx observes
// what happened to a process and does not decide what it means for the product
// under test.
type StepFailure string

const (
	StepOK        StepFailure = ""
	StepExited    StepFailure = "EXITED"
	StepTimeout   StepFailure = "TIMEOUT"
	StepCancelled StepFailure = "CANCELLED"
)

// StepResult is what one step did.
type StepResult struct {
	Name           string
	Output         []byte
	TruncatedBytes int64
	Failure        StepFailure
	ExitCode       int
	Duration       time.Duration
}

// Truncated reports whether output was dropped against the cap.
func (s StepResult) Truncated() bool { return s.TruncatedBytes > 0 }

// RunStep runs a command bounded on three axes: a deadline, an output cap, and
// the lifetime of its process group.
//
// All three are load-bearing. The Smartnode CLI prompts on a closed stdin
// without an EOF check, which produces tens of megabytes of output in seconds
// rather than a clean hang, so a harness without caps is taken down by the
// software it is testing.
func (r *RunContext) RunStep(
	ctx context.Context,
	name string,
	cmd *exec.Cmd,
	deadline time.Duration,
	byteCap int64,
	redactFn func([]byte) []byte,
) (StepResult, error) {
	isolateProcessGroup(cmd)

	// stdin is closed rather than inherited so a prompt fails fast instead of
	// blocking on a terminal that may not exist.
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "HOME="+r.HomeDir)

	var out bytes.Buffer
	writer := &BoundedWriter{Buf: &out, Limit: byteCap, Redact: redactFn}
	cmd.Stdout = writer
	cmd.Stderr = writer

	started := time.Now()
	if err := cmd.Start(); err != nil {
		return StepResult{Name: name, Failure: StepExited, ExitCode: -1},
			fmt.Errorf("start %s: %w", name, err)
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		pgid = cmd.Process.Pid
	}

	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		err := cmd.Wait()
		close(exited)
		done <- err
	}()

	timer := time.NewTimer(deadline)
	defer timer.Stop()

	var waitErr error
	failure := StepOK

	select {
	case <-ctx.Done():
		killProcessGroup(pgid, exited)
		<-done
		waitErr = ctx.Err()
		failure = StepCancelled
	case <-timer.C:
		killProcessGroup(pgid, exited)
		<-done
		waitErr = fmt.Errorf("step %s exceeded its %s deadline", name, deadline)
		failure = StepTimeout
	case err := <-done:
		waitErr = err
		if err != nil {
			failure = StepExited
		}
	}

	writer.Flush()

	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return StepResult{
		Name:           name,
		Output:         out.Bytes(),
		TruncatedBytes: writer.Dropped,
		Failure:        failure,
		ExitCode:       exitCode,
		Duration:       time.Since(started),
	}, waitErr
}
