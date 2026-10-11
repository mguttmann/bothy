---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Builder check (fresh reviewer), Team security and permissions
informed: Team apps and design
---

# Terminal in the web console

Partly supersedes [ADR 0028](0028-web-target-only-for-the-admin-and-statistics-console.md): the
web console gets the terminal in addition to administration and statistics. Everything else from
ADR 0028 still applies, in particular: no chat in the browser.

## Context and Problem Statement

ADR 0028 limits the web target to administration and statistics, to keep offline storage, push and
the control of agents out of the browser. The owner explicitly requires the terminal in the web
console as well (addendum of 2026-10-10) and a passkey confirmation on opening. So far the
console also allows step-up by a new OIDC login or by password with TOTP
([08-identity.md](../spec/08-identity.md), section 6.2). A browser has extensions and
XSS as attack surface. May the console open a terminal, and under which conditions?

## Decision Drivers

* Owner requirement: terminal in desktop, phone and web console.
* Passkey confirmation on opening without a fallback path.
* A terminal in the browser is remote access; XSS in the console must not open a host.
* No second UI technology; the same widget as in the apps.
* No chat in the browser (ADR 0028 stays).

## Considered Options

* Terminal in the console with mandatory passkey (`userVerification = required`) and hardened CSP
* No terminal in the console; terminal only in the native apps
* Terminal in the console with any step-up method of the console

## Decision Outcome

Chosen option: "Terminal in the console with mandatory passkey and hardened CSP", because it meets
the owner's requirement and excludes the weaker step-up methods of the console for the terminal.

1. `terminal.open` and `terminal.attach` in the console only with step-up by WebAuthn passkey with
   `userVerification = required`, fresh and consumed per opening. No fallback path; without a
   passkey, the console shows the way to set one up ([08-identity.md](../spec/08-identity.md),
   section 6.3).
2. The same CSP as the rest of the console: `connect-src 'self'` for the stream, Trusted Types,
   `frame-ancestors 'none'`. The terminal renders with the Flutter widget `xterm` into the canvas; it
   creates no HTML from terminal output [derived from the design of Flutter Web]. Controls
   SEC-TERM-* in [19-security.md](../spec/19-security.md).
3. The Noise key of the device lives only in the memory of the tab; pinned host keys live in the
   storage of the origin and are deleted on logout. In the console, pinning only catches a key
   change that an honest server reports; that is why deleting them costs no protection.
4. The quick reattach without step-up (`terminal.quick_reattach_window`) does not apply in the
   console: without a device key, only the cookie proves the request. Every attach needs a
   fresh passkey.
5. No chat, no offline storage, no push in the console (ADR 0028 unchanged).

### Consequences

* Good, because owners without the app, for example right after the installation, can reach a host.
* Good, because an attacker with a stolen console session but without a passkey opens no terminal.
* Bad, because people without a passkey have no terminal in the console. This is intended.
* Bad, because in the console, pinning does not protect against a malicious instance operator:
  they serve the code of the console and can send a version without the pin check. The only
  mitigation there is `host_service.require_local_confirm` ([24-terminal.md](../spec/24-terminal.md),
  section 5.1; threat model T-TM-01).
* Bad, because a new passkey is needed after every loss of connection; in the console that is
  acceptable because it runs at a desk.
* Bad, because malicious browser extensions can read along in an open session
  (residual risk in the threat model, T-TM-07); recommendation: a separate browser profile for
  administration.

### Confirmation

* ID-T28 in [08-identity.md](../spec/08-identity.md): a console with password step-up or a
  passkey without `userVerification = required` gets no terminal; reattaching in the console
  without a passkey fails even within `terminal.quick_reattach_window` (TM-T12).
* SEC-T05 (CSP in the headless browser) runs with an open terminal.
* The route test of the console from ADR 0028 stays: no routes for channels, threads or messages.

## Pros and Cons of the Options

### Mandatory passkey and hardened CSP

* Good, because the owner requirement is met and the passkey is bound to the origin (phishing
  protection).
* Bad, because another path in the browser must be checked.

### No terminal in the console

* Good, because the smallest attack surface.
* Bad, because it contradicts the owner requirement.

### Any step-up method of the console

* Good, because it is usable even without a passkey.
* Bad, because password with TOTP and OIDC login can be phished and miss the requirement
  "passkey".

## More Information

* Related: [ADR 0028](0028-web-target-only-for-the-admin-and-statistics-console.md),
  [ADR 0068](0068-terminal-through-a-host-service-with-noise-stream-and-invariant-i6.md), ADR 0020.
* Ruling T4 of the lead.
