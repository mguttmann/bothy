# Threat model

Status: draft M0, as of milestone M0 (planning), terminal addendum of 2026-10-10. Living document; maintenance rule in section 9.
Controls with IDs `SEC-...`: [../spec/19-security.md](../spec/19-security.md). Permissions and
guard rules `SR1` to `SR12`: [../spec/07-permissions.md](../spec/07-permissions.md). Identity:
[../spec/08-identity.md](../spec/08-identity.md). Privacy:
[../spec/20-privacy.md](../spec/20-privacy.md). Dev isolation:
[../spec/21-dev-isolation.md](../spec/21-dev-isolation.md). Terminal:
[../spec/24-terminal.md](../spec/24-terminal.md).

Method: system boundaries and data flows, assets, attackers, STRIDE per component, then a register
of the most important threats with rating, mitigation, test and residual risk. AI-specific
threats follow the classification of the OWASP Top 10 for LLM Applications 2025
(https://genai.owasp.org/llm-top-10/, retrieved 2026-10-10 [verified]).

## 1. Scope

In the model: server (all roles), data storage, native apps, admin console, agent protocol and
reference adapters, apps and webhooks, push, telemetry, backups, supply chain, multi-user dev
environment, terminal (pairing, relay, host service, terminal view of the clients).

Not in the model, with reason:

- Compromised server host or hypervisor: assumption in 19, section 1. The remedy is
  host hardening in operations.
- Physical attacks on server hardware, side channels in CPUs.
- Internal misuse at LLM, push or telemetry providers beyond the contracts: governed legally
  through data processing agreements ([../compliance/processors-template.md](../compliance/processors-template.md)).
- The inner behavior of agent runtimes (for example what Hermes Agent keeps in its memory):
  the product only limits what the runtime reaches and triggers.

## 2. System boundaries

![System boundaries and trust zones](diagrams/trust-boundaries.svg)

Source: [diagrams/trust-boundaries.mmd](diagrams/trust-boundaries.mmd). Re-render with mermaid-cli
11 (`npx -y @mermaid-js/mermaid-cli@11 -i trust-boundaries.mmd -o trust-boundaries.svg`).

| Boundary | Between | What it protects | Controls |
|---|---|---|---|
| TB1 | Device and network edge | Tokens, content in transit, identity | TLS, DPoP, console cookie, SEC-CORS-* |
| TB2 | Network edge and server process | Authenticity of proxy headers, plaintext only internally | SEC-TLS-1, `server.trusted_proxies`, 08 section 2.4 |
| TB3 | Server and data storage | Content, hashes, audit integrity | Roles in PostgreSQL, Unix socket, `verify-full`, ADR 0021 |
| TB4 | Server and agent adapter | who triggers which agent, what the agent sees | API token with DPoP, invocation token (I3), `authorize` at delivery, SR9 |
| TB5 | Runtime and LLM provider or tools | Content to third parties, effect of tools | Tool class (SR2), backend policy (SR1), data processing agreement |
| TB6 | Server and apps or external systems | Data to apps, writes from outside | Scopes plus grants, signed webhooks to allowed targets, incoming webhooks without triggering |
| TB7 | Server and external services | Identity, push metadata, telemetry, object bytes to an S3 service | Egress client `platform/egress` for every outgoing HTTP connection except the OTLP export to the collector (target allow list, IP check at connection setup, TLS always verified, lint against other clients; SEC-NET-1 to 5, SEC-TLS-4), OIDC check, push only with IDs, telemetry without content. S3 driver (M3-02): object bytes go to the S3 service only through the egress client (purpose `objectstore`, host policy of exactly the endpoint, `http` or private addresses only with an `allow_private` entry, no redirects), with access keys only from the secret provider (`file.s3.access_key_ref`, `file.s3.secret_key_ref`), never from the environment or the instance metadata service |
| TB8 | Operations and supply chain and instance | Root of trust, code integrity | Server command only on the host, signed releases, pinned actions |
| TB9 | Dev environment of a person and host of the runner | Host, secrets of the owner, other people | VM boundary, egress rules, readiness check (21) |
| TB10 | Device, server relay and host service of a connected host | Terminal content, shell on the host, LLM login of the host | Noise `NK` end-to-end, one-time token, fresh step-up, SR10, I6, host service without admin (24) |

## 3. Assets

| ID | Asset | Protection goal |
|---|---|---|
| A1 | Messages, threads, runs, agent outputs | Confidentiality, integrity |
| A2 | Files, transcripts | Confidentiality, integrity |
| A3 | Identities, sessions, device keys, tokens of people | Confidentiality, authenticity |
| A4 | Private signing keys, pepper | Confidentiality |
| A5 | Agent and app tokens, URLs of the incoming webhooks | Confidentiality |
| A6 | Permission configuration (grants, groups, owners, operators, backend policy, tool class) | Integrity |
| A7 | Audit log | Integrity, completeness |
| A8 | LLM access (API keys, personal subscriptions), outside the server | Confidentiality, contract compliance |
| A9 | Hosts of the agent runtimes with their secrets | Integrity, confidentiality |
| A10 | Backups | Confidentiality |
| A11 | Push credentials of the owner | Confidentiality |
| A12 | Metadata (who, when, with whom, devices, IPs) | Confidentiality, data minimization |
| A13 | Availability of chat and agents | Availability |
| A14 | Code, dependencies, releases | Integrity |
| A15 | Shell on connected hosts and their terminal content | Confidentiality, integrity |
| A16 | Keys of the host services (Noise, login) and pinned keys in the devices | Confidentiality, authenticity |

## 4. Attackers

| ID | Attacker | Abilities | Goal |
|---|---|---|---|
| E1 | Outsider without an account | reaches the network edge | takeover, data, disruption |
| E2 | Member with few permissions | valid account, own devices | read foreign spaces, misuse agents |
| E3 | Content attacker | brings text, files or links into a channel (directly, through an import, a webhook or a person who forwards something) | prompt injection against agents |
| E4 | Malicious or compromised agent runtime | valid agent token, sees what the agent sees | exfiltrate data, write into foreign channels |
| E5 | Malicious app | valid installation with scopes | exfiltrate data, deceive people |
| E6 | Thief or finder of a device | device, perhaps unlocked | content, session |
| E7 | Owner with operating error or abuse | all owner permissions | overly broad grants; insight into other people's content |
| E8 | Supply chain attacker | manipulated dependency, action, image | code in the server, in CI or in apps |
| E9 | Network attacker | reads or modifies traffic | tokens, content |
| E10 | Service provider with insight | sees push, telemetry or LLM data | profiling |
| E11 | Thief of a backup | has a backup file | content, hashes |
| E12 | Malicious instance operator of the server | controls the server, key distribution and relay | read along terminal sessions or open them with nobody at the controls |
| E13 | Attacker on a connected host | account on the host, without the permissions of the host service | gain the permissions of the host service, steal keys |

## 5. STRIDE per component

S = Spoofing, T = Tampering, R = Repudiation, I = Information Disclosure, D = Denial of Service,
E = Elevation of Privilege. "Register" refers to section 6.

### 5.1 Native app

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Stolen token used on another device | DPoP with a non-exportable key (08, 7.3) | T-TK-01 |
| T | Manipulated app bypasses client checks | no permission logic in the client; the server checks everything (07) | T-AZ-01 |
| R | Person denies a grant | step-up with biometrics, audit with device and session | T-AZ-04 |
| I | Local cache on a lost device | encrypted DB, app lock, wipe on `4012` (08, 9 and 10) | T-DV-01 |
| D | Mass reconnects after a restart | full jitter backoff (04, 6.5), `rl.gateway_connect` | T-DS-01 |
| E | Pending device gains access | tokens only for self-information (`403 device_pending` for everything else, no gateway), typed-in confirmation code (08, 5.2) | T-ID-05 |

### 5.2 Admin console in the browser

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Phishing of the owner login | passkeys are bound to the origin; step-up via passkey | T-AC-02 |
| T | CSRF on admin actions | `SameSite=Strict`, mandatory header, `Origin` check (SEC-CORS-4) | T-AC-01 |
| R | Owner denies a change | audit with step-up time and session | T-AZ-04 |
| I | XSS reads data from the console | CSP without foreign origins, Trusted Types, Flutter renders no HTML from data (19, 5) | T-AC-01 |
| D | Rate-limit lockout of the owner | lockout only of the password path (08, 13) | T-ID-03 |
| E | Clickjacking on confirmation buttons | `frame-ancestors 'none'`, `X-Frame-Options: DENY` | T-AC-01 |

### 5.3 Network edge and proxy

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Forged identity header | loopback only, trusted proxies only, binding per request (08, 2.4) | T-ID-02 |
| T | Forged `X-Forwarded-For` | IP only from trusted proxies (SEC-TLS-1, 08, 13) | T-ID-03 |
| I | Eavesdropping on the network | TLS, HSTS | T-NW-02 |
| D | Connection flood | rate limits, timeouts (SEC-IN-3) | T-DS-01 |
| E | Direct access bypassing the proxy | plaintext only on loopback and from trusted proxies | T-ID-02 |

### 5.4 API (REST)

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Requests with someone else's token | signature, `aud`, DPoP, session check | T-TK-01 |
| T | Duplicate JSON keys, unknown fields | strict decoding (SEC-IN-1, SEC-IN-2) | T-IN-01 |
| R | Person denies an admin action through the API | audit according to the column "Audit" in 05 | T-AZ-04 |
| I | IDOR, existence revealed through 403 or timing | one single decision point, 404 instead of 403, equality test (07, AZ-P1, AZ-P4) | T-AZ-01, T-AZ-02 |
| D | Large bodies, expensive searches | size limits, `rl.search`, page limits | T-DS-01 |
| E | Route without an action, administration without step-up | mandatory registration, step-up (07, SR5) | T-AZ-01 |

### 5.5 Gateway (WebSocket)

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Cross-site WebSocket hijacking with the console cookie | `Origin` check on upgrade (SEC-CORS-3) | T-AC-01 |
| T | Replay of `identify` | DPoP with a server nonce from `hello` (04, R-SY-01) | T-TK-01 |
| I | Delivery after access removal | permissions at delivery time, `access.changed` in log order (07, 8.2) | T-AZ-03 |
| D | Slow consumer | outbound buffer limit, `4014` (04) | T-DS-01 |
| E | Focus on foreign threads | `gateway.focus` checks per thread (05) | T-AZ-01 |

### 5.6 authz and identity

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Account takeover through email matching | matching only through `(issuer, subject)` or enrollment code (08, 3.1) | T-ID-01 |
| T | Stale permission cache | version counter per request, `valid_until` (07, 7.3) | T-AZ-03 |
| R | Probing of foreign resources goes unnoticed | denials in the audit, aggregated per window (R-AZ-04) | T-AZ-04 |
| I | Counters and search show foreign channels | SQL filter plus `authorize` per element (07, 7.4) | T-AZ-02 |
| D | Permission recomputation on many changes | global counter, recomputation only when needed; load test | T-DS-01 |
| E | Delegation beyond one's own level | R-AZ-05 made precise (07, 2.6) | T-AZ-05 |

### 5.7 PostgreSQL with event log and audit

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| T | Modification of the audit log | application role only `INSERT`/`SELECT`, hash chain, exported checkpoints (ADR 0021) | T-DB-01 |
| R | Proof of an action is removed afterwards | hash chain, checkpoints outside the database | T-DB-01 |
| I | Database dump | tokens only as HMAC, secrets only as references (03) | T-BK-01 |
| D | Lock bottleneck at the sequence counter | counter step last (04, 3.2); load test | T-DS-01 |
| E | SQL injection | only sqlc with parameters (SEC-IN-5) | T-IN-01 |

### 5.8 Files, object storage, virus scanner

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | File with a wrong type | detection by the server (SEC-UP-2) | T-UP-01 |
| T | File attached across channel boundaries | `scope_channel_id` (SEC-UP-9) | T-AI-01 |
| I | Location data in photos, permanent links | strip metadata, no object URLs to clients (SEC-UP-10, 11) | T-UP-01 |
| D | Decompression bomb, scanner outage | pixel limit, separate process, fail closed (SEC-UP-4 to 6) | T-UP-01 |
| E | Malicious code in parsers | parsers in a separate process, only pure Go in M1 | T-UP-01 |

### 5.9 Worker and jobs

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| T | Job with system permission reads without a filter | reviewed exception list for queries without a permission filter (07, AZ-P12) | T-AZ-02 |
| R | Expiry of grants without a trace | system actions `grant.expire` in the audit (07, 8.3) | T-AZ-04 |
| I | Webhook to a wrong target | allowlist, check at connection setup (SEC-NET-*) | T-NW-01 |
| D | Queue congestion | separate queues, metrics ([17-observability.md](../spec/17-observability.md)) | T-DS-01 |
| I | A worker consumer reads the whole event log without a permission filter | jobs check `push.deliver` or `webhook_out.deliver` before sending (R-AZ-11); the reads are on the exception list of system reads (07, 7.4); logs and job arguments carry only the consumer name, sequence numbers and IDs (M2-01) | T-AZ-02 |
| D | A worker consumer stands still and events fall out of `sync.event_retention` | gauge `bothy.event.consumer.pending_age`, alarm `BothyEventConsumerLag` at `consumer.pending_alert_after`, held strictly below the retention by the start check (M2-01) | T-DS-01 |

### 5.10 Agent adapters and runtime

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Forged invocation to the adapter | EdDSA-signed invocation token, `aud`, expiry (I3, R-SY-08) | T-TK-05 |
| T | Prompt injection through content | origin labeling, tool classes, approval cards | T-AI-01, T-AI-03 |
| R | Who triggered the agent? | `runs.triggered_by`, audit, token with `sub` | T-AZ-04 |
| I | Agent reads foreign channels for the triggering person | acting-for binding SR9, read horizon hint | T-AI-02 |
| D | Loops, cost explosion | agents never trigger, rate limits, cost limits | T-AI-07 |
| E | Person with few permissions uses an agent with a shell | SR2 tool class, SR3 dev isolation | T-AI-02, T-DEV-01 |

### 5.11 Apps and webhooks

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | Forged outgoing webhook at the receiver | asymmetric signature (ADR 0016) | T-PL-01 |
| T | Incoming webhook as a trigger | `origin = app`, never triggers (SR4) | T-TK-03 |
| R | Unclear who gave an app which scopes | audit of installation and scope changes with step-up | T-PL-01 |
| I | App reads more than needed | scopes plus grants (07, 2.10) | T-PL-01 |
| D | Webhook flood | `rl.webhook_in` | T-DS-01 |
| E | App confirms approval cards | apps never `trigger`, never `approve_actions` (SR4) | T-PL-01 |

### 5.12 Push

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| I | Push provider reads content | push only with IDs and a neutral placeholder (ADR 0031) | T-PU-01 |
| D | Provider outage | the app catches up on opening (04) | T-DS-01 |
| E | Push after access removal | `push.deliver` per push (07, 8.1) | T-AZ-03 |

### 5.13 Telemetry and logs

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| I | Secrets or content in logs and traces | allow lists, canary test (SEC-LOG-*, SEC-T10) | T-LG-01 |
| D | Telemetry blocks the message path | fail-open, queue (Baseline R-Telemetrie) | T-DS-01 |

### 5.14 Operations and backups

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| S | First visitor becomes owner | first owner only via server command (08, 4) | T-ID-04 |
| I | Backup stolen | encryption, keys kept separately (SEC-BAK-*) | T-BK-01 |
| T | Restore brings back deleted data | deletion journal (SEC-BAK-3) | T-BK-02 |

### 5.15 Supply chain

| STRIDE | Threat | Mitigation | Register |
|---|---|---|---|
| T | Manipulated dependency, action or image | lockfiles, SHA pinning, digests, Renovate with a waiting period (SEC-SC-*) | T-SC-01, T-SC-02 |
| E | Code from a foreign PR on a runner of the maintainers; poisoned cache reaches release secrets | only GitHub-hosted runners (SEC-SC-3, ADR 0039); release jobs without cache and without values of other jobs outside of conditions (D-W-52) | T-SC-03, T-SC-05 |
| S | Forged release | signature, provenance, SBOM (ADR 0036) | T-SC-04 |

### 5.16 Terminal

Pairing, relay, host service and terminal view ([24-terminal.md](../spec/24-terminal.md)).

| Part | STRIDE | Threat | Mitigation | Register |
|---|---|---|---|---|
| Pairing | S | A foreign host redeems an intercepted pairing code | code single-use and short-lived, the owner types in the fingerprint, failed attempts limited | T-TM-06 |
| Pairing | T | Server swaps the key of a host | pinning per device, hard warning, audit of every re-pairing, notification to all owners | T-TM-01 |
| Relay | I | Relay, proxy, logs or database dump read terminal content | Noise `NK` end-to-end; only length and time visible | T-TM-01 |
| Relay | T | Relay replays the first packet or puts the token into its own handshake | one-time token, `cek`, prologue, PTY only after `start` | T-TM-02 |
| Relay | D | Flood of openings, slow end | `rl.terminal`, `rl.host`, session limits, buffer limit with `4014` | T-DS-01 |
| Host service | E | Shell escalates to root or administrator on the host | user without admin and without groups with root-level permissions (for example `docker`), `doctor` refuses the start (HD1 to HD3); foreign home folders not readable (HD9; for the home of another session user, HD11) | T-TM-04 |
| Host service | I | Person with a terminal copies the keys of the host service and impersonates the host from elsewhere | sessions as a separate session user without read permission on the keys (HD8); a second live control connection disables the host; re-pairing | T-TM-05 |
| Host service | T | Shell replaces the PTY helper, forges check results and reads along later sessions | helper as a system service with binary, definition and channel endpoint owned by a trusted owner (root; on Windows SYSTEM, Administrators or TrustedInstaller); verification of the peer via the process ID of the service manager; memory protection of the helper (HD10); warning for owners on the host of another person | T-TM-12 |
| Host service | E | Background process or long-running process survives the access removal | process group, cgroup or Job Object per session; a separate session user per person, never reassigned (T12); hint to take it out after the access removal and to delete it together with its long-running processes | T-TM-11 |
| Host service | E | Person places settings or hooks of the coding agent in a folder that the session of another person uses, and they run there as the other person's session user | No shared locations (Ruling T13): working directory and `CLAUDE_CONFIG_DIR` per session user, index of the working directory only in the own list; HD11 for every configured path after resolution, including folder chain and chain files such as `CLAUDE.md` in it, on Windows also the drive root | T-TM-13 |
| Host service | D | One person with a terminal brings the host to a standstill for the others, with a subscription login in their account (HD6) or with a chain file in a shared folder on the path to the locations of others (HD11) | the report names every affected session user; folders on the path writable for no session user, on Windows also the drive root after `bothy-host install`; instance operator step in 16 section 4.4: remove the login or entry, or take the session user out with `bothy-host session-user remove` | T-TM-14 |
| Host service | R | Who worked on which host and when? | audit per opening, attach and end, with device and reason | T-AZ-04 |
| Permissions | E | Person with a grant opens a foreign host or the server host | SR10, server host owners only, checked twice | T-TM-03 |
| Permissions | E | A second person uses the personal subscription on a host | a separate session user per person, never shared (SR12, T12), without shared locations (T13, HD11); I6 transactional and on every opening; `doctor` HD6 as an additional check | T-TM-08 |
| Permissions | E | Agent opens a terminal after prompt injection | terminal only for people in a session with a fresh step-up | T-TM-09 |
| Device | S | Stolen session or stolen device opens a terminal | step-up `fresh` with biometrics or passkey per opening | T-TM-03 |
| Device | T | Escape sequences write to the clipboard or open links | SEC-TERM-6 | T-TM-10 |
| Console | I | Browser extension reads an open session | passkey required, CSP, recommendation of a separate browser profile | T-TM-07 |
| Console | S | A foreign page opens the stream path with the console cookie (cross-site WebSocket hijacking) | origin check on upgrade (08, section 7.5), `SameSite=Strict`; without the server's terminal session token and the tab's Noise key no PTY is created | T-TM-07 |

## 6. Threat register

### 6.1 Rating

- **L** (likelihood without the named mitigations): `low` needs rare
  prerequisites or insiders; `medium` is plausible in a targeted attack or an operating error;
  `high` is to be expected without a mitigation.
- **I** (impact): `low` affects one person or one thread and is fixed quickly;
  `medium` affects a space or a limited data leak; `high` affects the instance, a
  host, all content or has legal consequences.
- **Residual risk** after mitigations: level and what remains.

### 6.2 AI-specific threats

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-AI-01 | **Prompt injection across space boundaries.** Content from space A reaches an agent in space B: one agent serves both spaces, a file is passed on, a thread is moved, an import brings foreign texts, a runtime retains knowledge. (OWASP LLM01) | high | high | agent per audience recommended, read horizon hint H1 (07, 6); files only in their own channel (SEC-UP-9); moving only with the same operator (03, `threads`); origin of every content in the invocation (SEC-AI-1); imports with `origin = import` never trigger (SR4); trust mark per content in the context, files always as data ([ADR 0054](../adr/0054-origin-and-trust-marking-in-the-agent-context.md)) | AZ-M44, AZ-M45, SEC-T08, conformance test of the adapters (AZ-P6) | medium: injection itself cannot be prevented; the runtime memory is the responsibility of the instance operator |
| T-AI-02 | **Privilege escalation through agents (confused deputy).** A person with few permissions triggers an agent with powerful tools or a wide read horizon. (OWASP LLM06) | high | high | tool class `host` only for owners or one's own ready dev environment (SR2); acting-for binding (SR9); I1 for personal subscriptions (SR1); grants to agents only by the owner (07, 2.6) | AZ-M20, AZ-M44, AZ-P11, AZ-P7 | medium with `scoped`: tools with their own credentials act within the scope of these credentials; low with `chat` |
| T-AI-03 | **File and link content as an injection source.** A PDF, a transcript or a web page that a tool loads contains instructions. | high | medium | the server loads no URLs (SEC-NET-*); files labeled with the origin "file" (SEC-AI-1); external effects only through approval cards (`scoped`); network tools only in the runtime with a tool class; trust mark per content in the context, files always as data ([ADR 0054](../adr/0054-origin-and-trust-marking-in-the-agent-context.md)) | SEC-T09, SEC-T14 | medium: depends on the discipline of the runtime and on people who read cards |
| T-AI-04 | **Agent writes into foreign channels.** A compromised or deceived runtime posts in spaces that it does not serve. | medium | medium | agents write only with a grant or as operator (07, 2.9); acting-for binding when writing outside their own thread (SR9); `rl.agent_runtime` | AZ-M28, AZ-M29, AZ-P11 | low |
| T-AI-05 | **Approval obtained by deception through cards.** An agent builds a card that looks harmless, or imitates a system header. | medium | high | card header from server fields (SEC-OUT-4); `approve_actions` required; by default only the triggering person of the run (SEC-AI-4); audit of every confirmation | SEC-T14, AZ-M42, AZ-M43 | medium: a person can still confirm a poorly described effect |
| T-AI-06 | **Exfiltration through rendering.** Agent output contains image URLs with data in the path, or links that send data when loaded. (OWASP LLM05) | high | medium | clients load no foreign resources (SEC-OUT-2); link target visible (SEC-OUT-1) | SEC-T13 | low: only a deliberate click on a link remains |
| T-AI-07 | **Loops and consumption.** Agents trigger each other, or a person starts runs en masse. (OWASP LLM10) | medium | medium | only `origin = human` triggers (SR4); `rl.trigger`; at most one active run per thread (R-DM-14); cost limits per agent ([10-backend-policy.md](../spec/10-backend-policy.md)) | AZ-M30 | low |
| T-AI-08 | **Misuse of a personal subscription.** A second person uses an agent on the personal subscription of the owner; violation of the provider terms, suspension of the organization possible. | medium | high | I1 transactional, I2 before every invocation, I3 configured and reported mode in the adapter (SR1), a reported mode that cannot be determined aborts (`billing_mode_mismatch`, fail closed); badge "Personal" in every header | AZ-M21 to AZ-M25, AZ-P6, AZ-P7, AZ-M51 | low in the server; medium in the adapter: the proof of the reported mode with Hermes Agent depends on a non-public interface, and no test in the repository fully covers a mode falsely reported as safe (RK-13); whether a runtime is permitted in personal mode is the responsibility of the instance operator (Baseline R-Anthropic) |
| T-AI-09 | **Leak of the channel prompt.** An agent reproduces its prompt. (OWASP LLM07) | high | low | prompts readable only with `manage`, never in events (R-AZ-09); hint "no secrets in the prompt" (SEC-AI-8) | | low, as long as prompts contain no secrets |

### 6.3 Tokens, devices, identity

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-TK-01 | **Theft of a device token** (malware, log, proxy). | medium | high | DPoP with a non-exportable key, nonce, `jti` check, short lifetime (08, 7) | ID-T14, ID-T17 | low; malware that uses the key on the device remains possible while it runs |
| T-TK-02 | **Theft of an agent or app token** (configuration file, repository). | medium | medium | only the hash stored, prefix for secret scanners, DPoP required for agents, scopes, at most `write`, no triggering, expiry required (08, 12) | ID-T23, SEC-SEC-4; M1-44: `TestAPITokens_Issue/TestIDT23_OnlyHashStored`, `TestAPITokens_Issue/TestIDT23_ShownOnce`, `TestIDT23_WrongChecksumRejectedWithoutLookup`, `TestAPITokens_Authenticate/TestIDT23_ExpiredRejected`, `TestAPITokens_Authenticate/TestIDT23_RevokedRejected`, `TestAPITokens_Authenticate/TestIDT23_DPoPBoundAgentTokenWithoutProof`, `TestAPITokens_Lifecycle/TestRevokeToken_Immediate`, `TestAPITokens_Lifecycle/TestAPIToken_NoReplayAfterRevocation`, `TestAPITokens_Authenticate/TestAPITokenNeverLogged`, `test_bothy_token_with_valid_checksum_is_detected`, `test_checksum_vectors_match_go`; M2-24: `TestCreateToken_AppShownOnce`, `TestAppToken_EffectiveScopes`, `TestCreateToken_AppScopes`, `TestAuthenticateAPIToken_AppInstallation`, `TestAppScope_TokenAndInstallation`, `TestUninstall`, `TestUninstall_RacesCreateToken` | medium for app tokens without DPoP: reading and writing within the scope of the app until revocation |
| T-TK-03 | **Theft of the URL of an incoming webhook.** | medium | low | one channel, writes as the app, never triggers, `rl.webhook_in`, revocable | AZ-M53, SEC-T10 (URL not in the log) | low: spam in one channel until revocation |
| T-TK-04 | **Theft of a private signing key.** | low | high | only in the key store `secrets.signing_key_dir` (mode `0700`, files `0600`, [ADR 0066](../adr/0066-signing-keys-in-the-writable-key-store.md)), private file deleted at `retired`, scheduled rotation, emergency rotation with `mode = emergency` (19, 10) | SEC-T11, M1-32 (`TestEmergencyRotation_*`, canary) | medium: whoever has it forges tokens until the rotation; detection through audit anomalies |
| T-TK-05 | **Reuse of an invocation token.** | medium | medium | expiry `agent.invocation_token_ttl`, `aud`, `jti` single-use via compare-and-swap at `run.start` (R-SY-10) | AZ-P6 | low |
| T-DV-01 | **Lost or stolen device.** | high | medium | app lock, encrypted cache, revocation with `4012`, wipe at the next contact, push registrations of the device disabled in the revocation transaction (08, 9 and 10; no `revoked` push, D-W-108) | ID-T26, client tests in [15-client-architecture.md](../spec/15-client-architecture.md) | medium: a device without contact keeps the cache; protection then only through OS encryption |
| T-ID-01 | **Account takeover through identity providers** (matching by email, foreign tenant, mix-up). | medium | high | matching only through `(issuer, subject)`, `tid` list, `iss` check, PKCE (08, 2 and 3) | ID-T01 to ID-T05; M1-41: `TestOIDC_Login/TestIDT01_PKCEDowngradeRejected`, `TestOIDC_IssParameter/TestIDT02_StateIssAndVerifier`, `TestOIDC_Login/TestIDT03_IDTokenRejected`, `TestOIDC_Login/TestIDT04_NoMappingByEmail`, `TestOIDC_Entra/TestIDT05_EntraTenantAllowList`, `TestOIDC_Login/TestOIDCLogin_LeakedAuthorizationURLCannotLink`, `TestIDTokenClaimValidation`, `TestOIDCEgress_PrivateTargetsRejected` | low |
| T-ID-02 | **Forged proxy header.** | medium | high | driver only with loopback listeners (otherwise off, `/readyz` reports it, W4), only trusted peers, binding per request (08, 2.4) | ID-T08 to ID-T10, SEC-T01; Go tests `TestIDT08_DriverOffWithoutLoopback`, `TestTrustedHeaderLogin/TestIDT09_MissingLoginHeader401`, `TestTrustedHeaderLogin/TestIDT10_BindingMismatchRevokesSession`, `TestTrustedHeaderLogin/TestBinding_MissingHeaderRevokes`, `TestSECT01_HeaderFromUntrustedPeerIgnored`, `TestTrustedHeaderProvisioning/TestTrustedHeaderNeverLogged` (`internal/identity/adapters/httpapi`) | low; residual risk with a misconfigured proxy |
| T-ID-03 | **Guessing of passwords and codes.** | medium | medium | Argon2id, limits per account, code and IP, real client IP (08, 13) | ID-T11, ID-T20, ID-T21, ID-T25 | low |
| T-ID-04 | **Takeover during the initial installation.** | medium | high | `503 setup_required` until the server command (08, 4) | ID-T06, ID-T07 | low |
| T-ID-05 | **A foreign device gets approved** (a person blindly presses "Allow"). | medium | high | typed-in confirmation code, step-up when approving (08, 5.2) | ID-T12, ID-T13 | low |
| T-ID-06 | **Delayed disabling after leaving** (account disabled in the identity provider, the session continues). | medium | medium | `user.disable` immediately; new login at the latest after `auth.idp_recheck_interval` (08, 3.5) | ID-T22 | medium until the planned automatic adoption of disablings (ADR proposal SCIM) |

### 6.4 Permissions

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-AZ-01 | **Missing check at a route or op** (IDOR). | high | high | one single decision point, `x-authz-action` required, startup check, route test (ADR 0017) | AZ-P1, AZ-P2, AZ-P3 | low |
| T-AZ-02 | **Leak through counters, search, suggestions or response time.** | high | medium | SQL predicate plus `authorize` per element, lint, 404 equality (07, 7.4) | AZ-P4, AZ-P5, AZ-P12, AZ-M06 | low |
| T-AZ-03 | **Delayed access removal** (cache, open connection, ongoing run, push). | medium | high | version per request, re-evaluation on `access.changed`, cancellation of the runs, I2 and `push.deliver` (07, 8.2) | AZ-P9, AZ-P10, AZ-M51 | low; on expiry by time, the client cache is cleared only after `authz.expiry_sweep_interval` |
| T-AZ-04 | **Repudiation of permission changes and denials.** | medium | medium | audit according to 05, hash chain, step-up with device, system actions for expiries and key changes | AZ-P13 | low: a hard crash loses the open aggregations of denials of one window (`audit.deny_aggregate_window`, 19 SEC-AUD-6); allow decisions and permission changes are recorded in their transaction |
| T-AZ-05 | **Privilege escalation through delegation** (`manage` grants more than it has, revokes something stronger). | medium | medium | delegation limits (07, 2.6) | AZ-M37 to AZ-M39 | low |
| T-OW-01 | **Owner reads other people's DMs or misuses "View as person".** (insider, E7) | low | high | DMs never for owners (SR6), "View as person" read-only, with step-up, time limit and audit (07, 9.2) | AZ-M02, AZ-M46, AZ-M47 | medium: owners with database access technically have everything; to be settled organizationally through a works agreement (20, 9) |

### 6.5 Platform, network, data

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-PL-01 | **Malicious plugin or app.** Reads everything within its scopes, deceives people with cards or messages. | medium | medium | apps never in the server process (B6), scopes plus grants, at most `write`, no triggering, installation only by owners with step-up, badge "App" (SEC-OUT-3), webhooks only to allowed targets | AZ-M32 to AZ-M34, AZ-M49; M2-24: `TestAZM32_AppWritesWithGrantAndScope`, `TestAZM33_AppReadWithoutScope`, `TestAZM34_AppWithoutGrant`, `TestInstallApp_RejectsInvalidManifests`, `TestValidateManifest_Rules`, `TestInstallApp_OwnerStepUpAudit`, `TestUpgrade_NewScopesStayInactive`, `TestUpgrade_RemovedScopeActsAtOnce`, `TestSuspendResume`, `TestAppAccessChanged_SuspendResumeUninstall`, `TestAppAuditDetails_Shape`, `TestAppEventScope_AllCatalogueEvents` | medium: what an app may read, it can exfiltrate; that is trust in the provider of the app |
| T-UP-01 | **Malicious uploads** (malware, bombs, active content, location data). | high | medium | SEC-UP-1 to SEC-UP-11 | SEC-T08 | low; the virus scan does not detect everything, hence no inline rendering of active formats |
| T-NW-01 | **SSRF.** | medium | high | no loading of foreign URLs, egress client with IP check at connection setup (SEC-NET-*) | SEC-T09 | low |
| T-NW-02 | **Eavesdropping or redirection on the network.** | medium | high | TLS, HSTS, `verify-full`, no disabled certificate verification | SEC-T01, SEC-T02, SEC-T17 | low |
| T-PU-01 | **Push metadata.** Push providers see time, device, app, frequency. | high | low | only IDs and a neutral placeholder (ADR 0031); IDs random | tests in [13-push-notifications.md](../spec/13-push-notifications.md) | low: metadata stays visible; data processing according to 20 |
| T-BK-01 | **Theft of a backup.** | medium | high | encryption, key kept separately, no secrets in plaintext (SEC-BAK-1, 2) | restore drill | low, as long as the key stays separate |
| T-BK-02 | **Restore brings back deleted data.** | medium | medium | deletion journal (20, 4.6; SEC-BAK-3) | SEC-T15 | low |
| T-DB-01 | **Modification of the audit log with database access.** | low | high | permissions of the application role, hash chain, signed and exported checkpoints (ADR 0021) | SEC-T16; the application-role part is proven since M1-24 (`TestSECT16_AppRoleCannotUpdateOrDelete`, `TestAudit_TriggerBlocksOwnerUpdate` in `services/server/migrations`), the detection of a changed row and of a rewritten chain through the checkpoints of the sink is proven since M1-50 (`TestSECT16_ChangedRowIsDetected`, `TestSECT16_RewrittenChainIsDetectedByCheckpoint`, `TestVerifyIgnoresDatabaseCheckpoints` in `services/server/internal/audit/adapters/httpapi`) | medium: whoever controls the database and the export target can change both |
| T-LG-01 | **Secrets or content in logs and telemetry.** | high | medium | allow lists, type for secrets, route patterns (SEC-LOG-*) | SEC-T10 | low |
| T-IN-01 | **Input attacks** (injection, parser differences, bidi deception). | medium | medium | SEC-IN-1 to SEC-IN-8 | SEC-T06, SEC-T07 | low |
| T-DS-01 | **Overload** (connections, search, uploads, sequence counter). | medium | medium | rate limits (05), size and time limits, load tests ([23-quality.md](../spec/23-quality.md)) | load tests | medium: one host carries everything (Baseline B1, B13) |
| T-AC-01 | **Attacks on the console** (XSS, CSRF, clickjacking, session theft). | medium | high | BFF cookie, CSP, Trusted Types, origin check, short session, step-up via passkey (08, 7.5; 19, 5) | SEC-T03 to SEC-T05, ID-T19 | low; malicious browser extensions remain (recommendation: a separate browser profile for administration) |
| T-AC-02 | **Phishing of the owner.** | medium | high | passkeys are bound to the origin; OIDC through the provider with its MFA | | low with passkeys, medium with a plain password with TOTP |

### 6.6 Supply chain and dev environment

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-SC-01 | **Manipulated dependency.** (OWASP LLM03 for model and agent packages) | medium | high | lockfiles with checksums, Renovate with a waiting period, OSV-Scanner, license gate (SEC-SC-1) | gates in 23 | medium: a new, still unknown compromise is detected by no scanner |
| T-SC-02 | **Manipulated GitHub Action.** | medium | high | full SHA, zizmor, actionlint, minimal `permissions` (SEC-SC-2); example: the Trivy incident in Baseline R-Qualität | gates in 23 | low |
| T-SC-03 | **A self-hosted runner executes foreign code.** | high | high | only GitHub-hosted runners with a fixed label (SEC-SC-3, ADR 0039) | `runners-check` (rule `runner-not-github-hosted`) | low |
| T-SC-04 | **Forged release or update.** | low | high | signature, notarization, Sparkle EdDSA, provenance, SBOM (Baseline R-Desktop-Verteilung, ADR 0036) | release check in 18 | low |
| T-SC-05 | **Cache poisoning through a dependency PR.** A pull request (for example an update from Renovate) executes foreign code and modifies a cache that a later job with signing secrets uses. Second edge: a compromised job without signing secrets, in the release the job `resolve`, dictates its own path, a version or an artifact `dist-*` that a release job builds, signs or publishes. | medium | high | Every job runs in a new VM of a GitHub-hosted runner (ADR 0039); the release jobs in `release.yml` restore no cache; Renovate with a waiting period (ADR 0041). Release jobs use outputs of `resolve` only in the `if:` of the job or of a step (D-W-72), resolve the tag again from the fixed component list, abort on a mismatch and download artifacts only by ID (D-W-52). Environments count in any spelling, environment names only literally (D-W-110) | `runners-check` (rules `runner-not-github-hosted`, `environment-not-literal`, `release-untrusted-output`, `release-artifact-by-id`); that release jobs use no cache is checked by the review | low for release secrets and release content; medium for the deny list and `SONAR_TOKEN`, which jobs of pull requests from the repository get, and for gate results that a compromised pull request job can forge |
| T-DEV-01 | **Dev runner as a back door to the host.** A coding agent without tool permission prompts runs with the permissions of the host user; every person with `trigger` and every injection in the repository thereby has a shell on the host. | high | high | SR2 and SR3: owners only, until an environment per person with a VM boundary is ready (21) | AZ-M15 to AZ-M20, Red Team P3 | low today, because only owners trigger; medium after enabling, depending on the environment |

### 6.7 Terminal

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-TM-01 | **Reading terminal content in the server** (relay, proxy, log, database dump, backup, memory dump of an api process). | medium | high | Noise `NK` end-to-end (SEC-TERM-1); no content in log, audit, telemetry, event log | TM-T01, TM-T02, TM-T06 | low against these attackers. **Not protected** against a malicious instance operator of the server (E12): they can distribute their own key and allow sessions. The only countermeasures are pinning per device (warns only on a change after the first opening; ineffective in the web console, because the server delivers the code of the console), audit of the re-pairing and `host_service.require_local_confirm`; residual risk medium, honestly documented (Ruling T3) |
| T-TM-02 | **Replay and swap at the relay.** The relay repeats message 1 or inserts a foreign token to start a PTY that nobody operates. | medium | high | one-time token with `jti`, `cek` bound to the ephemeral key, prologue with session and host, PTY only after `start` (SEC-TERM-2) | TM-T03 to TM-T05 | low |
| T-TM-03 | **Unauthorized opening** through a stolen session, delegation, a foreign host or the server host. | medium | high | step-up `fresh` per opening, target `host` outside the tree, granted only by owners, SR10, server host owners only (SEC-TERM-3, 5, 10) | TM-T07, TM-T08, AZ-M54, AZ-M55, AZ-M58, AZ-M60 | low |
| T-TM-04 | **Privilege escalation on the host.** The shell runs as a user with admin permissions, passwordless sudo or in a group with root-level permissions (`docker`, `lxd`, `libvirt`, `disk`), or it reads the home folders of other users (on macOS through the group `staff`). | medium | high | `doctor` HD1 to HD3 refuses the start (fixed block list of groups plus `host_service.extra_forbidden_groups`, Docker socket), HD9 for readable home folders of users who are not session users; an open home of a session user blocks, according to HD11, only that session user, by name (24 section 2.2 item 4); installation with its own user, `session-user remove` closes the home, afterwards the instance operator deletes the user together with its long-running processes (16, 4.4) | TM-T13, TM-T24 | medium: gaps in the operating system of the host and local services remain; the host belongs to the trust zone of its assignee |
| T-TM-05 | **Theft of the keys of the host service**, for example by a person with a terminal on the host (also after their access removal) or from a backup of the state folder. With both keys the attacker logs in from elsewhere as the host, receives every `session.open`, passes the handshake and reads along every later session, including those of owners together with the passwords they type. They do not need a relay for this. | medium | high | sessions run as session users without read permission on the state folder (HD8, 24 section 2.2); file `0600` or DPAPI; a second live control connection disables the host with audit and notification to all owners (24, section 3.4); re-pairing discards both keys | TM-T18, TM-T22, TM-T23 | medium: whoever escalates to the service user on the host has the keys; if the real host is then off, the attacker receives sessions until the real host comes back and triggers the disabling. `host_service.require_local_confirm` closes this for hosts with a person in front of them |
| T-TM-06 | **Pairing of a foreign host** with an intercepted code. | low | high | code single-use, short-lived, format with checksum and secret scanner rule; the owner types in the fingerprint; failed attempts limited | TM-T19, TM-T20 (`TestTMT19_FingerprintMaxAttempts`, `TestTMT20_PairingCodeRejections`, `TestTMT20_WrongChecksumNoDatabase`, `TestHostPairingCode_MatchesGitleaksRule`, `test_pairing_code_with_valid_checksum_is_detected`, M2-41) | low |
| T-TM-07 | **Browser extension or XSS in the console** reads or controls an open session. | medium | high | passkey required per opening, CSP with Trusted Types, no HTML from output, origin check on upgrade of the stream path (SEC-TERM-7, ADR 0070) | SEC-T05, ID-T28, origin test (PLAN M6-18) | medium: a malicious extension with access to the origin remains; recommendation of a separate browser profile |
| T-TM-08 | **Misuse of someone else's LLM login** through the terminal, also through a subscription login that someone creates during a session, or by reading the API key from the environment of the profile `claude`. | medium | high | Load-bearing: one session user per person and host, never shared, not even after the end of the assignment; no terminal without an assignment (SR12); the host service binds every session user to the first person (Ruling T12, 24 section 2.2); no location connects the accounts again: working directories and `CLAUDE_CONFIG_DIR` per session user, HD11 for every configured path (Ruling T13, T-TM-13). In addition I6 with an individual grant, checked on every opening; as an additional check `doctor` HD6, which refuses the start on a contradiction to the declaration and runs before every PTY and periodically (a hit ends all sessions), only through `host_service.claude_bin` with a fixed search list, which HD10 checks like the helper, including folder chain and interpreter; key per host with a spending limit and a new key after access removal; profile `claude` with a controlled environment (24, 7) | TM-T09, TM-T13, TM-T27, TM-T28, TM-T29, AZ-M56, AZ-M57, AZ-M61 | low for other people's subscription logins: a login in a session user reaches only its person, even if HD6 does not see it (macOS without `host_service.claude_bin`, warning `auth_status_unchecked`) or its result is forged from the same account, as long as no location connects the accounts again (T13). Remains: a shared folder outside the configured paths in which a person starts the coding agent themselves (T-TM-13); a local privilege escalation on the host lifts the separation of the session users (T-TM-04); after retiring and re-pairing the host, or after loss of the state folder, only the instance operator keeps a previously assigned session user away from another person (24 section 2.2, 16 section 4.4); the declaration of the owner cannot be verified, `doctor` shows a false declaration `organization` with a hidden subscription only at known locations; with `organization` the API key is readable by every session |
| T-TM-09 | **Prompt injection opens a terminal.** | high | high | terminal actions only for people in a session with step-up; agent and app tokens rejected | TM-T17, AZ-M59 | low |
| T-TM-10 | **Escape sequences** write the clipboard, open links or query data. | medium | medium | SEC-TERM-6 | TM-T14 | low |
| T-TM-11 | **Session or processes remain after access removal.** The session stays open, or processes of the person keep running (`nohup`, `setsid`, child processes outside the ConPTY, long-running processes such as crontab, systemd services of the user, LaunchAgent, scheduled task). | medium | high | access removal ends sessions in the same transaction; the server additionally measures idle time; every session in its own process group, cgroup or Job Object, whose end ends the whole tree (24, 6.3, 6.5 and 8.3); a session user belongs to only one person and is never reassigned (SR12, T12) | TM-T10, TM-T11, TM-T28 | low for other people: no session end ends long-running processes that were set up as the session user, but they run in the session user of the person whose access was removed, which nobody else gets; they do not reach the keys (HD8), no foreign home folders (HD9; for the home of another session user, HD11) and cannot replace the helper (HD10). Remains: they keep using computing time, network and local services of the host until the instance operator deletes the session user together with its long-running processes (locking alone does not end them; 16 section 4.4 item 1); the console points this out after the access removal of a person who is not an owner; after `bothy-host session-user remove` they can reopen the home of their user until then and bring the host to a standstill through HD9 (T-TM-14); non-owners work only on their own host (SR10). After retiring and re-pairing the host, or after loss of the state folder, only the instance operator prevents the session user, together with such long-running processes, from being assigned to another person (24 section 2.2, 16 section 4.4) |
| T-TM-12 | **Replacing or tapping the PTY helper.** A shell or a long-running process of the session user ends the helper and puts its own in its place, attaches to the running helper or to shells of later sessions, forges HD6, HD8, HD9 and HD11 and reads the plaintext of later sessions of the same session user. | medium | high | helper as a system service under the session user; binary, definition and channel endpoint not writable for it; channel endpoint from the service manager; `bothy-host` checks user and process ID of the peer, and on a wrong peer the message of HD10 names the session user of this channel (24 section 2.2, rule for names); memory protection of the helper; on Linux `kernel.yama.ptrace_scope` at least `1`; HD8 additionally through the service user (HD10, 24 section 2.2); warning before the step-up for owners on the host of another person, and audit `foreign_assignee` (24, 8.4) | TM-T22, TM-T26, TM-T28 | medium: processes of the same user ID reach the PTY device of every session of this session user; since T12 these are only sessions of the same person, sessions of different people are separated by the user ID. Against an assignee who manipulates their own host or escalates privileges there, the only protection is that owners do not work there or take the warning seriously |
| T-TM-13 | **Shared location between session users.** Person A writes into a location that the session of B reads or executes: `.claude/settings.json` with hooks or `apiKeyHelper`, `CLAUDE.md` or `env` in a shared working directory or a folder above it, a shared `CLAUDE_CONFIG_DIR` or a program of a profile. The code of A then runs as the session user of B and reaches B's files and login; a `/login` in a shared `CLAUDE_CONFIG_DIR` reaches all people. | medium | high | Ruling T13 (24 section 2.2 item 6): working directories per session user (`host_service.claude_workdirs`, read only at start; HD7 removes a folder listed for two session users from both lists), index only in the own reported list (`wdi`, resolved by the host service in exactly this list of `osu`), `CLAUDE_CONFIG_DIR` with `organization` fixed in the home of the session user and with `personal` only in the home, no foreign chain files (`CLAUDE.md`, `CLAUDE.local.md`, `AGENTS.md`, `.mcp.json`, `.claude`, `.git`) in the folders above, programs including folder chain owned by a trusted owner (on Windows `SYSTEM`, `Administrators` or `TrustedInstaller`), the profile `claude` with `organization` only with `host_service.claude_bin`; HD11 checks every configured path after full resolution including links, and the home including folder chain and chain files, with one rule; "writable" includes creating entries; on Windows `bothy-host install` removes the permission to create folders in the drive root; before every PTY and periodically, and it blocks only the affected session user; shared code only through git (SEC-TERM-13) | TM-T29, TM-T24 | low for configured locations. Remains: a folder outside the configuration that two session users can write and in which a person starts the coding agent themselves in their shell (HD11 does not see it; the instance operator does not set it up, according to 16 section 4.4); a file that a later version of the coding agent loads from the folder chain and that is not yet among the chain files; HD11 never sees permissions that a session user opens and closes again between two checks, sees an entry that A creates in this time only at the next start of the service or call of `bothy-host doctor`, and does not see at all what A writes into an existing file that is writable for A |
| T-TM-14 | **Standstill of a host caused by one person.** With `none` or `organization` a person with a terminal logs in with a subscription in their own session (`/login`); HD6 detects this, the host service ends all sessions of all people and accepts no new ones until HD6 passes in every session user from `host_service.session_users` (Ruling T2). Second path: a chain file of one person in a folder on the path to the locations of several session users (for example a `C:\.git` from the time when the root of the drive was writable for everyone) makes HD11 fail for every other one of these session users. Third path: a long-running process that a person set up as the session user reopens the home of their former session user after `bothy-host session-user remove`; HD9 then refuses the start of the whole host on the next start or `bothy-host doctor`. | medium | medium | The report of HD6 names every affected session user with its own entry, and the console also the person of its assignment, also on a hit at start: the host service then does not start, but reports the result beforehand, and `bothy-host doctor` names the same names on the host (24 sections 2.2 and 7.4); instance operator step in 16 section 4.4: remove the login in exactly this account, or end the assignment and take the session user out of the host service with `bothy-host session-user remove` (locking the user alone does not end the standstill, because HD6 runs in every name from `host_service.session_users`); audit and `host.updated` to all owners. Against the second path: folders on the path to a location are writable for no session user (HD11); on Windows `bothy-host install` closes the drive root; the report of HD11 names every affected session user, and the instance operator removes the entry (16 section 4.4). A home that a person with terminal access opens for others is no path to the standstill: HD11 checks it as a location of this person and blocks only that person, not HD9 with "start refused" (24 section 2.2 item 4). An attempt to replace the PTY helper (T-TM-12) brings the host to a standstill through HD10 as well; the message then names the session user of this channel (24 section 2.2, rule for names). Against the third path: the message of HD9 to the server names every former session user whose home can be listed, the console also the person of its assignment (24 section 2.2, "Reason for HD9"); instance operator step in 16 section 4.4 item 1: right after `remove` remove the long-running processes of the user, end its processes and delete it together with its home, after which HD9 no longer checks its home; locking alone is not enough | TM-T21, TM-T24, TM-T26, TM-T29 (i) | medium: until the instance operator acts, nobody has a terminal on the host, or none of the blocked session users has one. This stays so because Ruling T2 forbids the start on a contradiction to the declaration and HD11 must not skip a foreign chain file; the second path exists only with an entry from a time when a folder on the path was writable; the consequence is reversible and stays on the host |

### 6.8 Visualization

Normative together with [25-visualization.md](../spec/25-visualization.md); controls SEC-VZ-* in
[19-security.md](../spec/19-security.md), section 11b.

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-VZ-01 | **Clickjacking through a Grafana embedding.** With `allow_embedding` Grafana no longer sends `X-Frame-Options`; a foreign page frames Grafana and lures a logged-in owner into clicks. | low | medium | embedding off by default; only behind an auth proxy that sets `frame-ancestors` to the origin of the console; `frame-src` of the console exactly this origin (SEC-VZ-2) | VZ-T12, SEC-T19 | low, as long as the proxy is configured correctly; the configuration of the proxy is the responsibility of the instance operator |
| T-VZ-02 | **Leak through the live stream.** A space manager sees runs, costs or instance values outside their channels, or content gets into frames. | medium | medium | delivery mode per frame, filter on visible channels, costs only with `cost.read.aggregate`, rows without a channel only for owners, no titles and details (SEC-VZ-3, SEC-VZ-4) | VZ-T05, VZ-T06, VZ-T07 | low |
| T-VZ-03 | **Performance monitoring of individual people** through runs in channels with only one person. | medium | medium | no values per person, no triggering person, no counters across people; live stream can be switched off; note for co-determination (20, section 9) | DP-T12 | medium: from runs in a channel with one person, conclusions about that person are possible; the owner can read the thread anyway |
| T-VZ-04 | **Unattended TV screen** shows costs or runs to visitors, or the session behind it is misused. | medium | low | TV mode without controls, costs hidden by default (`viz.tv_show_costs`), admin actions need step-up, no kiosk token (R-VZ-09) | | medium: the session on the screen has the read permissions of the logged-in person; recommendation: a separate person with only `manage` on the shown spaces |
| T-VZ-05 | **Malicious or modified Grafana plugin.** | low | medium | plugin from the own repository with supply chain checks; unsigned only allowed with its own ID (SEC-VZ-5); signature after V1 | VZ-T13 | low |
| T-VZ-06 | **Overload from many viewers** of the live stream. | medium | low | one base frame per node and interval, maximum number of subscriptions per node, rate limit `rl.read` for the REST query, cached frame | VZ-T08 | low |

## 7. What remains open

- Prompt injection is unsolved. The product does not rely on detection, but on
  boundaries: who triggers, what the agent reaches, what takes effect only with a card.
- The server does not see the memory of a runtime across runs (T-AI-01, H1).
- The terminal stream does not protect against a malicious instance operator of the server
  (T-TM-01); the product assumes that the server is trustworthy.
- Long-running processes that a person sets up as a session user on a host survive their
  access removal until the instance operator deletes this user together with them (T-TM-11); they
  stay in their own session user, which nobody else gets (T12).
  All sessions of one person on a host share one trust zone through their user ID; between
  people, the user ID separates only until a local privilege escalation (T-TM-04, T-TM-12)
  and only as long as no location connects the accounts again; HD11 checks the configured
  locations, not every folder of the host (T-TM-13).
- A person with a terminal can bring a host with `none` or `organization` to a standstill for
  everyone through a subscription login in their own account, until the instance operator
  removes the login or takes their session user out of the host service; likewise with a chain
  file from a time when a shared folder on the path was writable for them, until the instance
  operator removes it; likewise with a long-running process that reopens the home of their former
  session user after `bothy-host session-user remove` (HD9), until the instance operator deletes
  this user (T-TM-14).
- One host carries everything (T-DS-01). More availability means several roles on several hosts
  (Baseline B1), not part of M1.

## 8. Link to tests

Every threat in the register names tests from 07, 08 or 19. A threat without a test has an empty
entry in the column "Tests" and, in the next red team gate, must either get a test or justify its
residual risk explicitly.

## 9. Maintenance rule

The threat model is updated:

1. **At the start of every milestone:** enter the new components, data flows and boundaries of the milestone.
2. **Before every red team gate** (end of every milestone, before the merge into `main`, before every release):
   The reviewer compares the diff with the system boundary diagram. A new data flow across a
   boundary, a new external dependency, a new login method, a new tool class or a
   new app feature without an entry here is a finding that blocks the merge.
3. **Immediately** after a security incident, a finding from a penetration test or an
   ADR that changes a boundary from section 2.
4. Every change carries the milestone in the status header ("as of milestone ...") and goes in as a
   pull request with review by a second person or a fresh red team.
5. If a mitigation is removed or weakened, the residual risk of the entry rises in
   the same change; an entry never keeps a rating whose basis is gone.
