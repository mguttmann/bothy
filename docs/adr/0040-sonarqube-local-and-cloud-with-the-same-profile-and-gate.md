---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# SonarQube local and cloud with the same profile and gate

## Context and Problem Statement

SonarQube is to run locally before the push (ADR 0038) and, in the public state, rate pull
requests in SonarQube Cloud. Both sides must apply the same rules and the same
gate, otherwise the cloud finds after the push what passed locally. Two gaps
are known: the Community Build does not analyze Dart
(https://docs.sonarsource.com/sonarqube-community-build/analyzing-source-code/languages/overview/,
retrieved 2026-10-10) [verified], and the free cloud plan for private repositories
allows no custom profiles and gates; the OSS plan for public repositories
allows them
(https://docs.sonarsource.com/sonarqube-cloud/administering-sonarcloud/managing-subscription/subscription-plans.md,
retrieved 2026-10-10) [verified].

## Decision Drivers

* The same rules and the same gate locally and in the cloud, checked by a machine.
* No outflow of private code to a service before switching to public.
* Dart coverage despite the missing analyzer in the Community Build.
* Profile changes reviewable in the repository.

## Considered Options

* Both: local Community Build and SonarQube Cloud, profile and gate as files in the
  repository, applied and compared by a tool
* Only SonarQube Cloud
* Only a local Community Build
* No SonarQube; only language-specific linters

## Decision Outcome

Chosen option: "Both, with files in the repository", because only this way the same rules
provably apply locally and in the cloud.

* Profile "Bothy way" per language inherits from the built-in profile (`quality/sonar/profiles.json`,
  parent profile "Sonar way") and additionally activates only the rules from
  `quality/sonar/quality-profile-<language>.xml`. At the start these lists are empty:
  additional rules come one at a time with a reason in the pull request.
* Gate "Bothy way" (`quality/sonar/quality-gate.json`) with the four conditions of the
  built-in gate (no new findings, all new hotspots reviewed, coverage and
  duplication in new code). It uses only metrics that exist in both modes of SonarQube
  (https://docs.sonarsource.com/sonarqube-community-build/user-guide/code-metrics/metrics-definition,
  retrieved 2026-10-10) [verified].
* `tools/sonarqube/quality_sync.py apply` applies both, `check` compares. Equality
  locally and in the cloud follows from both passing against the same files.
* Dart: locally `flutter analyze` and `custom_lint` as a separate gate, in the cloud
  additionally SonarQube.
* In the private state only the local Community Build; SonarQube Cloud is connected when
  switching to public (docs/quality/public-switch.md).

### Consequences

* Good, because a rule change is a reviewable diff and both sides follow verifiably.
* Good, because no private code goes to the cloud.
* Bad, because the Community Build is released monthly and the cloud is updated
  continuously; the built-in parent profile can differ. What is compared is therefore
  the addition on top of the parent profile, not the full rule set.
* Bad, because built-in profile names can change: the vendor's public SonarQube server
  lists "Sonar way core", "Sonar way extended" and "Sonar way
  comprehensive" for Go (web API `api/qualityprofiles/search?language=go` on
  https://next.sonarqube.com/sonarqube, retrieved 2026-10-10) [verified]. If the
  parent profile is missing, `apply` aborts with a message instead of silently choosing another.

### Confirmation

* `tools/sonarqube/tests/test_quality_sync.py` checks applying, idempotency, the repair of
  deviating conditions and the detection of additional rules against a test server.
* `make sonar-config-check` against the local server and against SonarQube Cloud is part
  of the going-public checklist and of every SonarQube update.
* `tools/docs/tests/test_config_values.py` keeps the thresholds of the gate equal to the
  named values in chapter 23.

## Pros and Cons of the Options

### Both, with files in the repository

* Good, because of verifiable equality and a local check before the push.
* Bad, because a local server with a database is operated (Docker-compatible
  runtime, memory).

### Only SonarQube Cloud

* Good, because there is no local server.
* Bad, because findings come only after the push and private code leaves the premises.
* Bad, because custom profiles and gates are not possible in the private state.

### Only a local Community Build

* Good, because everything stays local.
* Bad, because there is no Dart analysis and no rating of fork contributions.

### No SonarQube

* Good, because there are fewer tools.
* Bad, because duplication, coverage of new code and security hotspots then have no
  shared gate.

## More Information

Step by step: docs/quality/sonarqube-local.md.
