# 19 Security

Status: draft M0. Normative for server, clients, admin console and operations.
The analysis (system boundaries, assets, attackers, STRIDE, threat register with residual risk) is in
[../security/threat-model.md](../security/threat-model.md). This chapter defines the controls that
the threat model refers to. Every control has an ID (`SEC-...`) and at least one
test (section 14).
Permissions: [07-permissions.md](07-permissions.md). Identity and tokens: [08-identity.md](08-identity.md).
Privacy: [20-privacy.md](20-privacy.md). Dev isolation:
[21-dev-isolation.md](21-dev-isolation.md). Quality gates and supply chain:
[23-quality.md](23-quality.md). Releases: [18-release.md](18-release.md).

## 1. Trust assumptions

| Assumption | Consequence if it breaks |
|---|---|
| The server host and its operating system are trustworthy. | Everything is open; the product cannot catch that. Hardening the host is the duty of the instance operator ([16-operations.md](16-operations.md)). |
| The owner is trustworthy, but not omniscient. | Owner mistakes are possible; hence preview, step-up, audit, invariants. DMs stay protected even from the owner (07, SR6). |
| Whoever has write access to the database can change everything. | The hash chain and exported checkpoints make changes to the audit log detectable, not impossible (ADR 0021). |
| Clients, agent runtimes, apps and content are not trustworthy. | The server checks every input and every action itself. |
| LLM output is not trustworthy. | Prompt injection is not solved; the limits are permissions, tool classes and approval cards, not the model. |

## 2. Transport

