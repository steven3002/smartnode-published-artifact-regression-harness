package artifact

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	releaseURLTemplate = "https://api.github.com/repos/rocket-pool/smartnode/releases/tags/%s"
	httpTimeout        = 30 * time.Second
)

// ReleaseAsset represents a single asset from a GitHub release.
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	// Digest is GitHub's own computation, format "sha256:<hex>".
	Digest string `json:"digest"`
}

// Release holds the metadata needed from a GitHub release.
type Release struct {
	TagName string         `json:"tag_name"`
	Assets  []ReleaseAsset `json:"assets"`
}

// AssetSet is the resolved set of URLs for verifying a single binary.
type AssetSet struct {
	Tag       string
	Binary    ReleaseAsset
	Signature ReleaseAsset
	// DigestHex is the SHA-256 hex digest from the GitHub API's asset
	// digest field. Empty if the API did not provide one.
	DigestHex string
}

// FetchRelease queries the GitHub releases API for the given tag and returns
// the parsed release metadata. Network and API errors are classified as
// INFRASTRUCTURE failures by the caller.
func FetchRelease(tag string) (*Release, error) {
	url := fmt.Sprintf(releaseURLTemplate, tag)

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch release %s: %w", tag, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("fetch release %s: HTTP %d: %s", tag, resp.StatusCode, body)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release %s: %w", tag, err)
	}
	return &rel, nil
}

// ResolveAssetSet locates the linux-amd64 CLI binary and its detached
// signature within a release. The digest comes from the GitHub API's
// asset digest field rather than a separately published file.
func ResolveAssetSet(rel *Release, binaryName string) (*AssetSet, error) {
	sigName := binaryName + ".sig"

	var binary, sig *ReleaseAsset
	for i := range rel.Assets {
		a := &rel.Assets[i]
		switch a.Name {
		case binaryName:
			binary = a
		case sigName:
			sig = a
		}
	}

	if binary == nil {
		return nil, fmt.Errorf("asset %q not found in release %s", binaryName, rel.TagName)
	}
	if sig == nil {
		return nil, fmt.Errorf("signature %q not found in release %s", sigName, rel.TagName)
	}

	var digestHex string
	if binary.Digest != "" {
		// GitHub format: "sha256:<hex>"
		if after, ok := strings.CutPrefix(binary.Digest, "sha256:"); ok {
			digestHex = after
		}
	}

	return &AssetSet{
		Tag:       rel.TagName,
		Binary:    *binary,
		Signature: *sig,
		DigestHex: digestHex,
	}, nil
}
