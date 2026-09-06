package jwt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// readImage is used to read a secret the host user cannot open.
const readImage = "alpine:latest"

// readTimeout bounds the privileged read.
const readTimeout = 30 * time.Second

// State describes a secret on disk without exposing its value.
type State struct {
	Present bool
	Empty   bool
	Valid   bool
}

// Diagnosis returns the code describing an unusable secret, or the empty string
// when the secret is well formed.
func (s State) Diagnosis() string {
	if s.Present && (s.Empty || !s.Valid) {
		return CodeMalformed
	}
	return ""
}

// CodeMalformed identifies an empty or malformed engine-API secret.
//
// This is the failure class the harness exists to detect: the Smartnode start
// script repairs a zero-byte secret for Besu, Reth and Erigon by testing
// [ ! -s ], tests only [ ! -f ] for Nethermind, and has no branch at all for
// Geth, which relies on the client generating its own.
const CodeMalformed = "JWT-001"

// MalformedMessage is the human-readable form of CodeMalformed.
const MalformedMessage = "JWT-001: JWT secret is empty or malformed"

// Inspect reports the state of the secret at path without returning its value.
//
// Container processes create the secret as root, so a direct read fails with
// EACCES on a host user's run and is retried from inside a container.
func Inspect(ctx context.Context, path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			data, err = readPrivileged(ctx, path)
		}
		if err != nil {
			if os.IsNotExist(err) {
				return State{Present: false}, nil
			}
			return State{}, fmt.Errorf("read secret: %w", err)
		}
	}

	trimmed := trimSpace(string(data))
	return State{
		Present: true,
		Empty:   len(trimmed) == 0,
		Valid:   Valid(trimmed),
	}, nil
}

// readPrivileged reads a root-owned file by mounting its directory into a
// container.
func readPrivileged(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	dir, file := splitPath(path)
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", dir+":/secret:ro", readImage, "cat", "/secret/"+file)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("privileged read of %s: %w", path, err)
	}
	return out, nil
}

func splitPath(path string) (dir, file string) {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i], path[i+1:]
		}
	}
	return ".", path
}
