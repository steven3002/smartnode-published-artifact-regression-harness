package docker

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/compose"
)

func TestVerifyStackImages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Ensure alpine is pulled
	exec.CommandContext(ctx, "docker", "pull", "alpine:latest").Run()

	// Tag alpine as something else (spoofing besu)
	exec.CommandContext(ctx, "docker", "tag", "alpine:latest", "hyperledger/besu:26.8.1").Run()

	// Run fake container
	exec.CommandContext(ctx, "docker", "rm", "-f", "test_spoof").Run()
	cmd := exec.CommandContext(ctx, "docker", "run", "-d", "--name", "test_spoof", "hyperledger/besu:26.8.1", "sleep", "1000")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run fake container: %v", err)
	}
	defer exec.Command("docker", "rm", "-f", "test_spoof").Run()

	stack := &compose.Stack{
		Services: map[string]compose.Service{
			"eth1": {
				Image:         "hyperledger/besu:26.8.1",
				ContainerName: "test_spoof",
			},
		},
	}

	_, err := VerifyStackImages(ctx, stack, "amd64", "linux")
	if err == nil {
		t.Fatalf("expected mismatch error, got nil")
	}

	if !strings.Contains(err.Error(), "mismatch detected") {
		t.Errorf("expected error to contain 'mismatch detected', got %v", err)
	}

	// Now test real image
	exec.CommandContext(ctx, "docker", "rm", "-f", "test_real").Run()
	cmd2 := exec.CommandContext(ctx, "docker", "run", "-d", "--name", "test_real", "alpine:latest", "sleep", "1000")
	if err := cmd2.Run(); err != nil {
		t.Fatalf("failed to run real container: %v", err)
	}
	defer exec.Command("docker", "rm", "-f", "test_real").Run()

	stackReal := &compose.Stack{
		Services: map[string]compose.Service{
			"test": {
				Image:         "alpine:latest",
				ContainerName: "test_real",
			},
		},
	}

	identities, err := VerifyStackImages(ctx, stackReal, "amd64", "linux")
	if err != nil {
		t.Fatalf("unexpected error for real image: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("expected 1 identity, got %d", len(identities))
	}

	if identities[0].IndexDigest == "" || identities[0].PlatformDigest == "" {
		t.Errorf("missing digests: %+v", identities[0])
	}
}
