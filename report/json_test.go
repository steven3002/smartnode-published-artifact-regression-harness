package report

import (
	"testing"
	"time"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

func TestJSONRoundTrip(t *testing.T) {
	orig := &result.Report{
		Timestamp: time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC),
		Artifact: result.ArtifactIdentity{
			RequestedRelease: "v1.23.0",
			ReportedVersion:  "1.23.0",
			BinarySHA256:     "e1cd355b4281f5286a3d72f718d67c9a44ddab50a2b33be566429278c8c2d212",
			SignatureValid:   true,
			KeyFingerprint:   "6D3E960BD402C64642A1EC84651023D62E70B5DD",
		},
		Images: []result.ImageIdentity{
			{Service: "eth1", Tag: "hyperledger/besu:26.8.1", Digest: "sha256:abcd", Matches: true},
		},
		Steps: []result.Step{
			{Name: "install", DurationSec: 1.5},
			{Name: "start", DurationSec: 42, ExitCode: 1, Failure: "EXITED"},
		},
		Profile:         "besu-teku",
		Network:         "hoodi",
		ReproductionCmd: "rp-regress run --profile besu-teku",
		Result: result.Result{
			Outcome:      result.OutcomeFail,
			FailureClass: result.ClassProduct,
			KnownIssue:   "JWT-001",
		},
	}

	data, err := ToJSON(orig)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	parsed, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if parsed.Artifact != orig.Artifact {
		t.Errorf("artifact identity: got %+v, want %+v", parsed.Artifact, orig.Artifact)
	}
	if parsed.Result.Outcome != orig.Result.Outcome {
		t.Errorf("outcome: got %v, want %v", parsed.Result.Outcome, orig.Result.Outcome)
	}
	if parsed.Result.KnownIssue != "JWT-001" {
		t.Errorf("known issue: got %q, want JWT-001", parsed.Result.KnownIssue)
	}
	if len(parsed.Images) != len(orig.Images) || parsed.Images[0].Digest != "sha256:abcd" {
		t.Errorf("image identities did not round-trip: %+v", parsed.Images)
	}
	if len(parsed.Steps) != len(orig.Steps) || parsed.Steps[1].Failure != "EXITED" {
		t.Errorf("steps did not round-trip: %+v", parsed.Steps)
	}
}

func TestJSONRejectsInvalidEnums(t *testing.T) {
	badOutcomeJSON := []byte(`{"result": {"outcome": "MAYBE"}}`)
	_, err := FromJSON(badOutcomeJSON)
	if err == nil {
		t.Error("Expected error for invalid outcome MAYBE, got nil")
	}

	badClassJSON := []byte(`{"result": {"outcome": "FAIL", "failure_class": "UNKNOWN_CLASS"}}`)
	_, err = FromJSON(badClassJSON)
	if err == nil {
		t.Error("Expected error for invalid failure_class UNKNOWN_CLASS, got nil")
	}

	// A known regression is metadata on a product failure, never an outcome
	// and never a failure class.
	knownRegressionJSON := []byte(`{"result": {"outcome": "KNOWN_REGRESSION"}}`)
	if _, err = FromJSON(knownRegressionJSON); err == nil {
		t.Error("KNOWN_REGRESSION was accepted as an outcome; it must be rejected")
	}

	successClassJSON := []byte(`{"result": {"outcome": "PASS", "failure_class": "SUCCESS"}}`)
	if _, err = FromJSON(successClassJSON); err == nil {
		t.Error("SUCCESS was accepted as a failure class; it must be rejected")
	}
}
