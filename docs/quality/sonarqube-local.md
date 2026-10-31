# SonarQube locally

The local SonarQube Community Build rates product code before the push with the same
profile and gate as SonarQube Cloud (ADR 0040). It runs from
`tools/sonarqube/compose.yaml`: web interface only on `127.0.0.1`, PostgreSQL only in
an internal network without a host port.

## Prerequisites

* A Docker-compatible runtime with Compose.
* Memory for the container VM according to the vendor's figures: the Community Build
  names 4 GB for up to one million lines
  (https://docs.sonarsource.com/sonarqube-community-build/server-installation/server-host-requirements/,
  retrieved 2026-10-10) [verified].
* The port `quality.sonar.local_port` (environment variable `SONARQUBE_PORT`) is free on the
  host. If it is taken, set `SONARQUBE_PORT` and choose `SONAR_HOST_URL` to match.
  The examples below use the default port.

## 1. Start

```sh
make sonar-up
```

On the first run, the target creates `tools/sonarqube/.env` with a random
database password (file permissions for the owner only, git-ignored), starts both
containers and waits until `api/system/status` reports the status `UP`, at most
`quality.sonar.startup_timeout_s`. The data is in named volumes; `make sonar-down`
stops without deleting them.

The built-in search engine otherwise requires kernel settings of the host
(`vm.max_map_count`); for local single use, the Compose file switches off this
startup check, as the vendor's quick start guide does
(https://docs.sonarsource.com/sonarqube-community-build/try-out-sonarqube/, retrieved
2026-10-10) [verified].

## 2. Change the admin password

Open `http://127.0.0.1:9000`, log in with `admin`/`admin` (vendor default, same
source) and change the password right away. The new password belongs in the password manager,
never in a file in the repository.

## 3. Create tokens without displaying them

Two tokens with separate permissions:

| Token | Type | For what |
|---|---|---|
| Configuration | `USER_TOKEN` of the admin, with expiry date | once for `make sonar-config`, revoke afterwards |
| Analysis | `GLOBAL_ANALYSIS_TOKEN` | `make sonar` and `make check` |

The types are named by the web API (`api/user_tokens/generate`, parameter `type`) [verified on the
vendor's public server, https://next.sonarqube.com/sonarqube/api/webservices/list,
retrieved 2026-10-10]. The token goes directly into a file outside the repository and
never appears in the terminal:

```sh
read -rs -p "SonarQube admin password: " SQ_ADMIN; echo
umask 077; mkdir -p ~/.config/bothy
curl -fsS -u "admin:${SQ_ADMIN}" -X POST "http://127.0.0.1:9000/api/user_tokens/generate" \
  --data-urlencode "name=bothy-analysis" --data-urlencode "type=GLOBAL_ANALYSIS_TOKEN" \
  | python3 -c 'import json, sys; print(json.load(sys.stdin)["token"])' > ~/.config/bothy/sonar-token
unset SQ_ADMIN
```

For the configuration, the same with `type=USER_TOKEN`, an `expirationDate` and a
separate file. In the shell that calls `make`:

```sh
export SONAR_TOKEN="$(cat ~/.config/bothy/sonar-token)"
```

Instead of a file, the keychain of the operating system also works.

## 4. Import the profile and gate

```sh
SONAR_TOKEN="$(cat ~/.config/bothy/sonar-config-token)" make sonar-config
SONAR_TOKEN="$(cat ~/.config/bothy/sonar-token)" make sonar-config-check
```

`sonar-config` creates the gate "Bothy way" (or repairs it) and makes it the
default, creates the profile "Bothy way" per language, sets the parent profile, imports the
additional rules from `quality/sonar/quality-profile-<language>.xml` and makes the profile
the default. Languages that the Community Build does not know (Dart, Swift) are
skipped and named. Afterwards, revoke the configuration token.

If the parent profile is missing (built-in profile names can change with a new
SonarQube version), `sonar-config` stops. Then check the built-in profiles,
adjust `quality/sonar/profiles.json` by pull request and import again.

