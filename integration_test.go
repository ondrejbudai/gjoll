//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	sandboxName = "fedora-libvirt"
	envFile     = "examples/fedora-libvirt.tf"
)

// gjoll runs the gjoll binary and returns combined output.
func gjoll(t *testing.T, args ...string) string {
	t.Helper()
	bin, err := filepath.Abs("gjoll")
	if err != nil {
		t.Fatalf("resolving gjoll binary path: %v", err)
	}
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gjoll %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestIntegration(t *testing.T) {
	// Build the binary
	build := exec.Command("go", "build", "-o", "gjoll", "./cmd/gjoll")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	t.Cleanup(func() { os.Remove("gjoll") })

	// Provision VM
	t.Log("Provisioning VM...")
	gjoll(t, "up", envFile)

	tornDown := false
	t.Cleanup(func() {
		if tornDown {
			return
		}
		t.Log("Tearing down VM...")
		bin, _ := filepath.Abs("gjoll")
		cmd := exec.Command(bin, "down", sandboxName)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("cleanup down failed: %v\n%s", err, out)
		}
	})

	t.Run("list", func(t *testing.T) {
		out := gjoll(t, "list")
		if !strings.Contains(out, sandboxName) {
			t.Errorf("list output does not contain %q:\n%s", sandboxName, out)
		}
	})

	t.Run("status", func(t *testing.T) {
		out := gjoll(t, "status", sandboxName)
		for _, want := range []string{"Name:", "Status:", "IP:", "User:"} {
			if !strings.Contains(out, want) {
				t.Errorf("status output missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("ssh", func(t *testing.T) {
		out := gjoll(t, "ssh", sandboxName, "uname", "-a")
		if !strings.Contains(out, "Linux") {
			t.Errorf("ssh uname output does not contain Linux:\n%s", out)
		}
	})

	t.Run("cp", func(t *testing.T) {
		content := "gjoll-integration-test-content\n"
		upload := filepath.Join(t.TempDir(), "upload.txt")
		download := filepath.Join(t.TempDir(), "download.txt")

		if err := os.WriteFile(upload, []byte(content), 0644); err != nil {
			t.Fatalf("writing upload file: %v", err)
		}

		// Upload
		gjoll(t, "cp", sandboxName, upload, ":/tmp/gjoll-test.txt")

		// Download
		gjoll(t, "cp", sandboxName, ":/tmp/gjoll-test.txt", download)

		got, err := os.ReadFile(download)
		if err != nil {
			t.Fatalf("reading download file: %v", err)
		}
		if string(got) != content {
			t.Errorf("roundtrip mismatch: got %q, want %q", got, content)
		}
	})

	t.Run("git-push-pull", func(t *testing.T) {
		// Create a temporary local git repo
		repoDir := t.TempDir()
		git := func(args ...string) string {
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repoDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
			}
			return string(out)
		}

		git("init")
		git("config", "user.email", "test@gjoll.dev")
		git("config", "user.name", "gjoll-test")

		testContent := "hello from gjoll integration test"
		if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte(testContent), 0644); err != nil {
			t.Fatal(err)
		}
		git("add", "README.md")
		git("commit", "-m", "initial commit")

		// Push to VM
		bin, _ := filepath.Abs("gjoll")
		pushCmd := exec.Command(bin, "push", sandboxName)
		pushCmd.Dir = repoDir
		if out, err := pushCmd.CombinedOutput(); err != nil {
			t.Fatalf("gjoll push failed: %v\n%s", err, out)
		}

		// Verify file landed on VM
		out := gjoll(t, "ssh", sandboxName, "cat", "~/project/README.md")
		if !strings.Contains(out, testContent) {
			t.Errorf("pushed file not found on VM, got: %q", out)
		}

		// Make a change on the VM
		gjoll(t, "ssh", sandboxName,
			"cd ~/project && git config user.email vm@test.com && git config user.name VM && echo vm-change >> README.md && git add . && git commit -m vm-commit")

		// Pull changes back
		pullCmd := exec.Command(bin, "pull", sandboxName)
		pullCmd.Dir = repoDir
		if out, err := pullCmd.CombinedOutput(); err != nil {
			t.Fatalf("gjoll pull failed: %v\n%s", err, out)
		}

		// Verify the VM change arrived locally
		showCmd := exec.Command("git", "show", "gjoll/"+sandboxName+":README.md")
		showCmd.Dir = repoDir
		showOut, err := showCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git show failed: %v\n%s", err, showOut)
		}
		if !strings.Contains(string(showOut), "vm-change") {
			t.Errorf("pulled content missing vm-change:\n%s", showOut)
		}
	})

	t.Run("down", func(t *testing.T) {
		gjoll(t, "down", sandboxName)
		tornDown = true

		// Verify sandbox is gone
		bin, _ := filepath.Abs("gjoll")
		cmd := exec.Command(bin, "status", sandboxName)
		if err := cmd.Run(); err == nil {
			t.Error("expected status to fail after down, but it succeeded")
		}
	})
}
