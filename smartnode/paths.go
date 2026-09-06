package smartnode

import "path/filepath"

// DataDirName is the Smartnode configuration directory created inside $HOME.
const DataDirName = ".rocketpool"

// SecretPath returns the engine-API secret's location for a run rooted at home.
//
// The file is created by the execution client container, which runs as root, so
// the host user can stat it but generally cannot read it.
func SecretPath(home string) string {
	return filepath.Join(home, DataDirName, "data", "secrets", "jwtsecret")
}
