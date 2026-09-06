package cli

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

// fixtureEmptyJWT is the only fixture the MVP defines.
const fixtureEmptyJWT = "empty-jwt"

// fixtureCommand injects a controlled fault and asserts the expected behaviour.
//
// Its exit semantics are inverted relative to run: zero means the fixture
// observed what it predicted, which for a defect profile is a failure.
func (c *CLI) fixtureCommand(args []string) int {
	var o options

	fs := pflag.NewFlagSet("fixture", pflag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.StringVar(&o.fixtureName, "name", "", "fixture to inject (empty-jwt)")
	fs.StringVar(&o.release, "release", "v1.23.0", "Smartnode release tag")
	commonFlags(fs, &o)
	help := fs.BoolP("help", "h", false, "show help")

	if err := fs.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			return result.ExitOK
		}
		return result.ExitHarness
	}

	if *help {
		fmt.Fprint(c.Stdout, `rp-regress fixture — inject a controlled fault and assert the outcome

A fixture exits 0 when it observes what it predicted:
  geth-lighthouse   predicts a product failure; the secret is not repaired
  besu-teku         predicts recovery; the secret is rewritten

Examples:
  rp-regress fixture --name empty-jwt --profile geth-lighthouse
  rp-regress fixture --name empty-jwt --profile besu-teku

Flags:
`)
		fs.SetOutput(c.Stdout)
		fs.PrintDefaults()
		return result.ExitOK
	}

	if err := resolveColor(fs, &o); err != nil {
		fmt.Fprintln(c.Stderr, "error:", err)
		return result.ExitHarness
	}

	if o.fixtureName == "" || o.profile == "" {
		fmt.Fprintln(c.Stderr, "error: --name and --profile are required")
		fmt.Fprintln(c.Stderr, "try: rp-regress fixture --name empty-jwt --profile geth-lighthouse")
		return result.ExitHarness
	}

	if o.fixtureName != fixtureEmptyJWT {
		fmt.Fprintf(c.Stderr, "error: unknown fixture %q (supported: %s)\n", o.fixtureName, fixtureEmptyJWT)
		return result.ExitHarness
	}

	return c.execute(o)
}
