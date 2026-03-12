package remote

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
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
    IdentitiesOnly yes
    IdentityAgent none
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
// If command is non-empty, it is passed as extra arguments to ssh.
// When a command is given, it is run as a subprocess and its exit status is
// returned. Without a command, the current process is replaced via exec(2)
// for a fully interactive session.
func Connect(configPath, name string, command ...string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found: %w", err)
	}

	argv := []string{"ssh", "-F", configPath, name}
	argv = append(argv, command...)

	if len(command) == 0 {
		return syscallExec(sshPath, argv, os.Environ())
	}

	cmd := exec.Command(sshPath, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WaitForSSH polls until SSH is reachable or timeout expires.
// The ip is used for a fast TCP probe; the actual SSH connection uses the
// config file which already contains all connection parameters.
func WaitForSSH(configPath, name, ip string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "22"), 5*time.Second)
		if err == nil {
			_ = conn.Close()
			// TCP is open, try actual SSH with a hard timeout.
			// BatchMode prevents hanging on password prompts when key
			// auth fails (e.g. cloud-init hasn't injected the key yet).
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, "ssh",
				"-F", configPath,
				"-o", "ConnectTimeout=5",
				"-o", "BatchMode=yes",
				name,
				"true",
			)
			err := cmd.Run()
			cancel()
			if err == nil {
				return nil
			}
		}
		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("SSH not reachable at %s after %s", ip, timeout)
}

// RunScript uploads and executes a script on the remote host.
func RunScript(configPath, name, content string) error {
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

	// Upload
	scp := exec.Command("scp", "-F", configPath, tmpFile.Name(), name+":/tmp/gjoll-init.sh")
	scp.Stdout = os.Stdout
	scp.Stderr = os.Stderr
	if err := scp.Run(); err != nil {
		return fmt.Errorf("uploading script: %w", err)
	}

	// Execute
	ssh := exec.Command("ssh", "-F", configPath, name, "chmod +x /tmp/gjoll-init.sh && /tmp/gjoll-init.sh")
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

// CopyFile copies a local file to the remote VM, preserving permissions.
// The localPath is expanded on the local machine; the remotePath is passed
// as-is so that ~ resolves on the remote side.
func CopyFile(configPath, name, localPath, remotePath string) error {
	local, err := ExpandTilde(localPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(local); err != nil {
		return fmt.Errorf("local file %s: %w", localPath, err)
	}

	// Ensure remote directory exists.
	remoteDir := path.Dir(remotePath)
	mkdir := exec.Command("ssh", "-F", configPath, name, "mkdir -p "+remoteDir)
	mkdir.Stdout = os.Stdout
	mkdir.Stderr = os.Stderr
	if err := mkdir.Run(); err != nil {
		return fmt.Errorf("creating remote directory %s: %w", remoteDir, err)
	}

	// Copy file with preserved permissions.
	scp := exec.Command("scp", "-p", "-F", configPath, local, name+":"+remotePath)
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
