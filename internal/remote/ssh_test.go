package remote

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ssh_config")

	err := WriteConfig(configPath, "mybox", "1.2.3.4", "fedora", "/path/to/key")
	if err != nil {
		t.Fatalf("WriteConfig() error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	content := string(data)
	checks := []string{
		"Host mybox",
		"HostName 1.2.3.4",
		"User fedora",
		`IdentityFile "/path/to/key"`,
		"StrictHostKeyChecking no",
		"UserKnownHostsFile /dev/null",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("SSH config missing %q", check)
		}
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

func TestCopySecretMissingFile(t *testing.T) {
	err := CopySecret("1.2.3.4", "user", "/fake/key", "/nonexistent/file", "/remote/dest")
	if err == nil {
		t.Fatal("CopySecret() expected error for non-existent local file")
	}
	if !strings.Contains(err.Error(), "local file") {
		t.Errorf("error = %q, want it to mention 'local file'", err.Error())
	}
}

func TestCopySecretDoesNotExpandRemoteTilde(t *testing.T) {
	localFile := filepath.Join(t.TempDir(), "creds.json")
	if err := os.WriteFile(localFile, []byte(`{}`), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Capture all commands executed by CopySecret.
	var commands [][]string
	original := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		commands = append(commands, append([]string{name}, args...))
		return exec.Command("true") // no-op
	}
	t.Cleanup(func() { execCommand = original })

	remotePath := "~/.config/gcloud/application_default_credentials.json"
	err := CopySecret("1.2.3.4", "fedora", "/fake/key", localFile, remotePath)
	if err != nil {
		t.Fatalf("CopySecret() unexpected error: %v", err)
	}

	if len(commands) != 2 {
		t.Fatalf("expected 2 commands (mkdir + scp), got %d", len(commands))
	}

	home, _ := os.UserHomeDir()

	// The mkdir command should contain the unexpanded ~ path.
	mkdirCmd := strings.Join(commands[0], " ")
	if !strings.Contains(mkdirCmd, "~/.config/gcloud") {
		t.Errorf("mkdir command = %q, want it to contain unexpanded ~", mkdirCmd)
	}
	if strings.Contains(mkdirCmd, home) {
		t.Errorf("mkdir command = %q, should not contain local home %q", mkdirCmd, home)
	}

	// The scp command should contain the unexpanded ~ path in the target.
	scpCmd := strings.Join(commands[1], " ")
	if !strings.Contains(scpCmd, ":"+remotePath) {
		t.Errorf("scp command = %q, want it to contain %q", scpCmd, ":"+remotePath)
	}
	if strings.Contains(scpCmd, home) {
		t.Errorf("scp command = %q, should not contain local home %q", scpCmd, home)
	}
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
