package smartnode

import (
	"context"
	"os/exec"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
)

// ConfigParams holds the required settings for a headless Smartnode configuration.
type ConfigParams struct {
	ExecutionClient string
	ConsensusClient string
	Network         string
	CheckpointURL   string
}

// configDeadline bounds headless configuration.
//
// A missed prompt does not present as a clean hang here: the Smartnode CLI
// prompt loop has no EOF check, so it floods output instead. The output cap in
// RunStep is what contains that; this deadline ends it.
const configDeadline = 2 * time.Minute

// Config applies the headless configuration for one client profile.
//
// MEV-Boost is disabled explicitly because it is not supported on Hoodi and
// leaving it at its default adds a service the profile does not exercise.
func Config(ctx context.Context, rc *runctx.RunContext, binPath string, params ConfigParams, redactFn func([]byte) []byte) (runctx.StepResult, error) {
	args := []string{
		"service", "config",
		"--executionClientMode", "local",
		"--consensusClientMode", "local",
		"--executionClient", params.ExecutionClient,
		"--consensusClient", params.ConsensusClient,
		"--smartnode-network", params.Network,
		"--consensusCommon-checkpointSyncUrl", params.CheckpointURL,
		"--enableMevBoost=false",
	}

	cmd := exec.Command(binPath, args...)
	return rc.RunStep(ctx, "configure", cmd, configDeadline, 4*1024*1024, redactFn)
}
