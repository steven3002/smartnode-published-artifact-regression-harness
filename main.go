package main

import (
	"os"

	"github.com/rocket-pool/smartnode/rp-regress/cli"
)

func main() {
	app := &cli.CLI{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Args:   os.Args,
		Env:    os.Getenv,
	}
	os.Exit(app.Run())
}
