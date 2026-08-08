# Configuration

ConfigForge configuration starts with `version: v1`. Phase 1 implements strict YAML loading, built-in defaults, typed environment overrides, and validation for features, route policies, privacy redaction, and logging.

Unknown YAML fields are rejected. Validation errors include field paths and YAML positions when available.
