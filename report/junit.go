package report

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

// The JUnit element names and attributes below follow the schema Jenkins,
// GitHub Actions test reporters and pytest all accept.
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

// ToJUnit renders the run for a CI test reporter.
//
// Each recorded step becomes a test case so a reader sees where time went and
// which step failed, followed by a case carrying the run's verdict.
//
// A product defect or a timeout is a <failure>: the run reached a verdict about
// the software. An infrastructure or harness problem is an <error>: the run did
// not get far enough to say anything about the product, and reporting it as a
// test failure would blame the wrong thing.
func ToJUnit(r *result.Report) ([]byte, error) {
	cases := make([]TestCase, 0, len(r.Steps)+1)

	for _, step := range r.Steps {
		tc := TestCase{
			Name:      "step: " + step.Name,
			ClassName: "rp-regress." + r.Profile,
			Time:      formatSeconds(step.DurationSec),
		}
		if step.Failure != "" {
			tc.Failure = &Failure{
				Message: fmt.Sprintf("step %s: %s (exit %d)", step.Name, step.Failure, step.ExitCode),
				Type:    step.Failure,
				Body:    r.ReproductionCmd,
			}
		}
		if step.TruncatedBytes > 0 {
			tc.SystemOut = fmt.Sprintf("output truncated: %d bytes dropped", step.TruncatedBytes)
		}
		cases = append(cases, tc)
	}

	verdict := TestCase{
		Name:      fmt.Sprintf("%s (%s)", r.Profile, r.Network),
		ClassName: "rp-regress",
		Time:      formatSeconds(r.TotalDuration()),
	}

	failures, errors := 0, 0
	for _, tc := range cases {
		if tc.Failure != nil {
			failures++
		}
	}

	if r.Result.Outcome == result.OutcomeFail {
		msg := fmt.Sprintf("Failure class: %s", r.Result.FailureClass)
		if r.Result.KnownIssue != "" {
			msg = fmt.Sprintf("%s: %s", r.Result.KnownIssue, msg)
		}
		if r.Result.Reason != "" {
			msg += " — " + r.Result.Reason
		}

		switch r.Result.FailureClass {
		case result.ClassProduct, result.ClassTimeout:
			verdict.Failure = &Failure{
				Message: msg,
				Type:    string(r.Result.FailureClass),
				Body:    r.ReproductionCmd,
			}
			failures++
		default:
			verdict.Error = &ErrorMsg{
				Message: msg,
				Type:    string(r.Result.FailureClass),
				Body:    r.ReproductionCmd,
			}
			errors++
		}
	}

	cases = append(cases, verdict)

	suites := TestSuites{Suites: []TestSuite{{
		Name:     "Release Regression",
		Tests:    len(cases),
		Failures: failures,
		Errors:   errors,
		Time:     formatSeconds(r.TotalDuration()),
		Cases:    cases,
	}}}

	return xml.MarshalIndent(suites, "", "  ")
}

// formatSeconds renders a duration the way JUnit consumers expect.
func formatSeconds(sec float64) string {
	return strconv.FormatFloat(sec, 'f', 3, 64)
}
