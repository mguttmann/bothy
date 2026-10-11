---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team Apps and design
consulted: Team Operations, Team Quality and repo
informed: all teams
---

# Working name and branding as configuration

## Context and Problem Statement

The product is provisionally called "Bothy" (name research: [naming.md](../design/naming.md)); the
final choice and the trademark check are pending. At the same time every instance should be able
to ship its own apps with its own name, logo, accent, bundle IDs, push topics and update channels,
and the product repo must stay free of company names (white label). Where may the name appear,
and how does the branding of an instance get into server, apps and builds?

## Decision Drivers

- Keep a name change after the trademark check cheap
- White label: no company data in the product repo
- Instances ship their own, signed apps (own IDs, push, updates)
- Contrast AA must not break through branding
- few, listed places instead of scattered constants

## Considered Options

1. Two levels: runtime branding in the instance settings, build branding in a file of the
   instance; the name in code only at listed places
2. Only runtime branding; one app for all instances
3. Name and branding hard-coded; a fork per instance

## Decision Outcome

Chosen option: "Two levels", because name, logo and accent can change at runtime without building
new apps, while bundle IDs, push topics, signing identities and update feeds must technically be
fixed at build time and are specific to each instance.

| Level | Content | Location |
|---|---|---|
| Runtime | `branding.product_name`, `branding.logo_file_id`, `branding.accent_color` | `instance_settings`, action `instance.branding.update` |
| Build | App name, bundle and package IDs, app icons from one SVG source, URL scheme, push (APNs topic, Firebase project), Sparkle feed and EdDSA key, MSIX identity and `.appinstaller` URL, crash service, default server | `branding.yaml` of the instance in the instance operator's repo; the product repo contains only an example with `example.com` |
| Code | Go module paths, package names (among them the Dart packages `bothy_viz` and `bothy_noise`), binary `bothy`, binary and service `bothy-host` of the terminal, plugin ID `bothy-live-panel` and folder `grafana-bothy-live` of the Grafana plugin, protocol identifiers of the terminal (`bothy-terminal/1`, `bothy-host-fp/1`), prefix of the API tokens and pairing codes `bothy_` (03 `api_tokens`), other kinds | listed by kind in [02-architecture.md](../spec/02-architecture.md), section 6 (the only list), checked by a test (PLAN M1-46) |

The accent is a key from the checked category palette (`spruce`, `loch`, `heather`,
`rowan`, `amber`, `gorse`, `moss`), not a free color value, so that every combination lies within
the contrast gate (ADR 0030). The category color `granite` is neutral and therefore not an accent.
The list of accents is here and in [01-product.md](../spec/01-product.md), section 12, not in the
design tokens; `kernel/designkeys` holds it as a constant. A new category color becomes an
accent only when it enters this list and passes the contrast gate.

### Consequences

- Good, because a name change affects `branding.*`, the listed code places (among them the
  token prefix `bothy_`, the binary `bothy-host` and the plugin ID) and the example configuration,
  nothing else.
- Bad, because two places outside the repo depend on the name: the plugin ID in Grafana's
  catalogue (no longer freely changeable after a signature, the first part must match the Grafana
  account, [25-visualization.md](../spec/25-visualization.md), section 8.3) and the
  protocol identifiers of the terminal (change only with a new protocol version). That is why the
  final choice of name is made before the signing of the plugin (PLAN V1-06 before the signing in
  V3).
- Good, because instances build their own, signed apps without forking the product.
- Good, because contrast AA holds with an instance accent as well.
- Bad, because every instance with its own apps needs its own Apple, Google and Microsoft accounts,
  keys and signatures (owner accounts, baseline R-Push and R-Desktop-Verteilung).
- Bad, because free brand colors are not possible.

### Confirmation

- A test searches the repo for the working name and allows it only at the listed places, in docs
  and in the build example.
- The server checks `branding.accent_color` against the accent list in `kernel/designkeys`; a
  test keeps every entry as a category color in `design/build/tokens.resolved.json` and the list
  equal to 01, section 12 (PLAN M1-46).
- The app build aborts if a required field in `branding.yaml` is missing.

## Pros and Cons of the Options

### Two levels

- Good, because it separates what can change at runtime from what must be signed.
- Bad, because there are two places for branding.

### Only runtime

- Good, because there is one app for all.
- Bad, because push topics, bundle IDs and update feeds would be other people's accounts;
  instances could not distribute their own apps.

### Hard-coded

- Good, because it is the simplest on the first day.
- Bad, because a name change is expensive; forks per instance violate white label and multiply
  maintenance.

## More Information

- Usage and branding rules: [01-product.md](../spec/01-product.md), section 12.
- Build configuration in the apps: [15-client-architecture.md](../spec/15-client-architecture.md),
  sections 11 and 15.
- Final name: open owner question; recommendation Bothy, alternatives Corvo and Gilde.
