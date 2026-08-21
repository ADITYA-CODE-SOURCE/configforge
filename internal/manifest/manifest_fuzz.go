package manifest

import (
	"testing"
)

func FuzzManifestLoad(f *testing.F) {
	f.Add(`{"name":"test","features":[]}`)
	f.Add(`{"a":1}`)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := Load(data, "test.yaml")
		_ = err
		// Should not panic on any input
	})
}