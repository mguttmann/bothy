# AUD-misc-ci-gates-01 Workflow gates: runs of main, the copies in build-verify, uv, upstream code in a container

Work package of the audit of 2026-11-11 for the workflows of the gates (`.github/workflows/ci.yml`,
`build-verify.yml`, `hermes-pin.yml`, `advisories.yml`, `.github/CODEOWNERS`). Findings:
`ci-gates-18` (ruling 1), `ci-gates-3` (ruling 2), `ci-gates-9` (ruling 3), `ci-gates-5`
(ruling 4), `ci-gates-1` (rulings 5 and 6), `ci-gates-6` (ruling 7), `ci-gates-19` (ruling 8).
Five are fixed, one is recorded with its open question, one needs no change.

| No | Date | Ruling | Reason |
|---|---|---|---|
| AUD-CG1/1 | 2026-11-12 | `cancel-in-progress` of `ci.yml` is `github.event_name == 'pull_request'` (before: every event but `push`). The group is unchanged. Only a newer run of the same pull request cancels a run in progress; a push and a manual run of `main` share the group of the ref and wait for the run before them. `tools/ci/tests/test_ci_ok.py` evaluates the group and the expression for a push, a manual run, a pull request and a merge group. | A manual run of `main` joined the group of the push run with a true `cancel-in-progress` and cancelled it: push run 40540821549 of 2026-10-27 lost its two Apple jobs when manual run 40557518657 started, and SonarQube got no analysis of that commit, which the comment above the setting ruled out. The event in the group name would also have fixed it, but then a manual run of `main` runs beside the push run instead of after it, and the push runs of `main` are queued on purpose (merges come faster than one full run). A waiting run of `main` can still be replaced by a newer one, by a push as before or now by a manual run; that run is of the same or a newer commit. A second manual run of a branch now waits instead of cancelling the first. |
| AUD-CG1/2 | 2026-11-12 | A job that `ci.yml` and `build-verify.yml` both have keeps the time limit, the runner, the condition, the matrix and every make command of `ci.yml`, held by `test_a_job_of_both_workflows_keeps_the_limits_and_make_commands_of_ci` for every shared job id except `plan` and `denylist`, which differ on purpose. `build-verify.yml` changes to that: `apps (iOS)` 90 minutes (PL/57), `test (server)` and `test (apps)` 60 minutes, `AREAS=none` in `test (apps)` and `test (SDKs and plugins)`. Not built: the composite action for the toolchain block and `build-verify.yml` as a caller of `ci.yml`. | The audit found three places where the copy had drifted, and the test held only the set of make targets and the steps of the platform jobs. The limit of `apps (iOS)` was the one of before PL/57, so a stalled start that `ci.yml` survives ended the job of a build branch; without `AREAS=none` the tools suite ran in three jobs. The two limits of 90 minutes had no ruling and the same commands as the jobs of `ci.yml`, which pass in 60. The test failed before the change for `apps-ios`, `test-apps`, `test-sdk-plugins` and `test-server`. The shared building blocks are the redesign that AUD-MISC-MISC-01/12 records with its size; nothing is added to it here. The step that resolves the tag again in every job of `release.yml` with a release environment is no copy to remove: each such job must resolve on its own runner (T-SC-05, rule `release-untrusted-output` of `tools/ci/check_runners.py`). |
| AUD-CG1/3 | 2026-11-12 | `gradle/actions/wrapper-validation` stays one step of the supply chain job and is now also a step of the job `security` of `build-verify.yml`, the same step in both (`tools/ci/tests/test_gradle_wrapper_validation.py`). The jobs that call `./gradlew` (`test (apps)`, the Android jobs) do not wait for it and get no step of their own. The comment in `ci.yml`, the row of gates.md and the test header say that the step is a gate of the merge through `ci-ok`, not of the run. A test holds that every job of `release.yml` with a JDK waits for the gate stage, which is `ci.yml`. | The comment promised a check "before any step can run it", which held only inside its own job. Making it a precondition of the other jobs buys nothing: in a pull request those jobs run the build scripts of the pull request anyway, so a changed wrapper gains no reach it did not have, and none of them holds a secret; what the step must prevent is a changed wrapper on `main`, and `ci-ok` does that. The one job that runs a wrapper with a signing key (`build-android` of `release.yml`) already waits for the whole workflow. `build-verify.yml` promises every gate of the required CI and lacked this one; the test failed for it before the change. |
| AUD-CG1/4 | 2026-11-12 | Every `astral-sh/setup-uv` step names `version: "0.13.7"`, the same exact version in every workflow (`test_every_job_installs_the_same_exact_uv`); toolchain.md lists it, held by `tools/docs/tests/test_toolchain_doc.py`. A developer's host keeps its own uv: no `required-version` in a `uv.toml`. Unchanged: `node-version: "24"` and `--python 3.14`. Not built: a lock with hashes for Semgrep. | Without `version` the action installs the newest release at the time of the run: the log of the job `plan` of run 43153132839 says "Could not determine uv version from uv.toml or pyproject.toml. Falling back to latest." uv installs every hash-locked Python tool, also in the jobs with a release environment, so a release of the last hour ran there before any cooldown. 0.13.7 is the version those runs used, published on 2026-11-03, older than `deps.min_release_age`. Renovate reads the input in its github-actions manager (`known-actions/github-releases.ts` of renovatebot/renovate, package `astral-sh/uv`, retrieved 2026-11-12), so the pin moves with the group "GitHub Actions" after the cooldown. A `required-version` would also bind every developer's uv to one exact version and stop a host with another one. Node is no download of the newest version: `setup-node` finds `24` in the tool cache of the runner image ("Found in cache @ /opt/hostedtoolcache/node/24.22.2/x64" in the same run), so an exact version would add a download whenever the image differs. Python follows uv, whose release freezes the list of versions it installs. |
| AUD-CG1/5 | 2026-11-12 | The job `record` of `hermes-pin.yml` runs `make -C plugins/hermes-adapter record` and `test` through `tools/ci/sandbox.sh`, in a container built from `tools/ci/sandbox.Dockerfile`: the ids of the runner's user and never root, `--cap-drop ALL`, `no-new-privileges`, no Docker socket, its own process and mount namespaces, the checkout mounted read-only with `.git`, writable only `plugins/hermes-adapter` and a home outside the checkout, uv mounted from the job. Before the command starts, a check inside the container ends with exit code 70 when it runs as root, can gain privileges, has a capability left, sees the Docker socket or can write the checkout. The network stays open. | The upstream code of the newest main commit ran as a plain step, once a day, in a run on `main`. Such a step runs as the user that owns the runner's files and has sudo without a password: it could read the credentials of the run from the runner's processes and write cache entries of `main`, which the jobs of `ci.yml` of every branch restore, the job with `SONAR_TOKEN` among them. `enable-cache: false` only kept the job itself from saving a cache. Taking sudo away would not have been enough: code of the same user can still change the files of the actions that run after it. The container shows it neither the processes nor the files of the runner. Everything the job runs after the upstream code comes from a place the container could not write: the script, its Dockerfile and uv are outside the writable folder. Tried on 2026-11-12 with Docker on Linux over a clone of `main` and uv 0.13.7: inside, a user id that is not 0, `CapEff` and `CapBnd` zero, `NoNewPrivs: 1`, no process in sight but its own, `touch` on the checkout and on `.git` refused; `make -C plugins/hermes-adapter record` and `make -C plugins/hermes-adapter test` pass for the pinned commit in the container, the tracked files differ afterwards only in the fixture folder, and the hand-over step copies every fixture file. |
| AUD-CG1/6 | 2026-11-12 | What the container leaves in `plugins/hermes-adapter` is untrusted for the job. A step copies only regular files named `[a-z0-9_]+.json` from the fixture folder of the checkout itself (the resolved path must be the one of the checkout) to a folder of the runner, and only that copy is uploaded. The header of the workflow says what holds. Not covered and not built: see "Open". | The upload followed whatever the upstream code left at the fixture path, so a link to a file of the runner would have been read by the upload step with the rights of the runner and handed to the job that commits. The job `propose` filters again, but after the upload a link is a regular file. `tools/ci/tests/test_sandbox.py` runs the real step against a link to a file outside, a fixture folder that is a link, and a foreign name: each fails the step. The header promised that no cache other workflows restore was in reach, which was the false sentence the audit names. |
| AUD-CG1/7 | 2026-11-12 | gates.md gets the section "Red runs": keeping `main` green and reading the runs of `main` and of the scheduled workflows is a duty of the maintainers; "the watcher" and "the keeper" of D-CSMQ-1 and PL/25 are named there as parts of it; it lists the scheduled workflows (held by `tools/docs/tests/test_red_runs_doc.py`) and says that only `denylist-main.yml` reports its own red run. gates.md and `advisories.yml` no longer name a keeper. Not built: an alert for a red scheduled run or a red run of `ci.yml` on `main`; "Open" names the question. | The finding is real: `hermes-pin.yml` failed on its schedule on four days in a row from 2026-11-09 (runs 42556605447, 42749433831, 42952829192, 43148253007, each at the same step) and nothing in the repository reported it; GitHub notifies one person for a scheduled run, the one who created the workflow or last changed its `cron` line. Two words for roles that no document defined were the part that could be fixed at once. The alert is a new surface and partly a decision of the owner: it writes public issues without a person in between, for runs that are known to be red today (V1-07, V1-08, V1-13), it needs either an alert job in each of these workflows with their guard tests or one workflow on the trigger `workflow_run`, which no workflow uses so far, and it cannot run before it is on `main`. |
| AUD-CG1/8 | 2026-11-12 | No change for `ci-gates-19`: `.github/CODEOWNERS` keeps `@OWNER` until V1-03, and no ruleset asks for a review by a code owner. | The finding names no defect of this package and says so itself. That `ci-ok` decides with the workflows and the plan of the pull request, and that no second person reviews, is the documented state of a project with one maintainer (push-discipline.md, "Protection of `main` (ruleset)"; SEC-ALERTS/10). The placeholder is the plan row V1-03 with its test, the ruleset the plan row V1-05 with public-switch.md items 14 and 22; a review by code owners needs a second person and a repository setting, which is the owner's step. |

