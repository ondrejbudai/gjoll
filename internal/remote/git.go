package remote

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ParseRefspec splits a refspec string on the first ":" into remote and local
// branch names. An empty string for either side means "use default".
//
// Examples:
//
//	""              → ("", "")
//	"feature"       → ("feature", "")
//	"feature:local" → ("feature", "local")
//	":local"        → ("", "local")
func ParseRefspec(arg string) (remoteBranch, localBranch string) {
	if arg == "" {
		return "", ""
	}
	if i := strings.IndexByte(arg, ':'); i >= 0 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}

// ensureRemote adds the named git remote if it doesn't exist, or updates its
// URL if it does.
func ensureRemote(remoteName, remoteURL string) error {
	if remoteExists(remoteName) {
		cmd := exec.Command("git", "remote", "set-url", remoteName, remoteURL)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: updating git remote %s: %v\n", remoteName, err)
		}
	} else {
		cmd := exec.Command("git", "remote", "add", remoteName, remoteURL)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("adding git remote: %w", err)
		}
	}
	return nil
}

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
	if err := ensureRemote(remoteName, remoteURL); err != nil {
		return err
	}

	// Push current branch
	pushCmd := exec.Command("git", "push", remoteName, "HEAD")
	pushCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	// Set remote HEAD to match the pushed branch so pull can resolve it
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	if branchOutput, err := branchCmd.Output(); err == nil {
		branch := strings.TrimSpace(string(branchOutput))
		if branch != "HEAD" {
			headCmd := exec.Command("ssh", "-F", configPath, name,
				fmt.Sprintf("cd %s && git symbolic-ref HEAD refs/heads/%s", remotePath, branch))
			if err := headCmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: setting remote HEAD to %s: %v\n", branch, err)
			}
		}
	}

	return nil
}

// GitPull fetches from the VM remote and creates a local branch.
//
// remotePath is the path to the repo on the VM (used to set up the remote if it
// doesn't exist yet). remoteBranch selects which branch to fetch; when empty the
// remote HEAD is detected automatically with a fallback to main/master.
// localBranch is the local branch name to create; when empty it defaults to
// "gjoll/<name>".
func GitPull(configPath, name, remotePath, remoteBranch, localBranch string) error {
	if remotePath == "" {
		remotePath = "~/project"
	}
	if localBranch == "" {
		localBranch = "gjoll/" + name
	}

	sshCmd := fmt.Sprintf("ssh -F '%s'", configPath)
	remoteName := "gjoll-" + name
	remoteURL := fmt.Sprintf("%s:%s", name, remotePath)

	if err := ensureRemote(remoteName, remoteURL); err != nil {
		return err
	}

	// Fetch
	fetchCmd := exec.Command("git", "fetch", remoteName)
	fetchCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	if err := fetchCmd.Run(); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	// Determine which remote branch to check out.
	remoteRef := ""
	if remoteBranch != "" {
		// Explicit branch — use it directly.
		remoteRef = remoteName + "/" + remoteBranch
	} else {
		// Try resolving HEAD from the remote first, then fall back to common names.
		refCmd := exec.Command("git", "remote", "show", remoteName)
		refCmd.Env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCmd)
		if refOutput, err := refCmd.Output(); err == nil {
			for _, line := range strings.Split(string(refOutput), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "HEAD branch:") {
					branch := strings.TrimSpace(strings.TrimPrefix(line, "HEAD branch:"))
					if branch != "(unknown)" {
						remoteRef = remoteName + "/" + branch
					}
					break
				}
			}
		}

		// Fallback: try common branch names
		if remoteRef == "" {
			for _, ref := range []string{"main", "master"} {
				candidate := remoteName + "/" + ref
				cmd := exec.Command("git", "rev-parse", "--verify", candidate)
				if cmd.Run() == nil {
					remoteRef = candidate
					break
				}
			}
		}
	}

	if remoteRef == "" {
		return fmt.Errorf("could not determine branch on remote %s", remoteName)
	}

	checkoutCmd := exec.Command("git", "checkout", "-B", localBranch, remoteRef)
	checkoutCmd.Stdout = os.Stdout
	checkoutCmd.Stderr = os.Stderr
	return checkoutCmd.Run()
}

func remoteExists(name string) bool {
	cmd := exec.Command("git", "remote", "get-url", name)
	return cmd.Run() == nil
}
