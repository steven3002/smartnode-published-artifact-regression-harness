package cli

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/smartnode"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/ui"
)

// options is everything a run needs, after parsing.
type options struct {
	release       string
	profile       string
	checkpointURL string
	outDir        string
	fixtureName   string
	noInput       bool
	colorMode     ui.ColorMode
	readyDeadline time.Duration
}

// commonFlags registers the flags shared by run and fixture.
//
// The checkpoint URL is a flag with a default rather than a constant: a run
// must be able to move to another provider without a rebuild, and provider
// choice is a documented precondition of the baseline.
func commonFlags(fs *pflag.FlagSet, o *options) {
	fs.StringVar(&o.profile, "profile", "", "client profile (geth-lighthouse, besu-teku)")
	fs.StringVar(&o.checkpointURL, "checkpoint-url", smartnode.DefaultCheckpointURL, "checkpoint sync provider")
	fs.StringVar(&o.outDir, "out-dir", ".", "directory to write report.md, report.json, results.xml and diagnostics/")
	fs.BoolVar(&o.noInput, "no-input", false, "refuse every prompt; fail instead of asking")
	fs.DurationVar(&o.readyDeadline, "readiness-deadline", 30*time.Minute, "how long the stack may take to become ready")
	fs.String("color", string(ui.ColorAuto), "colour output: auto, always or never")
}

// resolveColor applies the --color flag, which outranks the environment.
func resolveColor(fs *pflag.FlagSet, o *options) error {
	raw, err := fs.GetString("color")
	if err != nil {
		return err
	}
	switch ui.ColorMode(raw) {
	case ui.ColorAuto, ui.ColorAlways, ui.ColorNever:
		o.colorMode = ui.ColorMode(raw)
		return nil
	default:
		return fmt.Errorf("invalid --color %q: want auto, always or never", raw)
	}
}
