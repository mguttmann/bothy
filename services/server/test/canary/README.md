# Package `test/canary`

Status: built (M1-69). Component `testsupport` (M1-05), never imported by production code. The
in-process canary test SEC-T10 and DP-T09 behind threat T-LG-01
(`docs/security/threat-model.md`); the collector part is M1-73.

## Purpose

A scenario drives every enabled operation of `api/operations.yaml` and every client gateway op
of M1 against one harness server, with run-unique canary values in the inputs. Every output of the
server is captured and `Scan` proves that no value appears where the allow matrix of its class
forbids it. A new operation that the scenario does not call fails
`TestSEC_T10_ScenarioCoversEveryEnabledOperation`.

## Tests

| Test | What it proves |
|---|---|
| `scan_test.go` | the matcher: plain and case-insensitive, base64 at all three alignments in both alphabets, hex, JWS without the header segment, IPv4 and both IPv6 forms, ID forms, the allow matrix per class and sink, echo fields, a finding never prints its value |
| `ops_test.go` | the coverage record names an enabled operation without a call |
| `TestSEC_T10_DetectorFindsInjectedLog` | a canary logged through the OTLP log pipeline of the server as an attribute is exactly one finding at that key |
| `TestSEC_T10_DetectorFindsInjectedSpanAttribute` | a canary span attribute through the server's tracer provider is a trace finding |
| `TestSEC_T10_DetectorFindsStdLoggerLine` | a canary written through the standard logger during a run is a log finding |
| `TestSEC_T10_DetectorFindsClearPrincipalIDInLog` | a clear principal ID in a log attribute is an identity finding |
| `TestSEC_T10_CanaryAcrossAllOutputs` | the full scenario, zero findings; every class has values, every file of the secrets directory and the key store is registered, the IP canaries reached the audit IP path, the message delete left a redacted event, the permitted sinks saw their values |
| `TestSEC_T10_ScenarioCoversEveryEnabledOperation` | every enabled operation called, every op of `wire.GatewayOpClasses` sent |
| `TestSEC_T10_TerminalInputOutput` | the terminal step with Noise (M2-46): the canary input arrives in clear at the Fake-Host, the canary output at the device, the session token is the payload of message 1, the raw relay messages are a sink, and no terminal canary is in any sink |
| `TestDP_T09_NoClearPrincipalOrDeviceIDsInTelemetry` | no principal or device ID in logs, traces or metrics, and `bothy.principal.pseudo_id` on request spans |
| `TestSEC_T10_RateLimitedResponse` | a second server with default limits answers canary logins with `429` on `rl.public_auth`; zero findings |

The scenario runs once per test binary; the tests share its collected sinks.

```sh
cd services/server
go test -race -count=1 ./test/canary/... ./test/harness/...
```

## Files

- `values.go`: `Registry`, `Class`, `Canary`; `New` returns `cnry` plus 20 lowercase hex characters
  from `crypto/rand`, `Register` adds issued values.
- `sinks.go`, `collect.go`: records and fields, `LineWriter` for the log stream,
  `CollectTelemetry`, `CollectAudit`, `ErrorObserver`.
