package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFedoraLibvirtTerraformValidate(t *testing.T) {
	root := findGjollRepoRoot(t)
	src := filepath.Join(root, "examples", "fedora-libvirt")
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("tofu not installed")
	}

	workDir := t.TempDir()
	if err := copyTFFiles(src, workDir); err != nil {
		t.Fatalf("copyTFFiles() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "gjoll_injected.tf"), []byte(injectedTF), 0644); err != nil {
		t.Fatalf("writing injected tf: %v", err)
	}

	init := exec.Command("tofu", "-chdir="+workDir, "init", "-backend=false")
	init.Stdout = os.Stdout
	init.Stderr = os.Stderr
	if err := init.Run(); err != nil {
		t.Fatalf("tofu init: %v", err)
	}

	validate := exec.Command("tofu", "-chdir="+workDir, "validate")
	validate.Stdout = os.Stdout
	validate.Stderr = os.Stderr
	if err := validate.Run(); err != nil {
		t.Fatalf("tofu validate: %v", err)
	}
}

func findGjollRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(wd, "examples", "fedora-libvirt")); err == nil {
				return wd
			}
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("gjoll repo root not found")
		}
		wd = parent
	}
}