| ID | Control |
|---|---|
| SEC-TLS-1 | The public listener (`server.listen_addr`) accepts plaintext HTTP only on loopback or from peers in `server.trusted_proxies` ([16-operations.md](16-operations.md)) (reverse proxy on the same host or in the same container network). Every other plaintext connection is closed during connection setup, without reading or writing. The internal listener is exempt (R-SE-09). |
| SEC-TLS-2 | Where the server terminates TLS itself (`server.tls_cert_ref` and `server.tls_key_ref`, both set or both empty; with TLS, `server.public_url` must be `https`): at least TLS 1.2, TLS 1.3 preferred, cipher suites from the Go standard library without a list of our own. Reason: a list of our own becomes outdated; the default selection is maintained along with Go [derived]. No redirect listener for plaintext. |
| SEC-TLS-3 | HSTS with `http.hsts_max_age` on every response as soon as `server.public_url` uses `https` (also behind a TLS proxy): `Strict-Transport-Security: max-age=<seconds>`, without `includeSubDomains` (other subdomains belong to the instance operator) and without `preload` (not reversible). With an `http` URL (development on loopback) the header is absent. |
| SEC-TLS-4 | Outgoing TLS connections always verify the certificate and the host name. `InsecureSkipVerify` is forbidden by lint. |
| SEC-TLS-5 | PostgreSQL via Unix socket, with `sslmode=verify-full`, or in the internal Compose network `internal` without egress and without a host port with `sslmode=disable` ([16-operations.md](16-operations.md), section 4.1, rule 2). Virus scanner only via Unix socket (clamd has no authentication in its protocol; ClamAV recommends the local mode; https://raw.githubusercontent.com/Cisco-Talos/clamav/main/etc/clamd.conf.sample, retrieved 2026-10-10 [verified]). OTel Collector via loopback, TLS or a Compose network of the same host without a published port for receiving (telemetry carries no content and no secrets, Baseline B11 and SEC-LOG-6); every route over another network with TLS. |
| SEC-TLS-6 | Host names with public certificates appear in Certificate Transparency logs (https://tailscale.com/kb/1153/enabling-https, retrieved 2026-10-10 [verified]). The operations documentation recommends neutral host names. |

## 3. HTTP security headers

| ID | Responses | Headers |
|---|---|---|
| SEC-HDR-1 | all | `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Strict-Transport-Security` (SEC-TLS-3), no version information in `Server` |
| SEC-HDR-2 | API (JSON) | `Cache-Control: no-store`, `Cross-Origin-Resource-Policy: same-origin`, `Content-Type: application/json` or `application/problem+json` |
| SEC-HDR-3 | console (HTML and assets) | CSP according to section 5, `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Embedder-Policy: require-corp`, `X-Frame-Options: DENY`, `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()` |
| SEC-HDR-4 | file retrieval | `Content-Disposition: attachment` with `filename*` (RFC 6266), except for the inline list from [12-files.md](12-files.md), section 9.3; `Content-Security-Policy: default-src 'none'; sandbox`; `Cache-Control` according to 12, section 9.3 (always `private`, `no-store` with a ticket) |

COOP and COEP make the console "cross-origin isolated". Flutter needs this for multi-threaded
rendering with WebAssembly; without the headers it runs single-threaded
(https://docs.flutter.dev/platform-integration/web/initialization, option
`forceSingleThreadedSkwasm`, retrieved 2026-10-10 [verified]). They only work because all assets
come from the instance's own origin (section 5).

## 4. CORS and WebSocket origin

| ID | Control |
|---|---|
| SEC-CORS-1 | The API sends no CORS allow headers. Native apps need no CORS; the console is same-origin. |
| SEC-CORS-2 | Only exception: public, non-personal responses without login (`instance.discovery.read`, JWKS) with `Access-Control-Allow-Origin: *` and without credentials. |
| SEC-CORS-3 | WebSocket upgrade: if an `Origin` header is present, it must equal the origin of the instance; otherwise `403` before the upgrade. Native clients send no foreign `Origin`. Reason: protection of the console session (cookie) against cross-site WebSocket hijacking. |
| SEC-CORS-4 | State-changing requests of the console carry a dedicated mandatory header and a matching `Origin` (08, section 7.5). |

## 5. Content Security Policy of the web console

The console is a Flutter web application ([15-client-architecture.md](15-client-architecture.md),
ADR 0028). Goal: no script, no style, no font and no WebAssembly from foreign origins.

```text
Content-Security-Policy:
  default-src 'none';
  script-src 'self' 'wasm-unsafe-eval';
  style-src 'self' 'nonce-<random per response>';
  img-src 'self' blob: data:;
  font-src 'self';
  connect-src 'self';
  worker-src 'self' blob:;
  manifest-src 'self';
  base-uri 'none';
  form-action 'self';
  frame-ancestors 'none';
  object-src 'none';
  require-trusted-types-for 'script';
  trusted-types <policy names of the Flutter loader and the engine>;
  report-to csp
```

Rules:

1. CanvasKit or skwasm and the fallback fonts are shipped with the console. The loader gets
   `canvasKitBaseUrl` and `fontFallbackBaseUrl` pointing to the instance's own origin; otherwise the
   default for fallback fonts would be `https://fonts.gstatic.com/s/`
   (https://docs.flutter.dev/platform-integration/web/initialization, retrieved 2026-10-10 [verified]).
2. The loader gets the option `nonce`; according to the Flutter documentation it applies to scripts
   and styles that the engine inserts itself (same source [verified]). The server generates the
   nonce per response.
3. `'wasm-unsafe-eval'` is needed because the renderer compiles WebAssembly [derived]. No
   `'unsafe-eval'`, no `'unsafe-inline'` for scripts.
4. The names of the Trusted Types policies of the Flutter loader and the engine are not confirmed
   by primary sources [assumed: `flutter-js`, `flutter-engine`]. The test SEC-T05 loads the
   console in a headless browser with an enforced CSP and fails on every violation; the
   list in the repo follows this test, not this documentation.
5. In current browsers, `connect-src 'self'` covers the WebSocket to the same host [assumed];
   SEC-T05 checks that too. If it fails, `wss://<host>` is added.
6. Violations go to an endpoint of the instance (`report-to`), with a rate limit and without content
   in the log.
7. `frame-src` is in the CSP only if `console.grafana_embed_url` is set, and names exactly
   its origin (`frame-src https://grafana.example.com`); otherwise `default-src 'none'` blocks every
   iframe. The embedding is optional and off by default; its conditions (auth proxy,
   `allow_embedding`, `frame-ancestors`, COEP and CORP at the proxy) are in
   [25-visualization.md](25-visualization.md), section 9.2 (SEC-VZ-2). `frame-ancestors 'none'`
   and `X-Frame-Options: DENY` of the console itself stay unchanged.

## 6. Inputs and outputs

### 6.1 Input validation in the server

| ID | Control |
|---|---|
| SEC-IN-1 | Every request is checked against the OpenAPI or AsyncAPI schema before a handler runs ([06-api.md](06-api.md), ADR 0013). Schemas for writes have `additionalProperties: false`. |
| SEC-IN-2 | JSON decoding rejects duplicate keys, invalid UTF-8 and nesting that is too deep, with `400 malformed_request` ([06-api.md](06-api.md), section 5; on the gateway close code `4002`). Reason: `client_payload_sha256` (03) and processing must not see different readings of the same request. |
| SEC-IN-3 | Size limits before reading: `http.max_json_body_bytes` for JSON, `http.max_header_bytes` (both 02), `sync.ws_max_frame_bytes` (04), `file.max_upload_bytes` (03) as a stream limit. Time limits: `http.read_header_timeout`, `http.idle_timeout` (02). |
| SEC-IN-4 | Display texts (display names, names of channels, spaces, groups, tags and templates, titles, file names, product name of the branding, reasons of administrative actions) are checked for control characters and bidi control characters and rejected. Message content stays unchanged; clients isolate foreign names when rendering (Unicode isolation). |
| SEC-IN-5 | Database access only via sqlc with parameters. A lint forbids assembled SQL texts outside the generated code. |
| SEC-IN-6 | Regular expressions only with the Go library `regexp` (linear run time). |
| SEC-IN-7 | Patches to run documents only on allowed JSON Pointer paths (04, section 11.2). |
| SEC-IN-8 | Component definitions (`component.v1`) and command options are checked with length and count limits from the schema. |

### 6.2 Rendering in the clients

| ID | Control |
|---|---|
| SEC-OUT-1 | Markdown without raw HTML. Links only with `https`, `http`, `mailto` or internal references; the target is shown with its domain on tap or hover. |
| SEC-OUT-2 | Clients never load foreign resources from content automatically: no images via URL, no link preview, no favicons. Images only from files of the server. Reason: otherwise an agent or attacker could exfiltrate data via image URLs and learn the IP addresses of the readers (threat T-AI-06). |
| SEC-OUT-3 | Messages from agents, apps, imports and the system are visibly distinguished (badge according to `origin`). Only the server creates system messages (`kind = 'system'`). |
| SEC-OUT-4 | Approval cards show a header that the client builds from server fields (agent, run, required capability), separate from the agent's text. An agent cannot influence the header. |
| SEC-OUT-5 | Error responses according to RFC 9457 without a stack trace and without internal details; with `correlation_id`. |

## 7. Upload hardening

| ID | Control |
|---|---|
| SEC-UP-1 | Size limited before storing (`file.max_upload_bytes`, 03); abort with `413` as soon as the limit is reached in the stream. |
| SEC-UP-2 | The MIME type is detected by the server, never taken over from the client (03, `files.mime_type`). |
| SEC-UP-3 | Inline rendering only for the inline list from [12-files.md](12-files.md), section 9.3: raster images that the server has sanitized or re-encoded according to 12, section 7.1, their thumbnails and the audio and video types named there. SVG, HTML, PDF and everything else only as an attachment (SEC-HDR-4). |
| SEC-UP-4 | The image size is read from the file header before decoding; above `file.image.max_pixels` ([12-files.md](12-files.md)) no preview. Protection against decompression bombs. |
| SEC-UP-5 | Parsers for untrusted formats run in a separate process with memory and time limits, never in the API process. M1 has only image previews in pure Go (Baseline R-Objektspeicher). |
| SEC-UP-6 | Virus scan as a separate service (ADR 0033). Files are served only when `scan_status IN ('clean','skipped')` (03). If the scanner is not ready, files stay `pending` (fail closed). `infected`: never servable, notification to the owner. |
| SEC-UP-7 | Archives are never unpacked in the server. |
| SEC-UP-8 | Object keys are random and contain no names (03). Original names only sanitized in `Content-Disposition`. |
| SEC-UP-9 | A file is attached only to messages of its channel `scope_channel_id` (03). This way a file cannot cross space boundaries into the context of another agent. |
| SEC-UP-10 | Location data and other personal metadata are always removed from stored images: for JPEG and PNG losslessly on upload (segment and chunk filter), thumbnails re-encoded (ruling L16, [ADR 0047](../adr/0047-remove-image-metadata-losslessly.md), [12-files.md](12-files.md) section 7). |
| SEC-UP-11 | Clients get no permanent object storage URLs; every retrieval goes through `file.read` (07, section 8.1; [ADR 0048](../adr/0048-file-download-only-through-the-server-with-short-lived-tickets.md)). |

Storage locations, preview variants, quotas and the scan service in detail: [12-files.md](12-files.md).

## 8. SSRF ban and outgoing connections

Baseline section 4: no server-side loading of foreign URLs.

| ID | Control |
|---|---|
| SEC-NET-1 | All outgoing HTTP connections go through an egress client in the server (`platform/egress`). A lint forbids `http.DefaultClient`, `http.Get`, `http.Post`, custom transports and dialers outside this package. For `bothy healthcheck` that package has a loopback client (loopback only, `http` only, no redirect). The only exception is the OTLP export to the collector: its target comes from the configuration of the instance operator (`OTEL_EXPORTER_OTLP_*`), never from data, and the exporters of the SDK choose their dialer themselves. |
| SEC-NET-2 | The only allowed targets are: configured identity providers (the issuer and the endpoints of the same provider discovered from it), push services, `webhook.allowed_hosts` (03), object storage, transcription service and entries in `egress.allowed_hosts`. A consumer sets its targets when it creates its client (`Options.Hosts` of the egress client, PLAN M1-22); for an identity provider they are the host of the issuer and the hosts of `token_endpoint` and `jwks_uri` from its verified discovery document ([08-identity.md](08-identity.md), section 2.2). Everything else: rejection before the connection is set up, without a DNS query. Environment proxies (`HTTP_PROXY`, `HTTPS_PROXY`) do not apply, because otherwise the check would only see the proxy. `http` only for entries with `allow_private`. |
| SEC-NET-3 | The IP is checked after DNS resolution during connection setup (not before), so that DNS rebinding does not work; the connection goes to the checked address, never to the name again. Always blocked, even with `allow_private`: unspecified addresses (`0.0.0.0/8`, `::`), link-local including the metadata address of the cloud providers (`169.254.0.0/16`, `fe80::/10`; RFC 3927), multicast (`224.0.0.0/4`, `ff00::/8`), broadcast and reserved addresses (`240.0.0.0/4`), IETF protocol assignments (`192.0.0.0/24`), benchmarking (`198.18.0.0/15`), deprecated site-local (`fec0::/10`), discard-only (`100::/64`), IPv4-compatible addresses (`::/96`), NAT64 (`64:ff9b::/96`, `64:ff9b:1::/48`), 6to4 (`2002::/16`), Teredo (`2001::/32`) and the metadata addresses of the cloud providers that lie inside private ranges (`fd00:ec2::254`, host `.200` of `100.100.100.0/24`; sources in ruling M1-22/15 of docs/DECISIONS.md); IPv4-mapped IPv6 addresses are unpacked before the check. Blocked except with a matching entry in `egress.allowed_hosts` with `allow_private`: loopback, private networks (RFC 1918), shared address space (RFC 6598, also used by overlay networks), unique local addresses (RFC 4193). Documentation ranges (RFC 5737, RFC 3849) are not blocked; they are not routed on the internet. |
| SEC-NET-4 | No automatic following of redirects for webhooks; for OIDC only to allowed targets. |
| SEC-NET-5 | Time and size limits per response (`egress.timeout`, `egress.max_response_bytes`); a response that is too large is an error, never silently truncated. Rejected attempts are counted by `bothy.egress.blocked` ([signals.md](../observability/signals.md)) and logged with purpose, reason and host, never with path or content. |
| SEC-NET-6 | Clients load no link previews (SEC-OUT-2). Agent tools with network access live in the runtime, outside the server; their limit is the tool class (07, section 2.11). |

## 9. Logs without secrets and without content

| ID | Control |
|---|---|
| SEC-LOG-1 | Structured logs (`log/slog`). Secrets have their own type in the code, which always writes `[redacted]` when output. |
| SEC-LOG-2 | Headers are logged via an allow list, not via a deny list. `Authorization`, `DPoP`, `Cookie`, `Set-Cookie`, invocation tokens, `X-Forwarded-*` and identity headers of a proxy never appear. Reason: with a deny list, every new header slips through. |
| SEC-LOG-3 | Paths are logged as route patterns (`/api/v1/hooks/{webhook_id}/{token}`), never raw; query strings never. |
| SEC-LOG-4 | Never in the log: message content, file names, channel prompts, transcripts, search terms, display names, email addresses, push tokens. People appear only as an HMAC pseudonym (`bothy.principal.pseudo_id`, R-OB-03, [privacy.md](../observability/privacy.md) section 3), agents and apps with their handle; device IDs never. This applies to logs and spans, not to the audit log ([20-privacy.md](20-privacy.md)). |
| SEC-LOG-5 | IP addresses only in the audit log and only pseudonymized ([20-privacy.md](20-privacy.md), section 4.5), not in general logs. |
| SEC-LOG-6 | Telemetry attributes via an allow list per signal ([17-observability.md](17-observability.md)); no content (Baseline B11). |
| SEC-LOG-7 | Crash reports from apps are sanitized before sending; content, tokens and names are not included. |

## 10. Secrets

All secrets come from the secret provider interface (ADR 0011). The database holds
only hashes, values encrypted with an external key or references (`secret_ref`, 03).

| Secret | Location | Rotation |
|---|---|---|
| Private signing keys (access token, invocation token, outgoing webhooks, audit checkpoints) | generated by the server in the key store `secrets.signing_key_dir` ([ADR 0066](../adr/0066-signing-keys-in-the-writable-key-store.md)): Ed25519, `kid` = fingerprint according to RFC 7638, file `<purpose>-<kid>.pem`; public part in `server_signing_keys` | States `next`, `active`, `retiring`, `retired`. Scheduled after `signing_key.invocation_rotation_interval` (invocation token, 09) or `signing_key.access_token_rotation_interval` (access token), job `instance.key_rotation` every `signing_key.rotation_check_interval`, system action `signing_key.transition`; `webhook_out` and `audit_checkpoint` only on request (`signing_key.rotate` with `mode = scheduled`). A key stays `retiring` until the longest validity signed with it plus `auth.clock_leeway` has expired; then `retired`, and the private file is deleted. Emergency: `signing_key.rotate` with `mode = emergency` sets all keys of the purpose to `retired` immediately, because a leak of the store also exposes `next` and `retiring`. Processes pick up changes at the latest after `signing_key.cache_ttl` |
| Pepper for token hashes | secret provider | only after a compromise; then all tokens are issued anew, because hashes of unknown tokens cannot be converted |
| Key for TOTP secrets | secret provider, `totp_key_ref` per row | new key for new entries, old ones stay readable until re-keying |
| Database credentials | secret provider or environment | operations |
| OIDC client credentials | `secret_ref` in `identity_providers.config` | `private_key_jwt` preferred |
| Push credentials (APNs key, FCM service account, WNS client) | `secret_ref`; accounts of the owner | according to the requirements of the providers |
| Object storage credentials | secret provider | operations |
| Key for pseudonyms in the audit log and in telemetry | secret provider | according to [20-privacy.md](20-privacy.md) |
| Key of the backups | outside the instance, with the owner | operations |
| LLM API keys, subscription credentials | never in the server; with the adapter or in the runtime of the person (Baseline I5) | instance operator |

Rules:

| ID | Control |
|---|---|
| SEC-SEC-1 | The file driver of the secret provider reads only files with mode `0400` or `0600` that belong to the service user; otherwise the start is aborted. |
| SEC-SEC-2 | `config print` and diagnostic endpoints never show secrets, not even truncated. |
| SEC-SEC-3 | Secrets never appear in events, audit details, telemetry, error messages or crash reports. |
| SEC-SEC-4 | In the repository: gitleaks locally and in the gate, a dedicated rule for the token prefix from 08, section 12 ([23-quality.md](23-quality.md)). |
| SEC-SEC-5 | Files of the secret provider are outside the database backup or only encrypted in it (section 12). |

## 11. Controls for agents

| ID | Control | Location |
|---|---|---|
| SEC-AI-1 | Every piece of content that the server delivers to an agent carries its origin: author, `origin` (`human`, `agent`, `app`, `import`, `system`), kind (message, file, transcript, quote) and channel. The channel prompt is a separate field. Adapters must mark foreign content as data, not as instructions; the content of files is always data. | [09-agents.md](09-agents.md), [ADR 0054](../adr/0054-origin-and-trust-marking-in-the-agent-context.md) |
| SEC-AI-2 | Tool class with configured and reported value; `host` only for owners or one's own dev environment. | 07, SR2 |
| SEC-AI-3 | Acting-for binding: an agent in a run has at most the permissions of the triggering person. | 07, SR9 |
| SEC-AI-4 | Components with `required_capability = 'approve_actions'` have `restricted_to = 'run_starter'` by default. | [09-agents.md](09-agents.md) |
| SEC-AI-5 | Agents never trigger; agent messages have `origin = 'agent'`. | 07, SR4 |
| SEC-AI-6 | No automatically loaded foreign resources in clients. | SEC-OUT-2 |
| SEC-AI-7 | Rate limits per agent (`rl.agent_runtime`) and triggers per person (`rl.trigger`); cost limits per agent. | 05; [10-backend-policy.md](10-backend-policy.md) |
| SEC-AI-8 | Channel prompts are no place for secrets. The console says so during editing. Reason: an agent can repeat its prompt in its answer. | R-AZ-09 |

## 11a. Controls for the terminal

Normative together with [24-terminal.md](24-terminal.md); threats T-TM-* in
[../security/threat-model.md](../security/threat-model.md), section 6.7.

| ID | Control | Location |
|---|---|---|
| SEC-TERM-1 | Terminal content only end-to-end encrypted (Noise `NK`); the relay forwards binary frames unchanged and records only length, time and session ID. No content in log, audit, telemetry, event log, search, push or run context. | 24, sections 5 and 10 |
| SEC-TERM-2 | Terminal session token valid exactly once, bound to `sid` in the prologue and to the ephemeral key of the device (`cek`); PTY only after the first valid transport message. | 24, section 5.3 |
| SEC-TERM-3 | Step-up `fresh` per opening (passkey or step-up device key; in the console only a passkey with `userVerification = required`), consumed in the transaction of the opening. | 08, section 6.3 |
| SEC-TERM-4 | Host service and sessions as separate users without administrator privileges; no session user can read the keys (HD8); none of these users in a group with root-level privileges (HD2, for example `docker`); start only after a passed `doctor`; keys `0600` or DPAPI; no credentials as environment variables of the service. A second live control connection with the same keys disables the host. | 24, sections 2.2, 3.4 and 4.2 |
| SEC-TERM-5 | Permissions: target `host` only with `terminal`, only people and groups; SR10, SR11 (I6) and SR12; revocation ends sessions immediately. | 07, sections 2.12 and 5 |
| SEC-TERM-6 | The renderer blocks OSC 52 (clipboard), opens hyperlinks (OSC 8) only after a click with a visible target and answers no queries of the window title or the clipboard. | 24, section 11.2 |
| SEC-TERM-7 | Web console: the terminal runs under the same CSP (section 5) without exception; it renders into the canvas and creates no HTML from output; `connect-src 'self'` covers the stream. Trusted Types stays mandatory; the terminal widget needs no policy of its own [derived from the design of Flutter Web], SEC-T05 checks it with an open terminal. | [ADR 0070](../adr/0070-terminal-in-the-web-console.md) |
| SEC-TERM-8 | Key pinning per device with a hard warning on change; every re-pairing in the audit and as an event to all owners. In the web console the pinning does not protect against a malicious server, because the server delivers the code of the console; there only `host_service.require_local_confirm` helps against it. | 24, sections 3.3, 5.1 and 5.5 |
| SEC-TERM-9 | Profiles only from the configuration of the host service; the client chooses name and index, never command lines or paths. | 24, section 11.1 |
| SEC-TERM-10 | Server host only for owners, checked twice (server and host service). | 07, SR10 |
| SEC-TERM-11 | Every session in its own process group, on Linux in its own cgroup, on Windows in a Job Object; its end ends the whole process tree. Subscription login (HD6) checked before every PTY and periodically, as an additional check to SEC-TERM-12. | 24, sections 6.5 and 7.4 |
| SEC-TERM-12 | One session user per person and host, never shared, not even after the end of the assignment; without an assignment no terminal (SR12); the host service binds every session user to the first person (`osu` in the token). Load-bearing enforcement of "no foreign subscription login" (ruling T12). | 24, section 2.2; 07, SR12 |
| SEC-TERM-13 | No shared locations between session users: no location with code, settings or logins is writable for one session user and readable or executable for another; working directory and `CLAUDE_CONFIG_DIR` of the profile `claude` per session user; no foreign chain files (`CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.md`, `.mcp.json`, `.claude`, `.git`) in the directories above; programs including their directory chain owned by a trusted owner (Linux and macOS `root`, Windows `SYSTEM`, `Administrators` or `NT SERVICE\TrustedInstaller`), the profile `claude` with `organization` only with `host_service.claude_bin` and with `personal` with a `CLAUDE_CONFIG_DIR` in the home directory; HD11 checks every configured path after full resolution including the directory chain and chain files, "writable" includes creating entries, on Windows also at the drive root (ruling T13). | 24, section 2.2 item 6 and 7.4 |

## 11b. Controls for visualizations

Normative together with [25-visualization.md](25-visualization.md); threats T-VZ-* in
[../security/threat-model.md](../security/threat-model.md), section 6.8.

| ID | Control | Location |
|---|---|---|
| SEC-VZ-1 | No iframes and no WebViews with Grafana or other web content for statistics in iOS, Android, macOS and Windows. | 25, section 9.1 |
| SEC-VZ-2 | Embedding in the console only with `console.grafana_embed_url` set, whose origin differs from the origin of the console (otherwise `bothy config check` rejects it, because `allow-scripts allow-same-origin` removes the sandbox when the origin is the same); then `frame-src` of exactly this origin, iframe with `sandbox` and `referrerpolicy="no-referrer"`; Grafana only behind an auth proxy with a sender list that sets `frame-ancestors` to the origin of the console. | Section 5, rule 7; 25, section 9.2 |
| SEC-VZ-3 | Live frames, timeline and `stats_minutes` without content and without people; tools only with names. | 25, sections 1 and 5.2 |
| SEC-VZ-4 | Every live frame through the delivery mode of `authorize` with `stats.read`, filtered to visible channels; costs only with `cost.read.aggregate`. | 25, section 4 |
| SEC-VZ-5 | Grafana plugin unsigned only with its own ID in `allow_loading_unsigned_plugins`, never with a wildcard or other IDs. | 25, section 8.3 |

## 12. Supply chain, backups and audit integrity

Supply chain (details and tools in [23-quality.md](23-quality.md) and
[18-release.md](18-release.md); Baseline R-Qualität):

| ID | Control |
|---|---|
| SEC-SC-1 | Dependencies with lockfiles and checksums (Go `go.sum`, Dart `pubspec.lock`, Python lock with hashes, npm lock); updates via Renovate with a waiting period (ADR 0041); OSV-Scanner in the gate. |
| SEC-SC-2 | GitHub Actions pinned by full commit SHA, `permissions: contents: read` as the default, no `pull_request_target`, zizmor and actionlint mandatory (Baseline R-Qualität). |
| SEC-SC-3 | Only GitHub-hosted runners with a fixed label, no self-hosted runners: in a public repository such a runner would run code from foreign pull requests, and GitHub advises against it (https://docs.github.com/en/actions/reference/security/secure-use, retrieved 2026-10-10 [verified]). ADR 0039. |
| SEC-SC-4 | Container base images pinned by digest; releases signed, with SBOM and provenance (ADR 0036). |
| SEC-SC-5 | Tools with a known compromise only in a version proven to be safe (example Trivy, Baseline R-Qualität). |

Backups (requirements for [16-operations.md](16-operations.md)):

| ID | Control |
|---|---|
| SEC-BAK-1 | Backups are always encrypted; the key is not on the same host as the backup. |
| SEC-BAK-2 | Backups contain no files of the secret provider in plaintext (SEC-SEC-5). |
| SEC-BAK-3 | Every restore applies the deletion journal ([20-privacy.md](20-privacy.md), section 4.6) and increases the permission version so that all caches rebuild. |
| SEC-BAK-4 | After a restore, clients detect the jump back (`resync_required` with `cursor_ahead`, 04, section 7.3). |
| SEC-BAK-5 | The restore is rehearsed regularly; the rehearsal is a test in the operations runbook, not just a text. |

Audit integrity (ADR 0021):

| ID | Control |
|---|---|
| SEC-AUD-1 | The application role has only `INSERT` and `SELECT` on `audit_events`; `UPDATE` and `DELETE` are blocked (03). |
| SEC-AUD-2 | Every `audit.checkpoint_interval`, the job `audit.chain_verify` recomputes the segment since the last checkpoint and only then signs the next checkpoint (sequence, hash, time, previous checkpoint), which is stored at `audit.checkpoint_sink` outside the database; a broken chain is never signed ([02-architecture.md](02-architecture.md), section 12.3). If no target is set, every result of `audit.verify` carries the warning `checkpoint_sink_not_configured` and every run of the job writes it to the log; the console shows it permanently based on this result. |
| SEC-AUD-3 | `audit.verify` compares the recomputed chain with the checkpoints from the target, not from the database. |
| SEC-AUD-4 | Canonical form of a row (input of `hash`, 03 `audit_events`): a JSON object with the column names except `hash`; `seq` as an integer; IDs and `correlation_id` as UUIDs in lowercase with hyphens; `occurred_at` as RFC 3339 in UTC with exactly three decimal places and `Z` (the recorder truncates the time to milliseconds before storing); `prev_hash` as hex in lowercase; `details` as stored; NULL columns are omitted. Serialized according to RFC 8785 (JCS). This form is exactly the API representation `AuditEvent` without `hash`; an export (`audit.read`) returns it unchanged so that it can be verified without the server. |
| SEC-AUD-5 | Schema `audit_details.v1`: a flat object. Reserved fields: `ip_hmac` (64 hex characters), `ip_key_id` (UUID from `audit_ip_keys`), `aggregate` (`count` from 1, `first_at`, `last_at`), `violations` (for `i1_violation`: list of `{code, agent_id, credential_holder_id, principal_ids}`), `reason` (the reason that an administrative action with a mandatory reason requires, as the request states it; without control and bidi characters according to SEC-IN-4, at most `audit.reason_max_chars` characters, a longer reason results in `400 validation_failed` with code `too_long`). All other fields: keys `^[a-z][a-z0-9_]*$`, values only UUIDs, codes `^[a-z0-9][a-z0-9_.:-]*$`, integers, booleans, times in the form of SEC-AUD-4 or lists of these. No free text, no `null`, no floating-point numbers, never content, names, email addresses, tokens or enrollment codes. A value outside these forms is a programming error that the recorder rejects. If an action needs further free text, its ticket extends the schema here and in the code. |
| SEC-AUD-6 | Each process aggregates identical denials per `audit.deny_aggregate_window` (05, section 2.4); at most `audit.deny_aggregate_max_keys` open aggregations, beyond that the oldest one is written immediately. Residual risk: a hard crash loses the open aggregations of a window ([threat-model.md](../security/threat-model.md)). |
| SEC-AUD-7 | Format of the checkpoints: see the table below. `audit.verify` reads checkpoints only from the storage target, never from `audit_checkpoints`, and checks the signature, `instance_id` and the chaining of each checkpoint. |

Format of the checkpoints (03, `audit_checkpoints`):

| Part | Ruling |
|---|---|
| Payload | `{instance_id, seq, hash, occurred_at, previous_checkpoint_hash}` according to RFC 8785 (JCS); `seq`, `hash` and `occurred_at` belong to the captured row of `audit_events`, time in the form of SEC-AUD-4; `hash` and `previous_checkpoint_hash` as hex in lowercase; `previous_checkpoint_hash` is the SHA-256 of the canonical payload of the previous checkpoint and is absent for the first one |
| Signature | compact JWS with header `{alg, kid, typ: "audit-checkpoint+json"}`, key with purpose `audit_checkpoint` ([ADR 0066](../adr/0066-signing-keys-in-the-writable-key-store.md)); `audit_checkpoints.signature` stores the whole JWS as UTF-8; verification accepts only the algorithms from 03 `server_signing_keys`, never `none` |
| Storage with `file://` | file `<path>/<instance_id>/<seq with 20 digits, zero-padded>.jws` with the JWS; directory `0700`, file `0600`; written once (M1-50): a temporary file in the same directory created with `O_EXCL` and synced, then a hard link to the name, which fails when the name exists (a rename would replace a file a concurrent writer just stored), then `fsync` of the directory; an existing file with different content is an error and is never overwritten; the directory `<path>` itself must exist, the server never creates it |
| Storage with `s3://` | object `[<prefix>/]<instance_id>/<seq with 20 digits, zero-padded>.jws` with the JWS in the bucket of `audit.checkpoint_sink` (M7-22); written once with `PutObject` and `If-None-Match: *` after a read of the key: the same bytes count as stored, other bytes are an error and the stored object stays; the bucket has Object Lock with a default retention in mode `COMPLIANCE` of at least `retention.audit` (checked by `bothy config check` and the start), so an overwrite a storage lets through only adds a version; the server reads the oldest version of every key ([backup-restore.md](../operations/backup-restore.md) 8) |

## 13. Penetration test plan

### 13.1 Levels

| Level | When | Who | Scope |
|---|---|---|---|
| P0 automatic | every local gate and CI | tools | SAST (gosec, Semgrep for Go, Python, TypeScript; `dart analyze` and custom_lint for Dart), dependencies (OSV-Scanner), secrets (gitleaks), workflow analysis (zizmor, actionlint), schema-based fuzzing of the API against the OpenAPI document, Go fuzzing of the parsers (DPoP, JWS, Markdown, patches, uploads), permission test matrix and mandatory tests from 07 and 08, CSP test of the console |
| P1 internal red team | end of every milestone, before the merge into `main` | fresh, read-only reviewer plus attack tests on a test instance | threat register (all entries with likelihood `medium` or `high`), diff against the system boundary diagram, checklist according to OWASP ASVS 5.0 level 2 for server and console (https://owasp.org/www-project-application-security-verification-standard/, retrieved 2026-10-10 [verified]: 5.0.0 is the current version), OWASP Top 10 for LLM Applications 2025 for agent paths (https://genai.owasp.org/llm-top-10/, retrieved 2026-10-10 [verified]) |
| P2 external test | before the first productive use by a second person and before publication | external service provider (costs: decision of the owner) | permission bypass (IDOR, 404/403), tokens and DPoP, WebSocket, console (XSS, CSRF, clickjacking), uploads, SSRF, invocation tokens, prompt injection across space boundaries, apps and webhooks, mobile apps (storage, keys, transport), terminal (relay, host service, console) |
| P3 dev isolation | before enabling each environment type | red team | breakout and exfiltration tests according to [21-dev-isolation.md](21-dev-isolation.md), section 8 |
| P4 terminal | at the end of M2, M4, M5 and M6 | red team | attacks from the threat model, section 6.7 (T-TM-*): replay and swapping at the relay, tokens, step-up, SR10 and I6, privilege escalation on the host, escape sequences, console with extensions; report before the merge |

### 13.2 Rules

- Tests run only against a dedicated test instance with synthetic data (people Alex, Sam,
  Robin; domain `example.com`), never against the production instance and never with real accounts of third parties.
- Before P2: threat model up to date, scope and rules in writing, an emergency contact named.
- Findings are classified by severity, tracked as tickets and tested again after the fix.
  An open finding with severity "high" or "critical" blocks the publication.
- Every finding updates the threat register (maintenance rule in the threat model).

## 14. Tests

| ID | Test | Controls |
|---|---|---|
| SEC-T01 | A plaintext connection from an untrusted peer is rejected; with the proxy header driver and a non-loopback listener the driver stays off, `/readyz` reports `trusted_header_requires_loopback`, and enabling the driver yields `409` | SEC-TLS-1, 08 ID-T08 |
| SEC-T02 | Lint: no `InsecureSkipVerify`, no `http.DefaultClient`, no raw SQL | SEC-TLS-4, SEC-NET-1, SEC-IN-5 |
| SEC-T03 | Headers of every response kind against an expected list (API, console, file, error) | SEC-HDR-1 to 4 |
| SEC-T04 | CORS: no `Access-Control-Allow-*` except at the public endpoints; WebSocket with a foreign `Origin` rejected | SEC-CORS-1 to 4 |
| SEC-T05 | Console in a headless browser with an enforced CSP: zero violations, no fetch from a foreign origin, WebSocket works | Section 5 |
| SEC-T06 | Schema fuzzing of all routes: no `5xx` except expected ones, no acceptance of unknown fields, duplicate JSON keys rejected | SEC-IN-1, SEC-IN-2 |
| SEC-T07 | Bidi and control characters in display names rejected | SEC-IN-4 |
| SEC-T08 | Uploads: wrong type declaration, SVG with a script, decompression bomb, size over the limit, scanner not ready, infected test file (EICAR) | SEC-UP-1 to 7 |
| SEC-T09 | Egress: one address from each blocked range in SEC-NET-3 (IPv4 and IPv6, including the metadata address), DNS name with a private answer, redirect to a private address: all rejected | SEC-NET-1 to 5 |
| SEC-T10 | Canary test: send defined canary values (token, message text, display name, email, IP; with their milestones file name, prompt, search term, push token, form value) through all paths; none appears in logs, traces, metrics, audit details, error responses or foreign events; principal IDs of people appear in logs and traces only as a pseudonym (SEC-LOG-4). The ticket that builds a new path for such a field extends the scenario (PLAN) | SEC-LOG-1 to 7, SEC-SEC-3 |
| SEC-T11 | Secret provider file with mode `0644` | Startup abort, SEC-SEC-1 |
| SEC-T12 | `config print` with secrets set | no secrets in the output |
| SEC-T13 | Client rendering: Markdown with HTML, `javascript:` link, image URL, link preview | nothing executed, nothing loaded |
| SEC-T14 | Approval card with agent text that imitates a system header | header from server fields, text clearly separated |
| SEC-T15 | Restore of a backup with a message deleted afterwards | message stays deleted (deletion journal) |
| SEC-T16 | Audit: change of a row through the maintenance role, rewriting the whole chain, `UPDATE` with the application role | detected or refused, respectively (SEC-AUD-1 to 3) |
| SEC-T17 | TLS handshake with a version below 1.2 against a listener with its own TLS | rejected (SEC-TLS-2) |
| SEC-T18 | Terminal: tests TM-T01 to TM-T29 from [24-terminal.md](24-terminal.md), section 14; SEC-T05 and SEC-T10 with an open terminal | SEC-TERM-1 to 13 |
| SEC-T19 | Console without and with `console.grafana_embed_url`: CSP without or with exactly one `frame-src` origin, respectively; with the origin of the console as the value, `bothy config check` fails; SEC-T05 with an embedded dashboard behind a test proxy; static check of the app for WebView packages; canaries from VZ-T07 | SEC-VZ-1 to 5 |

## 15. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-SE-01 | Plaintext only from loopback and trusted proxies. | Compose and a reverse proxy work; direct plaintext from outside does not. |
| R-SE-02 | Console without foreign origins, with nonce, Trusted Types and COOP/COEP. | The console is the most valuable target in the browser; everything foreign is attack surface. |
| R-SE-03 | Allow lists for log headers and telemetry attributes instead of block lists. | Otherwise new fields are visible by default; that is the wrong error class. |
| R-SE-04 | One egress client with an IP check at connection setup. | SSRF protection in one place, effective against DNS rebinding as well. |
| R-SE-05 | Clients load no foreign resources from contents. | Closes data exfiltration through image URLs and tracking of the readers. |
| R-SE-06 | Image metadata is always removed, without a switch. | Location data in photos is a common, unnoticed leak; a switch to turn it off would be a trap with no benefit for a team chat. |
| R-SE-07 | External penetration test before the second productive person and before publication. | Study C: red team before the first real second person; a public product needs an independent review. |
| R-SE-08 | TLS in the server only with both references (`server.tls_cert_ref`, `server.tls_key_ref`), without its own cipher list and without a plaintext redirect. | An edge remains the normal case; without an edge the server should stand securely on its own. A redirect would accept plaintext from anyone. |
| R-SE-09 | The internal listener has no filter by peer. | It serves only health and stack-internal webhooks, evaluates no identity or forwarding headers and is never published on the host. A filter would force the instance operator to enter the whole Compose network in `server.trusted_proxies`; then forwarding headers of every container would also count at the public listener. |
| R-SE-10 | Stricter block list for outgoing connections; `allow_private` never opens link-local, multicast or unspecified addresses. | Known SSRF bypasses (mapped IPv6, NAT64, 6to4, Teredo) and metadata services of the clouds; an entry of the instance operator is meant to open an internal target, not the metadata. |
| R-SE-11 | Audit check offline: the canonical form equals the API representation, fixed schema of the details without free text. | An export can be checked without a server; free text would be a place for contents and secrets. |
| R-SE-12 | Terminal stream encrypted end to end; the protection goal leaves out a malicious instance operator of the server, with pinning and local confirmation as a fallback. | The server is trusted (section 1); the encryption protects against relays, logs, database dumps and processes that read along ([ADR 0068](../adr/0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md)). |
| R-SE-13 | Grafana embedding only as an option in the console, never in apps; `frame-src` only with a set URL. | With `allow_embedding` the clickjacking protection of Grafana falls away; the proxy replaces it with `frame-ancestors`. Apps need no web content ([ADR 0069](../adr/0069-visualization-on-two-levels-without-iframes-in-apps.md)). |

## 16. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `http.hsts_max_age` | `31536000s` | `max-age` for HSTS |
| `egress.allowed_hosts` | empty | additional allowed targets, form `host[:port][;allow_private]` ([configuration.md](../operations/configuration.md), section 2); duplicate entries are a load error |
| `egress.timeout` | `15s` | time limit of an outgoing request |
| `egress.max_response_bytes` | `1048576` | largest response of an outgoing request |
| `audit.checkpoint_interval` | `1h` | interval of the signed checkpoints of the audit log |
| `audit.checkpoint_sink` | empty | storage target of the checkpoints outside the database: empty (warning, SEC-AUD-2), `file://<absolute path>` on storage outside the database volume, or `s3://<bucket>[/<prefix>]` (M7-22), an S3-compatible object store with a retention lock, the recommended target for production: a bucket in the S3 bucket naming rules, an optional prefix of segments of `[a-z0-9-]` joined by single `/`. `bothy config check` refuses every other form; for `s3://` it and the server start read the Object Lock configuration of the bucket and fail unless Object Lock is enabled with a default retention in mode `COMPLIANCE` of at least `retention.audit` (`GOVERNANCE` fails) |
| `audit.checkpoint_s3_endpoint` | empty | endpoint of the object store of an `s3://` sink, an absolute `http` or `https` URL without path; its own endpoint, separate from `file.s3.endpoint` and the backup; `http` only with an `egress.allowed_hosts` entry with `allow_private` |
| `audit.checkpoint_s3_region` | `us-east-1` | region of the S3 signature of an `s3://` sink |
| `audit.checkpoint_s3_use_path_style` | `true` | path-style requests of an `s3://` sink instead of `<bucket>.<endpoint host>` |
| `audit.checkpoint_s3_access_key_ref` | `file:<secrets.dir>/audit_checkpoint_s3_access_key` | secret reference of the own access key ID of an `s3://` sink, read only with an `s3://` sink; supplied by the owner from a separate account ([backup-restore.md](../operations/backup-restore.md) 8) |
| `audit.checkpoint_s3_secret_key_ref` | `file:<secrets.dir>/audit_checkpoint_s3_secret_key` | secret reference of the own secret access key of an `s3://` sink, read only with an `s3://` sink |
| `audit.reason_max_chars` | `500` | maximum length of a mandatory reason of an administrative action in Unicode characters (SEC-AUD-5) |
| `signing_key.cache_ttl` | `30s` | longest time a process still accepts a retired key or does not know a new one |
| `signing_key.rotation_check_interval` | `1m` | interval of the job `instance.key_rotation` |
| `signing_key.access_token_rotation_interval` | `30d` | scheduled rotation of the key for access tokens |

Values of other chapters that this chapter refers to: `server.trusted_proxies`
([16-operations.md](16-operations.md)), `http.max_json_body_bytes`, `http.max_header_bytes`,
`http.read_header_timeout`, `http.idle_timeout` ([02-architecture.md](02-architecture.md)),
`file.image.max_pixels` ([12-files.md](12-files.md)), `console.grafana_embed_url`
([25-visualization.md](25-visualization.md)). Every name is in exactly one
chapter table (Ruling L4).
