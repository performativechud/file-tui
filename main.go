package main

import (
	"fmt"
	"os"

	// Imported before bubbletea so its init runs first and pins the terminal
	// background before bubbletea's init queries the terminal.
	_ "file-tui/internal/boot"

	tea "github.com/charmbracelet/bubbletea"
)

const version = "0.1.0"

func main() {
	initial := ""
	stopFlags := false
	for _, a := range os.Args[1:] {
		if !stopFlags && a == "--" {
			stopFlags = true
			continue
		}
		if !stopFlags && len(a) > 0 && a[0] == '-' {
			switch a {
			case "-h", "--help":
				fmt.Println("file-tui — convert a dropped or passed file")
				fmt.Println()
				fmt.Println("usage: file-tui [file]")
				return
			case "-v", "--version":
				fmt.Println("file-tui " + version)
				return
			default:
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", a)
				os.Exit(2)
			}
			continue
		}
		if initial == "" {
			initial = a
		}
	}

	tools := probeTools()
	m := newModel(initial, tools)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.send = p.Send
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "file-tui:", err)
		os.Exit(1)
	}
}
