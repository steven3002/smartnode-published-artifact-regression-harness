package smartnode_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/compose"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/runctx"
	"github.com/steven3002/smartnode-published-artifact-regression-harness/smartnode"
)

// binaryEnvVar names an already-verified Smartnode binary to exercise.
//
// This is an integration test against a real published artifact, so it is opt-in
// rather than skipped-by-default on a hardcoded path: a test that silently
// depends on a hand-placed file passes or fails for reasons unrelated to the code.
const binaryEnvVar = "RP_REGRESS_TEST_BINARY"

func TestHeadlessConfiguration(t *testing.T) {
	testBinPath := os.Getenv(binaryEnvVar)
	if testBinPath == "" {
		t.Skipf("set %s to a verified rocketpool binary to run this integration test", binaryEnvVar)
	}
	if _, err := os.Stat(testBinPath); err != nil {
		t.Fatalf("%s=%s is not usable: %v", binaryEnvVar, testBinPath, err)
	}

	// Both MVP profiles must configure headlessly.
	profiles := []struct {
		ec string
		cc string
	}{
		{"geth", "lighthouse"},
		{"besu", "teku"},
	}

	for _, p := range profiles {
		t.Run(p.ec+"-"+p.cc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			rc, err := runctx.New()
			if err != nil {
				t.Fatalf("failed to create run context: %v", err)
			}
			defer rc.Cleanup()

			// 1. Install templates
			_, err = smartnode.Install(ctx, rc, testBinPath, nil)
			if err != nil {
				t.Fatalf("Install failed: %v", err)
			}

			// 2. Configure headless
			params := smartnode.ConfigParams{
				ExecutionClient: p.ec,
				ConsensusClient: p.cc,
				Network:         smartnode.NetworkHoodi,
				CheckpointURL:   smartnode.DefaultCheckpointURL,
			}
			_, err = smartnode.Config(ctx, rc, testBinPath, params, nil)
			if err != nil {
				t.Fatalf("Config failed: %v", err)
			}

			// 3. Render compose stack
			stack, err := compose.Render(ctx, rc, testBinPath)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			// 4. Validate chain identity in the compose stack
			// The BEACON_NETWORK should be "hoodi" in eth1 and eth2 containers
			eth1, ok := stack.Services["eth1"]
			if !ok {
				t.Fatalf("eth1 service missing in stack")
			}

			if eth1.Environment["BEACON_NETWORK"] != smartnode.BeaconNetworkHoodi {
				t.Errorf("expected BEACON_NETWORK %s, got %s", smartnode.BeaconNetworkHoodi, eth1.Environment["BEACON_NETWORK"])
			}
			if eth1.Environment["NETWORK"] != smartnode.NetworkHoodi {
				t.Errorf("expected NETWORK %s, got %s", smartnode.NetworkHoodi, eth1.Environment["NETWORK"])
			}

			// Validate images are present
			images := stack.ExtractImages()
			if len(images) == 0 {
				t.Errorf("failed to extract any images from stack")
			}
		})
	}
}
