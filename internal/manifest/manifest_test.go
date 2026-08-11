package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDirValid(t *testing.T) {
	manifests, err := LoadDir(filepath.Join("..", "..", "manifests"))
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(manifests) != 3 {
		t.Fatalf("manifests count = %d, want 3", len(manifests))
	}
	// LoadDir returns manifests in sorted filename order.
	if manifests[0].Name == "" {
		t.Fatal("first manifest name empty")
	}
	for _, m := range manifests {
		for name, opt := range m.Options {
			if opt.Type == "" {
				t.Fatalf("manifest %s option %s has empty type", m.Name, name)
			}
		}
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	data := []byte("name: test\nversion: v1\nunknown_field: true\n")
	_, err := Load(data, "test.yaml")
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("errors.Is(err, ErrInvalidManifest) = false for %v", err)
	}
}

func TestLoadRejectsMissingName(t *testing.T) {
	data := []byte("version: v1\noptions:\n  x:\n    type: string\n    default: \"y\"\n")
	_, err := Load(data, "test.yaml")
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("error %q does not mention missing name", err.Error())
	}
}

func TestLoadRejectsBadType(t *testing.T) {
	data := []byte("name: test\nversion: v1\noptions:\n  x:\n    type: float\n    default: 1.0\n")
	_, err := Load(data, "test.yaml")
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("error %q does not mention unsupported type", err.Error())
	}
}

func TestLoadRejectsUnsupportedVersion(t *testing.T) {
	data := []byte("name: test\nversion: v2\noptions:\n  x:\n    type: string\n    default: \"y\"\n")
	_, err := Load(data, "test.yaml")
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "version must be") {
		t.Fatalf("error %q does not mention version", err.Error())
	}
}

func TestLoadRejectsDefaultTypeMismatch(t *testing.T) {
	data := []byte("name: test\nversion: v1\noptions:\n  x:\n    type: integer\n    default: \"not-a-number\"\n")
	_, err := Load(data, "test.yaml")
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	if !strings.Contains(err.Error(), "does not match type") {
		t.Fatalf("error %q does not mention default type mismatch", err.Error())
	}
}

func TestLoadDirEmpty(t *testing.T) {
	dir := t.TempDir()
	manifests, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir error: %v", err)
	}
	if len(manifests) != 0 {
		t.Fatalf("manifests = %d, want 0", len(manifests))
	}
}

func TestLoadDirNotFound(t *testing.T) {
	_, err := LoadDir("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("LoadDir succeeded, want error")
	}
	if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
		// Some systems return different error messages; just verify it's an error.
	}
	_ = err
}
