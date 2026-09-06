package report

import (
	"encoding/json"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

func ToJSON(r *result.Report) ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func FromJSON(data []byte) (*result.Report, error) {
	var r result.Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
