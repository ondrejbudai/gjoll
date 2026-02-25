package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gjoll-state-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	os.Setenv("XDG_DATA_HOME", dir)
	os.Exit(m.Run())
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	return dir
}

func TestSaveAndLoad(t *testing.T) {
	dir := setupTestDir(t)

	inst := &Instance{
		Name:       "test-box",
		EnvPath:    "/some/path/env.tf",
		PublicIP:   "1.2.3.4",
		InstanceID: "i-abc123",
		SSHUser:    "fedora",
		Status:     "running",
		CreatedAt:  time.Now().Truncate(time.Second),
	}

	instDir := filepath.Join(dir, "gjoll", "instances", "test-box")
	os.MkdirAll(instDir, 0755)

	if err := Save(inst); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load("test-box")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Name != inst.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, inst.Name)
	}
	if loaded.PublicIP != inst.PublicIP {
		t.Errorf("PublicIP = %q, want %q", loaded.PublicIP, inst.PublicIP)
	}
	if loaded.SSHUser != inst.SSHUser {
		t.Errorf("SSHUser = %q, want %q", loaded.SSHUser, inst.SSHUser)
	}
	if loaded.Status != inst.Status {
		t.Errorf("Status = %q, want %q", loaded.Status, inst.Status)
	}
}

func TestLoadNotFound(t *testing.T) {
	setupTestDir(t)

	_, err := Load("nonexistent")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent instance")
	}
}

func TestListEmpty(t *testing.T) {
	setupTestDir(t)

	instances, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(instances) != 0 {
		t.Errorf("List() returned %d instances, want 0", len(instances))
	}
}

func TestListWithInstances(t *testing.T) {
	dir := setupTestDir(t)

	// Create two instances
	for _, name := range []string{"box-a", "box-b"} {
		instDir := filepath.Join(dir, "gjoll", "instances", name)
		os.MkdirAll(instDir, 0755)
		inst := &Instance{
			Name:       name,
			PublicIP:   "1.2.3.4",
			InstanceID: "i-123",
			SSHUser:    "fedora",
			Status:     "running",
			CreatedAt:  time.Now(),
		}
		if err := Save(inst); err != nil {
			t.Fatalf("Save(%s) error: %v", name, err)
		}
	}

	instances, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(instances) != 2 {
		t.Errorf("List() returned %d instances, want 2", len(instances))
	}
}

func TestLockUnlock(t *testing.T) {
	setupTestDir(t)

	f, err := Lock("test-lock")
	if err != nil {
		t.Fatalf("Lock() error: %v", err)
	}
	Unlock(f)
}

func TestDelete(t *testing.T) {
	dir := setupTestDir(t)

	instDir := filepath.Join(dir, "gjoll", "instances", "to-delete")
	os.MkdirAll(instDir, 0755)

	inst := &Instance{Name: "to-delete", PublicIP: "1.2.3.4", InstanceID: "i-123", SSHUser: "fedora", Status: "running", CreatedAt: time.Now()}
	Save(inst)

	if err := Delete("to-delete"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if _, err := os.Stat(instDir); !os.IsNotExist(err) {
		t.Error("instance directory still exists after Delete()")
	}
}
