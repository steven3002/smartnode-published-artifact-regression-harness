package docker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Manifest struct {
	SchemaVersion int `json:"schemaVersion"`
	Manifests     []struct {
		Digest   string `json:"digest"`
		Platform struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

// DigestInfo holds the resolved digests for an image.
type DigestInfo struct {
	IndexDigest    string // The digest of the manifest list/index
	PlatformDigest string // The digest of the platform-specific manifest
}

// ResolveDigest unauthenticatedly resolves a docker image tag to its manifest digest for the specified os/arch.
func ResolveDigest(ctx context.Context, imageRef string, arch string, os string) (*DigestInfo, error) {
	cmd := exec.CommandContext(ctx, "docker", "buildx", "imagetools", "inspect", "--raw", imageRef)
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to inspect manifest: %v, stderr: %s", err, string(exitError.Stderr))
		}
		return nil, fmt.Errorf("failed to run docker buildx imagetools inspect: %v", err)
	}

	// Compute the index digest (sha256 of the raw output)
	hash := sha256.Sum256(output)
	indexDigest := fmt.Sprintf("sha256:%x", hash)

	var m Manifest
	if err := json.Unmarshal(output, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest output: %w", err)
	}

	info := &DigestInfo{
		IndexDigest: indexDigest,
	}

	// If it's a single manifest, it might not have the Manifests array
	if len(m.Manifests) == 0 {
		// It's a single image manifest, so the index digest is the platform digest
		info.PlatformDigest = indexDigest
		return info, nil
	}

	// It's a manifest list, find the platform-specific digest
	for _, manifest := range m.Manifests {
		if manifest.Platform.Architecture == arch && manifest.Platform.OS == os {
			info.PlatformDigest = manifest.Digest
			return info, nil
		}
	}

	return nil, fmt.Errorf("no manifest found for platform %s/%s", os, arch)
}

// GetContainerImageID returns the local Image ID (often a digest) that the running container was started with.
func GetContainerImageID(ctx context.Context, containerName string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", containerName, "--format", "{{.Image}}")
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to inspect container %s: %v, stderr: %s", containerName, err, string(exitError.Stderr))
		}
		return "", fmt.Errorf("failed to run docker inspect: %v", err)
	}
	// trim newline
	return string(output[:len(output)-1]), nil
}
