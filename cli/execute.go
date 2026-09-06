package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/artifact"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/compose"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/docker"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/health"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/jwt"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/profile"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/redact"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/report"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/smartnode"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/ui"
)

// restartWindow is how long container restart counts must stop increasing
// before a stack is judged stable.
//
// A healthy cold start restarts the beacon node several times: its entrypoint
// exits non-zero until the execution client has written the engine-API secret.
// Judging by an absolute restart count fails those healthy boots.
const restartWindow = 15 * time.Second

// pollInterval is how often container state is sampled.
const pollInterval = 5 * time.Second

// composeProject is the label Smartnode gives the stack it generates.
const composeProject = "rocketpool"

// execute runs the pipeline, tears down, writes the reports and returns the
// process exit status.
func (c *CLI) execute(o options) int {
	stderrFile, _ := c.Stderr.(*os.File)
	cap := ui.DetectCapability(stderrFile, o.colorMode, c.Env)

	prof, err := profile.Lookup(o.profile)
	if err != nil {
		fmt.Fprintln(c.Stderr, "error:", err)
		return result.ExitHarness
	}

	rep := &result.Report{
		Timestamp:       time.Now().UTC(),
		Profile:         prof.Name,
		Network:         smartnode.BeaconNetworkHoodi,
		ReproductionCmd: reproduction(o, prof.Name),
		Artifact:        result.ArtifactIdentity{RequestedRelease: o.release},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusWarn, "interrupted; tearing down"))
		cancel()
	}()

	rc, err := runctx.New()
	if err != nil {
		fmt.Fprintln(c.Stderr, "error:", err)
		return result.ExitHarness
	}

	redactor := redact.New()
	binPath := ""

	res := c.pipeline(ctx, o, prof, rc, rep, cap, redactor, &binPath)
	rep.Result = res

	captureContainerLogs(context.WithoutCancel(ctx), rc)
	c.teardown(ctx, rc, binPath, redactor, cap)

	if err := report.WriteAll(rep, rc.DataDir, o.outDir, redactor); err != nil {
		fmt.Fprintf(c.Stderr, "error: failed to write reports: %v\n", err)
		if res.Outcome == result.OutcomePass {
			rep.Result = result.Fail(result.ClassHarness, "reports could not be written")
			res = rep.Result
		}
	} else {
		fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusInfo, "reports written to "+o.outDir))
	}

	if err := rc.Cleanup(); err != nil {
		fmt.Fprintf(c.Stderr, "warning: cleanup incomplete: %v\n", err)
	}

	c.summarise(res, cap)

	if kind := fixtureKind(o, prof); kind != result.FixtureNone {
		return result.FixtureExitCode(res, kind)
	}
	return result.ExitCode(res)
}

