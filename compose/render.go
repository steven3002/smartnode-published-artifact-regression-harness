package compose

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
)

// Stack represents the parsed Docker Compose configuration.
type Stack struct {
	Services map[string]Service `yaml:"services"`
}

// Service represents a single service within the Docker Compose stack.
type Service struct {
	Image         string            `yaml:"image"`
	ContainerName string            `yaml:"container_name"`
	Command       []string          `yaml:"command"`
	Environment   map[string]string `yaml:"environment"`
}

// Render uses the CLI to generate and output the fully merged Docker Compose stack.
// It parses the YAML output into a Stack struct.
func Render(ctx context.Context, rc *runctx.RunContext, binPath string) (*Stack, error) {
	cmd := exec.Command(binPath, "service", "compose")

	result, err := rc.RunStep(ctx, "render-compose", cmd, 30*time.Second, 10*1024*1024, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to render compose stack: %w (exit code %d)\nOutput: %s", err, result.ExitCode, string(result.Output))
	}

	var stack Stack
	if err := yaml.Unmarshal(result.Output, &stack); err != nil {
		return nil, fmt.Errorf("failed to parse generated compose stack: %w", err)
	}

	return &stack, nil
}
