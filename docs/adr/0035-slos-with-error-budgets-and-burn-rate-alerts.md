---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team F (operations)
consulted: Team A (architecture), Team D (agent platform)
informed: all teams
---

# SLOs with error budgets and burn-rate alerts

## Context and Problem Statement

Instance operators should notice when people experience the service as disrupted, without being
woken up by single spikes. Alerts on fixed thresholds (for example "5xx above X") are either too
noisy or too late. How are targets, measurement and alerts defined and maintained as code?

## Decision Drivers

* Alerts reflect what people feel (delivery, reconnecting, push, runs)
* few, reliable page alerts; the rest as tickets
* targets and rules as files in the repository, checkable in the gate
* runs with the local Prometheus without Kubernetes

## Considered Options

* SLO specifications for Sloth, generated Prometheus rules checked in
* Pyrra (Kubernetes operator or file system mode with its own interface)
* Hand-written Prometheus rules for multi-window burn rates
* Threshold alerts without SLO

## Decision Outcome

Chosen option: "SLO specifications for Sloth", because Sloth generates recording rules,
metadata and multi-window alerts (page and ticket) from simple files, without a running service
(https://github.com/slok/sloth, retrieved 2026-10-10).

1. Six SLOs in `infra/observability/slo/` ([slos.md](../observability/slos.md)); period 30 days.
2. Burn rates according to the Google SRE Workbook (page 14.4 and 6, ticket 3 and 1;
   https://sre.google/workbook/alerting-on-slos/, retrieved 2026-10-10).
3. Generated rules are in `infra/observability/prometheus/rules/slo-*.yaml` and are never changed
   by hand.
4. `turn-duration` only as a ticket, because the model and the task determine the duration.
5. Thresholds are bucket boundaries of the histograms.

### Consequences

* Good, because changing a number is one line in a file and the generator does the rest.
* Good, because ticket alerts show slow degradation before it turns into page alerts.
* Bad, because Sloth is another tool in the chain (version pinned).
* Bad, because the targets are starting values; before 30 days of measurement they are guesses.
* Neutral, because other backends do not take over the SLOs automatically; the files describe
  them so that a backend ticket can rebuild them.

### Confirmation

* Check in the gate: regenerate the rules and compare them with the checked-in ones.
* `promtool check rules` in the gate (on 2026-10-10 locally green with Sloth v0.16.0 and Prometheus v3.15.0).
* SLO docs check: the table in [slos.md](../observability/slos.md) equals the files; thresholds
  are bucket boundaries from [signals.md](../observability/signals.md).

## Pros and Cons of the Options

### Sloth

* Good, because files in the format `prometheus/v1` are easy to read.
* Good, because page and ticket alerts with multi-window logic are generated ready to use.
* Bad, because it has no interface of its own (a dashboard from the recording rules is needed).

### Pyrra

* Good, because an interface for error budget and burn rate is included.
* Bad, because another running service is needed.
* Neutral, because the generated alerts are equivalent.

### Hand-written rules

* Good, because no tool is needed.
* Bad, because the multi-window logic would be maintained by hand per SLO, which is error-prone.

### Threshold alerts without SLO

* Good, because they are immediately understandable.
* Bad, because they separate spikes and creeping degradation poorly.
* Bad, because no error budget for planning arises.

## More Information

* Alerts and routes: [alerting.md](../observability/alerting.md).
