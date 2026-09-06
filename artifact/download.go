package artifact

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Download fetches a release asset to a file in dir, returning the local path.
// The caller is responsible for removing the file. Network errors surface as-is
// so the caller can classify them as INFRASTRUCTURE.
func Download(asset ReleaseAsset, dir string) (string, error) {
	dest := filepath.Join(dir, asset.Name)

	client := &http.Client{Timeout: 5 * 60 * 1e9} // 5 minutes for large binaries
	resp, err := client.Get(asset.BrowserDownloadURL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", asset.Name, resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", dest, err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(dest)
		return "", fmt.Errorf("write %s: %w", dest, err)
	}

	if err := f.Close(); err != nil {
		os.Remove(dest)
		return "", fmt.Errorf("close %s: %w", dest, err)
	}

	return dest, nil
}

// DownloadAssetSet fetches the binary and its detached signature into dir.
// Returns the local paths for each.
func DownloadAssetSet(assets *AssetSet, dir string) (binaryPath, sigPath string, err error) {
	binaryPath, err = Download(assets.Binary, dir)
	if err != nil {
		return "", "", err
	}

	sigPath, err = Download(assets.Signature, dir)
	if err != nil {
		os.Remove(binaryPath)
		return "", "", err
	}

	return binaryPath, sigPath, nil
}
