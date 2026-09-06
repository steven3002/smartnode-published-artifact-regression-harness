package cli

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

// runCommand verifies a published release and runs one client profile.
func (c *CLI) runCommand(args []string) int {
	var o options

	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.StringVar(&o.release, "release", "", "Smartnode release tag, for example v1.23.0")
	commonFlags(fs, &o)
	help := fs.BoolP("help", "h", false, "show help")

	if err := fs.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			return result.ExitOK
		}
		return result.ExitHarness
	}

	if *help {
		fmt.Fprint(c.Stdout, `rp-regress run — verify a published release and run a client profile

Examples:
  rp-regress run --release v1.23.0 --profile geth-lighthouse
  rp-regress run --release v1.23.0 --profile besu-teku --out-dir ./results

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

	if o.release == "" || o.profile == "" {
		fmt.Fprintln(c.Stderr, "error: --release and --profile are required")
		fmt.Fprintln(c.Stderr, "try: rp-regress run --release v1.23.0 --profile geth-lighthouse")
		return result.ExitHarness
	}

	return c.execute(o)
}
