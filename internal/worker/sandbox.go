package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath ensures rel path stays securely inside root, rejecting path traversal (..)
// absolute path escapes, and symlink breakout.
func ResolvePath(root, rel string) (string, error) {
	cleanRel := filepath.Clean(rel)
	if filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}

	// Normalize root. Resolve its symlinks too, so it compares against resolved
	// targets (on macOS /tmp is a symlink to /private/tmp)
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve root directory: %w", err)
	}
	if realRoot, err := filepath.EvalSymlinks(cleanRoot); err == nil {
		cleanRoot = realRoot
	}

	fullPath := filepath.Join(cleanRoot, cleanRel)

	// Guard against path traversal like ../../etc/passwd
	if !isWithin(cleanRoot, fullPath) {
		return "", fmt.Errorf("path escapes sandbox boundary: %s", rel)
	}

	// Resolve the deepest part of the path that exists and check where it really
	// points. The rest (not created yet, e.g. for write_file) is appended as is.
	existing, rest := fullPath, ""
	for {
		realPath, err := filepath.EvalSymlinks(existing)
		if err == nil {
			if !isWithin(cleanRoot, realPath) {
				return "", fmt.Errorf("symlink target escapes sandbox boundary: %s", rel)
			}
			return filepath.Join(realPath, rest), nil
		}
		// Present but unresolvable: a dangling symlink, which a write would follow
		if _, lerr := os.Lstat(existing); lerr == nil {
			return "", fmt.Errorf("unresolvable symlink in path: %s", rel)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", fmt.Errorf("path escapes sandbox boundary: %s", rel)
		}
		rest = filepath.Join(filepath.Base(existing), rest)
		existing = parent
	}
}

// isWithin reports whether path is root itself or lies below it. It compares
// whole path components, so /work/T01 does not contain /work/T010.
func isWithin(root, path string) bool {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relPath != ".." && !strings.HasPrefix(relPath, ".."+string(filepath.Separator))
}

// CheckCommand checks if cmd string matches the allowlist by token prefix (not substring).
// e.g. "go test -v" matches allowlist "go test", but "rm -rf" does not.
func CheckCommand(cmd string, allowlist []string) bool {
	// Split the way the command will be run, so quoting cannot hide what it is
	fields, err := SplitCommand(cmd)
	if err != nil {
		return false
	}

	for _, allowed := range allowlist {
		allowFields, err := SplitCommand(allowed)
		if err != nil {
			allowFields = strings.Fields(strings.TrimSpace(allowed))
		}
		if len(allowFields) == 0 {
			continue
		}

		if len(fields) >= len(allowFields) {
			match := true
			for i := range allowFields {
				if fields[i] != allowFields[i] {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// defaultPassEnv names the environment variables a worker command gets without
// being asked for: what a shell, the common toolchains and the network setup
// need to work, and nothing that normally carries a credential. A trailing *
// matches any name with that prefix.
var defaultPassEnv = []string{
	// Session and locale
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "TERM", "TZ",
	"LANG", "LANGUAGE", "LC_*", "TMPDIR", "TMP", "TEMP", "XDG_*",
	"CI", "NO_COLOR", "FORCE_COLOR",
	// Go
	"GOPATH", "GOROOT", "GOBIN", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOPROXY",
	"GOPRIVATE", "GONOPROXY", "GONOSUMDB", "GOSUMDB", "GOTOOLCHAIN", "GOWORK",
	"GOOS", "GOARCH", "GOEXPERIMENT", "CGO_ENABLED", "CGO_CFLAGS", "CGO_LDFLAGS",
	// Node.js
	"NODE_PATH", "NODE_OPTIONS", "NODE_ENV", "NVM_DIR", "NVM_BIN", "VOLTA_HOME",
	"COREPACK_HOME", "BUN_INSTALL", "DENO_DIR", "NPM_CONFIG_CACHE", "NPM_CONFIG_PREFIX", "npm_config_cache", "npm_config_prefix",
	// Python
	"PYTHONPATH", "PYTHONHOME", "VIRTUAL_ENV", "PYENV_ROOT", "PYENV_VERSION",
	"CONDA_PREFIX", "CONDA_DEFAULT_ENV", "PIP_CACHE_DIR", "UV_CACHE_DIR",
	// Rust, Java, Ruby and version managers
	"CARGO_HOME", "RUSTUP_HOME", "RUSTUP_TOOLCHAIN", "JAVA_HOME", "MAVEN_HOME",
	"GRADLE_USER_HOME", "GEM_HOME", "GEM_PATH", "RBENV_ROOT", "ASDF_DIR",
	"ASDF_DATA_DIR", "MISE_DATA_DIR",
	// Compilers and system libraries
	"CC", "CXX", "LD_LIBRARY_PATH", "DYLD_LIBRARY_PATH", "PKG_CONFIG_PATH", "SDKROOT",
	"DEVELOPER_DIR", "MACOSX_DEPLOYMENT_TARGET", "HOMEBREW_PREFIX", "HOMEBREW_CELLAR",
	"HOMEBREW_REPOSITORY", "NIX_*",
	// Network and containers
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY",
	"http_proxy", "https_proxy", "no_proxy", "all_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS", "REQUESTS_CA_BUNDLE",
	"CURL_CA_BUNDLE", "DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG",
}

// sensitiveNameParts mark a variable as a likely credential. They only veto
// names matched by a default wildcard; a name the user listed is passed as asked.
var sensitiveNameParts = []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL"}

// CommandEnv is the environment for commands run on a worker's behalf: the
// variables in defaultPassEnv plus those the user listed in [sandbox] pass_env.
// Everything else in Ringmaster's own environment is left out, so a credential
// reaches a worker command only if it was named. (An allowlist, because
// secrets do not announce themselves: DATABASE_URL holds a password too.)
func CommandEnv(passEnv []string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || name == "" {
			continue
		}
		if matchesEnvName(passEnv, name) || (matchesEnvName(defaultPassEnv, name) && !looksSensitive(name)) {
			env = append(env, kv)
		}
	}
	return env
}

func matchesEnvName(patterns []string, name string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
			if prefix != "" && strings.HasPrefix(name, prefix) {
				return true
			}
		} else if pattern == name {
			return true
		}
	}
	return false
}

func looksSensitive(name string) bool {
	upper := strings.ToUpper(name)
	for _, part := range sensitiveNameParts {
		if strings.Contains(upper, part) {
			return true
		}
	}
	return false
}
