package cli

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"ringmaster/internal/config"
)

// resolveWebPort picks the web UI's port: the --port flag, then
// $RINGMASTER_PORT, then [web] port in the project's ringmaster.toml, then in
// the fleet config, then the default.
func resolveWebPort(projectDir string) (int, error) {
	if webPort != 0 {
		return validPort(webPort, "--port")
	}
	if env := strings.TrimSpace(os.Getenv("RINGMASTER_PORT")); env != "" {
		n, err := strconv.Atoi(env)
		if err != nil {
			return 0, fmt.Errorf("RINGMASTER_PORT: %q is not a port number", env)
		}
		return validPort(n, "RINGMASTER_PORT")
	}
	if manifest, err := config.LoadProjectManifest(projectDir); err != nil {
		return 0, err
	} else if manifest.Web.Port != 0 {
		return validPort(manifest.Web.Port, "[web] port in "+config.ProjectManifestPath(projectDir))
	}
	if cfg != nil && cfg.Web.Port != 0 {
		return validPort(cfg.Web.Port, "[web] port in "+config.ConfigPath(cfgFile))
	}
	return config.DefaultWebPort, nil
}

func validPort(n int, source string) (int, error) {
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("%s: %d is not a valid port (1-65535)", source, n)
	}
	return n, nil
}

// checkPortFree fails early, with a hint, when something already listens on
// the port; otherwise the web server would die with a bare EADDRINUSE.
func checkPortFree(port int) error {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("port %d is not available (%v); is another Ringmaster running? Pick another with --port, RINGMASTER_PORT, or [web] port in ringmaster.toml", port, err)
	}
	return l.Close()
}
