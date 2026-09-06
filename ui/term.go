package ui

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// Capability holds terminal capability detection for a specific stream.
type Capability struct {
	IsTTY     bool
	Color     bool
	Unicode   bool
	WidthCols int
}

// ColorMode defines explicit color requests.
type ColorMode string

const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// DetectCapability determines terminal capability for the given file (e.g. os.Stdout or os.Stderr).
// colorMode comes from a flag like --color.
// env map is typically env override, normally pass os.Getenv.
func DetectCapability(f *os.File, colorMode ColorMode, getEnv func(string) string) Capability {
	fd := int(f.Fd())
	isTTY := term.IsTerminal(fd)

	width := 80
	if isTTY {
		if w, _, err := term.GetSize(fd); err == nil {
			width = w
		}
	}
	if width > 100 {
		width = 100
	} else if width < 40 {
		width = 40
	}

	// Unicode detection
	unicode := false
	termEnv := getEnv("TERM")
	if termEnv != "dumb" {
		// No locale set means UTF-8, but if any is set, check for UTF-8.
		lcAll := getEnv("LC_ALL")
		lcCtype := getEnv("LC_CTYPE")
		lang := getEnv("LANG")

		isSet := lcAll != "" || lcCtype != "" || lang != ""
		if !isSet {
			unicode = true
		} else {
			for _, v := range []string{lcAll, lcCtype, lang} {
				if v != "" {
					vl := strings.ToLower(v)
					if strings.Contains(vl, "utf-8") || strings.Contains(vl, "utf8") {
						unicode = true
					}
					break
				}
			}
		}
	}

	// Color precedence
	color := false
	if colorMode == ColorNever {
		color = false
	} else if colorMode == ColorAlways {
		color = true
	} else { // auto
		noColor := getEnv("NO_COLOR")
		rpNoColor := getEnv("RP_REGRESS_NO_COLOR")
		if noColor != "" || rpNoColor != "" {
			color = false
		} else if termEnv == "dumb" {
			color = false
		} else {
			color = isTTY
		}
	}

	return Capability{
		IsTTY:     isTTY,
		Color:     color,
		Unicode:   unicode,
		WidthCols: width,
	}
}