// pipeline performs the run and returns its verdict. Steps are appended to rep
// as they complete so a failure still reports everything that ran before it.
func (c *CLI) pipeline(
	ctx context.Context,
	o options,
	prof profile.Profile,
	rc *runctx.RunContext,
	rep *result.Report,
	cap ui.Capability,
	redactor *redact.Redactor,
	binPathOut *string,
) result.Result {
	say := func(status ui.Status, text string) {
		fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, status, text))
	}

	say(ui.StatusInfo, "checking checkpoint provider "+o.checkpointURL)
	if err := health.CheckCheckpointProvider(ctx, o.checkpointURL); err != nil {
		return result.Fail(result.ClassInfrastructure,
			fmt.Sprintf("checkpoint provider %s is unreachable: %v", o.checkpointURL, err))
	}

	say(ui.StatusInfo, "fetching release "+o.release)
	rel, err := artifact.FetchRelease(o.release)
	if err != nil {
		return result.Fail(result.ClassInfrastructure, fmt.Sprintf("fetch release: %v", err))
	}

	assets, err := artifact.ResolveAssetSet(rel, "rocketpool-cli-linux-amd64")
	if err != nil {
		return result.Fail(result.ClassInfrastructure, fmt.Sprintf("resolve assets: %v", err))
	}

	binPath, sigPath, err := artifact.DownloadAssetSet(assets, rc.DataDir)
	if err != nil {
		return result.Fail(result.ClassInfrastructure, fmt.Sprintf("download release assets: %v", err))
	}

	if res := c.trustSigningKey(rel, rc, cap); res != nil {
		return *res
	}

	say(ui.StatusInfo, "verifying artifact")
	verification, err := artifact.Verify(ctx, binPath, sigPath, rc.DataDir, o.release, assets.DigestHex)
	if verification != nil {
		rep.Artifact = result.ArtifactIdentity{
			RequestedRelease: o.release,
			ReportedVersion:  verification.ReportedVersion,
			BinarySHA256:     verification.BinarySHA256,
			SignatureValid:   verification.SignatureValid,
			SignatureReason:  verification.SignatureReason,
			KeyFingerprint:   verification.KeyFingerprint,
		}
	}
	if err != nil {
		return result.Fail(result.ClassProduct, fmt.Sprintf("artifact verification: %v", err))
	}
	if !verification.DigestMatch || !verification.SignatureValid || !verification.VersionMatch {
		return result.Fail(result.ClassProduct, "artifact verification failed: "+verification.SignatureReason)
	}
	say(ui.StatusPass, "artifact verified against pinned key "+artifact.PinnedFingerprint)

	cliPath := filepath.Join(rc.DataDir, "rocketpool")
	if err := os.Rename(binPath, cliPath); err != nil {
		return result.Fail(result.ClassHarness, fmt.Sprintf("stage verified binary: %v", err))
	}
	*binPathOut = cliPath

	say(ui.StatusInfo, "installing templates")
	step, err := smartnode.Install(ctx, rc, cliPath, redactor.Redact)
	if res := record(rep, rc, step, err); res != nil {
		return *res
	}

	say(ui.StatusInfo, "configuring for Hoodi")
	cfg := smartnode.ConfigParams{
		ExecutionClient: prof.ExecutionClient,
		ConsensusClient: prof.ConsensusClient,
		Network:         smartnode.NetworkHoodi,
		CheckpointURL:   o.checkpointURL,
	}
	step, err = smartnode.Config(ctx, rc, cliPath, cfg, redactor.Redact)
	if res := record(rep, rc, step, err); res != nil {
		return *res
	}

	secretPath := smartnode.SecretPath(rc.HomeDir)
	if o.fixtureName == fixtureEmptyJWT {
		if err := jwt.PlantEmpty(secretPath); err != nil {
			return result.Fail(result.ClassHarness, fmt.Sprintf("plant fixture: %v", err))
		}
		say(ui.StatusWarn, "fixture: planted a zero-byte engine-API secret")
	}

	say(ui.StatusInfo, "starting stack (first run pulls client images)")
	step, err = smartnode.Start(ctx, rc, cliPath,
		smartnode.StartServiceParams{Yes: true, IgnoreSlashTimer: true}, redactor.Redact)
	if res := record(rep, rc, step, err); res != nil {
		return *res
	}

	c.recordImages(ctx, rc, cliPath, rep, cap)

	say(ui.StatusInfo, "polling readiness")
	readyCtx, cancelReady := context.WithTimeout(ctx, o.readyDeadline)
	defer cancelReady()

	waits := newWaitReporter(c.Stderr, cap)
	defer waits.done()

	readiness := health.PollReadiness(readyCtx, composeProject, restartWindow, pollInterval, health.Opts{
		Observe: waits.report,

		// A malformed engine-API secret is a definitive answer: the clients
		// cannot authenticate to each other and no amount of further waiting
		// changes that.
		Diagnose: func() (string, string, bool) {
			state, err := jwt.Inspect(ctx, secretPath)
			if err != nil || state.Diagnosis() == "" {
				return "", "", false
			}
			return state.Diagnosis(),
				"stack did not become healthy; the engine-API secret is empty or malformed",
				true
		},
	})

	waits.done()

	if !readiness.Ready {
		if readiness.Diagnosis == jwt.CodeMalformed {
			say(ui.StatusFail, jwt.MalformedMessage)
			return result.Fail(result.ClassProduct, readiness.Reason).WithIssue(jwt.CodeMalformed)
		}
		if state, err := jwt.Inspect(ctx, secretPath); err == nil && state.Diagnosis() != "" {
			say(ui.StatusFail, jwt.MalformedMessage)
			return result.Fail(result.ClassProduct,
				"stack did not become healthy; the engine-API secret is empty or malformed").
				WithIssue(jwt.CodeMalformed)
		}
		switch readiness.Failure {
		case health.FailureTimeout:
			return result.Fail(result.ClassTimeout, "stack did not become ready: "+readiness.Reason)
		case health.FailureCancelled:
			return result.Fail(result.ClassHarness, readiness.Reason)
		default:
			return result.Fail(result.ClassProduct, "stack did not become ready: "+readiness.Reason)
		}
	}
	say(ui.StatusPass, "stack is ready")

	if err := health.CheckELChainID(ctx); err != nil {
		return result.Fail(result.ClassProduct, fmt.Sprintf("execution client is not on Hoodi: %v", err))
	}
	say(ui.StatusPass, fmt.Sprintf("execution client reports chain %d", smartnode.ChainIDHoodi))

	if err := health.CheckCLGenesis(ctx); err != nil {
		return result.Fail(result.ClassProduct, fmt.Sprintf("consensus client genesis mismatch: %v", err))
	}
	say(ui.StatusPass, "consensus client reports the Hoodi genesis")

	if err := health.CheckELCLAuth(ctx); err != nil {
		return result.Fail(result.ClassProduct, fmt.Sprintf("engine API is not authenticated: %v", err))
	}
	say(ui.StatusPass, "engine API authenticated")

	state, err := jwt.Inspect(ctx, secretPath)
	if err != nil {
		return result.Fail(result.ClassHarness, fmt.Sprintf("inspect engine-API secret: %v", err))
	}
	if code := state.Diagnosis(); code != "" {
		say(ui.StatusFail, jwt.MalformedMessage)
		return result.Fail(result.ClassProduct, "engine-API secret is empty or malformed").WithIssue(code)
	}
	if !state.Present {
		return result.Fail(result.ClassProduct, "no engine-API secret was created").WithIssue(jwt.CodeMalformed)
	}
	say(ui.StatusPass, "engine-API secret is well formed")

	return result.Pass()
}

