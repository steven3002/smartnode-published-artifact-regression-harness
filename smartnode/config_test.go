package smartnode_test

import (
	"context"
	"testing"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/compose"
	"github.com/rocket-pool/smartnode/rp-regress/runctx"
	"github.com/rocket-pool/smartnode/rp-regress/smartnode"
)

// binPath must be the path to the rocketpool binary for the test.
// Since tests run in the package dir, we can build it or assume it's at a specific path.
// For the regression harness, we typically rely on the artifact downloader, but here we can mock or use a provided path.
// We'll use a hardcoded path to the scratch binary we built for testing, or skip if not found.
const testBinPath = "/home/ubuntu/smartnode-release-regression/scratch/rocketpool"

func TestHeadlessConfiguration(t *testing.T) {
	// We want to test two profiles: geth-lighthouse and besu-teku
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
			_, err = smartnode.Install(ctx, rc, testBinPath)
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
			_, err = smartnode.Config(ctx, rc, testBinPath, params)
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
