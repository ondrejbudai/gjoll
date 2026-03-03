package remote

import (
	"os"
	"os/exec"
	"testing"
)

func TestRemoteExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("git init error: %v", err)
	}
	if err := exec.Command("git", "remote", "add", "test-remote", "ssh://example.com/repo").Run(); err != nil {
		t.Fatalf("git remote add error: %v", err)
	}

	if !remoteExists("test-remote") {
		t.Error("remoteExists() = false, want true for existing remote")
	}
	if remoteExists("nonexistent") {
		t.Error("remoteExists() = true, want false for nonexistent remote")
	}
}
