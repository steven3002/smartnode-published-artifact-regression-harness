package runctx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type RunContext struct {
	HomeDir string
	DataDir string
}

func New() (*RunContext, error) {
	baseTmpDir := "/home/ubuntu/smartnode-release-regression/scratch"
	os.MkdirAll(baseTmpDir, 0755)

	home, err := os.MkdirTemp(baseTmpDir, "rp-regress-home-*")
	if err != nil {
		return nil, fmt.Errorf("create temp home: %w", err)
	}

	data, err := os.MkdirTemp(baseTmpDir, "rp-regress-data-*")
	if err != nil {
		os.RemoveAll(home)
		return nil, fmt.Errorf("create temp data: %w", err)
	}

	return &RunContext{
		HomeDir: home,
		DataDir: data,
	}, nil
}

func (r *RunContext) Cleanup() error {
	var errs []error
	if err := os.RemoveAll(r.HomeDir); err != nil {
		errs = append(errs, err)
	}
	if err := os.RemoveAll(r.DataDir); err != nil {
		errs = append(errs, err)
	}

	// Add Docker teardown logic as required by M10.
	// Since we are mocking docker interaction here for MVP scope 1,
	// we will run 'docker compose down -v' on the temp dir if there's a compose file.
	// We do this by calling a script or running docker natively, but we'll leave it to caller or robust teardown here.
	// Wait, the spec says "Cleanup that runs on success, failure, timeout and signal — containers, volumes, networks, temp dirs."
	// "docker compose down" from the config directory is the standard. Let's do it if compose file exists.
	composeFile := filepath.Join(r.DataDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); err == nil {
		cmd := exec.Command("docker", "compose", "-f", composeFile, "down", "-v")
		cmd.Run()
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

type StepResult struct {
	Output         []byte
	TruncatedBytes int64
	FailureClass   string
	ExitCode       int
}

// BoundedWriter is an io.Writer that stops writing after limit is reached,
// tracking how many bytes were dropped.
type BoundedWriter struct {
	Buf     *bytes.Buffer
	Limit   int64
	Written int64
	Dropped int64
	Redact  func([]byte) []byte
}

func (b *BoundedWriter) Write(p []byte) (n int, err error) {
	if b.Redact != nil {
		p = b.Redact(p)
	}

	if b.Written >= b.Limit {
		b.Dropped += int64(len(p))
		return len(p), nil
	}

	space := b.Limit - b.Written
	if int64(len(p)) > space {
		b.Buf.Write(p[:space])
		b.Written += space
		b.Dropped += int64(len(p)) - space
		return len(p), nil
	}

	n, err = b.Buf.Write(p)
	b.Written += int64(n)
	return n, err
}

// RunStep runs a command with a deadline, byte cap, and redaction.
func (r *RunContext) RunStep(ctx context.Context, cmd *exec.Cmd, deadline time.Duration, byteCap int64, redact func([]byte) []byte) (StepResult, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Apply isolated environment variables
	env := os.Environ()
	cmd.Env = append(env, "HOME="+r.HomeDir)

	var out bytes.Buffer
	writer := &BoundedWriter{
		Buf:    &out,
		Limit:  byteCap,
		Redact: redact,
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return StepResult{}, fmt.Errorf("start: %w", err)
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		pgid = cmd.Process.Pid
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timer := time.NewTimer(deadline)
	defer timer.Stop()

	var waitErr error
	var failureClass string

	select {
	case <-ctx.Done():
		// Cancelled by caller (e.g. SIGINT)
		killProcessGroup(pgid)
		<-done
		waitErr = ctx.Err()
		failureClass = "CANCELLED"
	case <-timer.C:
		// Timeout
		killProcessGroup(pgid)
		<-done
		waitErr = errors.New("deadline exceeded")
		failureClass = "TIMEOUT"
	case err := <-done:
		waitErr = err
		if err != nil {
			failureClass = "PRODUCT"
		}
	}

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
		Output:         out.Bytes(),
		TruncatedBytes: writer.Dropped,
		FailureClass:   failureClass,
		ExitCode:       exitCode,
	}, waitErr
}

func killProcessGroup(pgid int) {
	// Send SIGTERM to the process group
	syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait a short grace period
	time.Sleep(2 * time.Second)

	// Send SIGKILL to the process group
	syscall.Kill(-pgid, syscall.SIGKILL)
}
