// Package manifest parses and validates ConfigForge manifest files. A manifest
// declares a component's configuration options once, including types,
// defaults, limits, environment-variable mappings, and descriptions.
package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest describes a single component's configuration surface.
type Manifest struct {
	Name        string            `json:"name" yaml:"name"`
	Version     string            `json:"version" yaml:"version"`
	Description string            `json:"description,omitempty" yaml:"description"`
	Options     map[string]Option `json:"options,omitempty" yaml:"options"`
}

// Option describes a single configuration option in a manifest.
type Option struct {
	Type        string   `json:"type" yaml:"type"`
	Minimum     *float64 `json:"minimum,omitempty" yaml:"minimum"`
	Maximum     *float64 `json:"maximum,omitempty" yaml:"maximum"`
	Default     any      `json:"default,omitempty" yaml:"default"`
	Env         string   `json:"env,omitempty" yaml:"env"`
	Description string   `json:"description,omitempty" yaml:"description"`
}

// ErrInvalidManifest marks manifest parsing or validation failures.
var ErrInvalidManifest = errors.New("invalid manifest")

// LoadDir loads all *.yaml manifests from dir in sorted filename order and
// returns them. All manifests must validate.
func LoadDir(dir string) ([]Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read manifests dir %s: %w", dir, err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	manifests := make([]Manifest, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		m, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// LoadFile parses and validates a single manifest file.
func LoadFile(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %s: %w", path, err)
	}
	return Load(data, path)
}

// Load parses a manifest from data. filename is used in error messages.
func Load(data []byte, filename string) (Manifest, error) {
	var m Manifest
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("%s: %w: %v", filename, ErrInvalidManifest, err)
	}
	if err := Validate(m, filename); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Validate checks a manifest for required fields and valid option types.
func Validate(m Manifest, filename string) error {
	var errs []string
	file := filename
	if file == "" {
		file = "manifest"
	}

	if strings.TrimSpace(m.Name) == "" {
		errs = append(errs, fmt.Sprintf("%s: name is required", file))
	}
	if m.Version == "" {
		errs = append(errs, fmt.Sprintf("%s: version is required", file))
	} else if m.Version != "v1" {
		errs = append(errs, fmt.Sprintf("%s: version must be %q, got %q", file, "v1", m.Version))
	}

	optionNames := make([]string, 0, len(m.Options))
	for name := range m.Options {
		optionNames = append(optionNames, name)
	}
	sort.Strings(optionNames)
	for _, name := range optionNames {
		opt := m.Options[name]
		if strings.TrimSpace(name) == "" {
			errs = append(errs, fmt.Sprintf("%s: option name must be non-empty", file))
		}
		if !validType(opt.Type) {
			errs = append(errs, fmt.Sprintf("%s: option %q has unsupported type %q", file, name, opt.Type))
		}
		if opt.Default == nil {
			errs = append(errs, fmt.Sprintf("%s: option %q requires a default", file, name))
		} else if !typeMatchesDefault(opt.Type, opt.Default) {
			errs = append(errs, fmt.Sprintf("%s: option %q default does not match type %q", file, name, opt.Type))
		}
		if opt.Minimum != nil && opt.Maximum != nil && *opt.Minimum > *opt.Maximum {
			errs = append(errs, fmt.Sprintf("%s: option %q minimum > maximum", file, name))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidManifest, strings.Join(errs, "; "))
	}
	return nil
}

func validType(t string) bool {
	switch t {
	case "string", "integer", "duration", "boolean", "string_array":
		return true
	default:
		return false
	}
}

func typeMatchesDefault(t string, defaultVal any) bool {
	switch t {
	case "string":
		_, ok := defaultVal.(string)
		return ok
	case "integer":
		switch v := defaultVal.(type) {
		case int:
			return true
		case int64:
			return true
		case float64:
			return float64(int64(v)) == v
		default:
			return false
		}
	case "duration":
		_, ok := defaultVal.(string)
		return ok
	case "boolean":
		_, ok := defaultVal.(bool)
		return ok
	case "string_array":
		_, ok := defaultVal.([]any)
		return ok
	default:
		return false
	}
}
