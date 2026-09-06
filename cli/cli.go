package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/health"
	"github.com/rocket-pool/smartnode/rp-regress/redact"
	"github.com/rocket-pool/smartnode/rp-regress/report"
	"github.com/rocket-pool/smartnode/rp-regress/result"
	"github.com/rocket-pool/smartnode/rp-regress/runctx"
	"github.com/rocket-pool/smartnode/rp-regress/ui"
	"github.com/spf13/pflag"
)

type CLI struct {
	Stdout io.Writer
	Stderr io.Writer
	Args   []string
	Env    func(string) string
}

func (c *CLI) Run() int {
	if len(c.Args) < 2 {
		c.printHelp()
		return 1
	}

	subCmd := c.Args[1]

	switch subCmd {
	case "run":
		return c.runRunCmd(c.Args[2:])
	case "fixture":
		return c.runFixtureCmd(c.Args[2:])
	case "-h", "--help", "help":
		c.printHelp()
		return 0
	default:
		fmt.Fprintf(c.Stderr, "Unknown subcommand %q\n", subCmd)
		c.printHelp()
		return 1
	}
}

func (c *CLI) printHelp() {
	fmt.Fprintln(c.Stdout, `rp-regress — Rocket Pool Smartnode Release Regression Harness

Examples:
  rp-regress run --release v1.23.0 --profile besu --checkpoint-url https://checkpoint-sync.hoodi.ethpandaops.io
  rp-regress fixture --name empty-jwt --profile besu

Common flags:
  -h, --help      Show this help
  --no-input      Refuse all prompts`)
}

func (c *CLI) runRunCmd(args []string) int {
	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	fs.SetOutput(c.Stderr)

	release := fs.String("release", "", "Smartnode release tag")
	profile := fs.String("profile", "", "Profile name (e.g. besu, geth)")
	checkpoint := fs.String("checkpoint-url", "", "Checkpoint sync URL")
	noInput := fs.Bool("no-input", false, "Refuse all prompts")
	help := fs.BoolP("help", "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			return 0
		}
		return 1
	}

	if *help {
		fmt.Fprintln(c.Stdout, "rp-regress run — test a Smartnode release artifact")
		fmt.Fprintln(c.Stdout, "\nExamples:")
		fmt.Fprintln(c.Stdout, "  rp-regress run --release v1.23.0 --profile besu --checkpoint-url https://... ")
		fs.PrintDefaults()
		return 0
	}

	if *release == "" || *profile == "" || *checkpoint == "" {
		fmt.Fprintln(c.Stderr, "Error: --release, --profile, and --checkpoint-url are required.")
		return 1
	}

	return c.executeStep("run", false, *noInput, *checkpoint)
}

func (c *CLI) runFixtureCmd(args []string) int {
	fs := pflag.NewFlagSet("fixture", pflag.ContinueOnError)
	fs.SetOutput(c.Stderr)

	name := fs.String("name", "", "Fixture name (e.g. empty-jwt)")
	profile := fs.String("profile", "", "Profile name")
	noInput := fs.Bool("no-input", false, "Refuse all prompts")
	help := fs.BoolP("help", "h", false, "Show help")

	if err := fs.Parse(args); err != nil {
		if err == pflag.ErrHelp {
			return 0
		}
		return 1
	}

	if *help {
		fmt.Fprintln(c.Stdout, "rp-regress fixture — run a specific failure fixture")
		fmt.Fprintln(c.Stdout, "\nExamples:")
		fmt.Fprintln(c.Stdout, "  rp-regress fixture --name empty-jwt --profile geth")
		fs.PrintDefaults()
		return 0
	}

	if *name == "" || *profile == "" {
		fmt.Fprintln(c.Stderr, "Error: --name and --profile are required.")
		return 1
	}

	return c.executeStep("fixture", true, *noInput, "")
}

func (c *CLI) executeStep(mode string, isFixture bool, noInput bool, checkpointURL string) int {
	fStdErr, _ := c.Stderr.(*os.File)
	capErr := ui.DetectCapability(fStdErr, ui.ColorAuto, c.Env)

	// Signal handling for graceful cleanup on Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	if checkpointURL != "" {
		if err := health.CheckCheckpointProvider(ctx, checkpointURL); err != nil {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, fmt.Sprintf("Checkpoint provider down: %v", err)))
			return result.ExitCode(result.ClassInfrastructure, isFixture)
		}
	}

	rc, err := runctx.New()
	if err != nil {
		fmt.Fprintf(c.Stderr, "Failed to create run context: %v\n", err)
		return result.ExitCode(result.ClassHarness, isFixture)
	}
	defer rc.Cleanup()

	rd := redact.New()

	fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusInfo, "Running isolated step..."))

	cmdStr := "sleep 1"
	if floodCmd := c.Env("TEST_FLOOD_CMD"); floodCmd != "" {
		cmdStr = floodCmd
	}
	cmd := exec.Command("sh", "-c", cmdStr)

	res, err := rc.RunStep(ctx, cmd, 500*time.Millisecond, 1024*1024, rd.Redact)

	rep := &result.Report{
		ArtifactIdentity: "TODO",
		ImageIdentities:  map[string]string{},
		Profile:          "TODO",
		Network:          "TODO",
		ReproductionCmd:  "TODO",
	}

	var exitClass result.FailureClass
	if err != nil {
		if res.FailureClass == "TIMEOUT" {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Step failed: TIMEOUT"))
			exitClass = result.ClassTimeout
		} else if res.FailureClass == "CANCELLED" {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Step cancelled"))
			exitClass = result.ClassHarness
		} else {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, fmt.Sprintf("Step failed: %v", err)))
			exitClass = result.ClassProduct
		}
		rep.Result = result.Result{
			Outcome:      result.OutcomeFail,
			FailureClass: exitClass,
		}
	} else {
		fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusPass, "Step completed successfully"))
		exitClass = result.ClassSuccess
		rep.Result = result.Result{
			Outcome: result.OutcomePass,
		}
	}

	if werr := report.WriteAll(rep, rc.DataDir, "", rd); werr != nil {
		fmt.Fprintf(c.Stderr, "Failed to write reports: %v\n", werr)
		if exitClass == result.ClassSuccess {
			exitClass = result.ClassHarness
		}
	}

	return result.ExitCode(exitClass, isFixture)
}
