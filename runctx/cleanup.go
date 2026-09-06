package runctx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// cleanupImage is the image used to remove container-owned files. It is pulled
// already by any run that reaches teardown.
const cleanupImage = "alpine:latest"

// cleanupTimeout bounds each external command teardown issues, so cleanup
// cannot itself become the hang it exists to prevent.
const cleanupTimeout = 2 * time.Minute

// Cleanup removes the containers, volumes and directories a run created.
//
// It runs on every exit path, including timeout and signal.
func (r *RunContext) Cleanup() error {
	var errs []error

	if err := r.composeDown(); err != nil {
		errs = append(errs, err)
	}
	for _, dir := range []string{r.HomeDir, r.DataDir} {
		if err := removeTree(dir); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// composeDown tears down the generated stack if one was rendered.
func (r *RunContext) composeDown() error {
	composeFile := filepath.Join(r.DataDir, "docker-compose.yml")
	if _, err := os.Stat(composeFile); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "down", "-v")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose down: %w: %s", err, out)
	}
	return nil
}

// removeTree deletes a directory created by a run.
//
// Containers in the generated stack run as root and write into the isolated
// home, so the host user cannot unlink what they leave behind and os.RemoveAll
// fails with EACCES. Removal is retried from inside a container, which has the
// privileges the host process deliberately does not.
func removeTree(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", dir+":/target",
		cleanupImage,
		"sh", "-c", "rm -rf /target/..?* /target/.[!.]* /target/*")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove %s via container: %w: %s", dir, err, out)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	return nil
}