// trustSigningKey imports the release's signing key only if its fingerprint is
// the one pinned in this repository.
//
// The key ships as an asset of the release it signs, so the key material is
// fetched from there but is never the trust anchor: a fingerprint that does not
// match the pin means the release was signed by something unexpected, which is
// a finding about the product rather than an infrastructure problem.
func (c *CLI) trustSigningKey(rel *artifact.Release, rc *runctx.RunContext, cap ui.Capability) *result.Result {
	var keyAsset *artifact.ReleaseAsset
	for i := range rel.Assets {
		if rel.Assets[i].Name == "fornax-signing-key.asc" {
			keyAsset = &rel.Assets[i]
			break
		}
	}
	if keyAsset == nil {
		res := result.Fail(result.ClassProduct, "release publishes no signing key asset")
		return &res
	}

	keyPath, err := artifact.Download(*keyAsset, rc.DataDir)
	if err != nil {
		res := result.Fail(result.ClassInfrastructure, fmt.Sprintf("download signing key: %v", err))
		return &res
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		res := result.Fail(result.ClassHarness, fmt.Sprintf("read signing key: %v", err))
		return &res
	}
	if err := artifact.ImportPinnedKey(keyData, rc.DataDir); err != nil {
		res := result.Fail(result.ClassProduct, fmt.Sprintf("signing key is not the pinned key: %v", err))
		return &res
	}
	return nil
}

