package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Sample represents a single observation of a container's state.
type Sample struct {
	ContainerName string
	IsRunning     bool
	RestartCount  int
	ObservedAt    time.Time
}

// GetContainerState returns the current state of a given container.
func GetContainerState(ctx context.Context, containerName string) (Sample, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format={{json .State}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return Sample{}, fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}

	var state struct {
		Running      bool `json:"Running"`
		Restarting   bool `json:"Restarting"`
		RestartCount int  `json:"RestartCount"`
	}

	if err := json.Unmarshal(out, &state); err != nil {
		return Sample{}, fmt.Errorf("failed to parse docker inspect output: %w", err)
	}

	return Sample{
		ContainerName: containerName,
		IsRunning:     state.Running,
		RestartCount:  state.RestartCount,
		ObservedAt:    time.Now(),
	}, nil
}

// GetStackStates samples every container in a compose project.
//
// Containers are found by the compose project label rather than by a name
// prefix, so a container that happens to share the prefix but belongs to another
// stack cannot influence a verdict.
func GetStackStates(ctx context.Context, projectName string) (map[string]Sample, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "label=com.docker.compose.project="+projectName, "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers for project %s: %w", projectName, err)
	}

	names := strings.Split(strings.TrimSpace(string(out)), "\n")
	res := make(map[string]Sample)
	for _, name := range names {
		if name == "" {
			continue
		}
		sample, err := GetContainerState(ctx, name)
		if err != nil {
			return nil, err
		}
		res[name] = sample
	}
	return res, nil
}