- `scan.go`: `Scan`, `Matches`, `Report`, the allow matrix.
- `secretfiles.go`: the forms of a secret file, the redirection of the standard loggers.
- `ops.go`: operation IDs of requests, the enabled list, `Coverage`.
- `scenario*.go`, `drive.go`: `StartRun`, `NewScenario` and the steps; `Exempt` (empty).
  `scenario_files.go` is the path "file name" (M3-03): an upload of Alex whose name is a canary
  of a scope no observer reads (before attachment only the uploader reads a file), with a part,
  completion, `getUpload`, `getFile`, an aborted second upload and a `413` and a `400` with it.
  `scenario_terminal.go` drives the terminal sessions (M2-44): a host paired and confirmed by the
  owner and connected by a Fake-Host, a shell opened, listed, read, attached and closed; the session
  tokens of the grants are registered as secrets.
  `scenario_commands.go` drives the six command operations (M2-16) and autocompletion (M2-17): Lotse registers, reads, updates
  and removes a control command of Support/Allgemein whose description is a canary, Alex lists,
  reads and invokes it with a canary option value, once more with a value outside SEC-IN-4 (400).
  Since M2-46 the host has a real X25519 key and a
  second shell runs the handshake `Noise_NK_25519_ChaChaPoly_SHA256` between the device test client
  and the Fake-Host (`test/terminal/fakehost`): `start`, a `data` frame with a canary input
  (content of the session's scope, which no observer reads), a `resize`, the Fake-Host's `data`
  frame with a canary output, and `close`. `Scenario.TerminalIO` keeps the positive controls.

## Sinks

1. Logs: the stdout JSON lines (`telemetry.logs.exporter=both`, `telemetry.logs.level=debug`), the
   OTLP log records, and the standard logger and slog's default logger during the run.
2. Traces: span names, attributes, events, status descriptions, link and resource attributes
   (`telemetry.traces.sampler_ratio=1.0`).
3. Metrics: metric names, attribute sets, exemplar attributes, resource of a manual reader.
4. Audit: every column of every `audit_events` row; `details` by member.
5. Error responses: body fields and every header of each answer with status 400 or higher.
6. Foreign events: the gateway frames, every `listEvents` page from cursor 0 and the snapshot of
   the observers Sam (reads Marketing and Support/Allgemein), Robin (reads Engineering/Projekt-A)
   and Alex (reads Support), and the `listEvents` pages of the operator Lotse.
7. Relay (M2-46): every raw binary WebSocket message the device and the Fake-Host of the terminal
   step sent and received, what passed the relay. No value of any class is allowed there: terminal
   content is encrypted end to end, and message 1 carries the session token only encrypted.

## Allow matrix

| Class | Allowed only in |
|---|---|
| `content` | events of observers that may read its scope (an instance-wide value: every observer); echo fields of error responses |
| `admin_reason` | as content, and `audit_events.details` member `reason` |
| `secret` | nowhere |
| `network` | nowhere (the audit log holds only `ip_hmac` with `ip_key_id`) |
| `identity` | everywhere except logs, traces and metrics |
| `manage_only` | success bodies of `getChannelBehavior`, `putChannelBehavior`, `listServedChannels` and `getServedChannelConfig` (no sink captures them) and `current` of a `412` of `putChannelBehavior`; nowhere else |

`manage_only` (M2-02) holds the channel prompt and the skill references. It is stricter than
`content`: a reader of the channel is no permitted observer, because prompt and skills are readable
only with `manage` or as the effective operator (R-AZ-09), and even the operator agent receives
only the settings version in its events. Error records name their operation, so only the `412` of
`putChannelBehavior`, whose caller passed `manage` before the version check, may echo the value.
Alex (reads Support without `manage`) is an observer with gateway, `listEvents` and `getSnapshot`;
the pages of the operator Lotse are foreign records too.

Echo fields (ruling M1-69/2) are the fields of an error response the contract returns to the
caller: `current` of `version_conflict` and `base_version_conflict` (06 "Errors" item 3, the
representation the caller may read) and the `pointer` of an `unknown_member` error, which names
the member the caller sent (W7, M1-18/3). A secret in an echo field is still a finding.

## Matching

Case-insensitive; every value also as hex of its bytes and as the stable part of its base64
encodings (standard and URL alphabet, with or without padding) at the three byte alignments. A
compact JWS is matched by its payload and signature segments (the header repeats across tokens).
Secret files: the trimmed content, the bytes a hex or base64 text decodes to, every PEM body line
and the private key material, the `d` of a JWK. IPv4 dotted, IPv6 compressed and expanded. IDs as
UUID text and as 32 hex digits. Texts shorter than `minNeedle` (`scan.go`) are not searched.

## Adding an operation

Call the new operation in the step of its area (`scenario_chat.go`, `scenario_admin.go`,
`scenario_auth.go`, `scenario_apps.go`, `scenario_files.go`) with `sc.Reg.New(...)` in every free-text input, the
scope of the channel or space it belongs to, and the expected status. An operation the scenario
cannot call goes into `Exempt` with its reason; the expected list is empty.

## Not covered

- The collector path and container stdout (M1-73).
- The logs of the PostgreSQL server itself.
- Libraries that write to the process file descriptors directly, not through `os.Stdout` writers
  of the server, the standard logger or slog.
- Texts shorter than `minNeedle` (`scan.go`): six-digit TOTP codes are registered, but would match
  unrelated digits.
- File names, transcripts, search terms, push tokens, component form values and tool arguments:
  their tickets in M2 and M3 extend the scenario (channel prompts and skill references since
  M2-02).
- The `current` member of a 412 and the `pointer` of `unknown_member` are judged as echoes to the
  caller, not as leaks (ruling M1-69/2).
