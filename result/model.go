package result

type Outcome string

const (
	OutcomePass Outcome = "PASS"
	OutcomeFail Outcome = "FAIL"
)

type Result struct {
	Outcome      Outcome      `json:"outcome"`
	FailureClass FailureClass `json:"failure_class,omitempty"`
	KnownIssue   string       `json:"known_issue,omitempty"`
}

type Report struct {
	ArtifactIdentity string            `json:"artifact_identity"`
	ImageIdentities  map[string]string `json:"image_identities"`
	Profile          string            `json:"profile"`
	Network          string            `json:"network"`
	ReproductionCmd  string            `json:"reproduction_command"`
	Result           Result            `json:"result"`
}
