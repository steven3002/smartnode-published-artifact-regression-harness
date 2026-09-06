package report

import (
	"bytes"
	"text/template"

	"github.com/steven3002/smartnode-published-artifact-regression-harness/result"
)

// mdTemplate renders a report a reader can act on without opening anything else:
// the verdict, what was verified, what actually ran, and how to reproduce it.
const mdTemplate = `# Release Regression Report

**Outcome:** {{.Result.Outcome}}
{{- if eq .Result.Outcome "FAIL"}}
**Failure class:** {{.Result.FailureClass}}
{{- if .Result.KnownIssue}}
**Known issue:** {{.Result.KnownIssue}}
{{- end}}
{{- if .Result.Reason}}
**Reason:** {{.Result.Reason}}
{{- end}}
{{- end}}

- **Profile:** {{.Profile}}
- **Network:** {{.Network}}
- **Run timestamp:** {{.Timestamp.UTC.Format "2006-01-02T15:04:05Z"}}

## Artifact identity

| Field | Value |
| --- | --- |
| Requested release | ` + "`{{.Artifact.RequestedRelease}}`" + ` |
| Reported version | ` + "`{{.Artifact.ReportedVersion}}`" + ` |
| Binary SHA-256 | ` + "`{{.Artifact.BinarySHA256}}`" + ` |
| Signature verified | {{if .Artifact.SignatureValid}}yes{{else}}no{{if .Artifact.SignatureReason}} — {{.Artifact.SignatureReason}}{{end}}{{end}} |
| Trusted key fingerprint | ` + "`{{.Artifact.KeyFingerprint}}`" + ` |

The fingerprint above is pinned in this repository. The signing key is published
as an asset of the release it signs, so it is never used as the trust anchor.

## Container images

{{if .Images -}}
| Service | Tag | Resolved digest | Running image | Matches |
| --- | --- | --- | --- | --- |
{{range .Images -}}
| {{.Service}} | ` + "`{{.Tag}}`" + ` | ` + "`{{if .PlatformDigest}}{{.PlatformDigest}}{{else}}{{.Digest}}{{end}}`" + ` | ` + "`{{.RunningID}}`" + ` | {{if .Matches}}yes{{else}}**no**{{end}} |
{{end}}
Tags are mutable. The digest column records what was actually pulled, and the
match column records whether the running container used it.
{{- else -}}
No images were resolved. The run did not reach a started stack.
{{- end}}

## Steps

{{if .Steps -}}
| Step | Duration (s) | Exit | Outcome |
| --- | --- | --- | --- |
{{range .Steps -}}
| {{.Name}} | {{printf "%.1f" .DurationSec}} | {{.ExitCode}} | {{if .Failure}}{{.Failure}}{{else}}ok{{end}}{{if .TruncatedBytes}} (output truncated, {{.TruncatedBytes}} bytes dropped){{end}} |
{{end}}
{{- else -}}
No steps were recorded.
{{- end}}

## Reproduction

` + "```" + `bash
{{.ReproductionCmd}}
` + "```" + `
`

var mdParsed = template.Must(template.New("report").Parse(mdTemplate))

// ToMarkdown renders the human-readable report.
func ToMarkdown(r *result.Report) ([]byte, error) {
	var buf bytes.Buffer
	if err := mdParsed.Execute(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
