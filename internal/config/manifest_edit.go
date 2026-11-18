package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	tableHeader = regexp.MustCompile(`^\s*\[([^\[\]]+)\]\s*(#.*)?$`)
	presetsLine = regexp.MustCompile(`^\s*presets\s*=`)
)

// AddProjectPresets adds preset names to [sandbox] presets in the project's
// ringmaster.toml, keeping the ones already there and the rest of the file as
// it was, comments included. It only ever adds: removing a preset is the
// user's edit to make. It returns the names that were new.
func AddProjectPresets(projectDir string, names []string) ([]string, error) {
	path := ProjectManifestPath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	m, err := LoadProjectManifest(projectDir)
	if err != nil {
		return nil, err
	}

	have := map[string]bool{}
	for _, n := range m.Sandbox.Presets {
		have[strings.ToLower(n)] = true
	}
	merged := append([]string{}, m.Sandbox.Presets...)
	var added []string
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" || have[n] || !validPresetName(n) {
			continue
		}
		have[n] = true
		merged = append(merged, n)
		added = append(added, n)
	}
	if len(added) == 0 {
		return nil, nil
	}

	quoted := make([]string, len(merged))
	for i, n := range merged {
		quoted[i] = strconv.Quote(n)
	}
	line := "presets = [" + strings.Join(quoted, ", ") + "]"

	lines := strings.Split(string(data), "\n")
	sandboxAt, sectionEnd, presetsAt := -1, len(lines), -1
	for i, l := range lines {
		if h := tableHeader.FindStringSubmatch(l); h != nil {
			if sandboxAt >= 0 {
				sectionEnd = i
				break
			}
			if strings.TrimSpace(h[1]) == "sandbox" {
				sandboxAt = i
			}
			continue
		}
		if sandboxAt >= 0 && presetsLine.MatchString(l) {
			presetsAt = i // a list spread over several lines is rewritten as one
		}
	}

	switch {
	case presetsAt >= 0:
		end := presetsAt
		for end < sectionEnd && !strings.Contains(lines[end], "]") {
			end++
		}
		if end >= sectionEnd {
			end = presetsAt
		}
		lines = append(append(append([]string{}, lines[:presetsAt]...), line), lines[end+1:]...)
	case sandboxAt >= 0:
		lines = append(append(append([]string{}, lines[:sandboxAt+1]...), line), lines[sandboxAt+1:]...)
	default:
		text := strings.TrimRight(string(data), "\n")
		if text != "" {
			text += "\n\n"
		}
		lines = strings.Split(text+"[sandbox]\n"+line+"\n", "\n")
	}

	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	// Never leave a file Ringmaster cannot read back
	check := &ProjectManifest{}
	if err := unmarshalManifest([]byte(out), check); err != nil {
		return nil, fmt.Errorf("could not add presets to %s without breaking it: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		return nil, err
	}
	return added, nil
}

var presetName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func validPresetName(n string) bool {
	return presetName.MatchString(n)
}
