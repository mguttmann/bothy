# Telemetry privacy

Status: draft M0. Rules for [17-observability.md](../spec/17-observability.md) 4. Applies to the
server, `bothy-backupd`, adapter SDKs and the collector. Privacy in general: chapter 20.

## 1. What never goes into telemetry

| Class | Examples |
|---|---|
| Content | Message text, drafts, channel prompts, skill names, file names, file contents, transcripts, search terms, form values of components |
| AI content | Prompts, responses, system instructions, tool definitions, arguments and results of tools |
| Secrets | Tokens, passwords, keys, cookies, the headers `Authorization` and `DPoP` |
| Direct personal data | Email addresses, display names, handles of people, IP addresses, device names |
| Free text | Error messages of third-party libraries, query strings, raw paths |

## 2. Two lines

| Line | Where | How |
|---|---|---|
| 1 | At the source (server, SDK) | Instrumentation sets only attributes from the catalogue ([signals.md](signals.md) 5.3); HTTP instrumentation without `url.query`; database spans without parameters; log messages are constants |
| 2 | In the collector | `transform/scrub` deletes known content fields; `redaction/allowlist` removes from spans and logs all keys outside the allowlist (including resource attributes) and replaces values that look like a token, a Bearer header, an Anthropic key or an email address with HMAC-SHA-256; `transform/scrub_metrics` deletes person attributes on metrics of third-party sources |

The second line catches errors of the first. Checked locally on 2026-10-10 with otelcol-contrib 0.162.0
and the real file `infra/otel/collector.yaml`: a span with content in `url.query`,
`message.content`, `gen_ai.input.messages` and `exception.message` and a log with `prompt` arrived
without these values; an email address and a Bearer header in allowed attributes arrived
as HMAC [verified]. This showed: the redaction processor also filters resource attributes;
`service.name` and `deployment.environment.name` are therefore explicitly in the allowlist.

Evidence for the processor (retrieved 2026-10-10):
https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/redactionprocessor/README.md
(the allowlist closes correctly, `hmac-sha256` requires a key of at least 32 bytes;
`otelcol-contrib validate` reported the minimum length) [verified].

## 3. Pseudonyms

1. People appear in spans and logs only as `bothy.principal.pseudo_id`. Derivation: period
   `p` = integer of Unix seconds divided by `privacy.telemetry_pseudonym_rotation` (UTC);
   period key `K_p` = HMAC-SHA-256(secret from `telemetry.pseudonym_key_ref`,
   `"bothy/telemetry-pseudonym/v1/"` + `p` in decimal); pseudonym = HMAC-SHA-256(`K_p`, principal ID
   as a UUID in lowercase), hex, truncated to 32 characters.
2. Whoever has the database and the secret can map a pseudonym to a person
   (`bothy telemetry pseudonym <principal_id> [--at <time>]` for the owner's troubleshooting).
   A telemetry provider alone cannot.
3. Pseudonyms change per period and cannot be linked across periods without the secret;
   this limits long-term profiles. There is only one secret to store
   ([16-operations.md](../spec/16-operations.md) 5.4). A change of the secret itself additionally
   separates all old pseudonyms.
4. Agents and apps are not people; their handles appear in plain text.
5. Device IDs never appear in spans, logs or metrics, not even as a pseudonym
   ([19-security.md](../spec/19-security.md) SEC-LOG-4).
6. Object IDs (thread, channel, message, run, file) appear in plain text in spans and logs, never
   on metrics. They are random UUIDs without content; they can be resolved only with database
   access, and then the permissions from `authorize` apply.

## 4. No monitoring of individual people

1. Metrics never carry attributes that identify a person.
2. Dashboards and alerts show no values per person.
3. The in-app statistics show counters about people only from `privacy.stats_min_group_size`
   ([admin-statistics.md](admin-statistics.md)).
4. Reason: telemetry serves operations. In Germany, an evaluation of the behavior or performance
   of individual employees can be subject to co-determination (§ 87(1) no. 6 of the Works
   Constitution Act, Betriebsverfassungsgesetz) [derived]. Whether a business may introduce
   telemetry is for the instance operator to clarify; the product makes evaluation for other purposes hard.

## 5. Retention

| Location | Period | Key |
|---|---|---|
| Prometheus (local) | `BOTHY_PROMETHEUS_RETENTION` | Flag `--storage.tsdb.retention.time` |
| Loki (local) | `BOTHY_LOKI_RETENTION` | `limits_config.retention_period`, compactor |
| Tempo (local) | `BOTHY_TEMPO_RETENTION` | `block_retention` |
| stdout logs (Docker) | Rotation by `BOTHY_LOG_MAX_SIZE` and `BOTHY_LOG_MAX_FILE` | Log driver |
| External backends | According to the provider's contract | Setting at the provider |

The retention periods of telemetry are shorter than those of the audit log and independent of them
([audit-vs-telemetry.md](audit-vs-telemetry.md)). A privacy request for erasure (chapter 20)
does not reach telemetry in a targeted way; therefore telemetry contains no direct personal data,
and the periods are short.

## 6. Canary test (gate)

1. Test data contains a random secret word per run in every content field (message,
   file name, search term, prompt, form value, tool argument of a test adapter,
   display name, email).
2. A scenario runs through the API, gateway, push (test provider), adapter and collector.
3. The collector exports with the overlay `local` and additionally to a file (test overlay
   `infra/compose/test/otel/file.yaml` with the `file` exporter, only for tests, not in the
   release bundle; checked with `otelcol-contrib validate` in `tools/opscheck`); the stdout of the
   containers is captured.
4. The secret word must not appear in any output. A match breaks the gate and names the location.
5. In addition: no principal ID of the test data may appear in plain text.

The in-process part of this gate is `services/server/test/canary` (M1-69); the collector part with the `file` exporter and container stdout is M1-73.

## 7. Apps

Apps send no telemetry to the collector. Crash reports go only to a target that the
publisher sets in `distribution.yaml` ([18-release.md](../spec/18-release.md) 8). Before
sending, the app removes message texts, inputs, file names and query strings ([15-client-architecture.md](../spec/15-client-architecture.md)). Without a target, the app sends nothing.
