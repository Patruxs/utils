package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"utils/internal/ui"
)

var version = "dev"

func main() {
	showPath := flag.Bool("showPath", false, "print the path to the running UTILS executable")
	uninstall := flag.Bool("uninstall", false, "remove the running UTILS executable")
	update := flag.Bool("update", false, "update UTILS to the latest GitHub Release")
	showVersion := flag.Bool("version", false, "print the current UTILS version")
	showHelp := flag.Bool("help", false, "show available UTILS commands")
	flag.Usage = func() {
		printCommandHelp(flag.CommandLine.Output(), os.Args[0])
	}
	flag.Parse()

	if *showHelp {
		printCommandHelp(os.Stdout, os.Args[0])
		os.Exit(0)
	}

	if *showPath {
		absPath, err := currentExecutablePath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to locate UTILS executable: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(absPath)
		os.Exit(0)
	}

	if *uninstall {
		if err := uninstallSelf(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to uninstall UTILS: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *update {
		if err := updateSelf(version); err != nil {
			fmt.Fprintf(os.Stderr, "failed to update UTILS: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("UTILS %s\n", version)
		os.Exit(0)
	}

	program := tea.NewProgram(ui.NewRouterWithVersion(version, ui.DefaultFeatures()...), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run TUI: %v\n", err)
		os.Exit(1)
	}
}

func printCommandHelp(w io.Writer, appName string) {
	fmt.Fprintf(w, "Usage: %s [--showPath | --update | --uninstall | --version | --help]\n\n", appName)
	fmt.Fprintln(w, "Available commands:")
	fmt.Fprintln(w, "  --showPath   print the path to the running UTILS executable")
	fmt.Fprintln(w, "  --update     update UTILS to the latest GitHub Release")
	fmt.Fprintln(w, "  --uninstall  remove the running UTILS executable")
	fmt.Fprintln(w, "  --version    print the current UTILS version")
	fmt.Fprintln(w, "  --help       show available UTILS commands")
}
