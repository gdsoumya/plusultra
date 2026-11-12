package version_test

import (
	"testing"

	"github.com/gdsoumya/plusultra/internal/version"
)

func TestVersionNotEmpty(t *testing.T) {
	if version.Version == "" {
		t.Fatal("Version must not be empty")
	}
}
