package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondrejbudai/gjoll/internal/paths"
)

// DefaultFedora43CloudURL matches the default in examples/fedora-libvirt/variables.tf.
const DefaultFedora43CloudURL = "https://download.fedoraproject.org/pub/fedora/linux/releases/43/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2"

// EnsureCachedImage downloads imageURL to the gjoll image cache if missing and
// returns the local file path.
func EnsureCachedImage(imageURL string) (string, error) {
	if imageURL == "" {
		return "", fmt.Errorf("image URL is empty")
	}
	if strings.HasPrefix(imageURL, "/") || strings.HasPrefix(imageURL, "file://") {
		return strings.TrimPrefix(imageURL, "file://"), nil
	}

	cacheDir, err := paths.ImageCacheDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("creating image cache dir: %w", err)
	}

	sum := sha256.Sum256([]byte(imageURL))
	cacheName := fmt.Sprintf("image-%s.qcow2", hex.EncodeToString(sum[:8]))
	cachePath := filepath.Join(cacheDir, cacheName)

	if info, err := os.Stat(cachePath); err == nil && info.Size() > 0 {
		return cachePath, nil
	}

	tmpPath := cachePath + ".partial"
	if err := downloadFile(imageURL, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("renaming cached image: %w", err)
	}

	return cachePath, nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 0}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	return nil
}

// baseImageURLFromTF returns the default base_image_url from copied .tf files when present.
func baseImageURLFromTF(tfDir string) (string, bool) {
	entries, err := os.ReadDir(tfDir)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tfDir, entry.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, `variable "base_image_url"`) {
			continue
		}
		if url := parseHCLDefaultURL(content, "base_image_url"); url != "" {
			return url, true
		}
	}
	return "", false
}

func parseHCLDefaultURL(content, varName string) string {
	idx := strings.Index(content, `variable "`+varName+`"`)
	if idx < 0 {
		return ""
	}
	rest := content[idx:]
	defaultIdx := strings.Index(rest, "default")
	if defaultIdx < 0 {
		return DefaultFedora43CloudURL
	}
	rest = rest[defaultIdx:]
	quote := strings.Index(rest, `"`)
	if quote < 0 {
		return ""
	}
	rest = rest[quote+1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// setupBaseImageCache sets TF_VAR_base_image_local_path when the env defines base_image_url.
func setupBaseImageCache(tfDir string) error {
	imageURL, ok := baseImageURLFromTF(tfDir)
	if !ok {
		return nil
	}

	fmt.Println("Ensuring base cloud image is cached...")
	localPath, err := EnsureCachedImage(imageURL)
	if err != nil {
		return fmt.Errorf("caching base image: %w", err)
	}
	if err := os.Setenv("TF_VAR_base_image_local_path", localPath); err != nil {
		return err
	}
	return nil
}
