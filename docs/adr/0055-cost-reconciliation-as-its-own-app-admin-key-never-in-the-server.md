---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team D (agent platform), Team C (security)
informed: Team F (operations)
---

# Cost reconciliation as its own app, admin key never in the server

## Context and Problem Statement

In the mode `organization` the product shows costs per run, thread, space and agent. These amounts
are estimates from token counts and a price table; surcharges, discounts and pricing tiers are
missing ([10-backend-policy.md](../spec/10-backend-policy.md), sections 12.1 and 12.3). The
actual costs come from the provider's cost API, but only with an admin access of the
organization. According to the documentation, this access can manage members, workspaces,
invitations and API keys; the documentation describes no read-only admin key [Q1, verified].
Where does the reconciliation run, without a break-in into the chat server reaching this access?

## Decision Drivers

- The server stores and brokers no credentials for model providers (B10, I5).
- A break-in into the server must not enable management of the provider organization.
- Reconciliation is optional; without it, amounts stay labeled as "estimate".
- A reconciliation runs unattended and cannot perform a step-up (R-AZ-03).
- Reconciled amounts are traceable and replace estimates without deleting them.

## Considered Options

- Own app outside the server with its own secret provider and scope `costs:reconcile`;
  off by default
- Reconciliation job in the server with the admin key in the server's secret provider
- Only manual reconciliation by the owner (entering amounts from the invoice)
- No reconciliation, only estimates

## Decision Outcome

Chosen option: "Own app outside the server", because the admin access then lies only in a
process that does nothing else, and the server never talks to the provider. Normative in
[10-backend-policy.md](../spec/10-backend-policy.md), section 12.4.

1. The app reads the admin key from its own secret provider and calls the cost API with
   daily buckets, grouped by workspace [Q2, verified]. One workspace with its own key per agent is
   recommended, because the API groups by workspace and not by key.
2. Per day and workspace it distributes the actual amount to the estimates of the assigned
   agents in proportion to the estimates; the rounding remainder goes to the largest entry. Amounts
   without a matching run it reports as "unassigned".
3. It submits new entries with `source = 'reconciled'`, `is_estimate = false` and
   `supersedes_id` through the action `cost.reconcile.submit` (scope `costs:reconcile`, without
   step-up, [05-actions.md](../spec/05-actions.md), section 4.12). The action only creates entries
   for existing runs and deletes nothing.
4. Limits of the app: interval between requests `reconciler.min_request_interval`, lookback
   `reconciler.lookback`, mapping `reconciler.workspace_map` (configuration of the app, not of the
   server).
5. The owner can still reconcile estimates by hand (`cost.reconcile`, step-up, audit).

### Consequences

- Good, because a break-in into the server does not reach the admin access; the app needs no
  inbound port.
- Good, because the server contains no provider code and no provider keys; the reconciliation for
  other providers is another app.
- Good, because reconciled entries replace the estimate but do not delete it; the trail stays
  checkable.
- Bad, because another component must be operated, updated and monitored.
- Bad, because the cost API only knows daily buckets: an amount per run is a proportional
  distribution, not a measurement. If several agents share a workspace, the amount is also
  distributed between them.
- Bad, because costs of the Priority tier are missing from the cost endpoint [Q2, verified]; such
  amounts stay "unassigned" or are missing.

### Confirmation

- Test BR-10 ([10-backend-policy.md](../spec/10-backend-policy.md), section 15): the
  distribution of a daily amount sums up to the actual amount, the remainder goes to the largest
  entry, estimates are superseded.
- Test BR-13: a secret canary value appears in no response and no log.
- Test: `cost.reconcile.submit` without scope `costs:reconcile` is rejected; with the scope the
  action only creates `reconciled` entries for existing runs and changes or deletes no others.
- Semgrep rule `bothy-go-no-model-credentials` (`.semgrep/`): the server reads no
  provider credentials from the environment.
- Build ticket M2-28 in [PLAN.md](../PLAN.md): the server stores no provider key.

## Pros and Cons of the Options

### Own app

- Good, because permissions and secret are tightly limited.
- Bad, because another process with its own configuration comes into being.

### Reconciliation job in the server

- Good, because there is nothing extra to operate.
- Bad, because the server then holds an access with which one can manage keys and members of the
  provider organization; that contradicts B10 and I5.
- Bad, because the server would talk to a model provider itself for the first time.

### Manual only

- Good, because no admin access is stored anywhere.
- Bad, because the work comes up daily and errors in the assignment to runs are likely.
  Remains as a correction path.

### No reconciliation

- Good, because there is nothing to build.
- Bad, because amounts stay estimates permanently; cache writes without a reported duration,
  surcharges and discounts distort them without correction (10, section 12.3).

## More Information

- Sources (retrieved 2026-10-10): Q1 Admin API,
  https://platform.claude.com/docs/en/manage-claude/admin-api [verified]; Q2 Usage and Cost API,
  https://platform.claude.com/docs/en/manage-claude/usage-cost-api [verified]. Both cited as in
  [10-backend-policy.md](../spec/10-backend-policy.md), section 12.4.
- Related: ADR 0022 (agents and plugins outside the server), ADR 0024 (backend policy),
  ADR 0011 (secret provider).
