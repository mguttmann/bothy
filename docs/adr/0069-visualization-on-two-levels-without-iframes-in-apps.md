---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Builder check (fresh reviewer), Team operations, Team apps and design
informed: all teams
---

# Visualization on two levels without iframes in apps

## Context and Problem Statement

The owner requires visualizations that impress: a live topology of the agent runs, a view of the
backend policy, delivery and sync live, an executive page with an SLO traffic light, a replayable
daily timeline and a TV mode, in the apps and the console and additionally in Grafana, both in the
same design language. He asks whether Grafana graphics can be brought into the app.
The statistics of the app must not depend on the observability stack (R-OB-08, 17 rule 7), and many
values (delivery latency, reconnecting, acceptance at the push provider) are in no domain table.
How do we build the visualizations without losing permissions, privacy and independence from the
stack?

## Decision Drivers

* R-OB-08: statistics and status page work without the observability stack.
* The permissions of the instance (`stats.read`, `cost.read.aggregate`, visible channels) also apply
  to graphics.
* No values per person (20, section 9).
* One design language, tokens from one source (ADR 0030).
* Owner requirement: no iframes in iOS, Android and desktop; Grafana embedding at most optional in
  the web console.
* Clickjacking: by default, Grafana protects itself against embedding.

## Considered Options

* Two levels: native visualizations from own measurement in the apps and the console, plus a
  custom Grafana panel plugin from OTel metrics; embedding only optional in the console
* Embed Grafana in all clients (iframe in the console, WebView in the apps)
* Native visualizations only, no Grafana plugin
* Grafana only; the apps link to Grafana

## Decision Outcome

Chosen option: "Two levels", because only this option meets permissions, independence from the
stack and the owner's requirement at the same time.

1. Level a: package `packages/bothy_viz` (Flutter) with six views. Data only from the own
   statistics API and the gateway event `stats.live`. The server measures itself through
   `kernel/measure` and writes minute aggregates to `stats_minutes`; SLI thresholds and targets
   are generated from the Sloth files; the traffic light is labeled "own measurement" (ruling T7).
2. Level b: plugin `plugins/grafana-bothy-live` (ID `bothy-live-panel`), generated with
   `@grafana/create-plugin`, data from Prometheus and Tempo through Grafana; unsigned until V1 and
   allowed only by its ID (ruling T11).
3. Shared design language: new roles `color.viz.*` in `design/tokens/` and a TypeScript output of
   `tools/design-tokens` for the plugin.
4. No iframes or WebViews with Grafana in iOS, Android, macOS and Windows. In the web console,
   embedding is optional and off by default (`console.grafana_embed_url` empty); only with
   `allow_embedding`, an auth proxy, `frame-ancestors`, COEP/CORP at the proxy and `frame-src` of
   exactly this origin.
5. The backend view is product-neutral; data about individual subscription seats or credentials
   do not belong in the product (I5, ruling T8).

Details: [25-visualization.md](../spec/25-visualization.md).

### Consequences

* Good, because the statistics work the same in every installation, even without Grafana.
* Good, because graphics follow the same permissions as the data and show no values per person.
* Good, because instance operators with Grafana get the same look in operations.
* Bad, because two implementations of the same views must be maintained (Dart and TypeScript).
* Bad, because the own measurement and Prometheus can differ slightly (different measuring
  points, lost counters after a crash); the traffic light says so explicitly.
* Bad, because an unsigned plugin needs an exception in Grafana and does not run in Grafana Cloud
  until it is signed.

### Confirmation

* VZ-T01 to VZ-T16 in [25-visualization.md](../spec/25-visualization.md), section 12.
* VZ-T03: generator of the SLI limits checked against the Sloth files in the gate.
* VZ-T16: static check that `apps/client` uses no WebView package and no iframe element.
* SEC-T19 in [19-security.md](../spec/19-security.md): `frame-src` only when
  `console.grafana_embed_url` is set.

## Pros and Cons of the Options

### Two levels

* Good, because R-OB-08 and the permissions of the instance apply.
* Good, because Grafana stays where it belongs: with operations.
* Bad, because more build work.

### Embed Grafana everywhere

* Good, because one implementation is enough.
* Bad, because the statistics would depend on the stack (violates R-OB-08).
* Bad, because Grafana does not know the permissions of the instance and needs a second login.
* Bad, because `allow_embedding` turns off Grafana's clickjacking protection and WebViews in
  apps are a further attack surface; ruled out by the owner.

### Native visualizations only

* Good, because one implementation is enough.
* Bad, because instance operators are left without suitable views in operations; the owner
  requires the plugin.

### Only Grafana with links

* Good, because the least build work.
* Bad, because the app would be left without statistics as soon as no stack runs, and Grafana
  knows no permissions per space.

## More Information

* Related: [ADR 0028](0028-web-target-only-for-the-admin-and-statistics-console.md),
  [ADR 0030](0030-design-tokens-as-one-source-with-a-generator.md),
  [ADR 0034](0034-opentelemetry-through-a-collector-local-default-stack.md),
  [ADR 0035](0035-slos-with-error-budgets-and-burn-rate-alerts.md),
  [ADR 0044](0044-working-name-and-branding-as-configuration.md).
* Grafana sources (retrieved 2026-10-10): https://grafana.com/developers/plugin-tools/,
  https://grafana.com/docs/grafana/latest/administration/plugin-management/plugin-sign/,
  https://grafana.com/docs/grafana/latest/setup-grafana/configure-grafana/.
* Rulings T7, T8 and T11 of the lead.
