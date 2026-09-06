package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// NetworkName is the Docker network created by the Smartnode stack.
const NetworkName = "rocketpool_net"

// HelperImage is the image used for the helper container.
const HelperImage = "curlimages/curl:latest"

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
		return nil, fmt.Errorf("helper curl failed: %w (stderr: %s)", err, stderr.String())
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
