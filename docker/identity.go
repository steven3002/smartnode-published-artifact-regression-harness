package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/rocket-pool/smartnode/rp-regress/compose"
)

// ImageIdentity holds the resolved digests and local ID for an image.
type ImageIdentity struct {
	ServiceName    string
	Tag            string
	IndexDigest    string
	PlatformDigest string
	LocalImageID   string
}

// VerifyStackImages resolves the registry digests for all images in the stack,
// retrieves the actual image ID used by each running container, and verifies
// that they match the expected digest.
func VerifyStackImages(ctx context.Context, stack *compose.Stack, arch, os string) ([]ImageIdentity, error) {
	var identities []ImageIdentity

	for serviceName, service := range stack.Services {
		if service.Image == "" {
			continue
		}

		// 1. Resolve registry digest
		digestInfo, err := ResolveDigest(ctx, service.Image, arch, os)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve digest for service %q (image %q): %w", serviceName, service.Image, err)
		}

		// 2. Get local container image ID
		containerName := service.ContainerName
		if containerName == "" {
			containerName = "rocketpool_" + serviceName
		}

		localID, err := GetContainerImageID(ctx, containerName)
		if err != nil {
			return nil, fmt.Errorf("failed to get container image ID for service %q (container %q): %w", serviceName, containerName, err)
		}

		// 3. Verify match by checking the Image's RepoDigests
		cmd := exec.CommandContext(ctx, "docker", "inspect", localID, "--format", "{{json .RepoDigests}}")
		repoDigestsBytes, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to inspect image %s: %w", localID, err)
		}

		// In containerd-enabled Docker, the LocalID itself might directly be the index digest.
		matchFound := false
		if localID == digestInfo.IndexDigest {
			matchFound = true
		} else {
			// Otherwise, check RepoDigests
			var repoDigests []string
			if err := json.Unmarshal(repoDigestsBytes, &repoDigests); err == nil {
				for _, rd := range repoDigests {
					// rd is typically "image@sha256:..."
					if len(rd) > 71 && rd[len(rd)-71:] == digestInfo.IndexDigest {
						matchFound = true
						break
					}
				}
			}
		}

		if !matchFound {
			return nil, fmt.Errorf("mismatch detected: container %q is running image ID %s which does not match resolved registry digest %s", containerName, localID, digestInfo.IndexDigest)
		}

		identities = append(identities, ImageIdentity{
			ServiceName:    serviceName,
			Tag:            service.Image,
			IndexDigest:    digestInfo.IndexDigest,
			PlatformDigest: digestInfo.PlatformDigest,
			LocalImageID:   localID,
		})
	}

	return identities, nil
}
