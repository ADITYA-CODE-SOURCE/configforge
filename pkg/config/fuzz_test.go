package config

import "testing"

func FuzzLoad(f *testing.F) {
	f.Add([]byte("version: v1\n"))
	f.Add([]byte("version: v1\nfeatures:\n  new_checkout:\n    enabled: true\n"))
	f.Add([]byte("version: v1\nprivacy:\n  redact_headers:\n    - authorization\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Load must never panic on arbitrary input; it returns an error
		// for invalid documents.
		_, _ = Load(data, WithFilename("fuzz.yaml"))
	})
}
