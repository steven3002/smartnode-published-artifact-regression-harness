package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/artifact"
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

	return c.executeStep("run", false, *noInput, *checkpoint, *release, *profile)
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

	return c.executeStep("fixture", true, *noInput, "", "", *profile)
}

func (c *CLI) executeStep(mode string, isFixture bool, noInput bool, checkpointURL, releaseTag, profileName string) int {
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

	fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusInfo, "Fetching release "+releaseTag+"..."))
	rel, err := artifact.FetchRelease(releaseTag)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Failed to fetch release: %v\n", err)
		return result.ExitCode(result.ClassInfrastructure, isFixture)
	}

	assets, err := artifact.ResolveAssetSet(rel, "rocketpool-cli-linux-amd64")
	if err != nil {
		fmt.Fprintf(c.Stderr, "Failed to resolve assets: %v\n", err)
		return result.ExitCode(result.ClassInfrastructure, isFixture)
	}

	binPath, sigPath, err := artifact.DownloadAssetSet(assets, rc.DataDir)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Failed to download assets: %v\n", err)
		return result.ExitCode(result.ClassInfrastructure, isFixture)
	}

	var keyAsset *artifact.ReleaseAsset
	for _, a := range rel.Assets {
		if a.Name == "fornax-signing-key.asc" {
			aCopy := a
			keyAsset = &aCopy
			break
		}
	}
	if keyAsset == nil {
		fmt.Fprintf(c.Stderr, "Signing key asset not found in release\n")
		return result.ExitCode(result.ClassInfrastructure, isFixture)
	}
	
	keyPath, err := artifact.Download(*keyAsset, rc.DataDir)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Failed to download signing key: %v\n", err)
		return result.ExitCode(result.ClassInfrastructure, isFixture)
	}
	
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Failed to read signing key: %v\n", err)
		return result.ExitCode(result.ClassInfrastructure, isFixture)
	}
	
	if err := artifact.ImportPinnedKey(keyData, rc.DataDir); err != nil {
		fmt.Fprintf(c.Stderr, "ImportPinnedKey failed: %v\n", err)
		return result.ExitCode(result.ClassInfrastructure, isFixture)
	}

	fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusInfo, "Verifying artifact..."))
	verReport, err := artifact.Verify(ctx, binPath, sigPath, rc.DataDir, releaseTag, assets.DigestHex)
	if err != nil {
		fmt.Fprintf(c.Stderr, "Verification failed: %v\n", err)
		return result.ExitCode(result.ClassProduct, isFixture)
	}
	if !verReport.VersionMatch || !verReport.DigestMatch || !verReport.SignatureValid {
		fmt.Fprintln(c.Stderr, "Verification failed (content)")
		return result.ExitCode(result.ClassProduct, isFixture)
	}

	cliPath := filepath.Join(rc.DataDir, "rocketpool")
	if err := os.Rename(binPath, cliPath); err != nil {
		fmt.Fprintf(c.Stderr, "Rename failed: %v\n", err)
		return result.ExitCode(result.ClassHarness, isFixture)
	}

	parts := strings.Split(profileName, "-")
	if len(parts) != 2 {
		fmt.Fprintf(c.Stderr, "Invalid profile %q\n", profileName)
		return result.ExitCode(result.ClassHarness, isFixture)
	}
	ecClient, ccClient := parts[0], parts[1]

	scriptPath := filepath.Join(rc.DataDir, "run.sh")
	scriptContent := fmt.Sprintf(`#!/bin/sh
set -e
export PATH="%s:$PATH"
rocketpool service install -d --yes
rocketpool service config --smartnode-network testnet --enableMevBoost=false --executionClient %s --consensusClient %s --consensusCommon-checkpointSyncUrl "%s"
rocketpool service start --yes --ignore-slash-timer
`, rc.DataDir, ecClient, ccClient, checkpointURL)
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	cmd := exec.Command("sh", scriptPath)

	fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusInfo, "Running setup & start..."))
	res, err := rc.RunStep(ctx, cmd, 2*time.Minute, 100*1024*1024, rd.Redact)

	rep := &result.Report{
		ArtifactIdentity: verReport.BinarySHA256,
		ImageIdentities:  map[string]string{},
		Profile:          profileName,
		Network:          "hoodi",
		ReproductionCmd:  fmt.Sprintf("rp-regress run --release %s --profile %s --checkpoint-url %s", releaseTag, profileName, checkpointURL),
	}

	var exitClass result.FailureClass
	if err != nil {
		if res.FailureClass == "TIMEOUT" {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Setup failed: TIMEOUT"))
			exitClass = result.ClassTimeout
		} else if res.FailureClass == "CANCELLED" {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Setup cancelled"))
			exitClass = result.ClassHarness
		} else {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, fmt.Sprintf("Setup failed: %v", err)))
			fmt.Fprintln(c.Stderr, "Setup Output:\n"+string(res.Output))
			exitClass = result.ClassProduct
		}
		rep.Result = result.Result{
			Outcome:      result.OutcomeFail,
			FailureClass: exitClass,
		}
	} else {
		// Poll readiness
		fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusInfo, "Polling readiness..."))
		pollCtx, pollCancel := context.WithTimeout(ctx, 5*time.Minute)
		readyRes := health.PollReadiness(pollCtx, "rocketpool", 15*time.Second, 5*time.Second)
		pollCancel()

		if !readyRes.Ready {
			exitClass = result.ClassProduct
			if readyRes.FailureClass == "TIMEOUT" {
				exitClass = result.ClassTimeout
			}
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Readiness failed: "+readyRes.Reason))
			rep.Result = result.Result{
				Outcome:      result.OutcomeFail,
				FailureClass: exitClass,
			}
		} else {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusPass, "Stack is ready!"))
			
			// Verify EL & CL network
			if err := health.CheckELChainID(ctx); err != nil {
				fmt.Fprintf(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Invalid EL chain ID: %v\n"), err)
				exitClass = result.ClassProduct
				rep.Result = result.Result{Outcome: result.OutcomeFail, FailureClass: exitClass}
			} else {
				fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusPass, "EL chain ID matches Hoodi"))
			}

			if err := health.CheckCLGenesis(ctx); err != nil {
				fmt.Fprintf(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Failed to get CL genesis: %v\n"), err)
				exitClass = result.ClassProduct
				rep.Result = result.Result{Outcome: result.OutcomeFail, FailureClass: exitClass}
			} else {
				fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusPass, "CL genesis verified"))
			}

			// Verify JWT
			jwtBytes, jwtErr := exec.Command("docker", "exec", "rocketpool_eth1", "cat", "/secrets/jwtsecret").Output()
			if jwtErr != nil {
				fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Failed to read JWT from container"))
				exitClass = result.ClassProduct
				rep.Result = result.Result{
					Outcome:      result.OutcomeFail,
					FailureClass: exitClass,
				}
			} else {
				jwtStr := strings.TrimSpace(string(jwtBytes))
				fmt.Fprintf(c.Stderr, "JWT: %s (len %d)\n", jwtStr, len(jwtStr))
				
				// JWT regex: ^(0x)?[0-9a-fA-F]{64}$
				matched, _ := regexp.MatchString(`^(0x)?[0-9a-fA-F]{64}$`, jwtStr)
				if !matched {
					fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusFail, "Invalid JWT format"))
					exitClass = result.ClassProduct
					rep.Result = result.Result{
						Outcome:      result.OutcomeFail,
						FailureClass: exitClass,
					}
				} else {
					exitClass = result.ClassSuccess
					rep.Result = result.Result{
						Outcome: result.OutcomePass,
					}
				}
			}
		}
	}

	// Teardown
	fmt.Fprintln(c.Stderr, ui.FormatStatus(capErr, ui.StatusInfo, "Tearing down..."))
	teardownScript := filepath.Join(rc.DataDir, "teardown.sh")
	os.WriteFile(teardownScript, []byte(fmt.Sprintf(`#!/bin/sh
export PATH="%s:$PATH"
export HOME="%s"
rocketpool service terminate --yes -d
`, rc.DataDir, rc.HomeDir)), 0755)
	exec.Command("sh", teardownScript).Run()

	if werr := report.WriteAll(rep, rc.DataDir, "", rd); werr != nil {
		fmt.Fprintf(c.Stderr, "Failed to write reports: %v\n", werr)
		if exitClass == result.ClassSuccess {
			exitClass = result.ClassHarness
		}
	}

	return result.ExitCode(exitClass, isFixture)
}
