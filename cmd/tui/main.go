package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"utils/internal/ui"
)

var version = "dev"

type cliAction int

const (
	actionRunTUI cliAction = iota
	actionHelp
	actionShowPath
	actionUninstall
	actionUpdate
	actionVersion
)

var errConflictingActions = errors.New("use only one of --showPath, --update, --uninstall, --version, --help")

func main() {
	action, err := parseAction(os.Args[0], os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	if errors.Is(err, errConflictingActions) {
		fmt.Fprintf(os.Stderr, "error: %v\n\n", err)
		printCommandHelp(os.Stderr, os.Args[0])
	}
	if err != nil {
		os.Exit(2)
	}

	switch action {
	case actionHelp:
		printCommandHelp(os.Stdout, os.Args[0])
		os.Exit(0)
	case actionShowPath:
		absPath, err := currentExecutablePath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to locate UTILS executable: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(absPath)
		os.Exit(0)
	case actionUninstall:
		if err := uninstallSelf(os.Stdin); err != nil {
			if errors.Is(err, errUninstallCanceled) {
				fmt.Println("Uninstall canceled.")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "failed to uninstall UTILS: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case actionUpdate:
		if err := updateSelf(version); err != nil {
			fmt.Fprintf(os.Stderr, "failed to update UTILS: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case actionVersion:
		fmt.Printf("UTILS %s\n", version)
		os.Exit(0)
	}

	program := tea.NewProgram(ui.NewRouterWithVersion(version, ui.DefaultFeatures()...), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run TUI: %v\n", err)
		os.Exit(1)
	}
}

func parseAction(appName string, args []string, output io.Writer) (cliAction, error) {
	flags := flag.NewFlagSet(appName, flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		printCommandHelp(output, appName)
	}
	actionFlags := []struct {
		action cliAction
		set    *bool
	}{
		{actionShowPath, flags.Bool("showPath", false, "print the path to the running UTILS executable")},
		{actionUninstall, flags.Bool("uninstall", false, "remove the running UTILS executable")},
		{actionUpdate, flags.Bool("update", false, "update UTILS to the latest GitHub Release")},
		{actionVersion, flags.Bool("version", false, "print the current UTILS version")},
		{actionHelp, flags.Bool("help", false, "show available UTILS commands")},
	}
	if err := flags.Parse(args); err != nil {
		return actionRunTUI, err
	}

	selected := actionRunTUI
	for _, actionFlag := range actionFlags {
		if !*actionFlag.set {
			continue
		}
		if selected != actionRunTUI {
			return actionRunTUI, errConflictingActions
		}
		selected = actionFlag.action
	}
	return selected, nil
}

func printCommandHelp(w io.Writer, appName string) {
	fmt.Fprintf(w, "Usage: %s [--showPath | --update | --uninstall | --version | --help]\n\n", appName)
	fmt.Fprintln(w, "Available commands:")
	fmt.Fprintln(w, "  --showPath   print the path to the running UTILS executable")
	fmt.Fprintln(w, "  --update     update UTILS to the latest GitHub Release")
	fmt.Fprintln(w, "  --uninstall  remove the running UTILS executable after confirmation")
	fmt.Fprintln(w, "  --version    print the current UTILS version")
	fmt.Fprintln(w, "  --help       show available UTILS commands")
}