## Open

For the next run in each area. None of these is a known wrong result of the product.

Upstream code of the Hermes adapter (rulings 5 and 6):

- The container shares the kernel of the runner; a flaw of the kernel or of the container
  runtime is outside what it holds. Its network is open, because the recording downloads the
  pinned commit and its packages: the code can send what it can read, which is the public
  checkout.
- After a maintainer merges a proposal, the same pin runs in `test (SDKs and plugins)` of
  `ci.yml` as a plain step, in the run of the pull request and then on `main`. The sandbox
  covers the daily run that no person starts. Running the tests of `plugins/hermes-adapter` in
  the same container in `ci.yml` is possible with the same script; it changes a required job
  and its coverage hand-over to `sonar` and is best done alone.
- The pinned binaries under `.cache/tools` are checked against a stamp in the same cache
  (`tools/bin/fetch_tools.py`); the check against the hash of `tools/bin/tools.lock.json` is the
  finding `ci-gates-10` of another work package.
- The image holds what the trial found the recording and the tests to need beside uv (git,
  make, `ps`, a Python on the search path) and installs it from the current Debian archive,
  without a snapshot. A tool that a later test needs and the image lacks shows as a failed
  test step of the proposal, not as a failed sandbox.
- The first run of the changed job on a hosted runner is the first scheduled run after the
  merge; a manual run on a branch would push the proposal branch from that branch. The
  proposal itself still fails at its last step until the repository setting "Allow GitHub
  Actions to create and approve pull requests" is on (D-HERMES-PIN-4), an owner step.

