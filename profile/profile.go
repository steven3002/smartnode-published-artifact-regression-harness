// Package profile defines which execution and consensus client pair a run
// exercises, and what the harness expects of that pair.
package profile

import "fmt"

// JWTRepair describes whether the Smartnode start script repairs a zero-byte
// engine-API secret for a given execution client.
type JWTRepair int

const (
	// RepairUnknown is used for clients outside the tested matrix.
	RepairUnknown JWTRepair = iota
	// Repairs means the start script tests [ ! -s ] and rewrites an empty file.
	Repairs
	// DoesNotRepair means an existing zero-byte secret survives startup.
	DoesNotRepair
)

// Profile is one execution/consensus client pair.
type Profile struct {
	Name            string
	ExecutionClient string
	ConsensusClient string
	JWTRepairs      JWTRepair
	RepairEvidence  string
}

// registry holds the profiles the MVP supports.
//
// The repair column is the finding this harness exists to demonstrate, and each
// entry names the line of the published start script that decides it.
var registry = map[string]Profile{
	"geth-lighthouse": {
		Name:            "geth-lighthouse",
		ExecutionClient: "geth",
		ConsensusClient: "lighthouse",
		JWTRepairs:      DoesNotRepair,
		RepairEvidence:  "start-ec.sh has no JWT branch for geth; the client generates its own secret from --authrpc.jwtsecret and does not rewrite an empty file",
	},
	"besu-teku": {
		Name:            "besu-teku",
		ExecutionClient: "besu",
		ConsensusClient: "teku",
		JWTRepairs:      Repairs,
		RepairEvidence:  "start-ec.sh:303 tests [ ! -s ] and regenerates an empty secret",
	},
}

// Lookup returns the named profile.
func Lookup(name string) (Profile, error) {
	p, ok := registry[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown profile %q (supported: %s)", name, Names())
	}
	return p, nil
}

// Names lists the supported profile names in a stable order.
func Names() string {
	return "besu-teku, geth-lighthouse"
}
