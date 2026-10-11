# Operations with Tailscale

Status: draft M0. Tailscale is a documented option, not a requirement (B9). Rules:
[16-operations.md](../spec/16-operations.md) 4.3. Identity provider `trusted_header`: chapter 08.

## 1. When to use this option

The instance is reachable only in the tailnet; Tailscale provides TLS and the identity of the people.
There is no public hostname and no own reverse proxy.

Consequences that must be clear beforehand:

1. Passkeys in native Apple apps need a publicly reachable Associated Domain; a
   pure tailnet domain is not enough in normal operation (Baseline R-Passkeys, source
   https://developer.apple.com/documentation/xcode/supporting-associated-domains, retrieved
   2026-10-10) [verified]. Step-up and device binding therefore run via the device key.
2. The Notification Service Extension fetches the content of a push via the normal API. If the VPN
   on the device is not connected at that moment, the neutral placeholder stays (R-Push).
3. Tagged devices and Funnel deliver no identity headers
   (https://tailscale.com/kb/1312/serve, retrieved 2026-10-10) [verified]. Agent adapters therefore
   log in with their API token, not via headers.
4. Device names with an HTTPS certificate appear in the public Certificate Transparency log
   (https://tailscale.com/kb/1153/enabling-https, retrieved 2026-10-10) [verified]. Choose a
   neutral `TS_HOSTNAME`.
5. The plan of the tailnet (number of people, commercial use) is a decision of the
   owner (https://tailscale.com/pricing, retrieved 2026-10-10) [verified].

## 2. Setup in the Compose stack

Profile `tailscale` instead of `direct`:

1. The service `tailscale` (image `tailscale/tailscale`) logs in once with `TS_AUTHKEY` and
   keeps its state in the volume `tailscale-state` (`TS_AUTH_ONCE=true`).
2. `TS_SERVE_CONFIG` (in `compose.yaml` as a `configs` entry) forwards HTTPS 443 to
   `http://127.0.0.1:8080` and forbids Funnel.
3. `api-tailscale` shares the network namespace of the sidecar (`network_mode: service:tailscale`)
   and its public listener listens only on `127.0.0.1:8080`. No other container and no other host
   reaches this listener.
4. The sidecar runs with kernel networking (`TS_USERSPACE=false`, device `/dev/net/tun`, capability
   `NET_ADMIN`), as in the example from Tailscale. Then item 3 is a property of the kernel:
   packets that arrive via `tailscale0` reach no socket that is bound only to `127.0.0.1`.
   Whether userspace mode forwards incoming connections to loopback ports
   is not documented (https://tailscale.com/kb/1112/userspace-networking, retrieved
   2026-10-10, describes only outgoing traffic) [verified]; therefore no userspace mode.
   Whether the Docker VM on the Mac provides `/dev/net/tun` is checked by the portability test
   [assumed].
5. The internal listener (`:8081`) is reachable for services in the stack under the name `api` (alias
   on the sidecar). It trusts no headers (R-OP-01); over the tailnet, the
   policy from section 3 blocks it.

Sources for the variables and the pattern (retrieved 2026-10-10):
https://tailscale.com/docs/features/containers/docker/docker-params and the example
https://github.com/tailscale-dev/docker-guide-code-examples (`07-ts-actual-server`) [verified].

Command line without containers (for example a host installation without Compose):

```
tailscale serve --bg --https=443 127.0.0.1:8080
tailscale serve status
tailscale serve --https=443 off
```

Flags according to https://tailscale.com/docs/reference/tailscale-cli/serve (retrieved 2026-10-10)
[verified]; the combination in the first line is composed, not literally sourced
[derived]. Without containers, the server itself must listen only on `127.0.0.1`, and no other
process of the host may be able to send headers to this port.

## 3. Tailnet policy (required)

1. The Bothy device gets a tag (for example `tag:bothy`) so that it is not tied to a person.
2. Grants allow only `tcp:443` on `tag:bothy` for the groups that use Bothy. All other
   ports are forbidden.
3. Reason: a second barrier next to kernel networking. If a tailnet device reaches only 443, every
   request lands at `tailscale serve`, which removes incoming identity headers and sets them itself.
   The portability test checks that nothing except 443 answers.
4. Funnel stays off (`AllowFunnel: false`).

## 4. Identity

| Header | Use |
|---|---|
| `Tailscale-User-Login` | subject in `user_identities` of the provider `trusted_header` |
| `Tailscale-User-Name` | display name on creation, only if `allow_provisioning` |
| `Tailscale-User-Profile-Pic` | ignored (the server loads no foreign URLs) |
| `Tailscale-App-Capabilities` | not evaluated; only `authorize` grants permissions (B7) |

Without `Tailscale-User-Login` (tagged device, Funnel), the request is unauthenticated. Non-ASCII
can be RFC-2047-encoded; the server decodes it before the comparison.

WebSocket via `tailscale serve`: not confirmed in the documentation [assumed]. The
portability test with profile `tailscale` checks the gateway explicitly.

## 5. Operations

| Task | Way |
|---|---|
| Certificate | `tailscaled` renews it itself with `serve`; a failure shows as a drop in the SLO `bothy-sync/reconnect` |
| Device lost, log in again | delete the volume `tailscale-state`, set a new `TS_AUTHKEY`, remove the old device in the tailnet |
| Update | raise `BOTHY_TAILSCALE_VERSION`, `docker compose up -d tailscale` |
| Troubleshooting | `docker compose exec tailscale tailscale status`, `tailscale serve status` |
