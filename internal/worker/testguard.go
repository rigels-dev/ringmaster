package worker

import (
	"path"
	"strings"

	"ringmaster/internal/config"
	"ringmaster/internal/schemas"
)

// DefaultTestFiles says which files count as tests when [sandbox] test_files
// is not set. An entry ending in / names a directory anywhere in the path;
// any other entry is a glob matched against the file name.
var DefaultTestFiles = []string{
	// Directories
	"test/", "tests/", "__tests__/", "spec/", "testdata/",
	// Go, Python, Ruby, Rust, Elixir
	"*_test.go", "test_*.py", "*_test.py", "conftest.py", "*_spec.rb", "*_test.rb", "*_test.rs", "*_test.exs",
	// JavaScript and TypeScript
	"*.test.*", "*.spec.*",
	// JVM, .NET, PHP
	"*Test.java", "*Tests.java", "*Test.kt", "*Tests.cs", "*Test.php",
}

// ProtectsTests reports whether a task's worker is barred from changing test
// files that already exist. Writing tests is what a "test" task is for, and
// the approved plan can exempt any other task with edits_tests.
func ProtectsTests(task schemas.TaskSpec, sandboxCfg config.SandboxConfig) bool {
	return !sandboxCfg.AllowTestEdits && !task.EditsTests && task.Type != schemas.TaskTypeTest
}

// TestFilePatterns returns the patterns in force: the configured ones, or the defaults.
func TestFilePatterns(sandboxCfg config.SandboxConfig) []string {
	if len(sandboxCfg.TestFiles) > 0 {
		return sandboxCfg.TestFiles
	}
	return DefaultTestFiles
}

// IsTestFile reports whether the slash-separated, repository-relative path
// matches one of the patterns.
func IsTestFile(file string, patterns []string) bool {
	file = path.Clean(strings.ReplaceAll(file, "\\", "/"))
	dirs := strings.Split(path.Dir(file), "/")
	base := path.Base(file)
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if dir, ok := strings.CutSuffix(pattern, "/"); ok {
			for _, d := range dirs {
				if d == dir && dir != "" {
					return true
				}
			}
			continue
		}
		if ok, err := path.Match(pattern, base); err == nil && ok {
			return true
		}
	}
	return false
}
