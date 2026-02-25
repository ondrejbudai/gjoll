package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/obudai/gjoll/internal/config"
	"github.com/obudai/gjoll/internal/paths"
	"github.com/obudai/gjoll/internal/remote"
	"github.com/obudai/gjoll/internal/state"
)

const injectedTF = `variable "gjoll_ssh_pubkey" {
  type        = string
  description = "SSH public key injected by gjoll"
}

variable "gjoll_name" {
  type        = string
  description = "Sandbox name injected by gjoll"
}
`

// DeriveName extracts a default sandbox name from an env path.
// "examples/fedora-dev.tf" → "fedora-dev"
// "examples/fedora-dev/" → "fedora-dev"
func DeriveName(envPath string) string {
	envPath = strings.TrimRight(envPath, "/")
	base := filepath.Base(envPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Provision creates a new sandbox from an environment config.
func Provision(name, envPath string) error {
	absEnvPath, err := filepath.Abs(envPath)
	if err != nil {
		return fmt.Errorf("resolving env path: %w", err)
	}

	instanceDir, err := paths.InstanceDir(name)
	if err != nil {
		return err
	}
	tfDir, err := paths.TerraformDir(name)
	if err != nil {
		return err
	}

	// Create directories
	if err := os.MkdirAll(tfDir, 0755); err != nil {
		return fmt.Errorf("creating terraform dir: %w", err)
	}

	// Copy .tf files
	if err := copyTFFiles(absEnvPath, tfDir); err != nil {
		return fmt.Errorf("copying tf files: %w", err)
	}

	// Generate SSH keypair
	keyPath, err := remote.GenerateKeypair(instanceDir)
	if err != nil {
		return fmt.Errorf("generating SSH keypair: %w", err)
	}

	pubKey, err := remote.ReadPublicKey(keyPath)
	if err != nil {
		return err
	}

	// Write injected variables
	if err := os.WriteFile(filepath.Join(tfDir, "gjoll_injected.tf"), []byte(injectedTF), 0644); err != nil {
		return fmt.Errorf("writing injected tf: %w", err)
	}

	// Write tfvars
	tfvars := map[string]string{
		"gjoll_ssh_pubkey": strings.TrimSpace(pubKey),
		"gjoll_name":       name,
	}
	tfvarsJSON, err := json.MarshalIndent(tfvars, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tfDir, "terraform.tfvars.json"), tfvarsJSON, 0644); err != nil {
		return fmt.Errorf("writing tfvars: %w", err)
	}

	// tofu init
	fmt.Println("Initializing OpenTofu...")
	if err := runTofu(tfDir, "init"); err != nil {
		return fmt.Errorf("tofu init: %w", err)
	}

	// tofu apply
	fmt.Println("Provisioning infrastructure...")
	if err := runTofu(tfDir, "apply", "-auto-approve"); err != nil {
		return fmt.Errorf("tofu apply: %w", err)
	}

	// Read outputs
	outputs, err := readOutputs(tfDir)
	if err != nil {
		return fmt.Errorf("reading outputs: %w", err)
	}

	// Save instance state
	inst := &state.Instance{
		Name:       name,
		EnvPath:    absEnvPath,
		PublicIP:   outputs.PublicIP,
		InstanceID: outputs.InstanceID,
		SSHUser:    outputs.SSHUser,
		Status:     "running",
		CreatedAt:  time.Now(),
	}
	if err := state.Save(inst); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Write SSH config
	sshConfig := remote.SSHConfigPath(instanceDir)
	if err := remote.WriteConfig(sshConfig, name, outputs.PublicIP, outputs.SSHUser, keyPath); err != nil {
		return fmt.Errorf("writing SSH config: %w", err)
	}

	// Run init script if present
	if outputs.InitScript != "" {
		fmt.Println("Waiting for SSH...")
		if err := remote.WaitForSSH(outputs.PublicIP, outputs.SSHUser, keyPath, 5*time.Minute); err != nil {
			return err
		}

		fmt.Println("Running init script...")
		if err := remote.RunScript(outputs.PublicIP, outputs.SSHUser, keyPath, outputs.InitScript); err != nil {
			return fmt.Errorf("init script: %w", err)
		}
	} else {
		fmt.Println("Waiting for SSH...")
		if err := remote.WaitForSSH(outputs.PublicIP, outputs.SSHUser, keyPath, 5*time.Minute); err != nil {
			fmt.Printf("Warning: SSH not yet reachable: %v\n", err)
		}
	}

	fmt.Printf("\nSandbox %q ready!\n", name)
	fmt.Printf("  IP:   %s\n", outputs.PublicIP)
	fmt.Printf("  User: %s\n", outputs.SSHUser)
	fmt.Printf("  SSH:  gjoll ssh %s\n", name)

	return nil
}

// Destroy tears down a sandbox and removes all local state.
func Destroy(name string) error {
	tfDir, err := paths.TerraformDir(name)
	if err != nil {
		return err
	}

	fmt.Println("Destroying infrastructure...")
	if err := runTofu(tfDir, "destroy", "-auto-approve"); err != nil {
		return fmt.Errorf("tofu destroy: %w", err)
	}

	if err := state.Delete(name); err != nil {
		return fmt.Errorf("removing instance data: %w", err)
	}

	fmt.Printf("Sandbox %q destroyed.\n", name)
	return nil
}

// Stop stops a running instance (preserves disk).
func Stop(name string) error {
	inst, err := state.Load(name)
	if err != nil {
		return err
	}

	cmd := exec.Command("aws", "ec2", "stop-instances", "--instance-ids", inst.InstanceID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping instance: %w", err)
	}

	inst.Status = "stopped"
	if err := state.Save(inst); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Printf("Sandbox %q stopped.\n", name)
	return nil
}

// Start resumes a stopped instance.
func Start(name string) error {
	inst, err := state.Load(name)
	if err != nil {
		return err
	}

	cmd := exec.Command("aws", "ec2", "start-instances", "--instance-ids", inst.InstanceID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting instance: %w", err)
	}

	// Wait for the instance to get a new public IP
	fmt.Println("Waiting for instance to start...")
	var newIP string
	for i := 0; i < 60; i++ {
		time.Sleep(5 * time.Second)
		ip, err := getPublicIP(inst.InstanceID)
		if err == nil && ip != "" {
			newIP = ip
			break
		}
	}

	if newIP == "" {
		return fmt.Errorf("timed out waiting for public IP")
	}

	inst.PublicIP = newIP
	inst.Status = "running"
	if err := state.Save(inst); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Update SSH config
	instanceDir, _ := paths.InstanceDir(name)
	keyPath := filepath.Join(instanceDir, "id_ed25519")
	sshConfig := remote.SSHConfigPath(instanceDir)
	if err := remote.WriteConfig(sshConfig, name, newIP, inst.SSHUser, keyPath); err != nil {
		return fmt.Errorf("updating SSH config: %w", err)
	}

	fmt.Printf("Sandbox %q started.\n", name)
	fmt.Printf("  IP: %s\n", newIP)

	return nil
}

func runTofu(chdir string, args ...string) error {
	fullArgs := append([]string{"-chdir=" + chdir}, args...)
	cmd := exec.Command("tofu", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func readOutputs(tfDir string) (*config.Outputs, error) {
	cmd := exec.Command("tofu", "-chdir="+tfDir, "output", "-json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tofu output: %w", err)
	}
	return config.ParseOutputs(out)
}

func getPublicIP(instanceID string) (string, error) {
	cmd := exec.Command("aws", "ec2", "describe-instances",
		"--instance-ids", instanceID,
		"--query", "Reservations[0].Instances[0].PublicIpAddress",
		"--output", "text",
	)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(out))
	if ip == "" || ip == "None" {
		return "", fmt.Errorf("no public IP")
	}
	return ip, nil
}

func copyTFFiles(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if !info.IsDir() {
		// Single file
		return copyFile(src, filepath.Join(dest, filepath.Base(src)))
	}

	// Directory — copy all .tf files
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	copied := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}
		if err := copyFile(filepath.Join(src, entry.Name()), filepath.Join(dest, entry.Name())); err != nil {
			return err
		}
		copied++
	}

	if copied == 0 {
		return fmt.Errorf("no .tf files found in %s", src)
	}

	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
