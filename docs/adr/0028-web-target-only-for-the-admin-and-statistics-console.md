---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team apps and design
consulted: Team security and permissions, Team operations
informed: all teams
---

# Web target only for the admin and statistics console

Status note (2026-10-10): Partly superseded by
[ADR 0070](0070-terminal-in-the-web-console.md). The console additionally gets the terminal with
a mandatory passkey; everything else in this decision still applies, in particular no chat in the
browser.

## Context and Problem Statement

Instance operators and owners need administration and statistics even without an installed app,
for example from a colleague's computer or right after installing an instance. A chat in the
browser would be technically possible. But it brings another client with offline storage, push,
sessions and attack surface (XSS, browser extensions) into a product whose content controls agent
sessions with tool power. What does the web target offer?

## Decision Drivers

- administration and statistics reachable without an app (Baseline B5)
- a small attack surface for controlling agents
- one client fewer with offline storage, push and session management
- the same building blocks as the apps (design system, API client, administration screens)
- accessibility in the browser

## Considered Options

1. Flutter Web only for administration and statistics (console)
2. Full chat client in the browser (Flutter Web)
3. No web target; administration only in the apps
4. Console in a separate web technology (for example TypeScript with an SPA framework)

## Decision Outcome

Chosen option: "Flutter Web only for administration and statistics", because it makes
administration reachable without an app, uses the same packages as the apps (`design_system`,
`api_client`, `auth`, `admin_ui`) and keeps the chat with its offline storage, push and agent
triggering out of the browser. According to its documentation, Flutter Web suits "app-centric
experiences" such as a console, not text-heavy pages
(https://docs.flutter.dev/platform-integration/web/faq, retrieved 2026-10-10
[verified according to research]).

### Consequences

- Good, because the console needs no local database, no outbox and no push.
- Good, because administration screens are built only once (`packages/admin_ui`).
- Good, because WebAuthn passkeys work in the browser without Associated Domains.
- Bad, because people without the app cannot chat; links from the console open the app.
- Bad, because Flutter Web turns on accessibility only after `SemanticsBinding.instance.ensureSemantics()`
  (https://docs.flutter.dev/ui/accessibility/web-accessibility, retrieved 2026-10-10
  [verified according to research]); the console always turns it on.
- Neutral, because the JavaScript build is used; Wasm helps today only in Chromium
  (https://docs.flutter.dev/platform-integration/web/wasm, retrieved 2026-10-10 [verified]).

### Confirmation

- Architecture test: `apps/console` does not depend on `sync_engine` and `local_store`.
- Route test of the console: there is no route for channels, threads or messages.
- A widget test checks that the console turns on semantics at startup.

## Pros and Cons of the Options

### Flutter Web only for the console

- Good, because shared packages, a small attack surface, no offline client in the browser.
- Bad, because no chat without the app.

### Full chat in the browser

- Good, because access without installation.
- Bad, because push, offline storage, keys in the browser and agent triggering from an
  environment with extensions and XSS risk.
- Bad, because another client with tests, golden images and accessibility.

### No web target

- Good, because even less surface.
- Bad, because administration right after installation and from other people's computers is
  missing.

### Console in a separate web technology

- Good, because SEO and the browser's text rendering.
- Bad, because a second UI technology, a second design system, duplicate administration screens.

## More Information

- Screens of the console: [01-product.md](../spec/01-product.md), section 5.5.
- Login of the console (passkeys, DPoP with WebCrypto): [15-client-architecture.md](../spec/15-client-architecture.md), sections 8 and 9.
- If a chat in the browser is wanted later, it needs a new ADR with a threat model.
