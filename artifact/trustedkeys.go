// Package artifact obtains a published Smartnode release and proves its identity.
package artifact

// PinnedFingerprint is the sole trust anchor for GPG signature verification.
// This fingerprint was confirmed against the Fornax signing key published with
// the rocket-pool/smartnode v1.23.0 release and independently verified via
// gpg --verify output (RSA key 6D3E960BD402C64642A1EC84651023D62E70B5DD,
// uid "Fornax <dante@rocketpool.net>").
//
// The signing key (fornax-signing-key.asc) ships as an asset of the same
// GitHub release it signs. Importing that key and verifying against it
// proves nothing — an attacker who replaces the binary can replace the key.
// This constant is the only value accepted by VerifySignature; there is no
// code path that imports or trusts a key from any other source.
const PinnedFingerprint = "6D3E960BD402C64642A1EC84651023D62E70B5DD"
