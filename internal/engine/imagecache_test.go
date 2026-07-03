package engine

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCachedImageHitMiss(t *testing.T) {

	body := []byte("fake-qcow2-image-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	path1, err := EnsureCachedImage(srv.URL)
	if err != nil {
		t.Fatalf("EnsureCachedImage() first call: %v", err)
	}
	if _, err := os.Stat(path1); err != nil {
		t.Fatalf("cached file missing: %v", err)
	}

	path2, err := EnsureCachedImage(srv.URL)
	if err != nil {
		t.Fatalf("EnsureCachedImage() second call: %v", err)
	}
	if path1 != path2 {
		t.Fatalf("cache paths differ: %q vs %q", path1, path2)
	}
}

func TestEnsureCachedImageLocalPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	local := filepath.Join(dir, "base.qcow2")
	if err := os.WriteFile(local, []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := EnsureCachedImage(local)
	if err != nil {
		t.Fatalf("EnsureCachedImage() local: %v", err)
	}
	if got != local {
		t.Fatalf("got %q, want %q", got, local)
	}
}

func TestParseHCLDefaultURL(t *testing.T) {
	t.Parallel()

	content := `
variable "base_image_url" {
  type    = string
  default = "https://example.com/image.qcow2"
}
`
	got := parseHCLDefaultURL(content, "base_image_url")
	want := "https://example.com/image.qcow2"
	if got != want {
		t.Fatalf("parseHCLDefaultURL() = %q, want %q", got, want)
	}
}

func TestBaseImageURLFromTF(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tf := `variable "base_image_url" {
  default = "https://example.com/from-tf.qcow2"
}`
	if err := os.WriteFile(filepath.Join(dir, "variables.tf"), []byte(tf), 0644); err != nil {
		t.Fatal(err)
	}

	url, ok := baseImageURLFromTF(dir)
	if !ok {
		t.Fatal("baseImageURLFromTF() = false, want true")
	}
	if url != "https://example.com/from-tf.qcow2" {
		t.Fatalf("url = %q", url)
	}
}

func TestBaseImageURLFromTFMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`provider "aws" {}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, ok := baseImageURLFromTF(dir); ok {
		t.Fatal("expected false for non-libvirt tf")
	}
}

func TestSetupBaseImageCacheSetsEnv(t *testing.T) {

	body := []byte("cached-image")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	tfDir := t.TempDir()
	tf := `variable "base_image_url" {
  default = "` + srv.URL + `"
}`
	if err := os.WriteFile(filepath.Join(tfDir, "variables.tf"), []byte(tf), 0644); err != nil {
		t.Fatal(err)
	}

	cacheRoot := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	t.Setenv("TF_VAR_base_image_local_path", "")

	if err := setupBaseImageCache(tfDir); err != nil {
		t.Fatalf("setupBaseImageCache(): %v", err)
	}

	localPath := os.Getenv("TF_VAR_base_image_local_path")
	if localPath == "" {
		t.Fatal("TF_VAR_base_image_local_path not set")
	}
	if !strings.HasSuffix(localPath, ".qcow2") {
		t.Fatalf("unexpected cache path %q", localPath)
	}
}
