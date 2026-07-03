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

	"github.com/ondrejbudai/gjoll/internal/config"
	"github.com/ondrejbudai/gjoll/internal/paths"
	"github.com/ondrejbudai/gjoll/internal/remote"
	"github.com/ondrejbudai/gjoll/internal/state"
)

const injectedTF = `variable "gjoll_ssh_pubkey" {
  type        = string
  description = "SSH public key injected by gjoll"
}

variable "gjoll_name" {
  type        = string
  description = "Sandbox name injected by gjoll"
}

variable "gjoll_instance_state" {
  type        = string
  description = "Desired instance state: running or stopped"
  default     = "running"
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
	if err := writeTFVars(tfDir, strings.TrimSpace(pubKey), name, "running"); err != nil {
		return err
	}

	localPath, err := setupBaseImageCache(tfDir)
	if err != nil {
		return err
	}
	if localPath != "" {
		if err := setTFVar(tfDir, "base_image_local_path", localPath); err != nil {
			return err
		}
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
		Proxies:    outputs.Proxies,
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
		if err := remote.WaitForSSH(sshConfig, name, outputs.PublicIP, 5*time.Minute); err != nil {
			return err
		}

		fmt.Println("Running init script...")
		if err := remote.RunScript(sshConfig, name, outputs.InitScript); err != nil {
			return fmt.Errorf("init script: %w", err)
		}
	} else {
		fmt.Println("Waiting for SSH...")
		if err := remote.WaitForSSH(sshConfig, name, outputs.PublicIP, 5*time.Minute); err != nil {
			fmt.Printf("Warning: SSH not yet reachable: %v\n", err)
		}
	}

	// Copy files if defined
	if len(outputs.CopyFiles) > 0 {
		fmt.Println("Copying files...")
		for _, f := range outputs.CopyFiles {
			fmt.Printf("  %s → %s\n", f.From, f.To)
			if err := remote.CopyFile(sshConfig, name, f.From, f.To); err != nil {
				return fmt.Errorf("copy file %s: %w", f.From, err)
			}
		}
	}

	fmt.Printf("\nSandbox %q ready!\n", name)
	fmt.Printf("  IP:   %s\n", outputs.PublicIP)
	fmt.Printf("  User: %s\n", outputs.SSHUser)
	fmt.Printf("  SSH:  gjoll ssh %s\n", name)

	return nil
}

// Stop stops a running sandbox by setting gjoll_instance_state to "stopped".
func Stop(name string) error {
	tfDir, err := paths.TerraformDir(name)
	if err != nil {
		return err
	}

	inst, err := state.Load(name)
	if err != nil {
		return err
	}

	if inst.Status == "stopped" {
		return fmt.Errorf("sandbox %q is already stopped", name)
	}

	if err := updateTFVarsState(tfDir, "stopped"); err != nil {
		return err
	}

	if err := ensureBaseImageInTFVars(tfDir); err != nil {
		return err
	}

	fmt.Println("Stopping instance...")
	if err := runTofu(tfDir, "apply", "-auto-approve"); err != nil {
		return fmt.Errorf("tofu apply: %w", err)
	}

	inst.Status = "stopped"
	if err := state.Save(inst); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Printf("Sandbox %q stopped.\n", name)
	return nil
}

// Start starts a stopped sandbox by setting gjoll_instance_state to "running".
// The IP address may change after start, so the SSH config is rewritten.
func Start(name string) error {
	instanceDir, err := paths.InstanceDir(name)
	if err != nil {
		return err
	}
	tfDir, err := paths.TerraformDir(name)
	if err != nil {
		return err
	}

	inst, err := state.Load(name)
	if err != nil {
		return err
	}

	if inst.Status == "running" {
		return fmt.Errorf("sandbox %q is already running", name)
	}

	if err := updateTFVarsState(tfDir, "running"); err != nil {
		return err
	}

	if err := ensureBaseImageInTFVars(tfDir); err != nil {
		return err
	}

	fmt.Println("Starting instance...")
	if err := applyWithRetry(tfDir); err != nil {
		return fmt.Errorf("tofu apply: %w", err)
	}

	// Refresh state to pick up new IP. Some providers (e.g. AWS) use a
	// separate resource for instance state, so the main instance resource
	// isn't refreshed during apply and its public_ip stays stale.
	if err := runTofu(tfDir, "apply", "-refresh-only", "-auto-approve"); err != nil {
		return fmt.Errorf("tofu refresh: %w", err)
	}

	// Re-read outputs to get new IP
	outputs, err := readOutputs(tfDir)
	if err != nil {
		return fmt.Errorf("reading outputs: %w", err)
	}

	inst.PublicIP = outputs.PublicIP
	inst.Status = "running"
	if err := state.Save(inst); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Rewrite SSH config with new IP
	keyPath := filepath.Join(instanceDir, "id_ed25519")
	sshConfig := remote.SSHConfigPath(instanceDir)
	if err := remote.WriteConfig(sshConfig, name, outputs.PublicIP, outputs.SSHUser, keyPath); err != nil {
		return fmt.Errorf("writing SSH config: %w", err)
	}

	fmt.Println("Waiting for SSH...")
	if err := remote.WaitForSSH(sshConfig, name, outputs.PublicIP, 5*time.Minute); err != nil {
		fmt.Printf("Warning: SSH not yet reachable: %v\n", err)
	}

	fmt.Printf("Sandbox %q started.\n", name)
	fmt.Printf("  IP:   %s\n", outputs.PublicIP)
	return nil
}

// Destroy tears down a sandbox and removes all local state.
func Destroy(name string) error {
	tfDir, err := paths.TerraformDir(name)
	if err != nil {
		return err
	}

	// Refresh .tf files from the original env path so template fixes apply to
	// sandboxes provisioned before the env file was updated.
	if inst, err := state.Load(name); err == nil && inst.EnvPath != "" {
		if err := copyTFFiles(inst.EnvPath, tfDir); err != nil {
			return fmt.Errorf("refreshing tf files: %w", err)
		}
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

// writeTFVars writes the terraform.tfvars.json file with all gjoll variables.
func writeTFVars(tfDir, pubKey, name, instanceState string) error {
	tfvars := map[string]string{
		"gjoll_ssh_pubkey":     pubKey,
		"gjoll_name":           name,
		"gjoll_instance_state": instanceState,
	}
	tfvarsJSON, err := json.MarshalIndent(tfvars, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tfDir, "terraform.tfvars.json"), tfvarsJSON, 0644); err != nil {
		return fmt.Errorf("writing tfvars: %w", err)
	}
	return nil
}

// setTFVar sets a single key in terraform.tfvars.json, preserving other fields.
func setTFVar(tfDir, key, value string) error {
	tfvarsPath := filepath.Join(tfDir, "terraform.tfvars.json")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		return fmt.Errorf("reading tfvars: %w", err)
	}

	var tfvars map[string]string
	if err := json.Unmarshal(data, &tfvars); err != nil {
		return fmt.Errorf("parsing tfvars: %w", err)
	}

	tfvars[key] = value

	out, err := json.MarshalIndent(tfvars, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tfvarsPath, out, 0644); err != nil {
		return fmt.Errorf("writing tfvars: %w", err)
	}
	return nil
}

// ensureBaseImageInTFVars restores base_image_local_path from tfvars or the
// image cache so stop/start apply does not drift libvirt_volume.base.
func ensureBaseImageInTFVars(tfDir string) error {
	tfvarsPath := filepath.Join(tfDir, "terraform.tfvars.json")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		return fmt.Errorf("reading tfvars: %w", err)
	}

	var tfvars map[string]string
	if err := json.Unmarshal(data, &tfvars); err != nil {
		return fmt.Errorf("parsing tfvars: %w", err)
	}

	if path := tfvars["base_image_local_path"]; path != "" {
		return os.Setenv("TF_VAR_base_image_local_path", path)
	}

	localPath, err := setupBaseImageCache(tfDir)
	if err != nil {
		return err
	}
	if localPath == "" {
		return nil
	}

	if err := setTFVar(tfDir, "base_image_local_path", localPath); err != nil {
		return err
	}
	return os.Setenv("TF_VAR_base_image_local_path", localPath)
}

// updateTFVarsState reads the existing tfvars file and updates only the
// gjoll_instance_state field.
func updateTFVarsState(tfDir, instanceState string) error {
	tfvarsPath := filepath.Join(tfDir, "terraform.tfvars.json")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		return fmt.Errorf("reading tfvars: %w", err)
	}

	var tfvars map[string]string
	if err := json.Unmarshal(data, &tfvars); err != nil {
		return fmt.Errorf("parsing tfvars: %w", err)
	}

	tfvars["gjoll_instance_state"] = instanceState

	out, err := json.MarshalIndent(tfvars, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tfvarsPath, out, 0644); err != nil {
		return fmt.Errorf("writing tfvars: %w", err)
	}
	return nil
}

// applyWithRetry runs tofu apply, and on failure refreshes state and retries
// once. Some providers (e.g. libvirt) produce inconsistent resource IDs when
// instances restart; a refresh resolves the stale state.
func applyWithRetry(tfDir string) error {
	if err := runTofu(tfDir, "apply", "-auto-approve"); err != nil {
		fmt.Println("Apply failed, refreshing state and retrying...")
		if rerr := runTofu(tfDir, "apply", "-refresh-only", "-auto-approve"); rerr != nil {
			return fmt.Errorf("refresh failed: %w (original error: %v)", rerr, err)
		}
		if rerr := runTofu(tfDir, "apply", "-auto-approve"); rerr != nil {
			return rerr
		}
	}
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

func copyFile(src, dest string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(out, in)
	return err
}
