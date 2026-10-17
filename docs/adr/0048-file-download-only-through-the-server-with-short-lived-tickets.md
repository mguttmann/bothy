---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team G (platform services), Team B (API), Team C (security)
informed: Team E (apps)
---

# File download only through the server with short-lived tickets instead of presigned GET

## Context and Problem Statement

Every file download must go through `authorize`, and a revocation takes effect immediately (B7,
[07-permissions.md](../spec/07-permissions.md), sections 8.1 and 8.2). The S3 driver could give clients
presigned download URLs and take load off the server. But media players and "Open with"
send their own HTTP requests with `Range` and cannot create a DPoP proof per request
([08-identity.md](../spec/08-identity.md), section 7.3). How do such downloads get access
without a link surviving a revocation?

## Decision Drivers

- `authorize(principal, file.read, file)` at download time, for the original, the preview and the
  transcript.
- A revocation (grant, session, device, disabling) takes effect on the next download.
- Headers such as `Content-Disposition`, `nosniff` and the sandbox CSP are set only by the server;
  the admin console runs in the browser ([12-files.md](../spec/12-files.md), section 9.3).
- The storage does not have to be reachable for clients; the file system driver has no URLs at
  all.
- Media players need simple GET requests with `Range`.
- The same behavior for both drivers (ADR 0010).

## Considered Options

- The server serves all bytes itself; short-lived HMAC tickets bound to the principal and the
  session for players, checked again on every download
- Presigned GET URLs of the object storage with a short validity
- The server serves directly; players get the normal access token in the URL
- The server checks, the reverse proxy serves the bytes (`X-Accel-Redirect` or similar)
- Cookie session for media downloads

## Decision Outcome

Chosen option: "The server serves all bytes itself, with short-lived tickets", because it is the
only one that enforces `authorize` on every download without depending on the storage or on the
edge. The rule is R-FI-03 in [12-files.md](../spec/12-files.md), section 9.

1. `GET /api/v1/files/{id}/content`, `.../thumbnails/{variant}` and `.../transcript` deliver the
   content through the server, bytes with `Range` (exactly one range). There are no presigned
   GET URLs; the port of the object storage has no operation for them.
2. `POST /api/v1/files/{id}/tickets {variant}` (action `file.read`) returns `{url, expires_at}`.
   The ticket is `base64url(payload || HMAC-SHA-256(key, payload))` with `file_id`, `variant`,
   `principal_id`, `auth_session_id` and the expiry `file.ticket_ttl`. The key comes from the
   secret provider (`file.ticket_key_ref`); two keys are valid at the same time so that a rotation
   works without an outage.
3. On every download with a ticket the server checks the signature, the expiry, whether the
   session is still valid, and `authorize` with the current permissions. Every failure results in
   the same `404` as an unknown ID.
4. Responses with a ticket carry `Cache-Control: private, no-store`. The parameter `ticket` is
   removed from logs and telemetry attributes; the collector filters it in addition
   ([17-observability.md](../spec/17-observability.md)).
5. The push preview uses the same tickets for the first image (`image_url`,
   [13-push-notifications.md](../spec/13-push-notifications.md), section 6.4).

HMAC instead of a signed token with the server keys: only the server checks the ticket;
a symmetric key is shorter and faster, and nobody outside needs a public key.

### Consequences

- Good, because a ticket does not survive any revocation: session revocation and permission
  revocation take effect on the next download.
- Good, because the object storage can stay in private networks and no bucket has to be publicly
  reachable (SEC-UP-11 in [19-security.md](../spec/19-security.md)).
- Good, because both drivers have the same path; clients see no difference.
- Bad, because every download goes through the server (bandwidth, open connections up to
  `http.transfer_timeout`). For a team chat with `file.max_upload_bytes` this is acceptable;
  downloads are throttled with `rl.read`.
- Bad, because a ticket URL is a bearer secret until `file.ticket_ttl`: whoever gets it within this
  time (for example through "Open with" to a third-party app) can download as long as the
  principal still has the permission.
- Neutral, because uploads may still optionally be presigned (R-FI-02); there the server checks
  size and checksum at the end.

### Confirmation

- Tests from [12-files.md](../spec/12-files.md), section 15: a ticket with `Range` returns
  `206`; after permission revocation, after session revocation and after expiry each returns
  `404`; another person's ID and an unknown ID return the same `404`.
- Architecture rule: only `internal/files/adapters/s3` imports the S3 SDK, and the port
  `ObjectStore` has no operation for signed download URLs (check of the port interface in the
  driver conformance test).
  The audit checkpoint sink `internal/audit/adapters/s3sink` (M7-22) is the one other package
  that imports the SDK; it serves no file and holds no presign client, so the rule of this ADR is
  unchanged.
- Canary test from [17-observability.md](../spec/17-observability.md), section 10: a ticket
  appears in no export.
- SEC-T03 (headers for HTML and SVG: `attachment`, `nosniff`, sandbox CSP).
- Build ticket M3-05 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### Server serves directly, with tickets

- Good, because permissions, headers and logging are in one place.
- Bad, because the server carries the bandwidth.

### Presigned GET URLs

- Good, because downloads do not put load on the server.
- Bad, because a URL is valid until it expires, even after a revocation; the server does not see
  the download and cannot check it.
- Bad, because the storage must be reachable for clients and the headers are in the hands of the
  storage.
- Bad, because the file system driver has no URLs; two download paths for two drivers.

### Access token in the URL

- Good, because no new mechanism is needed.
- Bad, because access tokens are bound to the device key (DPoP) and are not valid without a proof;
  a token without binding would weaken the whole token model.
- Bad, because tokens in URLs end up in logs of proxies and players and are valid for the whole
  API.

### Reverse proxy serves the bytes

- Good, because the server only checks and the bandwidth lies with the proxy.
- Bad, because the mechanism is proxy-specific and does not work with every edge of the instance
  ([16-operations.md](../spec/16-operations.md), section 4.3); the proxy would have to be able to read
  the storage. It stays possible as a later optimization, because the check stays in the server.

### Cookie session for media

- Good, because players often send cookies along.
- Bad, because the native apps have no cookie session; cookies exist only for the
  web console (08, section 7.5). A second session model for apps would be more attack surface
  than a tightly bound ticket.

## More Information

- Decision R-FI-03 in [12-files.md](../spec/12-files.md); endpoints in
  [06-api.md](../spec/06-api.md), section 10.
- Related: ADR 0010 (object storage), ADR 0017 (one authorization function, 404 instead of 403),
  ADR 0020 (DPoP), ADR 0047 (image metadata).
