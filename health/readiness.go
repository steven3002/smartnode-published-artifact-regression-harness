package health

import (
	"context"
	"fmt"
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/docker"
)

// ReadinessResult represents the outcome of the readiness poll.
type ReadinessResult struct {
	Ready        bool
	Restarts     map[string]RestartSeries
	Reason       string
	FailureClass string // "TIMEOUT" or "PRODUCT"
}

// PollReadiness waits until all containers in the given project are stable (not crash-looping)
// and both EL and CL endpoints respond with correct Hoodi configuration, or the context deadline is reached.
// The `window` parameter is the duration over which a container's restart count must not increase.
// The `pollInterval` determines how often to check state.
func PollReadiness(ctx context.Context, projectName string, window time.Duration, pollInterval time.Duration) ReadinessResult {
	series := make(map[string]RestartSeries)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Identify if it's a timeout or something else
			class := "TIMEOUT"
			if ctx.Err() != context.DeadlineExceeded {
				class = "PRODUCT"
			}
			return ReadinessResult{
				Ready:        false,
				Restarts:     series,
				Reason:       "readiness deadline exceeded",
				FailureClass: class,
			}
		case <-ticker.C:
			states, err := docker.GetStackStates(ctx, projectName)
			if err != nil {
				continue // ephemeral docker error, keep trying
			}

			// Record new samples
			for name, state := range states {
				series[name] = append(series[name], state)
			}

			// Check stability
			allStable := true
			for _, s := range series {
				if s.IsCrashLooping(window) {
					// It's considered unstable. But is it just starting, or genuinely wedged?
					// A container is wedged if it's been running/restarting longer than the window
					// and hasn't stabilized. Wait, IsCrashLooping just returns !HasStabilised.
					// We only fail if the deadline is exceeded.
					// However, if we want to fail early on a clear wedged state, we might need a longer observation.
					// For now, we only fail on deadline or context cancel.
					allStable = false
				} else {
					// To be stable, it must be running
					if !s[len(s)-1].IsRunning {
						allStable = false
					}
				}
			}

			if !allStable {
				continue
			}

			// If stable, check endpoints
			if err := CheckELChainID(ctx); err != nil {
				// Endpoint not ready yet
				fmt.Printf("EL not ready: %v\n", err)
				continue
			}

			if err := CheckCLGenesis(ctx); err != nil {
				// Endpoint not ready yet
				fmt.Printf("CL not ready: %v\n", err)
				continue
			}

			// Everything is stable and endpoints are responsive
			return ReadinessResult{
				Ready:    true,
				Restarts: series,
				Reason:   "all containers stable and endpoints responsive",
			}
		}
	}
}
