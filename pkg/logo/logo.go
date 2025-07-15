package logo

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// getTerminalWidth returns the terminal width, defaulting to 80 if unable to detect
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Default to 80 columns if we can't detect terminal size
		return 80
	}
	return width
}

// Display shows the startup logo with gradient blue colors, automatically choosing size based on terminal width
func Display() {
	width := getTerminalWidth()

	// Choose logo version based on terminal width
	// Full logo needs ~105 characters, simple needs ~42, compact needs ~35
	if width >= 105 {
		DisplayFull()
	} else if width >= 45 {
		DisplaySimple()
	} else {
		DisplayCompact()
	}
}

// DisplayFull shows the full ASCII art logo
func DisplayFull() {
	// Define gradient blue colors
	lightBlue := color.New(color.FgCyan, color.Bold)
	mediumBlue := color.New(color.FgBlue, color.Bold)
	darkBlue := color.New(color.FgBlue)
	navy := color.New(color.FgBlue, color.Faint)

	// Clear screen and move cursor to top (optional)
	fmt.Print("\n")

	// ASCII Art Logo for "dirsearch-go"
	logoLines := []struct {
		text  string
		color *color.Color
	}{
		{"    ██████╗ ██╗██████╗ ███████╗███████╗ █████╗ ██████╗  ██████╗██╗  ██╗      ██████╗  ██████╗ ", lightBlue},
		{"    ██╔══██╗██║██╔══██╗██╔════╝██╔════╝██╔══██╗██╔══██╗██╔════╝██║  ██║     ██╔════╝ ██╔═══██╗", lightBlue},
		{"    ██║  ██║██║██████╔╝███████╗█████╗  ███████║██████╔╝██║     ███████║     ██║  ███╗██║   ██║", mediumBlue},
		{"    ██║  ██║██║██╔══██╗╚════██║██╔══╝  ██╔══██║██╔══██╗██║     ██╔══██║     ██║   ██║██║   ██║", mediumBlue},
		{"    ██████╔╝██║██║  ██║███████║███████╗██║  ██║██║  ██║╚██████╗██║  ██║     ╚██████╔╝╚██████╔╝", darkBlue},
		{"    ╚═════╝ ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝      ╚═════╝  ╚═════╝ ", darkBlue},
		{"", nil},
		{"                           High-Performance Directory Scanner", navy},
		{"                                    Version 0.01", navy},
		{"", nil},
	}

	// Display the logo
	for _, line := range logoLines {
		if line.color != nil {
			line.color.Fprintln(os.Stdout, line.text)
		} else {
			fmt.Fprintln(os.Stdout, line.text)
		}
	}

	// Add some spacing after the logo
	fmt.Print("\n")
}

// DisplaySimple shows a simpler version of the logo for narrow terminals
func DisplaySimple() {
	// Define colors
	lightBlue := color.New(color.FgCyan, color.Bold)
	mediumBlue := color.New(color.FgBlue, color.Bold)
	darkBlue := color.New(color.FgBlue)

	fmt.Print("\n")

	// Simple ASCII art
	logoLines := []struct {
		text  string
		color *color.Color
	}{
		{"  ╔══════════════════════════════════════╗", lightBlue},
		{"  ║                                      ║", lightBlue},
		{"  ║           DIRSEARCH-GO               ║", mediumBlue},
		{"  ║                                      ║", lightBlue},
		{"  ║    High-Performance Directory        ║", darkBlue},
		{"  ║           Scanner v0.01              ║", darkBlue},
		{"  ║                                      ║", lightBlue},
		{"  ╚══════════════════════════════════════╝", lightBlue},
		{"", nil},
	}

	// Display the simple logo
	for _, line := range logoLines {
		if line.color != nil {
			line.color.Fprintln(os.Stdout, line.text)
		} else {
			fmt.Fprintln(os.Stdout, line.text)
		}
	}

	fmt.Print("\n")
}

// DisplayCompact shows a very compact version for minimal output
func DisplayCompact() {
	lightBlue := color.New(color.FgCyan, color.Bold)
	darkBlue := color.New(color.FgBlue)

	fmt.Print("\n")
	lightBlue.Print("  ▓▓▓ ")
	lightBlue.Print("DIRSEARCH-GO")
	lightBlue.Print(" ▓▓▓")
	fmt.Print("  ")
	darkBlue.Println("v0.01 - Directory Scanner")
	fmt.Print("\n")
}