// recordImages resolves every image in the generated stack to a registry digest
// and records what the running containers are actually using.
//
// A failure here is reported but does not end the run: image provenance is
// evidence about a stack that is already up, and losing it should not discard
// the verdict the run was started to reach.
func (c *CLI) recordImages(ctx context.Context, rc *runctx.RunContext, cliPath string, rep *result.Report, cap ui.Capability) {
	stack, err := compose.Render(ctx, rc, cliPath)
	if err != nil {
		fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusWarn, "could not render compose stack: "+err.Error()))
		return
	}

	identities, err := docker.VerifyStackImages(ctx, stack, runtime.GOARCH, runtime.GOOS)
	if err != nil {
		fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusWarn, "could not resolve image digests: "+err.Error()))
		return
	}

	for _, id := range identities {
		rep.Images = append(rep.Images, result.ImageIdentity{
			Service:        id.ServiceName,
			Tag:            id.Tag,
			Digest:         id.IndexDigest,
			PlatformDigest: id.PlatformDigest,
			LocalID:        id.LocalImageID,
			RunningID:      id.LocalImageID,
			Matches:        id.Matches,
		})
		if !id.Matches {
			fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusWarn, "image identity mismatch: "+id.Mismatch))
		}
	}
	fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusPass,
		fmt.Sprintf("recorded %d image identities", len(identities))))
}

// teardown stops the stack. It runs on every exit path, including a failure
// that never reached a started stack.
func (c *CLI) teardown(ctx context.Context, rc *runctx.RunContext, binPath string, redactor *redact.Redactor, cap ui.Capability) {
	if binPath == "" {
		return
	}
	fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusInfo, "tearing down"))

	// A cancelled context must not prevent teardown; that is when it matters most.
	teardownCtx := context.WithoutCancel(ctx)
	if _, err := smartnode.Terminate(teardownCtx, rc, binPath, redactor.Redact); err != nil {
		fmt.Fprintf(c.Stderr, "warning: teardown reported an error: %v\n", err)
	}
}

// summarise prints the one line a reader looks for.
func (c *CLI) summarise(res result.Result, cap ui.Capability) {
	if res.Outcome == result.OutcomePass {
		fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusPass, "PASS"))
		return
	}
	line := fmt.Sprintf("FAIL (%s)", res.FailureClass)
	if res.KnownIssue != "" {
		line = fmt.Sprintf("FAIL (%s, %s)", res.FailureClass, res.KnownIssue)
	}
	if res.Reason != "" {
		line += ": " + res.Reason
	}
	fmt.Fprintln(c.Stderr, ui.FormatStatus(cap, ui.StatusFail, line))
}

// record appends a completed step and converts a step failure into a verdict.
func record(rep *result.Report, rc *runctx.RunContext, step runctx.StepResult, err error) *result.Result {
	captureStepOutput(rc, step)

	rep.Steps = append(rep.Steps, result.Step{
		Name:           step.Name,
		DurationSec:    step.Duration.Seconds(),
		ExitCode:       step.ExitCode,
		TruncatedBytes: step.TruncatedBytes,
		Failure:        string(step.Failure),
	})
	if err == nil {
		return nil
	}

	var res result.Result
	switch step.Failure {
	case runctx.StepTimeout:
		res = result.Fail(result.ClassTimeout, fmt.Sprintf("step %s exceeded its deadline", step.Name))
	case runctx.StepCancelled:
		res = result.Fail(result.ClassHarness, fmt.Sprintf("step %s was cancelled", step.Name))
	default:
		res = result.Fail(result.ClassProduct,
			fmt.Sprintf("step %s failed with exit %d", step.Name, step.ExitCode))
	}
	return &res
}

// fixtureKind states what the fixture predicts for this profile.
func fixtureKind(o options, prof profile.Profile) result.FixtureKind {
	if o.fixtureName != fixtureEmptyJWT {
		return result.FixtureNone
	}
	if prof.JWTRepairs == profile.Repairs {
		return result.FixtureRepair
	}
	return result.FixtureDefect
}

// reproduction renders the command that reproduces this run.
func reproduction(o options, profileName string) string {
	if o.fixtureName != "" {
		return fmt.Sprintf("rp-regress fixture --name %s --profile %s --release %s --checkpoint-url %s",
			o.fixtureName, profileName, o.release, o.checkpointURL)
	}
	return fmt.Sprintf("rp-regress run --release %s --profile %s --checkpoint-url %s",
		o.release, profileName, o.checkpointURL)
}
