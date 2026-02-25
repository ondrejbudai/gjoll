package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDataDir(t *testing.T) {
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join("gjoll")) {
		t.Errorf("DataDir() = %q, want suffix 'gjoll'", dir)
	}
}

func TestDataDirXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/test-xdg")
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	want := filepath.Join("/tmp/test-xdg", "gjoll")
	if dir != want {
		t.Errorf("DataDir() = %q, want %q", dir, want)
	}
}

func TestDataDirXDGDefault(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	home, _ := os.UserHomeDir()
	var want string
	if runtime.GOOS == "darwin" {
		want = filepath.Join(home, "Library", "Application Support", "gjoll")
	} else {
		want = filepath.Join(home, ".local", "share", "gjoll")
	}
	if dir != want {
		t.Errorf("DataDir() = %q, want %q", dir, want)
	}
}

func TestInstanceDir(t *testing.T) {
	dir, err := InstanceDir("mybox")
	if err != nil {
		t.Fatalf("InstanceDir() error: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join("instances", "mybox")) {
		t.Errorf("InstanceDir() = %q, want suffix 'instances/mybox'", dir)
	}
}

func TestTerraformDir(t *testing.T) {
	dir, err := TerraformDir("mybox")
	if err != nil {
		t.Fatalf("TerraformDir() error: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join("instances", "mybox", "terraform")) {
		t.Errorf("TerraformDir() = %q, want suffix 'instances/mybox/terraform'", dir)
	}
}
