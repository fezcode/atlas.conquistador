package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"atlas.conquistador/internal/explorer"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-v" || arg == "--version" {
			fmt.Printf("atlas.conquistador v%s\n", Version)
			return
		}
		if arg == "-h" || arg == "--help" || arg == "help" {
			showHelp()
			return
		}
	}

	m := explorer.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running atlas.conquistador: %v\n", err)
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("Atlas Conquistador - A powerful and beautiful terminal-based file explorer.")
	fmt.Println("\nUsage:")
	fmt.Println("  atlas.conquistador          Start the interactive TUI")
	fmt.Println("  atlas.conquistador help     Show this help information")
	fmt.Println("  atlas.conquistador --version Show version info")
	fmt.Println("\nTUI Controls:")
	fmt.Println("  j/k, Arrows   Navigate through files")
	fmt.Println("  h, Left       Go to parent directory")
	fmt.Println("  l, Right      Enter directory / Open file externally")
	fmt.Println("  v             View file internally (plain text)")
	fmt.Println("  Space         Toggle task selection")
	fmt.Println("  /             Search / Go to specific path")
	fmt.Println("  c/x/p         Copy / Cut / Paste")
	fmt.Println("  d             Delete selected/current item (with confirmation)")
	fmt.Println("  s             Cycle sort order (Name -> Size -> Time)")
	fmt.Println("  ?             Show help screen")
	fmt.Println("  q, Esc        Quit")
}
