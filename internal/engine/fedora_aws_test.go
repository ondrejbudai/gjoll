package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFedoraAWSTerraformValidate(t *testing.T) {
	root := findGjollRepoRoot(t)
	src := filepath.Join(root, "examples", "fedora-aws")
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
