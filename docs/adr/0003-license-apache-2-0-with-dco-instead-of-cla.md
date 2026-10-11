---
status: accepted
date: 2026-10-10
decision-makers: Maintainer
---

# License Apache-2.0 with DCO instead of CLA

## Context and Problem Statement

The product becomes open source. Instance operators should be able to use it freely, adapt it and
embed it in their own infrastructure, also commercially. Contributions from outside need a
clear proof of origin, without scaring contributors off with a contract.

## Decision Drivers

* Free use by organizations of any size, also with their own extensions.
* An explicit patent license from the contributors.
* Proof of origin per contribution, machine-checkable.
* A low hurdle for contributors, no contract with a company.

## Considered Options

* Apache-2.0 with Developer Certificate of Origin (DCO)
* Apache-2.0 with Contributor License Agreement (CLA)
* MIT with DCO
* AGPL-3.0
* MPL-2.0

## Decision Outcome

Chosen option: "Apache-2.0 with DCO", because Apache-2.0 contains an explicit patent license
and is permissive, and because the DCO gives a checkable proof of origin per commit,
without contributors having to sign a contract. Contributions are under the same
license as the project (inbound = outbound, Apache-2.0 section 5).

### Consequences

* Good, because instance operators and integrators have no copyleft obligations for their own
  extensions; extensions run outside the server process anyway (planning basis B6).
* Good, because every commit carries `Signed-off-by` and this is checked by machine.
* Bad, because a later license change would need the consent of all contributors;
  this is intended.
* Dependencies must be compatible with Apache-2.0; the license gate checks this
  (docs/quality/licenses.md).

### Confirmation

* `tools/ci/license_check.py` (gate `licenses`): `LICENSE` is the unchanged
  Apache-2.0 text (sha256), every hand-written source file carries
  `SPDX-License-Identifier: Apache-2.0`.
* `tools/hooks/commit_msg.py --require-signoff`: commit-msg hook and gate `commitlint`
  in CI for every commit of a pull request.

## Pros and Cons of the Options

### Apache-2.0 with DCO

* Good, because a patent license and a patent retaliation clause are included.
* Good, because the DCO needs no signature and no company as a contracting party.
* Neutral, because `NOTICE` obligations apply on redistribution.

### Apache-2.0 with CLA

* Good, because the holder could relicense later.
* Bad, because a CLA scares contributors off and needs a process for signing.

### MIT with DCO

* Good, because very short and well known.
* Bad, because there is no explicit patent license.

### AGPL-3.0

* Good, because changes to hosted instances would have to be disclosed.
* Bad, because many organizations are not allowed to use AGPL software; this
  contradicts the goal of free use.

### MPL-2.0

* Good, because file-level copyleft with a patent license.
* Bad, because obligations per file burden instance operators with their own adaptations.

## More Information

Apache License 2.0: https://www.apache.org/licenses/LICENSE-2.0 (retrieved 2026-10-10)
[verified]. Developer Certificate of Origin 1.1: https://developercertificate.org/
(retrieved 2026-10-10) [verified]. Procedure for contributors: `CONTRIBUTING.md`.