## 5. Analyze

```sh
make sonar          # or make check
```

The scanner (`@sonar/scan`, without a local Java installation) reads
`sonar-project.properties` and waits for the gate result; a red gate makes
`make sonar` fail. As long as there is no product code, the target skips.

## 6. Alignment with SonarQube Cloud

In the public state (public-switch.md), against the cloud:

```sh
SONAR_HOST_URL=https://sonarcloud.io SONAR_ORGANIZATION="<organization>" SONAR_PROJECT_KEY="<project-key>" \
  SONAR_TOKEN="<token from the password manager>" make sonar-config        # once
SONAR_HOST_URL=https://sonarcloud.io SONAR_ORGANIZATION="<organization>" SONAR_PROJECT_KEY="<project-key>" \
  SONAR_TOKEN="<token>" make sonar-config-check
```

The analysis of `main` (ci.yml on every push to `main`) needs the main branch of the cloud
project to be named `main`. Until it is, the scanner reports `Branch name: main, type: short`
and the cloud fails the analysis ("CE Task finished abnormally with status: FAILED"). The job
`sonar` therefore runs on `main` only when the repository variable `SONAR_MAIN_ANALYSIS` is
`true`. Once: `make sonar-main-branch` (same variables as above) renames the main branch of the
project to `main` through the Web API (`api/project_branches/list`, then
`api/project_branches/rename` only when it carries another name, after deleting a short-lived
branch `main` that the failed analyses left, because the rename refuses a name in use); then
`gh variable set SONAR_MAIN_ANALYSIS --body true`. Pull requests are analysed either way.
The analysis of `main` also needs the project to be public: the free plan caps the lines
of code of a private project and refuses a larger analysis ("maximum allowed lines limit"),
a public one has no such cap. `make sonar-visibility` (same variables) sets the visibility to
`public` through `api/projects/update_visibility` when `api/projects/search` reports it private.
The gate of `main` needs a new code definition: without one of the project's own the cloud falls
back to the previous version, which needs a `sonar.projectVersion` the repository does not send,
and the gate has no status. `make sonar-new-code` (same variables) sets the last 30 days through
`api/settings/set` (`sonar.leak.period`, then `sonar.leak.period.type`; the cloud has no
`api/new_code_periods`) when `api/settings/values` shows no value of the project's own. The scan
of `main` does not wait for the gate (`SONAR_QUALITYGATE_WAIT=false` in ci.yml): it records the
overall state and the baseline and must not block `main`; pull requests wait and fail with it.

### Findings that only the analysis of main shows

From the analysis of 2026-10-25 on, the gate of `main` was `ERROR` (`new_security_rating` C)
while every pull request gate was green. Two vulnerabilities caused it; neither appeared in a
pull request analysis (docs/decisions.d/SONAR-MAIN-GATE.md):

* `pythonsecurity:S8707` in `sdk/python/src/bothy_sdk/auth.py`: a taint flow from the command
  line of `plugins/hermes-adapter` into `FileDpopKey.generate` of the SDK. The SDK pull request
  (289) had no source, so no flow; the adapter pull request (318) created the flow, but its sink
  lies in `auth.py`, which that pull request did not change. A pull request analysis keeps only
  issues on the code the pull request changes, so the finding first appeared in the next
  analysis of `main`.
* `text:S8569` in `packages/device_key/android/build.gradle.kts`: the file was new in pull
  request 116, whose analysis reported no vulnerability and has no S8569 issue on record, while
  the same rule did report the app's build file in pull request 360. Why pull request 116 missed
  it is not visible through the public API or the log of that run (unverified).

Two settings let such a finding stay unnoticed on `main`. The new code of `main` is the last
30 days, and the server dates an issue on an unchanged line back to the line's commit, so a
finding the analysis of `main` discovers late still counts as new code and fails the gate. The
scan of `main` does not wait for the gate (above), so the CI job stays green. Read the gate of
`main` after a merge, or whenever a pull request touches a file that code of another component
calls, with the public API (public project, no token):

