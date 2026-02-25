package remote

import (
	"os"
	"os/exec"
	"testing"
)

func TestRemoteExists(t *testing.T) {
	dir := t.TempDir()
	os.Chdir(dir)
	exec.Command("git", "init").Run()
	exec.Command("git", "remote", "add", "test-remote", "ssh://example.com/repo").Run()

	if !remoteExists("test-remote") {
		t.Error("remoteExists() = false, want true for existing remote")
	}
	if remoteExists("nonexistent") {
		t.Error("remoteExists() = true, want false for nonexistent remote")
	}
}
