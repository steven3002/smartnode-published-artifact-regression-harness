// Package cli is the command surface. It parses arguments and reports results;
// the work itself belongs to the packages it calls.
package cli

import (
	"fmt"
	"io"
)

// CLI is one invocation of the harness.
type CLI struct {
	Stdout io.Writer
	Stderr io.Writer
	Args   []string
	Env    func(string) string
}

// Run dispatches a subcommand and returns the process exit status.
func (c *CLI) Run() int {
	if len(c.Args) < 2 {
		c.printHelp(c.Stderr)
		return 1
	}

	switch c.Args[1] {
	case "run":
		return c.runCommand(c.Args[2:])
	case "fixture":
		return c.fixtureCommand(c.Args[2:])
	case "-h", "--help", "help":
		c.printHelp(c.Stdout)
		return 0
	default:
		fmt.Fprintf(c.Stderr, "unknown subcommand %q\n\n", c.Args[1])
		c.printHelp(c.Stderr)
		return 1
	}
}

// printHelp leads with examples, which is what a reader pattern-matches against
// before reading any description.
func (c *CLI) printHelp(w io.Writer) {
	fmt.Fprint(w, `rp-regress — Rocket Pool Smartnode published-artifact regression harness

Examples:
  rp-regress run --release v1.23.0 --profile geth-lighthouse
  rp-regress run --release v1.23.0 --profile besu-teku --checkpoint-url https://checkpoint-sync.hoodi.ethpandaops.io
  rp-regress fixture --name empty-jwt --profile geth-lighthouse

Commands:
  run       Verify a published release and run a client profile against Hoodi
  fixture   Inject a controlled fault and assert the expected behaviour

Profiles:
  geth-lighthouse   baseline; Geth does not repair a zero-byte engine-API secret
  besu-teku         control; Besu repairs one

Exit codes:
  0  pass, or a fixture that observed what it predicted
  1  product failure      2  infrastructure failure
  3  harness failure      4  timeout
  5  fixture did not observe what it predicted

Run "rp-regress run --help" for the full flag list.
`)
}
