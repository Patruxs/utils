package network

import (
	"context"
	"os/exec"
)

type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	Run(ctx context.Context, name string, args ...string) error
}

type execCommandRunner struct{}

func (execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

type commandSpec struct {
	name string
	args []string
}

const (
	commandPowerShell = "powershell"
	commandShell      = "sh"
	commandSudo       = "sudo"

	powerShellNoProfile       = "-NoProfile"
	powerShellExecutionPolicy = "-ExecutionPolicy"
	powerShellBypass          = "Bypass"
	powerShellCommand         = "-Command"
)
