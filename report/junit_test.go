package report

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

func TestJUnitOutput(t *testing.T) {
	r := &result.Report{
		Profile:         "besu",
		Network:         "hoodi",
		ReproductionCmd: "rp-regress run",
		Result: result.Result{
			Outcome:      result.OutcomeFail,
			FailureClass: result.ClassProduct,
			KnownIssue:   "JWT-001",
		},
	}

	data, err := ToJUnit(r)
	if err != nil {
		t.Fatalf("ToJUnit failed: %v", err)
	}

	// Validate it can unmarshal back into the struct
	var parsed TestSuites
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Invalid XML generated: %v\n%s", err, string(data))
	}

	if len(parsed.Suites) != 1 {
		t.Fatalf("Expected 1 suite, got %d", len(parsed.Suites))
	}

	suite := parsed.Suites[0]
	if suite.Failures != 1 {
		t.Errorf("Expected 1 failure, got %d", suite.Failures)
	}

	if len(suite.Cases) != 1 {
		t.Fatalf("Expected 1 case, got %d", len(suite.Cases))
	}

	tc := suite.Cases[0]
	if tc.Failure == nil {
		t.Fatalf("Expected failure node, got nil")
	}

	if tc.Failure.Type != "PRODUCT" {
		t.Errorf("Expected failure type PRODUCT, got %s", tc.Failure.Type)
	}

	if !strings.Contains(tc.Failure.Message, "JWT-001") {
		t.Errorf("Expected message to contain KnownIssue, got %s", tc.Failure.Message)
	}
}

func TestJUnitInfrastructureIsError(t *testing.T) {
	r := &result.Report{
		Profile:         "besu",
		Network:         "hoodi",
		ReproductionCmd: "rp-regress run",
		Result: result.Result{
			Outcome:      result.OutcomeFail,
			FailureClass: result.ClassInfrastructure,
		},
	}

	data, err := ToJUnit(r)
	if err != nil {
		t.Fatalf("ToJUnit failed: %v", err)
	}

	var parsed TestSuites
	xml.Unmarshal(data, &parsed)

	suite := parsed.Suites[0]
	if suite.Failures != 0 {
		t.Errorf("Expected 0 failures, got %d", suite.Failures)
	}
	if suite.Errors != 1 {
		t.Errorf("Expected 1 error, got %d", suite.Errors)
	}
	if suite.Cases[0].Error == nil {
		t.Fatal("Expected error node, got nil")
	}
	if suite.Cases[0].Error.Type != "INFRASTRUCTURE" {
		t.Errorf("Expected error type INFRASTRUCTURE, got %s", suite.Cases[0].Error.Type)
	}
}
