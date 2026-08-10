package engine

import (
	"strconv"
	"testing"

	"github.com/ADITYA-CODE-SOURCE/configforge/pkg/config"
)

func mustCompile(t *testing.T, cfg config.Config) *Engine {
	t.Helper()
	e, err := Compile(cfg)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	return e
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
