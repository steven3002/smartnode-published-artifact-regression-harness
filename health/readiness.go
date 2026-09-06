package health

import (
	"context"
	"fmt"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/docker"
)

// ReadinessFailure names why a stack did not become ready.
type ReadinessFailure string

const (
	// FailureNone is set when the stack became ready.
	FailureNone ReadinessFailure = ""
	// FailureTimeout means the deadline expired while the stack was still
	// coming up. That is not by itself a defect.
	FailureTimeout ReadinessFailure = "TIMEOUT"
	// FailureCrashLoop means a container kept restarting after a sustained
	// observation, which the stack is not expected to recover from.
	FailureCrashLoop ReadinessFailure = "CRASH_LOOP"
	// FailureCancelled means the run was interrupted.
	FailureCancelled ReadinessFailure = "CANCELLED"
	// FailureDiagnosed means a definitive cause was identified while waiting,
	// so there was no reason to keep waiting for it.
	FailureDiagnosed ReadinessFailure = "DIAGNOSED"
)

// ReadinessResult is the outcome of a readiness poll, including the restart
// history that produced it.
type ReadinessResult struct {
	Ready    bool
	Restarts map[string]RestartSeries
	Reason   string
	Failure  ReadinessFailure

	// Diagnosis is the code returned by the caller's Diagnose function, set only
	// when Failure is FailureDiagnosed.
	Diagnosis string
}

// runawayFactor sets how long a container may keep accumulating restarts,
// relative to the stability window, before it is called a crash loop.
//
// It is a multiple rather than a constant so a caller that widens the window for
// a slow host widens the patience with it.
const runawayFactor = 6

// PollReadiness waits for every container in the project to settle and for both
// client endpoints to answer.
//
// Stability is judged by restart counts having stopped increasing over window,
// never by an absolute count: a healthy cold start restarts the beacon node
// several times while the execution client writes the engine-API secret, so an
// absolute threshold reports healthy boots as failures.
//
// Opts carries the callbacks the poll needs from its caller.
type Opts struct {
	// Observe receives progress lines. It must not write to stdout: the
	// caller's results stream is not a log.
	Observe func(string)

	// Diagnose is consulted once the containers have settled but the endpoints
	// are still not answering. It returns a code and reason when it can name a
	// definitive cause, and false otherwise.
	//
	// This exists because a container can keep running while the client inside
	// it fails: the restart count plateaus, nothing looks like a crash loop, and
	// the poll would otherwise wait out the full deadline to report a cause that
	// was knowable in seconds. It is only consulted after the stack has settled,
	// so a slow but healthy start is never cut short by it.
	Diagnose func() (code string, reason string, ok bool)
}

func PollReadiness(
	ctx context.Context,
	projectName string,
	window time.Duration,
	pollInterval time.Duration,
	opts Opts,
) ReadinessResult {
	observe := opts.Observe
	if observe == nil {
		observe = func(string) {}
	}

	series := make(map[string]RestartSeries)
	runaway := runawayFactor * window

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			failure := FailureTimeout
			reason := "readiness deadline exceeded"
			if ctx.Err() != context.DeadlineExceeded {
				failure = FailureCancelled
				reason = "run was interrupted before the stack became ready"
			}
			return ReadinessResult{Restarts: series, Reason: reason, Failure: failure}

		case <-ticker.C:
			states, err := docker.GetStackStates(ctx, projectName)
			if err != nil {
				// Transient docker errors are expected while the stack is being
				// created; keep sampling rather than judging on one failure.
				continue
			}

			for name, state := range states {
				series[name] = append(series[name], state)
			}

			// An empty sample set is not a settled stack; it means the
			// containers do not exist yet.
			if len(series) == 0 {
				continue
			}

			settled := true
			for name, s := range series {
				if s.IsRunawayRestarting(runaway) {
					return ReadinessResult{
						Restarts: series,
						Reason: fmt.Sprintf("container %s is still restarting after %s (%d restarts)",
							name, runaway, s[len(s)-1].RestartCount),
						Failure: FailureCrashLoop,
					}
				}
				if s.IsCrashLooping(window) || !s[len(s)-1].IsRunning {
					settled = false
				}
			}
			if !settled {
				continue
			}

			if err := CheckELChainID(ctx); err != nil {
				observe(fmt.Sprintf("execution client not answering yet: %v", err))
				if res, ok := diagnosed(opts, series); ok {
					return res
				}
				continue
			}
			if err := CheckCLGenesis(ctx); err != nil {
				observe(fmt.Sprintf("consensus client not answering yet: %v", err))
				if res, ok := diagnosed(opts, series); ok {
					return res
				}
				continue
			}

			return ReadinessResult{
				Ready:    true,
				Restarts: series,
				Reason:   "all containers settled and both client endpoints responsive",
			}
		}
	}
}

// diagnosed asks the caller whether the settled-but-unresponsive stack has a
// definitive cause, and builds the result if so.
func diagnosed(opts Opts, series map[string]RestartSeries) (ReadinessResult, bool) {
	if opts.Diagnose == nil {
		return ReadinessResult{}, false
	}
	code, reason, ok := opts.Diagnose()
	if !ok {
		return ReadinessResult{}, false
	}
	return ReadinessResult{
		Restarts:  series,
		Reason:    reason,
		Failure:   FailureDiagnosed,
		Diagnosis: code,
	}, true
}
