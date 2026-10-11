# Roadmap

Bothy is a command center for AI agents in chat form: chat, live view, grants,
terminal, costs and statistics, self-hosted and open source.

State: planning (milestone M0). Every milestone is closed in production quality: all
gates green, red-team gate with the verdict JA, documentation updated. There are deliberately
no dates; the order after M1 is set by the [plan](PLAN.md). What the product will never
do is in [chapter 01](spec/01-product.md).

| Milestone | Content | Status |
|---|---|---|
| M0 Planning | Specification, core contract, architecture decisions, toolchain, build tickets | in progress |
| M1 Foundation | Toolchain and local gates; server core with identity, devices, grants and `authorize`; spaces, channels, threads, messages; event log and synchronization; audit; observability foundation; server image with smoke test | planned |
| M2 Agent platform and terminal | Adapter protocol, runs with live box, components, slash commands, backend policy with costs, apps and webhooks, SDKs, reference adapters for Hermes Agent and for a Claude Code runner; terminal with host service for macOS, Linux and Windows, end-to-end encrypted stream, its own permission per host and invariant I6 ([chapter 24](spec/24-terminal.md)) | planned |
| M3 Platform services | Files with malware scan, images, voice messages, search, notifications and push | planned |
| M4 Desktop clients | Design system, desktop app for macOS and Windows with offline operation, management and terminal | planned |
| M5 Mobile clients | iOS and Android with push, sharing from other apps, voice recording and terminal with extra keys bar | planned |
| M6 Console, statistics and visualization | Web console for management, statistics and terminal; live topology, backend view, delivery live, executive page, timeline and TV mode from own measurement ([chapter 25](spec/25-visualization.md)); the visualization packages are built as a track already after M1 | planned |
| M7 Operations, release and showcase | Backup and restore with test, privacy flows, status page, signing, auto-update, portability, Grafana panel plugin "Bothy Live", demo instance, images and video | planned |
| M8 Migration | Import from Discord, switchover channel by channel | planned |
| V1 Publication | Repository public, checklist [public-switch](quality/public-switch.md) | planned |
| V2 Multi-user development environment | Isolated development environments per person with their own terminal ([chapter 21](spec/21-dev-isolation.md)) | later |
| V3 Expansion | Features that [chapter 01](spec/01-product.md) lists as "later" | later |
