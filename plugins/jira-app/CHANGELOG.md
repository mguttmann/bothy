# Changelog

All notable changes to the Jira reference app (`plugins/jira-app`) are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
component adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is
the [`VERSION`](VERSION) file next to this one; the app is no release component
(docs/spec/11-extensions.md section 10).

## [Unreleased]

### Added

- Reference app for ticket systems (M2-50, docs/spec/11-extensions.md 10.2): its own Python
  process on the Python SDK, started as
  `uv run --directory plugins/jira-app --locked python -m jira_app --config <file>`.
- Jira webhooks at `POST /jira/webhook`, checked with `X-Hub-Signature` before anything is
  parsed: the method is read from the header and only `sha256` is accepted, every other method
  is refused before any HMAC is computed; HMAC-SHA256 over the raw body compared in constant
  time; duplicates by the SHA-256 of the body; the events `jira:issue_created`,
  `jira:issue_updated` and `comment_created`.
- One forum post per Jira issue (keyed by the issue ID) with one message per event; Jira text
  only in code spans and code blocks, links only to `<jira_base_url>/browse/<KEY>`; a post out of
  reach is replaced once.
- `/issue` with a form through `delivery.interactions_url`: the issue is created through the
  Jira REST API (version 3, ADF, basic authentication) at most once and the app answers in the
  post.
- Optional mirroring of people's replies to Jira comments with loop and echo protection.
- SQLite state directory with `flock` lock, job queue and idempotency identifiers; JSON logs
  with an attribute allow list; manifest, example configuration and tests against a Fake-Jira
  API and the real server.
