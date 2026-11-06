package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/aymanbagabas/go-pty"
	"github.com/spf13/cobra"
)

// ptyBridgeCmd runs an interactive shell on a pseudo-terminal for the web UI
// terminal. The web server talks to it over plain pipes, so Node needs no
// native PTY module:
//
//	stdin:  one JSON message per line, {"t":"i","d":"<keys>"} for input or
//	        {"t":"r","c":<cols>,"r":<rows>} for a resize
//	stdout: the raw terminal output
//
// The bridge exits when the shell exits or when stdin is closed.
var ptyBridgeCmd = &cobra.Command{
	Use:    "pty-bridge",
	Short:  "Internal: shell on a pseudo-terminal for the web UI terminal",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("cwd")
		cols, _ := cmd.Flags().GetInt("cols")
		rows, _ := cmd.Flags().GetInt("rows")
		return runPtyBridge(dir, cols, rows, os.Stdin, os.Stdout)
	},
}

func init() {
	ptyBridgeCmd.Flags().String("cwd", "", "working directory of the shell")
	ptyBridgeCmd.Flags().Int("cols", 80, "initial terminal width")
	ptyBridgeCmd.Flags().Int("rows", 24, "initial terminal height")
	RootCmd.AddCommand(ptyBridgeCmd)
}

type ptyMessage struct {
	Type string `json:"t"`
	Data string `json:"d"`
	Cols int    `json:"c"`
	Rows int    `json:"r"`
}

func runPtyBridge(dir string, cols, rows int, in io.Reader, out io.Writer) error {
	shell, shellArgs := defaultShell()

	p, err := pty.New()
	if err != nil {
		return fmt.Errorf("failed to open pseudo-terminal: %w", err)
	}
	defer p.Close()
	if cols > 0 && rows > 0 {
		_ = p.Resize(cols, rows)
	}

	c := p.Command(shell, shellArgs...)
	c.Dir = dir
	c.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	if err := c.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", shell, err)
	}

	// Shell output → stdout
	go func() { _, _ = io.Copy(out, p) }()

	// stdin messages → shell; closing stdin (web server gone) ends the shell
	go func() {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			var msg ptyMessage
			if json.Unmarshal(scanner.Bytes(), &msg) != nil {
				continue
			}
			switch msg.Type {
			case "i":
				_, _ = io.WriteString(p, msg.Data)
			case "r":
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = p.Resize(msg.Cols, msg.Rows)
				}
			}
		}
		if c.Process != nil {
			_ = c.Process.Kill()
		}
	}()

	_ = c.Wait()
	return nil
}

// defaultShell is cmd.exe on Windows and bash elsewhere, falling back to
// $SHELL and then sh when bash is not installed.
func defaultShell() (string, []string) {
	if runtime.GOOS == "windows" {
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return comspec, nil
		}
		return "cmd.exe", nil
	}
	if path, err := exec.LookPath("bash"); err == nil {
		return path, []string{"-l"}
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh, nil
	}
	return "/bin/sh", nil
}
