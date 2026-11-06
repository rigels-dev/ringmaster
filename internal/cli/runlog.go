package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RunLogName is the persistent orchestration log in .ringmaster/logs/. Every
// `ringmaster run` appends to it (whether started from the CLI or the web UI),
// so the output of earlier runs and finished tasks is kept.
const RunLogName = "run.jsonl"

// runLogRotateBytes is the size at which run.jsonl is archived on the next run.
const runLogRotateBytes = 10 * 1024 * 1024

// RunLogEntry is one line of run.jsonl. Lines starting with "[Run]" mark the
// start and end of a run.
type RunLogEntry struct {
	Time   string `json:"ts"`
	Line   string `json:"line"`
	Stream string `json:"stream,omitempty"` // "err" for stderr
}

type runLog struct {
	mu         sync.Mutex
	f          *os.File
	origStdout *os.File
	origStderr *os.File
	pipes      []*os.File
	wg         sync.WaitGroup
}

// startRunLog tees this process's stdout and stderr into the project's run
// log, line by line with timestamps, while still printing them as before.
// The returned function writes the closing marker and restores the streams.
func startRunLog(projectDir string, header string) (func(footer string), error) {
	logsDir := filepath.Join(projectDir, ".ringmaster", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(logsDir, RunLogName)
	if fi, err := os.Stat(path); err == nil && fi.Size() > runLogRotateBytes {
		archive := filepath.Join(logsDir, fmt.Sprintf("run-%s.jsonl", time.Now().Format("20060102-150405")))
		_ = os.Rename(path, archive)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	rl := &runLog{f: f, origStdout: os.Stdout, origStderr: os.Stderr}
	rl.append("[Run] "+header, "")

	tee := func(orig *os.File, stream string) (*os.File, error) {
		r, w, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		rl.pipes = append(rl.pipes, w)
		rl.wg.Add(1)
		go func() {
			defer rl.wg.Done()
			reader := bufio.NewReader(r)
			for {
				line, err := reader.ReadString('\n')
				if len(line) > 0 {
					_, _ = io.WriteString(orig, line)
					rl.append(trimNewline(line), stream)
				}
				if err != nil {
					_ = r.Close()
					return
				}
			}
		}()
		return w, nil
	}

	outW, err := tee(rl.origStdout, "")
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	errW, err := tee(rl.origStderr, "err")
	if err != nil {
		_ = outW.Close()
		_ = f.Close()
		return nil, err
	}
	os.Stdout = outW
	os.Stderr = errW

	return func(footer string) {
		os.Stdout = rl.origStdout
		os.Stderr = rl.origStderr
		for _, w := range rl.pipes {
			_ = w.Close()
		}
		rl.wg.Wait()
		rl.append("[Run] "+footer, "")
		_ = rl.f.Close()
	}, nil
}

func (rl *runLog) append(line, stream string) {
	data, err := json.Marshal(RunLogEntry{Time: time.Now().UTC().Format(time.RFC3339Nano), Line: line, Stream: stream})
	if err != nil {
		return
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	_, _ = rl.f.Write(append(data, '\n'))
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