Red runs (ruling 7):

- Question for the owner: should a red scheduled run, and a red run of `ci.yml` on `main`, open
  or update a public issue as `denylist-main.yml` does? Recommendation: yes for the scheduled
  workflows, with one issue per workflow and a label of its own; for `ci.yml` on `main` only
  once `ci` is green there (V1-08), because until then every push would add a comment. Size: a
  script after `tools/denylist/main_alert.py` with its tests, a label, and either an `alert` job
  in each workflow (the guard tests of `codeql.yml` and of the platform workflows fix their job
  lists) or one workflow on `workflow_run` that reads only the conclusion, the name and the link
  of the finished run and checks out nothing of it.

Tool versions (ruling 4):

- Semgrep runs through `uvx --from semgrep==<version>`: its own version is pinned, the packages
  it depends on are resolved at run time without hashes. A lock of its own (a small uv project
  for Semgrep with `uv.lock`) brings a second Python lock into the lists of the advisory and
  license gates and their tests.

CI manifests (ruling 2): the item of AUD-misc-misc-01 "Open" stands unchanged.

## Evidence

- Ruling 1: `test_a_run_of_main_in_progress_is_never_cancelled` failed before the change for a
  manual run after a push run and after a manual run.
- Ruling 2: `test_a_job_of_both_workflows_keeps_the_limits_and_make_commands_of_ci` failed
  before the change for four jobs.
- Ruling 3: `test_the_security_job_validates_every_tracked_wrapper[build-verify.yml]` and
  `test_both_workflows_pin_the_same_validation_step` failed before the change.
- Ruling 4: `test_every_job_installs_the_same_exact_uv` failed before the change for every
  step.
- Rulings 5 and 6: `tools/ci/tests/test_sandbox.py` runs the real script with a docker program
  that records its arguments, runs the check of the script on the test host (which is no such
  container, so the command must not start) and runs the real hand-over step of the workflow.
  The run in a real container is the trial above and, from the merge on, every run of the job.
