# FIX-apt-snapshot-retry The host image of the terminal suite survives snapshot.debian.org outages

| No | Date | Ruling | Reason |
|---|---|---|---|
| FAS/1 | 2026-11-05 | `infra/compose/test/host/Dockerfile` runs `apt-get update` and `apt-get install` in one bounded loop of six attempts: three on the pinned `snapshot.debian.org` timestamp, then three on the current trixie packages of `deb.debian.org`, announced on stderr. The snapshot stays the first source, so the package set stays the pinned one whenever the snapshot answers (D-M2-48-3). | Main runs 42082056885 (shards 2 to 5 and `console-e2e`) and 42081964628 (shard 2) failed in the RUN of the host image, in `termsuite-images` and `console-e2e` (there with APT's exit code 100; the cut error of `termsuite-images` shows no code). A local build of the same Dockerfile showed the cause: `snapshot.debian.org` answered single package files with `503 No healthy backends`, `503 first byte timeout`, `504 Gateway Timeout` or a failed connection. Retries alone do not help: a loop of six attempts over eleven minutes failed on the same files (cron, strace) every time, and a local build of the new Dockerfile against the real snapshot needed the fallback too. |
| FAS/2 | 2026-11-05 | APT's own retries are off in that image (`Acquire::Retries "0"`); the loop retries instead. | With APT's own retries (the default, and `5` before), APT 3.0 of trixie printed "Tried to start delayed item ... but failed" up to 1.4 million times while the snapshot failed, and one run grew to 12 GB until the kernel killed it (OOM). With retries off, the same downloads stayed under 70 MB. |
| FAS/3 | 2026-11-05 | `stack.HostImage` adds the last three APT error lines (`E: ...`) of the build log to its error. | The error of the failed runs named only the cut RUN instruction, so the CI log showed no reason for the failure. |
| FAS/4 | 2026-11-05 | `tools/ci/tests/test_dockerfile_fetch_retry.py` holds every RUN that installs from `snapshot.debian.org` to FAS/1 and FAS/2 (the loop and the retries off). APT against `deb.debian.org` stays unchecked (PL/33). | Second failure of the class "a transient download error fails an image build" after PL/33, which assumed that APT retries by itself. No job failed on `deb.debian.org`. |

## Open minors

- `tools/console-e2e/console_e2e.py` builds the host image with `--quiet`, so its log still shows only the RUN instruction on a failure.
- Every shard of a run builds the host image on its own and fetches from the snapshot again; a CI cache of the image would spare the snapshot.
