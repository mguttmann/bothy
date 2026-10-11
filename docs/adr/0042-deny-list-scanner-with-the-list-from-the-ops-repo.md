---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# Deny-list scanner with the list from the ops repo

## Context and Problem Statement

The product repository must not contain anything instance-specific (ADR 0002): no company
and person names, domains, host names, internal identifiers. Which terms are forbidden
is itself instance knowledge and must not be in the product. Leaks arise not only in
files, but also in file names, commit messages, commit identities, branch and
tag names and in old commits.

## Decision Drivers

* The list lives outside the product repository.
* If the list is missing, the scan counts as failed, not as passed.
* All leak locations are checked, on request the whole history.
* CI logs in the public repository do not reveal the list.
* Contributions from forks without access to the list are still checked.

## Considered Options

* Custom scanner (Python, standard library) with a private list and public
  generic rules
* gitleaks with a custom, private configuration
* Only review by people

## Decision Outcome

Chosen option: "Custom scanner", because only it covers all leak locations, fails without a list
and masks hits.

* List: `BOTHY_DENYLIST_FILE` or the git-ignored file `.denylist-path` with the path;
  no hard-wired default path. Format: one rule per line, `#` comment, `re:`
  for regular expressions, otherwise literal, case-insensitive.
* Allowed commit identities (`allow-authors.txt` next to the list) are not reported as author and
  committer, so that the public identity of the maintainers does not block their commits;
  likewise in a `Signed-off-by` line that repeats exactly the author or committer
  of the same commit. In `Co-authored-by`, in other people's sign-offs and in the text
  the private list applies as everywhere (requirement 22, D-W-37). Only `Signed-off-by` and
  `Co-authored-by` are identity lines; other lines such as `Reviewed-by` are text.
* Checked: content and name of every tracked file (including binaries), commit messages
  and identities, branch and tag names, tag messages; `--history` additionally every
  object on every ref.
* `--allow-missing` only for fork pull requests: then only the public rules
  from `tools/denylist/generic.rules` run (home paths, tailnet names, email addresses outside
  the example domains, private IPv4 ranges, Discord IDs). The full list runs locally at the
  maintainer's before the merge and after the merge in `denylist-main.yml` over the commits
  pushed to `main` (D-W-34), so that a forgotten run does not go unnoticed.
* In CI the list comes from a secret of the protected environment `denylist`; the
  PR branch name is passed only through an environment variable, never on a
  command line.

### Consequences

* Good, because a leak in any of these locations shows up before the push.
* Good, because outputs show only the location and the rule line; paths with hits are masked.
* Bad, because a custom tool has to be maintained. It is small and fully tested.
* Bad, because an allowed identity must not itself contain anything forbidden; the
  scanner does not check this, the list of allowed identities is maintained and reviewed in
  the ops repository.

### Confirmation

`tools/denylist/tests/test_denylist_scan.py` checks every leak location, the failure without a
list, `--allow-missing`, allowed identities, masking and the generic rules
against the repository itself. Gate `denylist` in `make check` and as a separate CI job,
plus the run after the merge (`denylist-main.yml`); `make denylist-history` in the
going-public checklist.

## Pros and Cons of the Options

### Custom scanner

* Good, because file names, identities, refs and history are covered.
* Good, because fail closed and masked output are built in.
* Bad, because it is custom code.

### gitleaks with a private configuration

* Good, because it is a proven scanner with a history mode.
* Bad, because branch and tag names and commit identities are not checked and a
  missing configuration falls back to the default rules instead of failing.
* Neutral, because gitleaks still runs for real secrets (gate `gitleaks`).

### Only review

* Good, because it needs no tool.
* Bad, because people overlook names in commit metadata and old commits.

## More Information

Guide: tools/denylist/README.md. Switching to public: docs/quality/public-switch.md.
