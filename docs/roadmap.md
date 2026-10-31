# Roadmap

Bothy is a command center for AI agents in chat form: chat, live view, grants,
terminal, costs and statistics, self-hosted and open source.

State: in development, no release yet. M0 is done and the milestones M1 to M8 are built (their
build tickets are on `main`), and Bothy is in daily use at one self-hosted installation. The
repository is public; the rest of V1 is open. There are no published server images and no
distributed apps yet: the server image and the apps are built from source
([README: Try it today](../README.md#try-it-today)). How a milestone is closed is in
[PLAN 4](PLAN.md#4-closing-a-milestone). There are deliberately no dates; the order is set by the
[plan](PLAN.md). What the product will never do is in [chapter 01](spec/01-product.md).

Next steps:

1. The first release with signed server images and the install bundle, so the one-command quick
   start works ([Release](operations/release.md), [18 Release](spec/18-release.md)).
2. Distributed desktop and phone apps from the release chain, so trying Bothy no longer starts
   with building the apps ([apps/distribution](../apps/distribution/README.md)).
3. Push in the distributed apps; today push needs the operator's own Apple and Firebase
   accounts.

| Milestone | Content | Status |
|---|---|---|
| M0 Planning | Specification, core contract, architecture decisions, toolchain, build tickets | done |
| M1 Foundation | Toolchain and local gates; server core with identity, devices, grants and `authorize`; spaces, channels, threads, messages; event log and synchronization; audit; observability foundation; server image with smoke test | built |
| M2 Agent platform and terminal | Adapter protocol, runs with live box, components, slash commands, backend policy with costs, apps and webhooks, SDKs, reference adapters for Hermes Agent and for a Claude Code runner; terminal with host service for macOS, Linux and Windows, end-to-end encrypted stream, its own permission per host and invariant I6 ([chapter 24](spec/24-terminal.md)) | built |
| M3 Platform services | Files with malware scan, images, voice messages, search, notifications and push | built |
| M4 Desktop clients | Design system, desktop app for macOS and Windows with offline operation, management and terminal | built |
| M5 Mobile clients | iOS and Android with push, sharing from other apps, voice recording and terminal with extra keys bar | built |
| M6 Console, statistics and visualization | Web console for management, statistics and terminal; live topology, backend view, delivery live, executive page, timeline and TV mode from own measurement ([chapter 25](spec/25-visualization.md)); the visualization packages are built as a track already after M1 | built |
| M7 Operations, release and showcase | Backup and restore with test, privacy flows, status page, signing, auto-update, portability, Grafana panel plugin "Bothy Live", demo instance, images and video | built |
| M8 Migration | Import from Discord, switchover channel by channel | built |
| V1 Publication | Repository public, checklist [public-switch](quality/public-switch.md) | in progress: the repository is public; enable list, external test, placeholders, checklist and name are open |
| V2 Multi-user development environment | Isolated development environments per person with their own terminal ([chapter 21](spec/21-dev-isolation.md)) | later |
| V3 Expansion | Features that [chapter 01](spec/01-product.md) lists as "later" | later |
