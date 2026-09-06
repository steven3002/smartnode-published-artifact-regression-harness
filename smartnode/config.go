package smartnode

import (
	"context"
	"os/exec"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/runctx"
)

// ConfigParams holds the required settings for a headless Smartnode configuration.
type ConfigParams struct {
	ExecutionClient string
	ConsensusClient string
	Network         string
	CheckpointURL   string
}

// Config runs the headless configuration of the Smartnode.
// It uses the provided binary path and executes `rocketpool service config`
// with the necessary flags to set the clients, network, and checkpoint URL.
func Config(ctx context.Context, rc *runctx.RunContext, binPath string, params ConfigParams) (runctx.StepResult, error) {
	args := []string{
		"service", "config",
		"--executionClientMode", "local",
		"--consensusClientMode", "local",
		"--executionClient", params.ExecutionClient,
		"--consensusClient", params.ConsensusClient,
		"--smartnode-network", params.Network,
		"--consensusCommon-checkpointSyncUrl", params.CheckpointURL,
	}

	cmd := exec.Command(binPath, args...)
	// Configuration is a fast local operation, but we give it 30 seconds to be safe.
	// We allow up to 1MB of output, though it should be minimal.
	return rc.RunStep(ctx, cmd, 30*time.Second, 1024*1024, nil)
}
