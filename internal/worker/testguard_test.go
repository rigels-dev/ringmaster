package worker

import (
	"testing"

	"ringmaster/internal/config"
	"ringmaster/internal/schemas"
)

func TestIsTestFileDefaults(t *testing.T) {
	tests := []string{
		"calc_test.go", "internal/worker/sandbox_test.go",
		"tests/test_api.py", "pkg/test_models.py", "pkg/models_test.py", "conftest.py",
		"web/tests/trust.test.ts", "src/components/Button.spec.tsx", "src/__tests__/app.js",
		"spec/models/user_spec.rb", "src/test/java/com/x/FooTest.java", "Api.Tests/UserTests.cs",
		"internal/parser/testdata/input.json", "test/helpers.js", `web\tests\trust.test.ts`,
	}
	for _, file := range tests {
		if !IsTestFile(file, DefaultTestFiles) {
			t.Errorf("%s should count as a test file", file)
		}
	}
	notTests := []string{
		"calc.go", "main.py", "src/latest.go", "contest.py", "src/testing.go", "attestation/sign.go",
		"docs/testing.md", "src/protest/main.go", "testimonials/index.html", "README.md", "spectrum/x.go",
	}
	for _, file := range notTests {
		if IsTestFile(file, DefaultTestFiles) {
			t.Errorf("%s should not count as a test file", file)
		}
	}
}

func TestIsTestFileCustomPatterns(t *testing.T) {
	patterns := []string{"features/", "*.feature", " *_check.sh ", "/"}
	for file, want := range map[string]bool{
		"features/login.txt":   true,
		"a/features/b/c.txt":   true,
		"smoke.feature":        true,
		"scripts/db_check.sh":  true,
		"calc_test.go":         false, // custom patterns replace the defaults
		"myfeatures/login.txt": false,
		"features":             false, // a file, not a directory
	} {
		if got := IsTestFile(file, patterns); got != want {
			t.Errorf("IsTestFile(%q) = %v, want %v", file, got, want)
		}
	}
	if IsTestFile("anything.go", nil) {
		t.Error("no patterns should match nothing")
	}
}

func TestProtectsTests(t *testing.T) {
	backend := schemas.TaskSpec{Type: schemas.TaskTypeBackend}
	if !ProtectsTests(backend, config.SandboxConfig{}) {
		t.Error("protection should be on by default")
	}
	if ProtectsTests(schemas.TaskSpec{Type: schemas.TaskTypeTest}, config.SandboxConfig{}) {
		t.Error("a test task writes tests")
	}
	if ProtectsTests(schemas.TaskSpec{Type: schemas.TaskTypeBackend, EditsTests: true}, config.SandboxConfig{}) {
		t.Error("the plan exempted this task")
	}
	if ProtectsTests(backend, config.SandboxConfig{AllowTestEdits: true}) {
		t.Error("the sandbox setting turns protection off")
	}

	if got := TestFilePatterns(config.SandboxConfig{}); len(got) != len(DefaultTestFiles) {
		t.Errorf("default patterns: %v", got)
	}
	if got := TestFilePatterns(config.SandboxConfig{TestFiles: []string{"*.feature"}}); len(got) != 1 || got[0] != "*.feature" {
		t.Errorf("custom patterns: %v", got)
	}
}
