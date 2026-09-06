package ui

import "fmt"

// Status represents the state of a step or task.
type Status int

const (
	StatusPass Status = iota
	StatusFail
	StatusWarn
	StatusInfo
	StatusCurrent
)

// ANSI colors
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorCyan    = "\033[36m"
	ColorMagenta = "\033[35m"
)

// FormatStatus returns a formatted string for the given status, text, and capability.
func FormatStatus(cap Capability, status Status, text string) string {
	var symbol string
	var color string

	switch status {
	case StatusPass:
		color = ColorGreen
		if cap.Unicode {
			symbol = "✓"
		} else {
			symbol = "ok"
		}
	case StatusFail:
		color = ColorRed
		if cap.Unicode {
			symbol = "✗"
		} else {
			symbol = "x"
		}
	case StatusWarn:
		color = ColorYellow
		if cap.Unicode {
			symbol = "⚠"
		} else {
			symbol = "!"
		}
	case StatusInfo:
		color = ColorCyan
		if cap.Unicode {
			symbol = "·"
		} else {
			symbol = "-"
		}
	case StatusCurrent:
		color = ColorMagenta
		if cap.Unicode {
			symbol = "◆"
		} else {
			symbol = ">"
		}
	}

	if cap.Color {
		return fmt.Sprintf("%s%s%s %s", color, symbol, ColorReset, text)
	}
	return fmt.Sprintf("%s %s", symbol, text)
}
