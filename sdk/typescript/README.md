# sdk/typescript

Status: planned. This folder contains no code yet.

## Purpose

TypeScript SDK for apps, integrations and agent adapters in JavaScript environments.

## Structure (planned)

| Layer | Content |
|---|---|
| Core | types generated with openapi-typescript from `docs/api/openapi.yaml`; a runtime client built on them (choice in the build ticket); generated and compiled in the gate |
| Hand-written layer | modules `auth`, `gateway`, `invocation`, `runs`, `components`, `commands`, `outbox`, `retry`, `heartbeat`, `webhooks`, `testing` according to docs/spec/11-extensions.md section 11.2 |

## Guidelines

- ESM; active LTS lines of Node.js.
- In the browser only parts without secrets; agent and app tokens never belong in a browser.
- Check of the invocation token and the webhook signatures with a JOSE library that supports EdDSA
  and an algorithm allow-list (candidate and evidence in docs/spec/11-extensions.md
  section 11.3).
- Errors typed according to the problem format from docs/spec/06-api.md.

## Test doubles

`testing` provides the same FakeServer and the same contract scenarios as the Python SDK.
