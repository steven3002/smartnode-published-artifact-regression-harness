package ui

import (
	"errors"
	"fmt"
	"os"
	"os/signal"

	"golang.org/x/term"
)

var ErrInputRequired = errors.New("input required but cannot prompt")

// AskSecret reads a secret from the controlling terminal (not stdin).
func AskSecret(cap Capability, noInput bool, prompt string) (string, error) {
	if noInput {
		return "", fmt.Errorf("%w: --no-input is set", ErrInputRequired)
	}
	if !cap.IsTTY {
		return "", fmt.Errorf("%w: not a TTY", ErrInputRequired)
	}

	// Controlling terminal
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open /dev/tty: %w", err)
	}
	defer f.Close()

	fmt.Fprint(f, prompt)

	fd := int(f.Fd())

	// Setup signal handling for SIGINT to restore terminal state
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	oldState, err := term.GetState(fd)
	if err != nil {
		return "", err
	}

	// Read in a goroutine so an interrupt can restore the terminal before exit.
	type result struct {
		pass string
		err  error
	}
	resChan := make(chan result, 1)

	go func() {
		pass, err := term.ReadPassword(fd)
		resChan <- result{string(pass), err}
	}()

	select {
	case res := <-resChan:
		fmt.Fprintln(f)
		signal.Stop(sigChan)
		return res.pass, res.err
	case <-sigChan:
		// Restore terminal state
		term.Restore(fd, oldState)
		fmt.Fprintln(f)
		os.Exit(130)
		return "", nil // Unreachable
	}
}
