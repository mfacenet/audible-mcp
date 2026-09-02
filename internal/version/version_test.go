package version

import "testing"

func TestVersionNonEmpty(t *testing.T) {
	t.Parallel()
	if Version == "" {
		t.Fatal("Version must be non-empty")
	}
}
