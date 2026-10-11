---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Dependency updates with Renovate

## Context and Problem Statement

The repository pins everything: actions by commit SHA, images by digest, Python and
Node tools by lock file, release binaries by sha256, individual versions in the
Makefile. Without automatic updates the pins become outdated; with updates that are too hasty, a
compromised version gets in quickly (Trivy incident 2026-03-19,
https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23, retrieved
2026-10-10) [verified]. Which tool keeps the pins current?

## Decision Drivers

* Cover all pin locations, including custom formats (`tools/bin/tools.lock.json`, Makefile).
* Waiting time before new versions (cooldown), security updates without waiting time.
* Grouped pull requests, so that `make check` runs once per group.
* Actions and images stay pinned.

## Considered Options

* Renovate
* Dependabot
* Manual updates on a calendar

## Decision Outcome

Chosen option: "Renovate", because it covers custom pin formats through regex managers, maintains
uv lock files (manager pep621, https://docs.renovatebot.com/modules/manager/pep621/, retrieved
2026-10-10) [verified] and with `config:best-practices` pins actions and images to digests
(https://docs.renovatebot.com/presets-config/, retrieved 2026-10-10) [verified].

Configuration (`renovate.json`): weekly, grouped (actions, Python tools,
Node tools, release binaries, local SonarQube, Compose images), waiting time
`deps.min_release_age`, major updates only after approval on the Dependency Dashboard.
Exceptions:

* Release binaries: Renovate raises the version, a maintainer renews the
  sha256 pins with `make tools-lock`. Until then CI fails; this way every new binary goes
  through a human check.
* Trivy is switched off for Renovate and stays on the version that the advisory names as safe,
  until the check from docs/quality/toolchain.md approves a newer one.
* Images of the auxiliary services in the Compose stack: a regex manager reads the comments
  `# renovate: datasource=docker depName=<image>` above the variables in
  `infra/compose/.env.example` and raises the variable and the default value `${BOTHY_<X>_VERSION:-...}` in
  `infra/compose/compose.yaml` in one pull request (group "Compose images"). The validators
  of the same components (`otelcol-contrib`, `promtool`, `amtool`, `loki`, `tempo` in
  `tools/bin/tools.lock.json`) are switched off for Renovate; the maintainer updates them in the
  same pull request with `make tools-lock`. Until then the check `tool-versions` fails, because a
  validator of another version accepts or rejects other keys (PLAN M1-04).

### Consequences

* Good, because all pins have an update path and nothing becomes outdated unnoticed.
* Good, because the waiting time usually catches short-lived malicious versions before they
  are proposed.
* Bad, because release binaries need two steps. That is intended.

### Confirmation

* `make lint` checks `renovate.json` against the Renovate schema (check-jsonschema).
* `tools/docs/tests/test_config_values.py` keeps `minimumReleaseAge` equal to
  `deps.min_release_age` in chapter 23.
* `tools/bin/tests/test_fetch_tools.py` checks that a binary without a matching sha256
  is not installed.
* Check `tool-versions` (PLAN M1-04): validator pins equal the image versions in
  `.env.example` and the default values in `compose.yaml`.

## Pros and Cons of the Options

### Renovate

* Good, because regex managers, groups, waiting time and digest pinning are built in.
* Neutral, because the hosted app is a third-party service.

### Dependabot

* Good, because it is built into GitHub.
* Bad, because custom pin formats (lock file of the binaries, Makefile) are not
  covered.

### Manual updates

* Good, because of full control.
* Bad, because experience shows that they do not happen, especially for security updates.

## More Information

Whether the hosted Renovate app is used for the private repository or only from switching to
public on is decided by the owner (open question in the return report).
