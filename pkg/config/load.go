package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFile loads a ConfigForge YAML file using built-in defaults, environment overrides,
// and finally explicit file values. Unknown YAML fields are rejected.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	cfg, err := Load(data, WithFilename(path))
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadOption customizes configuration loading.
type LoadOption func(*loadOptions)

type loadOptions struct {
	filename string
}

// WithFilename annotates validation errors with a file name.
func WithFilename(filename string) LoadOption {
	return func(opts *loadOptions) {
		opts.filename = filename
	}
}

// Load parses and validates a ConfigForge YAML document.
func Load(data []byte, options ...LoadOption) (*Config, error) {
	opts := loadOptions{}
	for _, option := range options {
		option(&opts)
	}

	cfg := Defaults()
	if err := applyEnv(&cfg); err != nil {
		return nil, fmt.Errorf("apply environment overrides: %w", err)
	}

	positions, duplicateErrs, err := inspectYAML(data, opts.filename)
	if err != nil {
		return nil, err
	}
	if len(duplicateErrs) > 0 {
		return nil, &ValidationError{Errors: duplicateErrs}
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, formatDecodeError(opts.filename, err)
	}

	normalize(&cfg)
	if err := Validate(cfg, opts.filename, positions); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func inspectYAML(data []byte, filename string) (map[string]Position, []FieldError, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, nil, formatDecodeError(filename, err)
	}

	positions := map[string]Position{}
	var duplicateErrs []FieldError
	if len(document.Content) == 0 {
		return positions, nil, nil
	}
	walkNode(document.Content[0], "", positions, &duplicateErrs, filename)
	return positions, duplicateErrs, nil
}

func walkNode(node *yaml.Node, path string, positions map[string]Position, duplicateErrs *[]FieldError, filename string) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		seen := map[string]Position{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			childPath := joinPath(path, key.Value)
			position := Position{Line: value.Line, Column: value.Column}
			if value.Kind == yaml.MappingNode || value.Kind == yaml.SequenceNode {
				position = Position{Line: key.Line, Column: key.Column}
			}

			if first, ok := seen[key.Value]; ok {
				*duplicateErrs = append(*duplicateErrs, FieldError{
					File:    filename,
					Line:    key.Line,
					Column:  key.Column,
					Path:    childPath,
					Message: fmt.Sprintf("is duplicated; first defined at line %d column %d", first.Line, first.Column),
				})
			} else {
				seen[key.Value] = Position{Line: key.Line, Column: key.Column}
			}

			positions[childPath] = position
			walkNode(value, childPath, positions, duplicateErrs, filename)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			positions[childPath] = Position{Line: child.Line, Column: child.Column}
			walkNode(child, childPath, positions, duplicateErrs, filename)
		}
	}
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func formatDecodeError(filename string, err error) error {
	if err == nil {
		return nil
	}

	message := err.Error()
	if filename != "" && strings.HasPrefix(message, "yaml:") {
		message = filename + strings.TrimPrefix(message, "yaml")
	}
	return fmt.Errorf("%w: %s", ErrInvalidConfig, message)
}