```sh
curl -fsS "https://sonarcloud.io/api/qualitygates/project_status?projectKey=<project-key>"
curl -fsS "https://sonarcloud.io/api/issues/search?componentKeys=<project-key>&types=VULNERABILITY&statuses=OPEN"
```

A finding is fixed in code when it is real. A false positive is marked on its line with
`NOSONAR` (`# NOSONAR` in Python, `// NOSONAR` in Go, Kotlin and Dart), the reason in one line
next to it and a ruling in the ticket's `docs/decisions.d/` file; a rule is never deactivated
or weakened for a single finding.

Where the token exists only as the GitHub Actions secret `SONAR_TOKEN`, the manual workflow
`.github/workflows/sonar-config.yml` runs these targets against the cloud
(`gh workflow run sonar-config.yml --ref main`, variable `SONAR_ORGANIZATION`). The cloud
addresses the gate by its id in `create_condition` and `set_as_default`
(https://sonarcloud.io/api/webservices/list, retrieved 2026-10-13) [verified], the Community
Build by its name, and it has no built-in profile "Sonar way": there the profiles inherit from
`cloud_parent` of `quality/sonar/profiles.json` (ADR 0040). `quality_sync.py` chooses both by
`--organization`.

SonarQube Cloud makes a custom quality gate or quality profile the organization default only
on plans that include it; otherwise `api/qualitygates/set_as_default` answers HTTP 403 and
`api/qualityprofiles/set_default` HTTP 400 ("Organization ... is not allowed to perform this
action"). In cloud mode `quality_sync.py` therefore never makes the profiles the organization
default. It assigns each profile to the project (`api/qualityprofiles/add_project`) and the
gate as well (`api/qualitygates/select`); the project key comes from `--project` or
`SONAR_PROJECT_KEY` (the workflow takes the repository variable of that name), and cloud mode
stops without it. It still tries to make the gate the organization default.

A call the plan refuses (HTTP 400 or 403 with "Organization ... is not allowed to ...", such as
"... to perform this action" or "... to modify Quality gates") is a named gap, not a failure:
`apply` prints it with the server's reason and continues with the next language; it selects
the gate after the profiles. Any other refusal, such as HTTP 403
"Insufficient privileges" for a token without the permission, still stops `apply`. Examples:
`quality_sync: gap: profile go: not assigned to project: plan does not allow it (...)`,
`quality_sync: gap: gate not assigned to project: plan does not allow it (...)` and
`quality_sync: gap: gate not default: plan does not allow it`. `check` reads what the project
uses (`api/qualityprofiles/search?project=...`, `api/qualitygates/get_by_project`): a
built-in profile or gate there is reported as the same gap, any other profile or gate fails
the check. The gate "Bothy way", its conditions and the profiles are still compared; every
other difference (a condition, a parent, an activated rule) still fails the check, and the
exit code is 0 when only gaps remain. On the Community Build nothing is assigned to a project:
the gate and the profiles become the defaults, a refused `set_as_default` stops `apply`, and a
gate that is not the default fails `check`.

Both checks (local and cloud) pass against the same files, so the same rules
apply. What is compared is the addition on top of the parent profile and every gate condition. The
built-in parent profile itself can differ between the Community Build (monthly releases)
and the cloud (continuous); that is the known residual deviation. When to check:
after every update of the local image, after every change under `quality/sonar/` and
monthly.

## Add a rule

1. Name the rule key and the rationale in the pull request (which error, which
   occurrence).
2. Enter the rule in `quality/sonar/quality-profile-<language>.xml` under `<rules>`
   (format of the SonarQube backup file: `repositoryKey`, `key`, optionally `priority`
   and `parameters`).
3. `make sonar-config` locally, after the merge against the cloud; `sonar-config-check` in
   both places.
