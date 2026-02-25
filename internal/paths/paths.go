package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataDir returns the base data directory for gjoll.
// If $XDG_DATA_HOME is set, uses $XDG_DATA_HOME/gjoll on all platforms.
// Otherwise: Linux: ~/.local/share/gjoll, macOS: ~/Library/Application Support/gjoll
func DataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gjoll"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "gjoll"), nil
	}

	return filepath.Join(home, ".local", "share", "gjoll"), nil
}

// InstanceDir returns the data directory for a specific sandbox instance.
func InstanceDir(name string) (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "instances", name), nil
}

// TerraformDir returns the OpenTofu workspace directory for a sandbox instance.
func TerraformDir(name string) (string, error) {
	instanceDir, err := InstanceDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(instanceDir, "terraform"), nil
}
