package cli

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ringmaster/internal/config"
)

func withPortSources(t *testing.T, flag int, env string, fleet int) {
	t.Helper()
	oldPort, oldCfg := webPort, cfg
	t.Cleanup(func() { webPort, cfg = oldPort, oldCfg })
	webPort = flag
	t.Setenv("RINGMASTER_PORT", env)
	cfg = config.DefaultConfig()
	cfg.Web.Port = fleet
}

func projectWithPort(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(dir, "ringmaster.toml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The most specific source wins: flag, environment, project, fleet, default.
func TestResolveWebPortPrecedence(t *testing.T) {
	const manifest = "name = \"p\"\n\n[web]\nport = 4003\n"
	cases := []struct {
		name     string
		flag     int
		env      string
		manifest string
		fleet    int
		want     int
	}{
		{"default", 0, "", "", 0, config.DefaultWebPort},
		{"fleet", 0, "", "", 4004, 4004},
		{"project over fleet", 0, "", manifest, 4004, 4003},
		{"env over project", 0, "4002", manifest, 4004, 4002},
		{"flag over env", 4001, "4002", manifest, 4004, 4001},
		{"manifest without web section", 0, "", "name = \"p\"\n", 4004, 4004},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withPortSources(t, c.flag, c.env, c.fleet)
			got, err := resolveWebPort(projectWithPort(t, c.manifest))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("port %d, want %d", got, c.want)
			}
		})
	}
}

func TestResolveWebPortRejectsBadValues(t *testing.T) {
	cases := []struct {
		name     string
		flag     int
		env      string
		manifest string
		fleet    int
		mention  string
	}{
		{"flag out of range", 70000, "", "", 0, "--port"},
		{"negative flag", -1, "", "", 0, "--port"},
		{"env not a number", 0, "http", "", 0, "RINGMASTER_PORT"},
		{"env out of range", 0, "0", "", 0, "RINGMASTER_PORT"},
		{"project out of range", 0, "", "[web]\nport = 99999\n", 0, "ringmaster.toml"},
		{"fleet out of range", 0, "", "", 99999, "[web] port"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withPortSources(t, c.flag, c.env, c.fleet)
			_, err := resolveWebPort(projectWithPort(t, c.manifest))
			if err == nil || !strings.Contains(err.Error(), c.mention) {
				t.Errorf("error %v, want one naming %q", err, c.mention)
			}
		})
	}
}

func TestCheckPortFree(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port

	if err := checkPortFree(port); err == nil || !strings.Contains(err.Error(), "--port") {
		t.Errorf("busy port: error %v, want a hint to pick another", err)
	}
	l.Close()
	if err := checkPortFree(port); err != nil {
		t.Errorf("free port: %v", err)
	}
}

// The launcher shows the address the web UI will really use.
func TestLauncherShowsPort(t *testing.T) {
	if view := initialLauncherModel(4321).View(); !strings.Contains(view, "http://localhost:4321") {
		t.Errorf("launcher view does not show the port:\n%s", view)
	}
}
