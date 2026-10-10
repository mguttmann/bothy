# Security Policy

## Reporting a vulnerability

Please report vulnerabilities privately. Do not open a public issue, discussion or pull
request for them.

- Use GitHub private vulnerability reporting: the "Report a vulnerability" button on the
  repository's Security tab (Security, Advisories, New draft advisory).
- If that is not possible, write to security@example.com (placeholder; the maintainers
  replace it before the repository is public).

Please include the affected component and version, the steps to reproduce, the impact you
expect, and whether the issue is already known to others. Remove real personal data and
secrets from your report.

## What happens next

The times below are targets, not guarantees. They are maintained as named values in
docs/spec/23-qualitaet.md; a test keeps this table and that chapter equal.

| Step | Target |
|---|---|
| Acknowledge the report (`security.report_ack_target`) | 3 business days |
| First assessment with severity (`security.assessment_target`) | 10 business days |
| Fix released for critical severity (`security.fix_target_critical`) | 30 days |
| Fix released for high severity (`security.fix_target_high`) | 60 days |
| Public disclosure at the latest (`security.disclosure_deadline`) | 90 days |

We publish a GitHub security advisory with the fix and credit the reporter unless they
prefer otherwise. We do not pay bounties.

## Supported versions

Security fixes go into the latest minor release of each component (server, apps, SDKs,
agent adapters). Before version 1.0 there are no backports to older releases. Each
component's CHANGELOG.md lists the security fixes.

## Scope

In scope: the code in this repository and the release artifacts built from it (container
images, apps, SDK packages). Out of scope: the configuration of a particular installation,
third-party services an installation connects to, and denial of service by volume.

## Verifying releases

Release artifacts come with an SBOM and, in the public phase, with signed build
provenance. See docs/quality/licenses.md and docs/spec/18-release.md for how to verify them.
