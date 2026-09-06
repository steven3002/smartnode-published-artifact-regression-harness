# Smartnode Release Regression Harness

This is a bounded regression harness for Rocket Pool's Smartnode. It is designed to verify the correctness of execution-layer (EL) and consensus-layer (CL) configurations across different client profiles on the Hoodi testnet.

## What this harness proves

This harness provides reproducible evidence for:
- **Release Artifact Integrity:** Verifies the cryptographic signatures and SHA-256 digests of the published release binaries.
- **Image Identity:** Confirms that the Docker images used in the stack exactly match the expected manifest index digests.
- **Stack Readiness:** Ensures that the entire Smartnode stack (EL + CL) can successfully reach a healthy, synced state on the Hoodi testnet.
- **Targeted Regression Checks:** Executes specific fixtures (e.g., the `empty-jwt` fixture) to prove exactly how Smartnode scripts handle defects, confirming whether they crash or successfully self-repair.

## What this harness does NOT prove

This harness is **not** a certification system and **not** an official release gate. Official release blocking would require upstream to adopt the workflow, which is outside the author's control. It is explicitly bounded to the Hoodi testnet and a specific subset of client combinations (currently Geth+Lighthouse and Besu+Teku). It does not perform long-running operational monitoring, performance benchmarking, or test every permutation of the Smartnode configuration.

## Pinned Signing Fingerprint

The GPG fingerprint for the Rocket Pool release signing key (Fornax) is pinned in this repository:
`6D3E960BD402C64642A1EC84651023D62E70B5DD`

**Why is it pinned here?**
The signing key (`fornax-signing-key.asc`) is shipped *inside* the release that it signs. Verifying a release using a key fetched from that same release is a circular chain of trust. To prevent a compromised release from simply substituting its own key, the expected fingerprint must be pinned out-of-band in this repository.

## Runner Requirements

These requirements are based on factual measurements taken on a self-hosted runner, not estimates. A hosted GitHub standard runner (2 vCPU, 7 GB RAM, 14 GB SSD) meets these requirements.

- **CPU:** 2 vCPU
- **RAM:** 7 GB (Observed peak memory usage for the heaviest profile, Besu + Teku, is ~3.5 GB used total, leaving ample headroom without swap).
- **Disk:** ~6.0 GB (Approximately 5.6 GB for `rocketpool_eth1clientdata` and 450 MB for `rocketpool_eth2clientdata` during initial checkpoint sync window).
- **Wall-clock Time:** Approximately 70–80 seconds to reach readiness from a cold start (measured on Geth + Lighthouse).

## Reproduction Steps

To reproduce a run from a clean directory:

1. Clone this repository:
   ```bash
   git clone https://github.com/rocket-pool/smartnode-release-regression.git
   cd smartnode-release-regression
   ```

2. Build the harness:
   ```bash
   go build -o rp-regress .
   ```

3. Execute a normal profile run:
   ```bash
   ./rp-regress run --release v1.23.0 --profile geth-lighthouse --checkpoint-url https://checkpoint-sync.hoodi.ethpandaops.io
   ```

4. Execute a fixture run:
   ```bash
   ./rp-regress fixture --name empty-jwt --profile geth-lighthouse
   ```

All commands must output `PASS`. Reports will be generated in `report.json`, `report.md`, and `results.xml`, with sanitized logs stored in the `diagnostics/` directory.
