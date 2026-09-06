# Smartnode Release Regression Harness

`rp-regress` takes a **published** Rocket Pool Smartnode release artifact — not a
local build, not source — and proves whether that exact artifact still installs,
configures and runs a healthy Ethereum node stack on the Hoodi testnet.

It exists because this surface is not covered anywhere else. The Smartnode build
and unit-test workflows never start a container, render a compose file, or touch
a published artifact, so a packaging or runtime regression can ship with CI green.

## What it proves

- **Artifact identity.** The SHA-256 digest and GPG signature of the published
  Linux binary, verified against a fingerprint pinned in this repository, and the
  version the binary reports about itself.
- **Headless configuration.** That the artifact can be configured unattended
  against Hoodi and render a valid Docker Compose stack.
- **Readiness.** That the stack reaches a healthy state, judged by restart counts
  having stopped increasing rather than by an absolute count.
- **Chain identity.** That the execution client reports chain `560048` and the
  consensus client reports the Hoodi genesis, and that the engine API is
  authenticated.
- **Image identity.** That every container is running the image digest that was
  resolved and pulled, rather than only recording a mutable tag.
- **A real regression class.** That a zero-byte engine-API secret is
  unrecoverable on some execution clients and repaired on others.

## What it does not prove

It is a bounded regression harness, **not** a certification system and **not** an
official release gate. Official release blocking would require the upstream
maintainers to adopt the supplied workflow, which is outside this project's
control. It does not test Rocket Pool contracts, node registration, deposits,
validator duties, rewards, withdrawals, mainnet, or every client combination.

## The JWT finding

`start-ec.sh` in the published release contains four engine-API secret guards,
and they do not agree with each other:

| Execution client | Guard | Repairs a zero-byte secret? |
| --- | --- | --- |
| Nethermind | `[ ! -f ]` + `openssl rand -hex 32` | no |
| Besu | `[ ! -s ]` | yes |
| Reth | `[ ! -s ]` | yes |
| Erigon | `[ ! -s ]` | yes |
| Geth | *no branch exists* — the client generates its own | no |

`-f` tests that a file exists. `-s` tests that it exists **and is non-empty**.
That single character is the difference between a stack that recovers from a
partially written secret and one that never starts again.

The `empty-jwt` fixture demonstrates both halves: on `geth-lighthouse` the stack
never becomes healthy and `JWT-001` is raised; on `besu-teku` the secret is
rewritten and the stack comes up.

## Pinned signing fingerprint

```
6D3E960BD402C64642A1EC84651023D62E70B5DD
```

The signing key `fornax-signing-key.asc` is published as an asset of the release
it signs. Verifying a release with a key fetched from that same release is
circular — anyone who can replace the binary can replace the key. The key
material is therefore read from the release, but it is imported into a throwaway
keyring first and **only** trusted if its fingerprint matches the constant above.
Do not replace this pin with a download.

## Requirements

- Go (the version in `go.mod`)
- Docker, with a running daemon and permission to start containers
- GnuPG
- Outbound network access to GitHub, Docker Hub and a checkpoint provider
- Disk: the client images alone exceed 5 GB before any chain data. A GitHub
  hosted runner provides 14 GB, which is not enough headroom to be relied on;
  use a self-hosted runner or a host with tens of gigabytes free.

Run state is written under `$XDG_CACHE_HOME/rp-regress` (usually
`~/.cache/rp-regress`), **not** the system temporary directory, which is a small
tmpfs on many hosts. Override with `RP_REGRESS_WORK_DIR`.

## Usage

```bash
go build -o rp-regress ./cmd/rp-regress

# Verify a release and run a client profile
./rp-regress run --release v1.23.0 --profile geth-lighthouse

# Inject a controlled fault and assert the expected behaviour
./rp-regress fixture --name empty-jwt --profile geth-lighthouse
```

Profiles: `geth-lighthouse` (baseline) and `besu-teku` (repair control).

Useful flags: `--checkpoint-url` (provider, configurable by design),
`--out-dir` (where reports are written), `--readiness-deadline`,
`--color auto|always|never`, `--no-input`.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | the run passed, or a fixture observed what it predicted |
| 1 | product failure — the software under test did the wrong thing |
| 2 | infrastructure failure — a provider, network or registry problem |
| 3 | harness failure |
| 4 | timeout |
| 5 | a fixture did **not** observe what it predicted |

**Fixture exit codes are inverted on purpose.**
`fixture --name empty-jwt --profile geth-lighthouse` exits **0** while reporting
`FAIL`, because a product failure is exactly what that fixture predicts. It exits
`5` if the defect stops reproducing — a fixture that quietly passes when its
regression disappears is a green light that means nothing.

## Output

Each run writes to `--out-dir`:

- `report.md` — the human-readable report
- `report.json` — the same run, machine-readable, with a validated enum schema
- `results.xml` — JUnit, one test case per step plus the verdict
- `diagnostics/` — sanitised logs

Results go to stdout; progress, warnings and errors go to stderr, so redirecting
stdout yields a usable file. Secrets are redacted at the sink, including across
read boundaries, and diagnostics exclude binaries and keyring material.

## Failure classes

`PRODUCT`, `INFRASTRUCTURE`, `HARNESS`, `TIMEOUT`. The distinction is
load-bearing: a harness that blames the product for a checkpoint provider outage
teaches its readers to ignore it. A known regression is metadata attached to a
product failure, never an outcome of its own.

## Upstream workflow

`upstream-release-regression.patch` adds a `release-regression.yml` workflow to
`rocket-pool/smartnode`. It triggers on `release: published` and
`workflow_dispatch`, never on tag push: a tag is created before its assets finish
uploading, so a tag-push trigger races the upload and fails against a healthy
release. Upstream adoption is desirable but is not a completion condition for
this project.
