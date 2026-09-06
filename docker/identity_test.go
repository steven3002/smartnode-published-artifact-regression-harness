package docker

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/compose"
)

// spoofTag must be a real registry reference, so its digest can be resolved,
// while the local content behind it is something else. That is exactly the
// substitution the identity check exists to catch.
//
// It is deliberately an image no client profile uses. Retagging a real client
// image to stage this test leaves the host with that tag pointing at the wrong
// content, which then makes every later run of that profile fail an identity
// check for a reason unrelated to the release under test.
const spoofTag = "busybox:1.36"

func TestVerifyStackImages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	exec.CommandContext(ctx, "docker", "pull", "alpine:latest").Run()

	// Point a throwaway tag at content that is not what the tag names, so the
	// identity check has something to catch.
	if err := exec.CommandContext(ctx, "docker", "tag", "alpine:latest", spoofTag).Run(); err != nil {
		t.Fatalf("failed to stage spoofed tag: %v", err)
	}
	defer exec.Command("docker", "rmi", "-f", spoofTag).Run()

	exec.CommandContext(ctx, "docker", "rm", "-f", "test_spoof").Run()
	cmd := exec.CommandContext(ctx, "docker", "run", "-d", "--name", "test_spoof", spoofTag, "sleep", "1000")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to run fake container: %v", err)
	}
	defer exec.Command("docker", "rm", "-f", "test_spoof").Run()

	stack := &compose.Stack{
		Services: map[string]compose.Service{
			"eth1": {
				Image:         spoofTag,
				ContainerName: "test_spoof",
			},
		},
	}

	// A mismatch is recorded on the identity rather than returned as an error,
	// so the report can show which image diverged without discarding the rest.
	identities, err := VerifyStackImages(ctx, stack, "amd64", "linux")
	if err != nil {
		t.Fatalf("VerifyStackImages returned an error for a mismatch: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("got %d identities, want 1", len(identities))
	}
	if identities[0].Matches {
		t.Error("substituted image was reported as matching; the check cannot fail")
	}
	if !strings.Contains(identities[0].Mismatch, "resolved digest") {
		t.Errorf("mismatch was not explained: %q", identities[0].Mismatch)
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

	identities, err = VerifyStackImages(ctx, stackReal, "amd64", "linux")
	if err != nil {
		t.Fatalf("unexpected error for real image: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("expected 1 identity, got %d", len(identities))
	}
	if !identities[0].Matches {
		t.Errorf("unmodified image was reported as a mismatch: %s", identities[0].Mismatch)
	}
	if identities[0].IndexDigest == "" || identities[0].PlatformDigest == "" {
		t.Errorf("missing digests: %+v", identities[0])
	}
}
