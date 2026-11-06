package version

import (
	"os"
	"strings"
	"testing"
)

// The VERSION file is the release's source of truth and must follow the scheme.
func TestVersionFileFollowsScheme(t *testing.T) {
	data, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	if _, err := Parse(strings.TrimSpace(string(data))); err != nil {
		t.Fatal(err)
	}
}

func TestCodenamesAreAlphabetical(t *testing.T) {
	for i, name := range Codenames {
		if want := byte('a' + i); name[0] != want {
			t.Errorf("codename %d is %q, want one starting with %q", i, name, want)
		}
	}
}

func TestParseAndNeedsUpdate(t *testing.T) {
	cur, err := Parse("v0.1.0-antares")
	if err != nil {
		t.Fatal(err)
	}
	fix, _ := Parse("v0.1.3-antares")
	next, _ := Parse("v0.2.0-bellatrix")
	if NeedsUpdate(cur, fix) {
		t.Error("a minor release on the same codename is not a required update")
	}
	if !NeedsUpdate(cur, next) {
		t.Error("a new codename is an update")
	}
	for _, bad := range []string{"0.1.0-antares", "v0.1-antares", "v0.1.0", "v0.1.0-pluto", "v0.1.0-Antares"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}
