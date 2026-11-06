package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestPtyBridgeRunsShellInDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	dir := t.TempDir()
	inR, inW := io.Pipe()
	out := &syncBuffer{}

	done := make(chan error, 1)
	go func() { done <- runPtyBridge(dir, 100, 30, inR, out) }()

	send := func(msg ptyMessage) {
		line, _ := json.Marshal(msg)
		_, _ = inW.Write(append(line, '\n'))
	}
	send(ptyMessage{Type: "r", Cols: 120, Rows: 40})
	send(ptyMessage{Type: "i", Data: "echo marker-$((40+2)); pwd; stty size\n"})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s := out.String()
		if strings.Contains(s, "marker-42") && strings.Contains(s, "40 120") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	s := out.String()
	if !strings.Contains(s, "marker-42") {
		t.Fatalf("shell output missing command result:\n%s", s)
	}
	if !strings.Contains(s, dir) {
		t.Errorf("shell did not start in %s:\n%s", dir, s)
	}
	if !strings.Contains(s, "40 120") {
		t.Errorf("resize was not applied (want stty size 40 120):\n%s", s)
	}

	// Closing stdin (web server gone) must end the shell and the bridge
	_ = inW.Close()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("bridge did not exit after stdin closed")
	}
}
