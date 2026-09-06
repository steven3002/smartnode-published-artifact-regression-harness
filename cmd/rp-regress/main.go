package main

import (
	"os"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/cli"
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
