package report

import (
	"encoding/json"

	"github.com/rocket-pool/smartnode/rp-regress/result"
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
