# Quality and repository

How every change is checked before it reaches `main`, and how the repository
is published. The binding rules are in
chapter [23 Quality](../spec/23-quality.md); these pages are the guides for them.

| Page | Content |
|---|---|
| [gates.md](gates.md) | Every gate with tool, threshold and where it runs; hooks; component detection |
| [toolchain.md](toolchain.md) | Every tool with version, purpose, license and reason; update rules |
| [push-discipline.md](push-discipline.md) | From branch to merge: commits, push, pull request, process failures |
| [red-team-gate.md](red-team-gate.md) | Independent review before every push (evidence `.review/<sha>.md`) and per milestone, verdict `Urteil: JA` or `Urteil: NEIN` |
| [sonarqube-local.md](sonarqube-local.md) | Local SonarQube, token, profile and gate, reconciliation with the cloud |
| [public-switch.md](public-switch.md) | Checklist for publishing: push, public right away, SonarQube Cloud and CodeRabbit, branch protection, afterwards every ticket as a pull request |
| [licenses.md](licenses.md) | Allowed licenses, license gate, SBOM and checking releases |
| [accessibility-checklist.md](accessibility-checklist.md) | Accessibility checklist of the desktop app (filled by M4-32, M4-36 and M4-37) |
| [accessibility-mobile.md](accessibility-mobile.md) | Accessibility checklist of the phone apps with VoiceOver and TalkBack (M5-10) |

Short version for everyday use:

```sh
make bootstrap   # once per clone
make check       # before every push (the pre-push hook does it automatically)
make help        # all targets
```
