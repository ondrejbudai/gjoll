package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/obudai/gjoll/internal/paths"
)

// Instance holds metadata about a provisioned sandbox.
type Instance struct {
	Name       string    `json:"name"`
	EnvPath    string    `json:"env_path"`
	PublicIP   string    `json:"public_ip"`
	InstanceID string    `json:"instance_id"`
	SSHUser    string    `json:"ssh_user"`
	Status     string    `json:"status"` // running, stopped, unknown
	CreatedAt  time.Time `json:"created_at"`
}

// Save writes the instance metadata to instance.json in the instance directory.
func Save(inst *Instance) error {
	dir, err := paths.InstanceDir(inst.Name)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling instance: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, "instance.json"), data, 0644)
}

// Load reads the instance metadata from instance.json.
func Load(name string) (*Instance, error) {
	dir, err := paths.InstanceDir(name)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, "instance.json"))
	if err != nil {
		return nil, fmt.Errorf("loading instance %q: %w", name, err)
	}

	var inst Instance
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, fmt.Errorf("parsing instance %q: %w", name, err)
	}

	return &inst, nil
}

// Delete removes the entire instance directory.
func Delete(name string) error {
	dir, err := paths.InstanceDir(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// List returns all saved instances.
func List() ([]*Instance, error) {
	dataDir, err := paths.DataDir()
	if err != nil {
		return nil, err
	}

	instancesDir := filepath.Join(dataDir, "instances")
	entries, err := os.ReadDir(instancesDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var instances []*Instance
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		inst, err := Load(entry.Name())
		if err != nil {
			continue // skip corrupted entries
		}
		instances = append(instances, inst)
	}

	return instances, nil
}

// Lock acquires an exclusive file lock on the instance directory.
// Returns the lock file which must be closed to release the lock.
func Lock(name string) (*os.File, error) {
	dir, err := paths.InstanceDir(name)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lockPath := filepath.Join(dir, "gjoll.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquiring lock: %w", err)
	}

	return f, nil
}

// SharedLock acquires a shared file lock on the instance directory.
func SharedLock(name string) (*os.File, error) {
	dir, err := paths.InstanceDir(name)
	if err != nil {
		return nil, err
	}

	lockPath := filepath.Join(dir, "gjoll.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		f.Close()
		return nil, fmt.Errorf("acquiring shared lock: %w", err)
	}

	return f, nil
}

// Unlock releases a file lock.
func Unlock(f *os.File) {
	if f != nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}
}
