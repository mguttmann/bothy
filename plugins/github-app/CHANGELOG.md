# Changelog

All notable changes to the GitHub reference app (`plugins/github-app`) are documented in this
file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this
component adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is
the [`VERSION`](VERSION) file next to this one; the app is no release component
(docs/spec/11-extensions.md section 10).

## [Unreleased]

### Added

- Reference app for code hosting (M2-49, docs/spec/11-extensions.md 10.1): its own Node.js
  process on the TypeScript SDK, started as `node dist/main.js --config <file>`.
- GitHub webhooks at `POST /github/webhook`, checked with `X-Hub-Signature-256` over the raw body
  in constant time before anything is parsed; duplicates by `X-GitHub-Delivery`; the events
  `issues`, `issue_comment`, `pull_request`, `pull_request_review` and `ping`.
- One forum post per issue or pull request with one message per event; GitHub text only in code
  spans and code blocks; a post out of reach is replaced once.
- `/issue` with a form through `delivery.interactions_url`: the issue is created through the
  GitHub REST API at most once and the app answers in the post.
- Optional mirroring of people's replies to GitHub comments with loop and echo protection.
- State directory with lock, job queue and idempotency identifiers; JSON logs with an attribute
  allow list; manifest, example configuration and tests against a Fake-GitHub API and the real
  server.
