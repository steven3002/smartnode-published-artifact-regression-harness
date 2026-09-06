package health

import (
	"testing"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/docker"
)

// sample builds one observation at t seconds into the run.
func sample(name string, sec int, restarts int, running bool) docker.Sample {
	base := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	return docker.Sample{
		ContainerName: name,
		IsRunning:     running,
		RestartCount:  restarts,
		ObservedAt:    base.Add(time.Duration(sec) * time.Second),
	}
}

// TestHealthyColdStartIsNotACrashLoop is the false-failure case this package
// exists to prevent. A healthy beacon node restarts several times waiting for
// the execution client to write the engine-API secret, then settles. A rule
// keyed on an absolute restart count fails that healthy boot.
func TestHealthyColdStartIsNotACrashLoop(t *testing.T) {
	s := RestartSeries{
		sample("eth2", 0, 0, false),
		sample("eth2", 5, 1, false),
		sample("eth2", 10, 2, true),
		sample("eth2", 15, 3, true),
		sample("eth2", 20, 3, true),
		sample("eth2", 35, 3, true),
		sample("eth2", 50, 3, true),
	}

	if !s.HasStabilised(15 * time.Second) {
		t.Error("a boot that restarted 3 times and then settled was reported unstable")
	}
	if s.IsRunawayRestarting(30 * time.Second) {
		t.Error("a settled boot was reported as a runaway restart")
	}
}

// TestRunawayRestartIsDetected covers the opposite case: a container whose
// restart count is still climbing after a sustained observation.
func TestRunawayRestartIsDetected(t *testing.T) {
	var s RestartSeries
	for i := 0; i <= 12; i++ {
		s = append(s, sample("eth1", i*5, i, i%2 == 0))
	}

	if s.HasStabilised(15 * time.Second) {
		t.Error("a container restarting every poll was reported as stable")
	}
	if !s.IsRunawayRestarting(30 * time.Second) {
		t.Error("sustained restart growth was not detected")
	}
}

// TestRunawayNeedsSustainedObservation guards against calling a crash loop too
// early, which would turn a slow start into a false product failure.
func TestRunawayNeedsSustainedObservation(t *testing.T) {
	s := RestartSeries{
		sample("eth2", 0, 0, false),
		sample("eth2", 5, 1, false),
		sample("eth2", 10, 2, true),
	}

	if s.IsRunawayRestarting(30 * time.Second) {
		t.Error("10 seconds of restarts was called a runaway before the observation period elapsed")
	}
}

// TestStabilisedRequiresRunning keeps an exited container from counting as
// settled just because its restart count stopped moving.
func TestStabilisedRequiresRunning(t *testing.T) {
	s := RestartSeries{
		sample("eth1", 0, 2, false),
		sample("eth1", 30, 2, false),
	}

	if s.HasStabilised(15 * time.Second) {
		t.Error("a container that is not running was reported as stable")
	}
}

// TestEmptySeriesIsNotStable checks the zero value: nothing observed is not the
// same as nothing wrong.
func TestEmptySeriesIsNotStable(t *testing.T) {
	var s RestartSeries
	if s.HasStabilised(time.Second) {
		t.Error("an empty series was reported as stable")
	}
	if s.IsRunawayRestarting(time.Second) {
		t.Error("an empty series was reported as a runaway restart")
	}
}
