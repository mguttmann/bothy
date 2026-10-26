# Embedding a Grafana dashboard in the console

Status: M6-17. Rules: [25-visualization.md](../spec/25-visualization.md) 9.2 (all conditions),
[19-security.md](../spec/19-security.md) 5 rule 7 and SEC-VZ-2, ADR 0069 item 4, threat T-VZ-01
of the threat model. The embedding is optional and off by default.

The web console can show one Grafana dashboard to owners on the page "Operations (Grafana)". It is
the one place where the console loosens its Content Security Policy, so it needs a Grafana that
sits behind an auth proxy on its own origin. Without `console.grafana_embed_url` nothing changes:
no page, no `frame-src`, the CSP of the console stays as it is.

## 1. What the server does with the key

`console.grafana_embed_url` (`BOTHY_CONSOLE__GRAFANA_EMBED_URL`) is the URL of the dashboard, for
example `https://grafana.example.com/d/ops/operations?orgId=1&kiosk`.

- The CSP of every console response gains `frame-src https://grafana.example.com`: exactly the
  origin of the URL (scheme, host, port; the port 443 is left out), no path, no wildcard, no
  second origin. `frame-ancestors 'none'` and `X-Frame-Options: DENY` of the console stay; the
  console itself is never embedded.
- `getInstance` names the URL as `grafana_embed_url` only to owners (the same filter as
  `telemetry_dashboard_url`). Only then does the console show the page.
- The console embeds the dashboard in an iframe with `sandbox="allow-scripts allow-same-origin"`
  and `referrerpolicy="no-referrer"`, and offers a link that opens it in a new tab
  (`noopener`, `noreferrer`). The console passes no identity and no token to Grafana: the person
  logs in to Grafana through the auth proxy.
- `bothy config check` and the server start refuse a value that is not `https`, that has user
  information or a host that is no DNS name or IP address (no wildcard), or that is on the host
  of `server.public_url`: with
  `allow-scripts allow-same-origin` an iframe of the console's own origin lifts the sandbox, and
  Grafana's scripts would reach the console with its terminal; on another port of the same host
  the browser would send the console's session cookie to Grafana, because cookies are not
  separated by port. Give Grafana its own subdomain.
- `bothy config check` warns when the value is on another site than `server.public_url` (another
  registrable domain by the public suffix list, or another scheme): browsers increasingly block
  third-party cookies in iframes, so the auth proxy's cookie may not be sent [derived]. The
  embedding may still work; the recommended setup is a subdomain of the same registrable domain,
  for example the console on `chat.example.com` and Grafana on `grafana.example.com`.

## 2. What the proxy in front of Grafana must do

The console is cross-origin isolated (SEC-HDR-3), so the browser loads the dashboard only when
every Grafana response carries the headers below (25 9.2 item 3). The proxy, not Grafana, sets
them, because `frame-ancestors` replaces the clickjacking protection that Grafana gives up with
`allow_embedding = true`.

| Header | Value | Why |
|---|---|---|
| `Content-Security-Policy` | `frame-ancestors https://chat.example.com` (the origin of `server.public_url`) | only the console may frame Grafana (T-VZ-01) |
| `Cross-Origin-Resource-Policy` | `cross-origin` | the cross-origin isolated console may load the frame |
| `Cross-Origin-Embedder-Policy` | `require-corp` | COEP of the console applies to its iframes as well |

Further rules:

1. Only owners of the instance get through the proxy. The proxy logs people in with your identity
   provider and admits only the group of the owners; the product does not pass on who is logged in
   to the console.
2. Grafana trusts the user header only from the proxy: `[auth.proxy] whitelist` names the address
   of the proxy, otherwise anyone who reaches Grafana can forge the header (25 9.2 item 2).
3. The proxy overwrites the user header on every request, so a client cannot send its own.
4. With `Cross-Origin-Embedder-Policy: require-corp`, Grafana pages cannot load resources of
   other origins that do not send CORP; turn off what Grafana fetches from elsewhere, for example
   `[security] disable_gravatar = true` and `[news] news_feed_enabled = false` [derived].

## 3. Example

Example only: the product does not ship or run Grafana for this, and the configuration below is not
tested against a real Grafana (the console E2E test uses a static test proxy, see section 5).
Hosts: the console at `https://chat.example.com`, Grafana at `https://grafana.example.com`, an
OAuth2 proxy (`oauth2-proxy`) that admits only the owners' group of the identity provider.

Grafana (`grafana.ini`):

```ini
[server]
root_url = https://grafana.example.com/

[security]
# Grafana stops sending X-Frame-Options: deny; the proxy's frame-ancestors replaces it.
allow_embedding = true
disable_gravatar = true

[news]
news_feed_enabled = false

[auth]
disable_login_form = true

[auth.proxy]
enabled = true
header_name = X-WEBAUTH-USER
header_property = username
auto_sign_up = true
# The address of the reverse proxy below; only it may send X-WEBAUTH-USER.
whitelist = <address of the reverse proxy>
```

Reverse proxy (Caddy) on `grafana.example.com`:

```text
grafana.example.com {
	# Only people the OAuth2 proxy admits (the owners' group) get through.
	forward_auth oauth2-proxy:4180 {
		uri /oauth2/auth
		copy_headers X-Auth-Request-User
	}
	header {
		Content-Security-Policy "frame-ancestors https://chat.example.com"
		Cross-Origin-Resource-Policy cross-origin
		Cross-Origin-Embedder-Policy require-corp
	}
	reverse_proxy grafana:3000 {
		# Overwrites any header of the same name the client sent.
		header_up X-WEBAUTH-USER {http.request.header.X-Auth-Request-User}
	}
}
```

OAuth2 proxy: admit only the owners, for example with `--allowed-group=<owners group>` and a
cookie for the registrable domain (`--cookie-domain=.example.com`), so the cookie is first-party in
the iframe of the console.

Server:

```sh
BOTHY_CONSOLE__GRAFANA_EMBED_URL=https://grafana.example.com/d/ops/operations?orgId=1&kiosk
bothy config check
```

## 4. Check the setup

1. `bothy config check` prints `configuration ok` and no warning about
   `console.grafana_embed_url`.
2. `curl -sI https://chat.example.com/console/ | grep -i content-security-policy` shows
   `frame-src https://grafana.example.com` and `frame-ancestors 'none'`.
3. `curl -sI https://grafana.example.com/` (logged in) shows the three headers of section 2.
4. Log in to the console as an owner: the navigation shows "Operations (Grafana)" and the
   dashboard loads. A person who is no owner sees no such page.

## 5. What is proven and what is not

`make console-e2e` (phase 1b, `tools/console-e2e/README.md`) runs the console of the server
image with the key set against a static test proxy on its own origin that sends the headers of
section 2: the CSP names exactly that origin, the page loads the test dashboard inside the cross-
origin isolated console without a violation, no request carries a referrer, a cookie or an
authorization header to the proxy, a frame of another origin is blocked, a person who is no owner
gets no `grafana_embed_url`, and `bothy config check` refuses the console's own origin.

Not proven: a real Grafana behind a real auth proxy and identity provider, and whether a given
browser sends the proxy's cookie in the iframe; the configuration of the proxy is the
responsibility of the instance operator (T-VZ-01).
