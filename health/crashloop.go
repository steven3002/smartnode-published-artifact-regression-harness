package health

import (
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/docker"
)

// RestartSeries is a chronological list of observations for a single container.
type RestartSeries []docker.Sample

// HasStabilised answers whether the container's restart count has stopped increasing
// over the given window. It returns true if the container is currently running,
// and there is at least one observation at least `window` ago with the exact same
// restart count as the most recent observation.
func (s RestartSeries) HasStabilised(window time.Duration) bool {
	if len(s) == 0 {
		return false
	}

	latest := s[len(s)-1]
	if !latest.IsRunning {
		return false
	}

	// Find the earliest sample sharing the latest restart count. If that sample
	// is at least a window old, the count has stopped increasing.
	for i := 0; i < len(s); i++ {
		sample := s[i]
		if sample.RestartCount == latest.RestartCount {
			if latest.ObservedAt.Sub(sample.ObservedAt) >= window {
				return true
			}
			// Samples are chronological, so if the first match is not old
			// enough, no later one will be either.
			return false
		}
	}

	return false
}

// IsCrashLooping reports that a container has not yet settled.
//
// This is not on its own a reason to fail: a healthy cold start restarts the
// beacon node several times while it waits for the execution client to write
// the engine-API secret. Use IsRunawayRestarting to distinguish a stack that is
// still coming up from one that never will.
func (s RestartSeries) IsCrashLooping(window time.Duration) bool {
	return !s.HasStabilised(window)
}

// IsRunawayRestarting reports that a container is still accumulating restarts
// after being observed for at least the given period.
//
// The distinction from IsCrashLooping is the whole point of this package. A
// container that restarted three times in its first thirty seconds and then
// settled is healthy. One whose count is still climbing after a sustained
// observation is not, and waiting for the deadline to say so wastes the
// difference between a fast verdict and a slow one.
func (s RestartSeries) IsRunawayRestarting(observation time.Duration) bool {
	if len(s) < 2 {
		return false
	}

	latest := s[len(s)-1]
	oldest := s[0]
	if latest.ObservedAt.Sub(oldest.ObservedAt) < observation {
		return false
	}

	// Compare against the oldest sample still inside the observation period, so
	// restarts from a long-settled start-up do not count against a container
	// that has since become stable.
	cutoff := latest.ObservedAt.Add(-observation)
	baseline := oldest
	for _, sample := range s {
		if sample.ObservedAt.After(cutoff) {
			break
		}
		baseline = sample
	}

	return latest.RestartCount > baseline.RestartCount
}
