package remote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfig(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		checks []string
	}{
		{
			name: "IPv4",
			ip:   "1.2.3.4",
			checks: []string{
				"Host mybox",
				"HostName 1.2.3.4",
				"User fedora",
				`IdentityFile "/path/to/key"`,
				"IdentitiesOnly yes",
				"IdentityAgent none",
				"StrictHostKeyChecking no",
				"UserKnownHostsFile /dev/null",
			},
		},
		{
			name: "IPv6",
			ip:   "2a03:3b40:282:1000:be24:11ff:fed8:71bf",
			checks: []string{
				"HostName 2a03:3b40:282:1000:be24:11ff:fed8:71bf",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "ssh_config")

			err := WriteConfig(configPath, "mybox", tt.ip, "fedora", "/path/to/key")
			if err != nil {
				t.Fatalf("WriteConfig() error: %v", err)
			}

			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("reading config: %v", err)
			}

			content := string(data)
			for _, check := range tt.checks {
				if !strings.Contains(content, check) {
					t.Errorf("SSH config missing %q", check)
				}
			}
		})
	}
}

func TestSSHConfigPath(t *testing.T) {
	path := SSHConfigPath("/some/instance/dir")
	want := filepath.Join("/some/instance/dir", "ssh_config")
	if path != want {
		t.Errorf("SSHConfigPath() = %q, want %q", path, want)
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error: %v", err)
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", filepath.Join(home, "foo/bar")},
		{"~/.ssh/id_ed25519", filepath.Join(home, ".ssh/id_ed25519")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		got, err := ExpandTilde(tt.input)
		if err != nil {
			t.Errorf("ExpandTilde(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ExpandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCopyFileMissingFile(t *testing.T) {
	err := CopyFile("/fake/config", "mybox", "/nonexistent/file", "/remote/dest")
	if err == nil {
		t.Fatal("CopyFile() expected error for non-existent local file")
	}
	if !strings.Contains(err.Error(), "local file") {
		t.Errorf("error = %q, want it to mention 'local file'", err.Error())
	}
}

func TestCopyFileTildeExpansion(t *testing.T) {
	home, _ := os.UserHomeDir()
	localFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(localFile, []byte("test"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// CopyFile with a tilde local path should not contain the literal ~
	// in the expanded path. We can't easily test the full command without
	// a real SSH server, but we can verify the tilde expansion logic works
	// by testing ExpandTilde directly (covered above) and verifying that
	// a non-tilde local path passes the stat check.
	err := CopyFile("/fake/config", "mybox", localFile, "~/remote/dest")
	// Will fail on the SSH command (no server), but should NOT fail on stat
	if err != nil && strings.Contains(err.Error(), "local file") {
		t.Errorf("CopyFile() should not fail on local file stat for %q", localFile)
	}

	// Tilde local path that doesn't exist should still report the original path
	err = CopyFile("/fake/config", "mybox", "~/nonexistent/file.txt", "/remote/dest")
	if err == nil {
		t.Fatal("CopyFile() expected error for non-existent local file")
	}
	if !strings.Contains(err.Error(), "~/nonexistent/file.txt") {
		t.Errorf("error = %q, want it to mention original tilde path", err.Error())
	}
	_ = home // used indirectly through ExpandTilde
}

func TestReadPublicKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")

	// Write a fake public key
	pubContent := "ssh-ed25519 AAAA... user@host\n"
	if err := os.WriteFile(keyPath+".pub", []byte(pubContent), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := ReadPublicKey(keyPath)
	if err != nil {
		t.Fatalf("ReadPublicKey() error: %v", err)
	}
	if got != pubContent {
		t.Errorf("ReadPublicKey() = %q, want %q", got, pubContent)
	}
}
