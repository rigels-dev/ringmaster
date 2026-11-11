package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"ringmaster/internal/config"
	"ringmaster/internal/skills"
	"ringmaster/internal/tui"
	"ringmaster/internal/version"
)

var (
	launcherTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#9A3412", Dark: "#DEAA3B"}).
				Padding(0, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#B6432A", Dark: "#DEAA3B"})

	launcherItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#EFE4CE"})

	launcherSelectedItemStyle = lipgloss.NewStyle().
					Foreground(lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#FFFFFF"}).
					Background(lipgloss.AdaptiveColor{Light: "#DEAA3B", Dark: "#DEAA3B"}).
					Bold(true).
					Padding(0, 1)

	launcherDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#8493A0"}).
				Italic(true)
)

type launcherChoice int

const (
	choiceWeb launcherChoice = iota
	choiceTUI
	choiceCancel
)

type launcherModel struct {
	port     int
	cursor   int
	selected launcherChoice
	quitting bool
}

func initialLauncherModel(port int) launcherModel {
	return launcherModel{
		port:     port,
		cursor:   0,
		selected: choiceCancel,
	}
}

func (m launcherModel) Init() tea.Cmd {
	return nil
}

func (m launcherModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			m.selected = choiceCancel
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < 1 {
				m.cursor++
			}

		case "1":
			m.selected = choiceWeb
			m.quitting = true
			return m, tea.Quit

		case "2":
			m.selected = choiceTUI
			m.quitting = true
			return m, tea.Quit

		case "enter", " ":
			if m.cursor == 0 {
				m.selected = choiceWeb
			} else {
				m.selected = choiceTUI
			}
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m launcherModel) View() string {
	if m.quitting {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(launcherTitleStyle.Render("🎪 RINGMASTER CONTROL PLANE"))
	sb.WriteString("\n\n")
	sb.WriteString("Select your preferred interface mode:\n\n")

	options := []struct {
		title string
		desc  string
	}{
		{
			title: fmt.Sprintf("[1] Web UI Dashboard (Browser · http://localhost:%d)", m.port),
			desc:  "Interactive chat briefing, real-time multi-worker fleet, and visual task pipeline",
		},
		{
			title: "[2] Terminal TUI (In-Terminal)",
			desc:  "Steampunk circus terminal interface for providers, models, and CLI workflows",
		},
	}

	for i, opt := range options {
		prefix := "  "
		itemStr := opt.title
		if i == m.cursor {
			prefix = "▸ "
			itemStr = launcherSelectedItemStyle.Render(opt.title)
		} else {
			itemStr = launcherItemStyle.Render(opt.title)
		}

		sb.WriteString(fmt.Sprintf("%s%s\n", prefix, itemStr))
		sb.WriteString(fmt.Sprintf("    %s\n\n", launcherDescStyle.Render(opt.desc)))
	}

	sb.WriteString(launcherDescStyle.Render("[↑/↓] Navigate · [Enter / 1 / 2] Select · [q] Quit\n"))
	return sb.String()
}

func RunInteractiveLauncher() error {
	// Like every project command, the launcher only runs inside a project
	projectDir, err := ResolveProjectDir()
	if err != nil {
		return err
	}

	port, err := resolveWebPort(projectDir)
	if err != nil {
		return err
	}

	p := tea.NewProgram(initialLauncherModel(port))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(launcherModel)
	if !ok || m.selected == choiceCancel {
		return nil
	}

	if m.selected == choiceWeb {
		return StartWebUI(projectDir, port)
	}

	return StartTUI()
}

func StartTUI() error {
	path := config.ConfigPath(cfgFile)
	loaded := cfg
	if loaded == nil {
		loaded = config.DefaultConfig()
	}
	p := tea.NewProgram(withInstalledSkills(tui.NewConfigModel(loaded, path)), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// withInstalledSkills fills the TUI's Skills tab: the fleet's skills, plus the
// project's when run inside one.
func withInstalledSkills(m tui.ConfigModel) tui.ConfigModel {
	projectDir, _ := ResolveProjectDir()
	_, projectOff := skillDisabledLists(projectDir)
	return m.WithSkills(skills.Discover(projectDir), projectOff)
}

func StartWebUI(projectDir string, port int) error {
	// Locate the web UI in the Ringmaster installation (never in the project,
	// which may well have a web/.output of its own)
	root := config.RootDir()
	serverScript := findWebServer(root)
	if serverScript == "" {
		return fmt.Errorf("Ringmaster web UI not found: set RINGMASTER_WEB_DIR, or run 'make web' in a source checkout")
	}
	if _, err := exec.LookPath("node"); err != nil {
		return fmt.Errorf("the web UI needs Node.js 22 or newer on your PATH: %w", err)
	}

	if err := checkPortFree(port); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, "node", serverScript)
	cmd.Dir = filepath.Dir(serverScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Isolate into own process group so killpg terminates all child threads
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("NITRO_PORT=%d", port),
		"HOST=127.0.0.1",
		"NITRO_HOST=127.0.0.1",
		fmt.Sprintf("RINGMASTER_PROJECT_DIR=%s", projectDir),
		fmt.Sprintf("RINGMASTER_VERSION=%s", version.Version),
		fmt.Sprintf("RINGMASTER_CONFIG=%s", config.ConfigPath(cfgFile)),
	)
	if root != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("RINGMASTER_ROOT=%s", root))
	}
	if exe, err := os.Executable(); err == nil {
		cmd.Env = append(cmd.Env, fmt.Sprintf("RINGMASTER_BIN=%s", exe))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Watch for interrupt and cleanly kill the entire child process group
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()

	fmt.Println("\n🎪 Starting Ringmaster Web UI...")
	fmt.Printf("   Project Context: %s\n", projectDir)
	fmt.Printf("   URL: http://localhost:%d\n", port)
	fmt.Println("   Status: Running. Press Ctrl+C to stop.")

	err := cmd.Run()
	if ctx.Err() != nil {
		fmt.Println("\n✓ Web UI stopped cleanly.")
		return nil
	}
	return err
}

// findWebServer returns the web UI's server entry: $RINGMASTER_WEB_DIR, a
// packaged install (<prefix>/share/ringmaster/web), or a source checkout
// (<root>/web/.output). Returns "" when none exists.
func findWebServer(root string) string {
	var dirs []string
	if dir := os.Getenv("RINGMASTER_WEB_DIR"); dir != "" {
		dirs = append(dirs, dir)
	}
	if root != "" {
		dirs = append(dirs,
			filepath.Join(root, "share", "ringmaster", "web"),
			filepath.Join(root, "web", ".output"),
		)
	}
	for _, dir := range dirs {
		script := filepath.Join(dir, "server", "index.mjs")
		if fi, err := os.Stat(script); err == nil && !fi.IsDir() {
			return script
		}
	}
	return ""
}
