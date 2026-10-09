package network

import (
	"context"
	"os/exec"
	"time"
)

type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execCommandRunner struct{}

func (execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = commandWaitDelay
	return cmd.CombinedOutput()
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
