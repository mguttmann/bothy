---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team B (API), Team D (agent platform), Team A (architecture)
informed: Team E (apps), Team F (operations)
---

# One event envelope for gateway, REST catch-up and webhooks

## Context and Problem Statement

Events reach their receivers on four paths: as `dispatch` on the WebSocket gateway, through
`GET /api/v1/events` (REST catch-up), through the REST consumer of the agents and as an outgoing
webhook, also as a transport for agents ([04-sync-protocol.md](../spec/04-sync-protocol.md),
sections 2, 7 and 15). Agents additionally need details that apply to only one delivery:
a freshly issued invocation token and the flag of a redelivery. Does every event have the same
form on every path, and where are the delivery details?

## Decision Drivers

- SDKs, adapters and apps decode events with one type, no matter on which path they arrive
  (ADR 0026, R-API-06 in [06-api.md](../spec/06-api.md)).
- A receiver that switches from the gateway to webhooks does not change its processing.
- Invocation tokens are created anew per delivery and must never be stored (I3, ADR 0023).
- Fields have the same name everywhere, in snake_case (06, section 2).
- Webhooks follow Standard Webhooks for headers and signature (ADR 0016).

## Considered Options

- One envelope for all paths; delivery details as separate fields in the envelope, never in the
  log
- Own format per path (gateway frame, REST page, webhook body following the recommendation of
  Standard Webhooks with `type`, `timestamp`, `data`)
- CloudEvents 1.0 in the JSON format as the envelope
- One envelope for the event plus a separate wrapper for delivery details

## Decision Outcome

Chosen option: "One envelope for all paths, delivery details as fields in the envelope", because
it gives one form, one schema and one decoding for all receivers. The fields are normative in
[04-sync-protocol.md](../spec/04-sync-protocol.md), section 2.2 (R-SY-17).

1. Fields of the envelope: `id`, `seq` (durable only), `type`, `schema_version`, `occurred_at`,
   `actor`, `scope`, `resource`, `audience`, `payload`, `redacted`, `truncated`, `correlation_id`,
   `trace_context` (only to agent adapters), `client_msg_id`, `permission_version` (durable
   only). Ephemeral events use the same envelope without `seq`.
2. Delivery fields: `invocation_token` (invocation token for events that carry an invocation) and
   `redelivered` (redelivery after `agent.ack_timeout`). They are created per delivery and
   never stand in `event_log` ([03-data-model.md](../spec/03-data-model.md), `event_log`). For
   webhooks `sync_epoch` is added (ADR 0059).
3. On the gateway the envelope is the body of `{"op": "dispatch", "d": ...}`; in
   `GET /api/v1/events` an element of `events`; in the webhook the whole body, signed byte for
   byte. `webhook-id` is the event ID ([webhooks.md](../api/webhooks.md), section 2).
4. Schema: `EventEnvelope` in `docs/api/openapi.yaml`, per type `allOf` of the envelope and the
   type part; the AsyncAPI only references it (06, section 11).

### Consequences

- Good, because one type per event is enough in all generated clients and webhook receivers use
  the same fixtures as gateway clients.
- Good, because delivery fields are clearly marked and the storage rule stays simple: the log
  never contains a token.
- Bad, because the webhook body deviates from the recommendation of Standard Webhooks
  (`occurred_at` instead of `timestamp`, no `data`). The recommendation is not mandatory
  [Q1, verified]; headers and signature follow the standard.
- Neutral, because the schema contains fields that are never set for some receivers
  (delivery fields to people); there they are missing (ADR 0053).
- Neutral, because a change to the envelope affects all paths; changes are therefore additive only
  (04, section 6.1).

### Confirmation

- Gate G9 ([06-api.md](../spec/06-api.md), section 15): every sent gateway frame matches
  its schema.
- Gate G10: webhook conformance with test vectors.
- Contract test: the same event through the gateway, `GET /api/v1/events` and a webhook yields the
  same envelope; differences only in delivery fields.
- Test: `event_log` has no column and no payload with `invocation_token` or `redelivered`;
  a delivery to a person contains neither of the two fields.
- Build ticket M1-16 in [PLAN.md](../PLAN.md) (envelopes without JSON `null`, `correlation_id`
  equal to the trace ID).
- Built in M1-16 (`services/server/internal/sync`): `TestEnvelope_FieldsMatchOpenAPI` holds that
  the JSON members of `sync.Envelope` and its nested types equal the properties of
  `EventEnvelope`, `EventActor`, `EventScope` and `EventResource` plus `type` and `payload`;
  `TestEnvelope_DeliveryFieldsOnlyWhenSet` holds that delivery members appear only from
  `DeliveryFields`, and `TestMigration_EventLogMatchesSpec` that `event_log` has no column for
  them.

## Pros and Cons of the Options

### One envelope with delivery fields

- Good, because one form everywhere.
- Bad, because the boundary between stored and delivery-related fields is held only by rule and
  test, not by the form.

### Own format per path

- Good, because every path can follow its own convention.
- Bad, because three schemas, three decoders and three test suites are created and a change of
  the path changes code.

### CloudEvents 1.0

- Good, because a widespread standard with the required fields `id`, `source`, `specversion`,
  `type` [Q2, verified].
- Bad, because names of context attributes may contain only lowercase letters and digits
  [Q2, verified]; `permission_version`, `client_msg_id` and `correlation_id` would have to be
  named differently than in the API and the data model.
- Bad, because `seq`, `scope`, `resource` and `audience` would be extensions that no
  standard tool evaluates; the benefit of the standard would stay small.

### Separate wrapper for delivery details

- Good, because the form itself shows what is stored.
- Bad, because agent consumers would get a second form and the webhook body would no longer be
  the event; signature and duplicate detection through `webhook-id` would become more complicated.

## More Information

- Sources (retrieved 2026-10-10): Q1 Standard Webhooks, section "Payload structure",
  https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md
  [verified]; Q2 CloudEvents 1.0, sections "Naming Conventions" and "REQUIRED Attributes",
  https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md [verified].
- Related: ADR 0009 (event log), ADR 0013 (spec-first), ADR 0016 (webhooks), ADR 0023
  (invocation token), ADR 0053 (no JSON `null`).
