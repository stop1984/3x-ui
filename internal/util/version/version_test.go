package version

import "testing"

func TestCompareRejectsUnexpectedFormats(t *testing.T) {
	if _, ok := Compare("latest", "2.9.3"); ok {
		t.Fatal("expected non-semver latest tag to be rejected")
	}
	if _, ok := Compare("v2.9", "2.9.3"); ok {
		t.Fatal("expected short version to be rejected")
	}
	if _, ok := Compare("v3.11.0-other.1", "3.11.0"); ok {
		t.Fatal("expected a foreign version suffix to be rejected")
	}
}
