# tests/portability

Status: built (M7-14). The portability test of
[docs/spec/16-operations.md](../../docs/spec/16-operations.md) 11.1: it installs a released
product version like a stranger, verifies what runs and drives the whole stack through its public
API. `portability.yml` runs it after every product release on `ubuntu-24.04` and
`ubuntu-24.04-arm`; `make portability-trial` proves it on the build host against the local trial
release.

## Files

| Path | Content |
|---|---|
| `install-options` | The options of `install.sh` after `sh -s --`, one line (M7-29); the test sets `--storage` per run and adds `--bundle` for the source `release-download`. `make install-options` holds the line equal to 16 3.1. |
| `adapter.py` | The test adapter: an agent adapter on the Python SDK whose runtime is the SDK's test double `RecordingHandler`; it answers every run with `pong`. |
| `ruff.toml` | Lints `adapter.py` with the rules of `tools/`. |
| [`../../tools/portability/portability_run.py`](../../tools/portability/portability_run.py) | The runner behind `make portability` with `PORTABILITY_TAG`. |
| [`../../tools/portability/portability_trial.py`](../../tools/portability/portability_trial.py) | The local proof behind `make portability-trial`. |
| [`../../services/server/test/smoke`](../../services/server/test/smoke/README.md) | The probe `bothy-smoke` of M1-73 with the subcommands `run`, `portability` and `local-scan`. |

The runner lives in `tools/portability`, because the Python tooling of the repository (ruff,
mypy, bandit, pytest with coverage) runs there (docs/decisions.d/M7-14.md D-M7-14-6).

## Interface

| Variable | Content |
|---|---|
| `PORTABILITY_TAG` | Product tag under test, `vX.Y.Z` or `vX.Y.Z-beta.N`. Without it `make portability` is the Compose smoke test of M1-73 (`infra/compose/smoke.sh`), unchanged. |
| `PORTABILITY_INSTALL_SOURCE` | `url`: `install.sh` and the bundle from `https://github.com/$GITHUB_REPOSITORY/releases/download/$PORTABILITY_TAG/`, `install.sh` run with the documented `curl` command. `release-download`: both from `PORTABILITY_RELEASE_DIR`, run as `sh install.sh --bundle <bundle>`. |
| `PORTABILITY_RELEASE_DIR` | The folder of `gh release download <tag>`; read only with `release-download`. |

`GITHUB_REPOSITORY` names the identity of keyless signatures; it is required with `url` and
optional with `release-download` (without it only the key of the private phase can pass).

An invalid value ends the run with exit code `2` and a message that names the variable, before
anything is downloaded, pulled or installed. Trial-only, refused when `GITHUB_ACTIONS` is set:
`PORTABILITY_TRIAL_PUBLIC_KEY` (the public key of the trial), `PORTABILITY_TRIAL_STORAGES`
(`fs`, `s3` or both), `PORTABILITY_TRIAL_FAIL_AFTER_INSTALL=1` and `PORTABILITY_TRIAL_KEEP=1`
(after a failure the installation, the probe image and the temporary directory stay for
debugging; remove them by hand).
`uv run --project tools python tools/portability/portability_trial.py --cosign <path> --syft <path>
--hold` builds the trial release and serves it until stopped, and prints the variables for runs
of `make portability` by hand.

```sh
# the release of a tag, as portability.yml runs it
PORTABILITY_TAG=v0.1.0 PORTABILITY_INSTALL_SOURCE=url GITHUB_REPOSITORY=<owner>/bothy make portability

# a downloaded release
gh release download v0.1.0 --dir /tmp/rel
PORTABILITY_TAG=v0.1.0 PORTABILITY_INSTALL_SOURCE=release-download PORTABILITY_RELEASE_DIR=/tmp/rel make portability

# the local proof on the build host (Docker with buildx; never in CI)
make portability-trial [TRIAL_PLATFORMS=linux/arm64]
```

Needs Docker Engine 26 with Compose 2.26 and `uv`; nothing is skipped when one is missing.

## Steps

Per storage run (`fs`: profiles `direct,scan`; `s3`: `direct,scan,s3`):

1. Read `BOTHY_REGISTRY` and `BOTHY_VERSION` from the bundle's `.env.example`; pull every image its
   `compose.yaml` names with `${BOTHY_VERSION}` without a login; verify the digest of each with
   cosign in a container pinned by digest (the key of the private phase from the repository, or
   keyless with the identity of the release workflow). An unsigned image ends the run here.
2. `install.sh` without a terminal with the options of `install-options`; the enrollment code it
   prints goes to a file of mode `0600`, never to the log.
3. The containers of the installation must run exactly the verified images.
4. Overlay `local`: Prometheus, Loki and Tempo started, the collector exports there.
5. The smoke scenario of M1-73 (`bothy-smoke run`): owner from the setup wizard, space, channel,
   thread, messages, live WebSocket reception and catch-up, step-up, revocation.
6. The portability scenario (`bothy-smoke portability`) with a fresh code of the owner: the agent
   `echo` with a DPoP-bound token, the test adapter (`adapter.py`) connected, a mention in a thread,
   the run completed with the reply `pong`; an upload clean and one with the EICAR test file
   (`infected`); the search finds the trigger message; the status page names its components.
7. `bothy-backupd backup pg_full`, `logical` and, with `fs`, `objects`, then
   `bothy-backupd restore-test`. With `s3` the object backup is a mirror into an off-site bucket,
   which the installation of the test does not have.
8. `bothy-smoke local-scan`: everything Loki, Prometheus and Tempo received, with the container
   logs and the adapter's log, holds no canary of the run (the canary, tokens, enrollment codes,
   the agent's token); log lines, series and traces of the server must exist.
9. VZ-T13 (M7-26): Grafana of the overlay starts with the plugin build of the bundle; through its
   HTTP API on the loopback address of the container (`curl` there, the admin password read there
   and handed over on stdin) the panel plugin `bothy-live-panel` is listed with the signature
   `unsigned`, Grafana reports no plugin error, the plugin's `module.js` is served, and the
   dashboard `bothy-live` is provisioned with only installed panel types. Every line of Grafana's
   log that says unsigned and names a plugin ID names `bothy-live-panel`, and the dashboard
   provisioning logs no error. The report lists those log lines.
10. `fs` only: the tailnet case without an account; in the namespace of the `tailscale` sidecar
   the public listener listens on loopback only and the internal listener is the only one beyond
   it.

At the end the digests of the verified images and the duration per run are printed. On a failure
the run prints `docker compose ps --all` and the last 200 log lines of every service of exactly
its installation (its folder and its project `bothy-portability-<random>`), then cleans up:
containers, volumes and networks of the project, the installation folder, the probe image and the
temporary directory. A leftover object of the project fails the run.

## Tests

```sh
cd tools && uv run pytest portability          # the runner and the trial (make test)
cd services/server && go test -race ./test/smoke/...   # the probe
```

## Not proven here

- The status check on a real release candidate on amd64 and arm64: it runs with the owner's first
  product tag (D-W-125).
- The public `curl` path without a login (docs/quality/public-switch.md step 21).
- A real tailnet: it needs an account.
