package report

import (
	"encoding/xml"
	"fmt"

	"github.com/rocket-pool/smartnode/rp-regress/result"
)

// JUnit model matching standard schemas
type TestSuites struct {
	XMLName xml.Name    `xml:"testsuites"`
	Suites  []TestSuite `xml:"testsuite"`
}

type TestSuite struct {
	Name     string     `xml:"name,attr"`
	Tests    int        `xml:"tests,attr"`
	Failures int        `xml:"failures,attr"`
	Errors   int        `xml:"errors,attr"`
	Time     string     `xml:"time,attr"`
	Cases    []TestCase `xml:"testcase"`
}

type TestCase struct {
	Name      string    `xml:"name,attr"`
	ClassName string    `xml:"classname,attr"`
	Time      string    `xml:"time,attr"`
	Failure   *Failure  `xml:"failure,omitempty"`
	Error     *ErrorMsg `xml:"error,omitempty"`
	SystemOut string    `xml:"system-out,omitempty"`
}

type Failure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

type ErrorMsg struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

func ToJUnit(r *result.Report) ([]byte, error) {
	tc := TestCase{
		Name:      fmt.Sprintf("%s (%s)", r.Profile, r.Network),
		ClassName: "rp-regress",
		Time:      "0", // we could add actual time to Report if needed
	}

	if r.Result.Outcome == result.OutcomeFail {
		msg := fmt.Sprintf("Failure Class: %s", r.Result.FailureClass)
		if r.Result.KnownIssue != "" {
			msg += fmt.Sprintf(" (Known Issue: %s)", r.Result.KnownIssue)
		}
		if r.Result.FailureClass == result.ClassProduct || r.Result.FailureClass == result.ClassTimeout {
			tc.Failure = &Failure{
				Message: msg,
				Type:    string(r.Result.FailureClass),
				Body:    r.ReproductionCmd,
			}
		} else {
			// Infrastructure and Harness are typically errors, not test failures in JUnit semantics
			tc.Error = &ErrorMsg{
				Message: msg,
				Type:    string(r.Result.FailureClass),
				Body:    r.ReproductionCmd,
			}
		}
	}

	ts := TestSuite{
		Name:     "Release Regression",
		Tests:    1,
		Failures: 0,
		Errors:   0,
		Time:     "0",
		Cases:    []TestCase{tc},
	}

	if r.Result.Outcome == result.OutcomeFail {
		if r.Result.FailureClass == result.ClassProduct || r.Result.FailureClass == result.ClassTimeout {
			ts.Failures = 1
		} else {
			ts.Errors = 1
		}
	}

	suites := TestSuites{
		Suites: []TestSuite{ts},
	}

	return xml.MarshalIndent(suites, "", "  ")
}
