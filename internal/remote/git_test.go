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

func TestEnsureRemote(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		wantURL string
	}{
		{
			name:    "adds remote when missing",
			setup:   func(t *testing.T) {},
			wantURL: "ssh://example.com/new",
		},
		{
			name: "updates URL when remote exists",
			setup: func(t *testing.T) {
				t.Helper()
				if err := exec.Command("git", "remote", "add", "gjoll-test", "ssh://example.com/old").Run(); err != nil {
					t.Fatalf("git remote add: %v", err)
				}
			},
			wantURL: "ssh://example.com/updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chdir(dir); err != nil {
				t.Fatalf("Chdir() error: %v", err)
			}
			if err := exec.Command("git", "init").Run(); err != nil {
				t.Fatalf("git init error: %v", err)
			}

			tt.setup(t)

			if err := ensureRemote("gjoll-test", tt.wantURL); err != nil {
				t.Fatalf("ensureRemote() error: %v", err)
			}

			out, err := exec.Command("git", "remote", "get-url", "gjoll-test").Output()
			if err != nil {
				t.Fatalf("git remote get-url: %v", err)
			}
			got := string(out[:len(out)-1]) // trim trailing newline
			if got != tt.wantURL {
				t.Errorf("remote URL = %q, want %q", got, tt.wantURL)
			}
		})
	}
}
