package health

import (
	"time"

	"github.com/rocket-pool/smartnode/rp-regress/docker"
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

	// We look for the earliest sample that shares the same restart count.
	// If the time elapsed since that sample is >= window, we are stable.
	for i := 0; i < len(s); i++ {
		sample := s[i]
		if sample.RestartCount == latest.RestartCount {
			if latest.ObservedAt.Sub(sample.ObservedAt) >= window {
				return true
			}
			// Since samples are chronological, if the first one we find with the same
			// restart count isn't old enough, no subsequent ones will be either.
			return false
		}
	}

	return false
}

// IsCrashLooping checks if a series indicates a permanent crash loop (restarts increasing over time).
// This is simply the opposite of HasStabilised when we reach a deadline, but we can also
// use it to detect runaway restarts if needed.
func (s RestartSeries) IsCrashLooping(window time.Duration) bool {
	return !s.HasStabilised(window)
}
