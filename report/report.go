package report

import (
	"bytes"
	"text/template"

	"github.com/rocket-pool/smartnode/rp-regress/result"
)

const mdTemplate = `# Run Report

**Outcome:** {{.Result.Outcome}}
{{if eq .Result.Outcome "FAIL"}}**Failure Class:** {{.Result.FailureClass}}
{{if .Result.KnownIssue}}**Known Issue:** {{.Result.KnownIssue}}
{{end}}{{end}}

## Context

- **Artifact Identity:** ` + "`{{.ArtifactIdentity}}`" + `
- **Profile:** {{.Profile}}
- **Network:** {{.Network}}

## Images

{{range $k, $v := .ImageIdentities}}- **{{$k}}**: ` + "`{{$v}}`" + `
{{end}}

## Reproduction

` + "```" + `bash
{{.ReproductionCmd}}
` + "```" + `
`

func ToMarkdown(r *result.Report) ([]byte, error) {
	t, err := template.New("report").Parse(mdTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
