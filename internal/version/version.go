// Package version names Ringmaster releases as v{major}.{mid}.{minor}-{codename}.
//
// Codenames are stars (Quasar stands in for Q), in alphabetical order. The codename moves to the next
// letter whenever major or mid changes; a minor (fix) release keeps it. A new
// codename therefore always means an update worth installing.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is stamped at build time from the VERSION file:
//
//	go build -ldflags "-X ringmaster/internal/version.Version=$(cat VERSION)"
var Version = "dev"

// Codenames in release order; the first release line is Antares.
var Codenames = []string{
	"antares", "bellatrix", "capella", "deneb", "electra", "fomalhaut", "gemma",
	"hadar", "izar", "jabbah", "kochab", "lesath", "mira", "nunki", "okab",
	"polaris", "quasar", "rigel", "sirius", "thuban", "unukalhai", "vega", "wezen",
	"xamidimura", "yed", "zosma",
}

var pattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)-([a-z]+)$`)

// Release is a parsed version string.
type Release struct {
	Major, Mid, Minor int
	Codename          string
}

// Parse reads "v0.1.0-antares" and checks the codename is a known one.
func Parse(s string) (Release, error) {
	m := pattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return Release{}, fmt.Errorf("version %q is not v{major}.{mid}.{minor}-{codename}", s)
	}
	r := Release{Codename: m[4]}
	r.Major, _ = strconv.Atoi(m[1])
	r.Mid, _ = strconv.Atoi(m[2])
	r.Minor, _ = strconv.Atoi(m[3])
	if CodenameIndex(r.Codename) < 0 {
		return Release{}, fmt.Errorf("unknown codename %q", r.Codename)
	}
	return r, nil
}

// CodenameIndex is the codename's position in Codenames, or -1.
func CodenameIndex(name string) int {
	for i, c := range Codenames {
		if c == name {
			return i
		}
	}
	return -1
}

// NeedsUpdate reports whether moving from current to latest crosses a
// codename, i.e. a major or mid release.
func NeedsUpdate(current, latest Release) bool {
	return CodenameIndex(latest.Codename) > CodenameIndex(current.Codename)
}
