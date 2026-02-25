package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"examples/fedora-dev.tf", "fedora-dev"},
		{"examples/fedora-dev/", "fedora-dev"},
		{"examples/fedora-dev", "fedora-dev"},
		{"/absolute/path/ubuntu-claude.tf", "ubuntu-claude"},
		{"simple.tf", "simple"},
		{"dir/nested/env/", "env"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DeriveName(tt.input)
			if got != tt.want {
				t.Errorf("DeriveName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCopyTFFilesSingle(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create a single .tf file
	tfContent := `resource "aws_instance" "test" {}`
	srcFile := filepath.Join(srcDir, "main.tf")
	os.WriteFile(srcFile, []byte(tfContent), 0644)

	if err := copyTFFiles(srcFile, destDir); err != nil {
		t.Fatalf("copyTFFiles() error: %v", err)
	}

	// Verify file was copied
	data, err := os.ReadFile(filepath.Join(destDir, "main.tf"))
	if err != nil {
		t.Fatalf("reading copied file: %v", err)
	}
	if string(data) != tfContent {
		t.Errorf("copied content = %q, want %q", string(data), tfContent)
	}
}

func TestCopyTFFilesDirectory(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create .tf files and a non-.tf file
	os.WriteFile(filepath.Join(srcDir, "main.tf"), []byte("main"), 0644)
	os.WriteFile(filepath.Join(srcDir, "vars.tf"), []byte("vars"), 0644)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("readme"), 0644)

	if err := copyTFFiles(srcDir, destDir); err != nil {
		t.Fatalf("copyTFFiles() error: %v", err)
	}

	// Verify .tf files were copied
	if _, err := os.Stat(filepath.Join(destDir, "main.tf")); err != nil {
		t.Error("main.tf not copied")
	}
	if _, err := os.Stat(filepath.Join(destDir, "vars.tf")); err != nil {
		t.Error("vars.tf not copied")
	}

	// Verify non-.tf file was NOT copied
	if _, err := os.Stat(filepath.Join(destDir, "README.md")); !os.IsNotExist(err) {
		t.Error("README.md should not be copied")
	}
}

func TestCopyTFFilesEmptyDir(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	err := copyTFFiles(srcDir, destDir)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
}
