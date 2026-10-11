# sdk/python

Status: planned. This folder contains no code yet.

## Purpose

Python SDK for agent adapters (among them the Hermes adapter) and apps.

## Structure (planned)

| Layer | Content |
|---|---|
| Core | generated with openapi-python-client from `docs/api/openapi.yaml`; never changed by hand; generated and checked in the gate |
| Hand-written layer | modules `auth`, `gateway`, `invocation`, `runs`, `components`, `commands`, `outbox`, `retry`, `heartbeat`, `webhooks`, `testing` according to docs/spec/11-extensions.md section 11.2 |

## Guidelines

- Asynchronous API first, a synchronous wrapper for scripts.
- Supported are the CPython versions that python.org lists as maintained.
- Few dependencies, compatible with the environment of Hermes Agent (checked in the fake Hermes test).
- Check of the invocation token only with `EdDSA` and `typ = agent-invocation+jwt`
  (docs/spec/09-agents.md section 4.5).
- Errors typed according to the problem format from docs/spec/06-api.md.

## Test doubles

`testing` provides an in-process FakeServer (127.0.0.1, port 0) with fault injection and the
contract scenarios from docs/spec/09-agents.md section 16, so that every adapter can run them
against itself.
