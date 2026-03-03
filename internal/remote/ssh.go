package remote

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GenerateKeypair creates an ed25519 SSH keypair in the given directory.
func GenerateKeypair(dir string) (string, error) {
	keyPath := filepath.Join(dir, "id_ed25519")

	if _, err := os.Stat(keyPath); err == nil {
		return keyPath, nil // already exists
	}

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ssh-keygen: %w", err)
	}

	return keyPath, nil
}

// ReadPublicKey reads the SSH public key from the given private key path.
func ReadPublicKey(keyPath string) (string, error) {
	data, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", fmt.Errorf("reading public key: %w", err)
	}
	return string(data), nil
}

// WriteConfig writes an SSH config file for a sandbox instance.
func WriteConfig(configPath, name, ip, user, keyPath string) error {
	content := fmt.Sprintf(`Host %s
    HostName %s
    User %s
    IdentityFile "%s"
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    LogLevel ERROR
`, name, ip, user, keyPath)

	return os.WriteFile(configPath, []byte(content), 0644)
}

// SSHConfigPath returns the SSH config file path for a sandbox.
func SSHConfigPath(instanceDir string) string {
	return filepath.Join(instanceDir, "ssh_config")
}

// Connect execs into an SSH session for the named sandbox.
func Connect(configPath, name string) error {
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found: %w", err)
	}

	return syscallExec(ssh, []string{"ssh", "-F", configPath, name}, os.Environ())
}

// WaitForSSH polls until SSH is reachable or timeout expires.
func WaitForSSH(ip, user, keyPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", ip+":22", 5*time.Second)
		if err == nil {
			_ = conn.Close()
			// TCP is open, try actual SSH
			cmd := exec.Command("ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				"-o", "LogLevel=ERROR",
				"-i", keyPath,
				fmt.Sprintf("%s@%s", user, ip),
				"true",
			)
			if cmd.Run() == nil {
				return nil
			}
		}
		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("SSH not reachable at %s after %s", ip, timeout)
}

// RunScript uploads and executes a script on the remote host.
func RunScript(ip, user, keyPath, content string) error {
	tmpFile, err := os.CreateTemp("", "gjoll-init-*.sh")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: removing temp file %s: %v\n", tmpFile.Name(), err)
		}
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing script: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	sshOpts := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", keyPath,
	}
	target := fmt.Sprintf("%s@%s", user, ip)

	// Upload
	scpArgs := append([]string{"scp"}, sshOpts...)
	scpArgs = append(scpArgs, tmpFile.Name(), target+":/tmp/gjoll-init.sh")
	scp := exec.Command(scpArgs[0], scpArgs[1:]...)
	scp.Stdout = os.Stdout
	scp.Stderr = os.Stderr
	if err := scp.Run(); err != nil {
		return fmt.Errorf("uploading script: %w", err)
	}

	// Execute
	sshArgs := append([]string{"ssh"}, sshOpts...)
	sshArgs = append(sshArgs, target, "chmod +x /tmp/gjoll-init.sh && /tmp/gjoll-init.sh")
	ssh := exec.Command(sshArgs[0], sshArgs[1:]...)
	ssh.Stdout = os.Stdout
	ssh.Stderr = os.Stderr
	if err := ssh.Run(); err != nil {
		return fmt.Errorf("running script: %w", err)
	}

	return nil
}

// ExpandTilde replaces a leading ~ with the user's home directory.
func ExpandTilde(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding ~: %w", err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}

// CopySecret copies a local file to the remote VM, preserving permissions.
func CopySecret(ip, user, keyPath, localPath, remotePath string) error {
	local, err := ExpandTilde(localPath)
	if err != nil {
		return err
	}
	rem, err := ExpandTilde(remotePath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(local); err != nil {
		return fmt.Errorf("local file %s: %w", localPath, err)
	}

	sshOpts := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", keyPath,
	}
	target := fmt.Sprintf("%s@%s", user, ip)

	// Ensure remote directory exists
	remoteDir := filepath.Dir(rem)
	mkdirArgs := append([]string{"ssh"}, sshOpts...)
	mkdirArgs = append(mkdirArgs, target, "mkdir -p "+remoteDir)
	mkdir := exec.Command(mkdirArgs[0], mkdirArgs[1:]...)
	mkdir.Stdout = os.Stdout
	mkdir.Stderr = os.Stderr
	if err := mkdir.Run(); err != nil {
		return fmt.Errorf("creating remote directory %s: %w", remoteDir, err)
	}

	// Copy file with preserved permissions
	scpArgs := append([]string{"scp", "-p"}, sshOpts...)
	scpArgs = append(scpArgs, local, target+":"+rem)
	scp := exec.Command(scpArgs[0], scpArgs[1:]...)
	scp.Stdout = os.Stdout
	scp.Stderr = os.Stderr
	if err := scp.Run(); err != nil {
		return fmt.Errorf("copying %s to %s: %w", localPath, remotePath, err)
	}

	return nil
}

// Rsync syncs files between local and remote using rsync.
func Rsync(configPath, name, src, dest string) error {
	cmd := exec.Command("rsync", "-avz",
		"-e", fmt.Sprintf("ssh -F '%s'", configPath),
		src,
		fmt.Sprintf("%s:%s", name, dest),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
