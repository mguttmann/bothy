# Red-team gate

An independent reviewer actively looks for ways to break the rules of the product. Its
verdict is `JA` (yes: may be merged, pushed or published) or `NEIN` (no) with a reason.

## When

| Occasion | Scope | Evidence |
|---|---|---|
| Before every push and every local merge of a ticket (requirement 23, PLAN 3.9) | diff of the branch against `main` | `.review/<sha>.md` of the pushed or merged commit; the pre-push hook checks it (`tools/hooks/review_gate.py`) |
| At the end of every milestone | whole state against the state of the last milestone | report in the ops repository, linked in the close of the milestone |
| Before going public | whole state | report in the ops repository (public-switch.md, step 1) |

The review per push is short and limited to the diff; the overall review finds what
only becomes visible in the interplay of several tickets.

## Who checks

* Independent: not the person or session that built the state. In
  single-person operation, this is a fresh session of an AI reviewer without context from the
  build session, or a second person.
* Read-only: the reviewer changes nothing in the working tree, does not push and has no secrets.
  It gets the diff, the specification and the threat model.

## What is checked

1. **Permissions.** Every new route, every event, every delivery goes through `authorize`
   with an action from chapter 05; no view permission results in 404. Is there a way to see data
   of a foreign space (counters, search, suggestions, push, webhooks)?
2. **Backend policy.** Can a person other than the `credential_holder` trigger an
   agent with `billing_mode = personal` (I1 to I3)?
3. **Agents.** Can content from one space control an agent in another space?
   Can an agent with powerful tools be triggered by a person with few
   permissions?
4. **Supply chain and CI.** New actions pinned, no new triggers with foreign code,
   no secrets in jobs that do not need them; new dependencies with an allowed
   license.
5. **White label and privacy.** Nothing instance-specific, no contents in
   telemetry, no personal data in logs.
6. **Gates.** Was a gate weakened, a threshold lowered, a test deleted or
   skipped?

## Flow

1. The build session reports "done" with a green `make check`.
2. The reviewer receives the diff (`git diff <base>...HEAD`), chapters 07, 10, 19 and this page.
3. The reviewer writes a report: findings with file and line, severity, and as the last
   line `Urteil: JA` or `Urteil: NEIN <reason>`. For the review per push, it is located at
   `.review/<sha>.md` (`<sha>` from `git rev-parse HEAD`).
4. On `NEIN`: fix, tests for every finding, then the reviewer checks only the delta
   again. If the same `NEIN` comes again, the owner decides, not another round.
5. On `JA`: merge or close of the milestone, respectively. The report per push stays local in
   `.review/`; the pull request (section "Review" of the template) names the reviewed commit,
   the verdict and the fixes in fresh words, never the raw report, because the body becomes
   the squash commit on `main` (LG/1 in docs/DECISIONS.md).
6. The threat model (`docs/security/threat-model.md`) is updated with new threats and
   mitigations before the milestone counts as done.

## What the gate does not replace

It does not replace penetration tests (chapter 19) or gates; it checks what tools
cannot check: intent, interplay and missing checks.
