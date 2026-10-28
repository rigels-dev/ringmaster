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
	"ringmaster/internal/tui"
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
	cursor   int
	selected launcherChoice
	quitting bool
}

func initialLauncherModel() launcherModel {
	return launcherModel{
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
			title: "[1] Web UI Dashboard (Browser · http://localhost:12053)",
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
	// Verify current directory or workspace has ringmaster context
	cwd, _ := os.Getwd()

	projectDir := cwd
	hasProject := false

	for _, checkFile := range []string{"ringmaster.toml", "BRIEF.md"} {
		if _, err := os.Stat(filepath.Join(cwd, checkFile)); err == nil {
			hasProject = true
			break
		}
	}

	// Outside a project, fall back to the last active project; inside one, remember it as active
	if !hasProject {
		activeFile := filepath.Join(os.Getenv("HOME"), ".ringmaster", "active_project")
		if data, err := os.ReadFile(activeFile); err == nil {
			stored := filepath.Clean(string(data))
			if fi, err := os.Stat(stored); err == nil && fi.IsDir() {
				projectDir = stored
				hasProject = true
			}
		}
	} else {
		activeDir := filepath.Join(os.Getenv("HOME"), ".ringmaster")
		_ = os.MkdirAll(activeDir, 0755)
		_ = os.WriteFile(filepath.Join(activeDir, "active_project"), []byte(projectDir), 0644)
	}

	p := tea.NewProgram(initialLauncherModel())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(launcherModel)
	if !ok || m.selected == choiceCancel {
		return nil
	}

	if m.selected == choiceWeb {
		return StartWebUI(projectDir, hasProject)
	}

	return StartTUI()
}

func StartTUI() error {
	p := tea.NewProgram(tui.NewConfigModel(cfg, cfgFile), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func StartWebUI(projectDir string, hasProject bool) error {
	// Locate web output
	root := config.RootDir()
	candidates := []string{
		filepath.Join(projectDir, "web", ".output", "server", "index.mjs"),
	}
	if root != "" {
		candidates = append(candidates, filepath.Join(root, "web", ".output", "server", "index.mjs"))
	}

	var serverScript string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			serverScript = c
			break
		}
	}

	if serverScript == "" {
		return fmt.Errorf("Ringmaster Web UI build not found. Run 'npm run build' inside web/ first")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, "node", serverScript)
	cmd.Dir = filepath.Dir(serverScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Isolate into own process group so killpg terminates all child threads
	cmd.Env = append(os.Environ(),
		"PORT=12053",
		"NITRO_PORT=12053",
		"HOST=127.0.0.1",
		"NITRO_HOST=127.0.0.1",
		fmt.Sprintf("RINGMASTER_PROJECT_DIR=%s", projectDir),
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
	if hasProject {
		fmt.Printf("   Project Context: %s\n", projectDir)
	}
	fmt.Println("   URL: http://localhost:12053")
	fmt.Println("   Port: 12053 (Custom Dedicated Ringmaster Port)")
	fmt.Println("   Status: Running. Press Ctrl+C to stop.")

	err := cmd.Run()
	if ctx.Err() != nil {
		fmt.Println("\n✓ Web UI stopped cleanly.")
		return nil
	}
	return err
}
