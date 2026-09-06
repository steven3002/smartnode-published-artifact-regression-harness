// Package result is the model every run converges on. It knows nothing about
// how a run is rendered.
package result

import "time"

// Outcome is whether the run passed. It is the only place pass or fail is
// recorded.
type Outcome string

const (
	OutcomePass Outcome = "PASS"
	OutcomeFail Outcome = "FAIL"
)

// Result is the verdict for one run.
//
// FailureClass is empty when Outcome is OutcomePass: a passing run has no
// failure to classify, and a second field that can also express success would
// let the two disagree.
type Result struct {
	Outcome      Outcome      `json:"outcome"`
	FailureClass FailureClass `json:"failure_class,omitempty"`

	// KnownIssue references a catalogued defect, such as JWT-001. It describes
	// a product failure; it is never a substitute for an outcome.
	KnownIssue string `json:"known_issue,omitempty"`

	// Reason is the human-readable explanation shown with a failure.
	Reason string `json:"reason,omitempty"`
}

// Pass builds a passing result.
func Pass() Result { return Result{Outcome: OutcomePass} }

// Fail builds a failing result of the given class.
func Fail(class FailureClass, reason string) Result {
	return Result{Outcome: OutcomeFail, FailureClass: class, Reason: reason}
}

// WithIssue attaches a known-issue reference to a failure.
func (r Result) WithIssue(code string) Result {
	r.KnownIssue = code
	return r
}

// ArtifactIdentity records what was verified, and how.
//
// Every field is required in the report: a run that cannot say which key it
// trusted has not proved anything about the binary it executed.
type ArtifactIdentity struct {
	RequestedRelease string `json:"requested_release"`
	ReportedVersion  string `json:"reported_version"`
	BinarySHA256     string `json:"binary_sha256"`
	SignatureValid   bool   `json:"signature_valid"`
	SignatureReason  string `json:"signature_reason,omitempty"`
	KeyFingerprint   string `json:"trusted_key_fingerprint"`
}

// ImageIdentity records what actually ran, rather than what was requested.
//
// Tags are mutable, so a report listing only tags proves nothing about the
// bytes that executed.
type ImageIdentity struct {
	Service        string `json:"service"`
	Tag            string `json:"tag"`
	Digest         string `json:"digest,omitempty"`
	PlatformDigest string `json:"platform_digest,omitempty"`
	LocalID        string `json:"local_id,omitempty"`
	RunningID      string `json:"running_id,omitempty"`
	Matches        bool   `json:"matches"`
}

// Step is one bounded unit of work, recorded whether or not it failed.
type Step struct {
	Name           string  `json:"name"`
	DurationSec    float64 `json:"duration_seconds"`
	ExitCode       int     `json:"exit_code"`
	TruncatedBytes int64   `json:"truncated_bytes,omitempty"`
	Failure        string  `json:"failure,omitempty"`
}

// Report is one run, complete.
type Report struct {
	Timestamp       time.Time        `json:"timestamp"`
	Artifact        ArtifactIdentity `json:"artifact"`
	Images          []ImageIdentity  `json:"images"`
	Steps           []Step           `json:"steps"`
	Profile         string           `json:"profile"`
	Network         string           `json:"network"`
	ReproductionCmd string           `json:"reproduction_command"`
	Result          Result           `json:"result"`
}

// TotalDuration sums the recorded steps.
func (r *Report) TotalDuration() float64 {
	var total float64
	for _, s := range r.Steps {
		total += s.DurationSec
	}
	return total
}
