// Package runctx provides the sandbox every harness step executes inside:
// filesystem isolation, deadlines, bounded output, process-tree lifetime and
// teardown.
//
// It holds no knowledge of what any step means. That belongs to the caller.
package runctx

import (
	"fmt"
	"os"
	"path/filepath"
)

// WorkDir returns the base directory runs are created under.
//
// The system temporary directory is deliberately not the default: it is a small
// tmpfs on many hosts, and a synced execution client needs tens of gigabytes of
// real disk. The user cache directory is on the same backing store as $HOME,
// which is where that space actually is.
//
// RP_REGRESS_WORK_DIR overrides it for hosts that keep bulk storage elsewhere.
func WorkDir() (string, error) {
	if dir := os.Getenv(WorkDirEnv); dir != "" {
		return dir, nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("locate cache directory: %w", err)
	}
	return filepath.Join(cache, "rp-regress"), nil
}

// WorkDirEnv names the environment variable that overrides WorkDir.
const WorkDirEnv = "RP_REGRESS_WORK_DIR"

// RunContext is one run's isolated filesystem.
//
// HomeDir is exported to the step as $HOME. An isolated HOME is required rather
// than merely convenient: the Smartnode installer reads $HOME/.rocketpool even
// when --path is supplied, so --path alone does not isolate a run from the
// operator's real configuration.
type RunContext struct {
	HomeDir string
	DataDir string
}

// New creates an isolated home and data directory beneath the system temporary
// directory, honouring TMPDIR.
//
// The base must not be inside the repository. Container processes write into
// HomeDir as root, so run state left behind there is both unremovable by the
// host user and a route for generated API tokens to reach a commit.
func New() (*RunContext, error) {
	base, err := WorkDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("create work directory %s: %w", base, err)
	}

	home, err := os.MkdirTemp(base, "home-*")
	if err != nil {
		return nil, fmt.Errorf("create isolated home: %w", err)
	}

	data, err := os.MkdirTemp(base, "data-*")
	if err != nil {
		os.RemoveAll(home)
		return nil, fmt.Errorf("create isolated data dir: %w", err)
	}

	return &RunContext{HomeDir: home, DataDir: data}, nil
}
