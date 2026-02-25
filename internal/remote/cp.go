package remote

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Copy copies files between local and remote using scp.
// Remote paths are prefixed with ":".
// Examples:
//
//	Copy(cfg, name, "./file.txt", ":/home/fedora/")  — upload
//	Copy(cfg, name, ":/home/fedora/file.txt", "./")  — download
func Copy(configPath, name, src, dest string) error {
	srcRemote := strings.HasPrefix(src, ":")
	destRemote := strings.HasPrefix(dest, ":")

	if srcRemote && destRemote {
		return fmt.Errorf("both source and destination cannot be remote")
	}
	if !srcRemote && !destRemote {
		return fmt.Errorf("one of source or destination must be remote (prefix with :)")
	}

	if srcRemote {
		src = name + ":" + strings.TrimPrefix(src, ":")
	}
	if destRemote {
		dest = name + ":" + strings.TrimPrefix(dest, ":")
	}

	cmd := exec.Command("scp", "-r", "-F", configPath, src, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scp: %w", err)
	}

	return nil
}
