package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// NetworkName is the Docker network created by the Smartnode stack.
const NetworkName = "rocketpool_net"

// HelperImage is the image used for the helper container.
const HelperImage = "curlimages/curl:latest"

// ProbeError explains why a probe did not reach a service.
//
// Probes run while the stack is still coming up, so most failures are ordinary
// startup states rather than defects. Reason says which, in the terms a reader
// needs, instead of surfacing a curl exit status they would have to look up.
type ProbeError struct {
	Reason   string
	Starting bool
	ExitCode int
}

func (e *ProbeError) Error() string { return e.Reason }

// Starting reports whether err describes a service that has not finished
// starting, as opposed to one that is misbehaving.
func Starting(err error) bool {
	var pe *ProbeError
	return errors.As(err, &pe) && pe.Starting
}

// explainCurl translates a curl exit status into what it means for a service
// that may still be starting.
//
// These are the codes that actually occur here: a client opens its HTTP port
// only once it has finished initialising, so a refused connection is the normal
// state for the first minute of a run rather than a fault.
func explainCurl(code int, stderr string) *ProbeError {
	e := &ProbeError{ExitCode: code}
	switch code {
	case 6:
		// The name is served by the daemon's embedded DNS, which only carries
		// containers currently attached to the network. A name that does not
		// resolve therefore means the container is not registered yet — it may
		// be starting, restarting, or absent. It does not mean the network is
		// missing, and claiming so sends a reader to check the wrong thing.
		e.Reason = "container is not registered on the stack network yet; it may still be starting or restarting"
		e.Starting = true
	case 7:
		e.Reason = "connection refused; the service has not opened its port yet"
		e.Starting = true
	case 28:
		e.Reason = "request timed out"
		e.Starting = true
	case 52:
		e.Reason = "the service accepted the connection but sent no reply"
		e.Starting = true
	case 56:
		e.Reason = "the connection was reset while reading the reply"
		e.Starting = true
	default:
		e.Reason = fmt.Sprintf("probe failed (curl exit %d)", code)
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		e.Reason += ": " + trimmed
	}
	return e
}

// RunHelper executes a curl command inside the stack network.
func RunHelper(ctx context.Context, args ...string) ([]byte, error) {
	cmdArgs := append([]string{
		"run", "--rm", "--network", NetworkName, HelperImage, "-s",
	}, args...)

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, explainCurl(exitErr.ExitCode(), stderr.String())
		}
		return nil, fmt.Errorf("could not run probe helper: %w", err)
	}

	return stdout.Bytes(), nil
}

// PostJSONRPC executes an HTTP POST with JSON body to a target container.
func PostJSONRPC(ctx context.Context, targetContainer string, port int, body string) ([]byte, error) {
	url := fmt.Sprintf("http://%s:%d", targetContainer, port)
	return RunHelper(ctx, "-X", "POST", "-H", "Content-Type: application/json", "-d", body, url)
}

// GetHTTP executes an HTTP GET to a target container.
func GetHTTP(ctx context.Context, targetContainer string, port int, path string) ([]byte, error) {
	url := fmt.Sprintf("http://%s:%d%s", targetContainer, port, path)
	return RunHelper(ctx, url)
}
