package remote

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitPush pushes the current local git repo to the VM.
// It sets up the remote repo on first push and adds/updates the git remote locally.
func GitPush(configPath, name, remotePath string) error {
	if remotePath == "" {
		remotePath = "~/project"
	}

	sshCmd := fmt.Sprintf("ssh -F '%s'", configPath)
	remoteName := "gjoll-" + name

	// Initialize repo on VM (idempotent)
	initCmd := exec.Command("ssh", "-F", configPath, name,
		fmt.Sprintf("mkdir -p %s && cd %s && git init && git config receive.denyCurrentBranch updateInstead",
			remotePath, remotePath))
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		return fmt.Errorf("initializing remote repo: %w", err)
	}

	remoteURL := fmt.Sprintf("%s:%s", name, remotePath)

	// Add or update git remote
	if remoteExists(remoteName) {
		cmd := exec.Command("git", "remote", "set-url", remoteName, remoteURL)
		cmd.Run() // ignore errors, best effort
	} else {
		cmd := exec.Command("git", "remote", "add", remoteName, remoteURL)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("adding git remote: %w", err)
		}
	}

	// Push current branch
	pushCmd := exec.Command("git", "push", remoteName, "HEAD")
	pushCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// GitPull fetches from the VM remote and creates a local branch.
func GitPull(configPath, name, branchName string) error {
	if branchName == "" {
		branchName = "gjoll/" + name
	}

	sshCmd := fmt.Sprintf("ssh -F '%s'", configPath)
	remoteName := "gjoll-" + name

	if !remoteExists(remoteName) {
		return fmt.Errorf("git remote %q not found — run 'gjoll push' first", remoteName)
	}

	// Fetch
	fetchCmd := exec.Command("git", "fetch", remoteName)
	fetchCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	if err := fetchCmd.Run(); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	// Create branch from fetched HEAD
	// First, find the default branch on the remote
	refCmd := exec.Command("git", "remote", "show", remoteName)
	refCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
	refOutput, err := refCmd.Output()
	if err != nil {
		// Fallback: try common branch names
		for _, ref := range []string{"main", "master"} {
			remoteBranch := remoteName + "/" + ref
			cmd := exec.Command("git", "rev-parse", "--verify", remoteBranch)
			if cmd.Run() == nil {
				checkoutCmd := exec.Command("git", "checkout", "-B", branchName, remoteBranch)
				checkoutCmd.Stdout = os.Stdout
				checkoutCmd.Stderr = os.Stderr
				return checkoutCmd.Run()
			}
		}
		return fmt.Errorf("could not determine remote branch: %w", err)
	}

	// Parse HEAD branch from `git remote show` output
	for _, line := range strings.Split(string(refOutput), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HEAD branch:") {
			branch := strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:"))
			remoteBranch := remoteName + "/" + branch
			checkoutCmd := exec.Command("git", "checkout", "-B", branchName, remoteBranch)
			checkoutCmd.Stdout = os.Stdout
			checkoutCmd.Stderr = os.Stderr
			return checkoutCmd.Run()
		}
	}

	return fmt.Errorf("could not determine HEAD branch on remote %s", remoteName)
}

func remoteExists(name string) bool {
	cmd := exec.Command("git", "remote", "get-url", name)
	return cmd.Run() == nil
}
