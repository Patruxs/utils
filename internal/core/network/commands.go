package network

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execCommandRunner struct {
	onLine func(string)
}

func (r execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = commandWaitDelay
	if r.onLine == nil {
		return cmd.CombinedOutput()
	}

	var output bytes.Buffer
	lines := &lineWriter{onLine: r.onLine}
	writer := io.MultiWriter(&output, lines)
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	lines.flush()
	return output.Bytes(), err
}

type lineWriter struct {
	mu      sync.Mutex
	onLine  func(string)
	pending []byte
}

func (w *lineWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, data...)
	for {
		index := bytes.IndexByte(w.pending, '\n')
		if index < 0 {
			return len(data), nil
		}
		w.emit(w.pending[:index])
		w.pending = w.pending[index+1:]
	}
}

func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		w.emit(w.pending)
		w.pending = nil
	}
}

func (w *lineWriter) emit(line []byte) {
	text := strings.TrimRight(string(line), "\r")
	if strings.TrimSpace(text) != "" {
		w.onLine(text)
	}
}

type commandSpec struct {
	name string
	args []string
}

const (
	commandWaitDelay = 3 * time.Second

	commandPowerShell = "powershell"
	commandShell      = "sh"
	commandSudo       = "sudo"

	powerShellNoProfile       = "-NoProfile"
	powerShellExecutionPolicy = "-ExecutionPolicy"
	powerShellBypass          = "Bypass"
	powerShellCommand         = "-Command"
)
