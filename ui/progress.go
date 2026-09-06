package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type Progress struct {
	cap      Capability
	out      io.Writer
	mu       sync.Mutex
	lines    []string
	done     chan struct{}
	step     string
	stopOnce sync.Once
}

func NewProgress(cap Capability, out io.Writer) *Progress {
	p := &Progress{
		cap:   cap,
		out:   out,
		done:  make(chan struct{}),
		lines: make([]string, 0),
	}
	if p.cap.IsTTY {
		go p.renderLoop()
	}
	return p
}

func (p *Progress) SetStep(step string) {
	p.mu.Lock()
	p.step = step
	if !p.cap.IsTTY {
		fmt.Fprintln(p.out, FormatStatus(p.cap, StatusCurrent, step))
	}
	p.mu.Unlock()
}

func (p *Progress) Log(line string) {
	p.mu.Lock()
	if !p.cap.IsTTY {
		fmt.Fprintln(p.out, line)
	} else {
		p.lines = append(p.lines, line)
		if len(p.lines) > 3 {
			p.lines = p.lines[len(p.lines)-3:]
		}
	}
	p.mu.Unlock()
}

func (p *Progress) Stop() {
	p.stopOnce.Do(func() {
		close(p.done)
		if p.cap.IsTTY {
			p.clearRegion()
		}
	})
}

// clearRegion clears the current live region on TTY.
func (p *Progress) clearRegion() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Clear the lines we've drawn.
	// Typically, 1 line for step + len(lines)
	count := 1 + len(p.lines)
	for i := 0; i < count; i++ {
		fmt.Fprint(p.out, "\033[2K") // Clear line
		if i < count-1 {
			fmt.Fprint(p.out, "\033[1A") // Move up
		}
	}
	fmt.Fprint(p.out, "\r") // Move to start
}

func (p *Progress) renderLoop() {
	framesUni := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	framesAscii := []string{"|", "/", "-", "\\"}

	frames := framesAscii
	if p.cap.Unicode {
		frames = framesUni
	}

	ticker := time.NewTicker(90 * time.Millisecond)
	defer ticker.Stop()

	frameIdx := 0
	lastCount := 0

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.mu.Lock()

			// Move up and clear from last render
			for i := 0; i < lastCount; i++ {
				fmt.Fprint(p.out, "\033[1A\033[2K\r")
			}

			// Render
			colorPre := ""
			colorPost := ""
			if p.cap.Color {
				colorPre = ColorCyan
				colorPost = ColorReset
			}

			stepLine := fmt.Sprintf("%s%s%s %s", colorPre, frames[frameIdx], colorPost, p.step)
			// Truncate to width
			if len(stepLine) > p.cap.WidthCols && p.cap.WidthCols > 0 {
				stepLine = stepLine[:p.cap.WidthCols]
			}
			fmt.Fprintln(p.out, stepLine)

			for _, l := range p.lines {
				if len(l) > p.cap.WidthCols && p.cap.WidthCols > 0 {
					l = l[:p.cap.WidthCols]
				}
				fmt.Fprintln(p.out, "  "+l)
			}

			lastCount = 1 + len(p.lines)
			frameIdx = (frameIdx + 1) % len(frames)

			p.mu.Unlock()
		}
	}
}
