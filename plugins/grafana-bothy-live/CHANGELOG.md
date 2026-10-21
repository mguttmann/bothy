# Changelog

All notable changes to the Grafana panel plugin `bothy-live-panel` (`plugins/grafana-bothy-live`)
are documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one (docs/spec/18-release.md, component
`grafana-bothy-live`).

## [Unreleased]

### Added

- Panel plugin `bothy-live-panel`, scaffolded with `@grafana/create-plugin` 7.13.0 for
  Grafana 13.2.3 (TypeScript, React), unsigned.
- Five views from metrics only (docs/spec/25-visualization.md section 8.2): live topology as
  lanes per agent, backend view, delivery and sync, executive page with the SLO traffic light
  from the Sloth recording rules, and the timeline as playback over the dashboard period.
- Theme adapter: Grafana's light or dark theme selects the token theme of
  `design/build/ts/tokens.gen.ts`; the bundled Atkinson Hyperlegible fonts with their OFL texts.
- `queries.yaml` as the only source of the queries, the generator `scripts/gen-queries.mts`
  and its check `make generate-check`.
- Lint rule `bothy/no-color-literal`: no colour literal in `src/`.
