// SPDX-License-Identifier: Apache-2.0

// Package egress is the one way the server opens outgoing HTTP connections
// (docs/spec/19-security.md "SSRF ban and outgoing connections", SEC-NET-1 to
// SEC-NET-5, SEC-TLS-4, R-SE-04, R-SE-10). The Semgrep rule bothy-go-egress-only makes every
// other client, transport and dialer in production code a gate failure, and
// bothy-go-tls-verify forbids switching off certificate checks.
//
// A consumer (OIDC discovery and JWKS, push, webhooks, object storage, transcription) builds
// its own *Client with New from the configuration egress.* (Config), its options (Options:
// purpose, host policy, redirect mode, own time and size limits) and the dependencies (Deps).
// Every request then passes these rules:
//
//  1. Before any connection, in the transport (so also for HTTPClient): a URL with user info
//     is refused (url_not_allowed); the scheme is https, or http only when a rule of
//     egress.allowed_hosts with allow_private matches host and port (scheme_not_allowed);
//     host and port match a rule or the consumer policy (host_not_allowed). A refused host is
//     never resolved. A rule without port matches only the default port of the scheme.
//  2. At connection time: the host is resolved, every address is unmapped from IPv4-mapped
//     IPv6 and stripped of its zone, and only addresses that pass the check are dialed; the
//     dialer connects to the checked address, never to the name again, and checks the socket
//     address once more in net.Dialer.Control. If no address passes: address_blocked. DNS
//     rebinding does not help, because the check sees the address that is dialed.
//  3. Always blocked: unspecified (0.0.0.0/8, ::), link-local with the metadata address of
//     the clouds (169.254.0.0/16, fe80::/10), multicast, broadcast and reserved
//     (224.0.0.0/4, ff00::/8, 240.0.0.0/4), 192.0.0.0/24, 198.18.0.0/15, fec0::/10, 100::/64,
//     ::/96, NAT64 (64:ff9b::/96, 64:ff9b:1::/48), 6to4 (2002::/16), Teredo (2001::/32) and
//     the metadata addresses of the clouds inside private ranges (fd00:ec2::254 and host
//     .200 of 100.100.100.0/24).
//     Blocked unless a matching rule has allow_private: loopback, RFC 1918, 100.64.0.0/10
//     and fc00::/7. Documentation ranges (RFC 5737, RFC 3849) count as public.
//  4. allow_private exists only in egress.allowed_hosts, which the operator writes; a consumer
//     policy can never open private addresses or plain http, because its list can come from
//     data such as a discovery document.
//  5. Redirects: RedirectNever hands the 3xx response back (webhooks); RedirectChecked follows
//     only hops that pass rules 1 to 3 again, at most 10 (OIDC). A redirect whose Location does
//     not parse ends with an error in both modes, without quoting the header. A 101 answer
//     (protocol switch) is always an error: no consumer upgrades a connection, and no deadline
//     would bound reading the switched connection.
//  6. Environment proxies (HTTP_PROXY, HTTPS_PROXY) are ignored, because a proxy would make the
//     address check see only the proxy. TLS needs at least version 1.2 and always verifies
//     the certificate against Deps.RootCAs (nil: the system roots) and the host name.
//  7. Limits: the whole request, from name resolution to the end of the body and over all
//     redirects, takes at most egress.timeout or Options.Timeout (a library that puts the
//     transport of HTTPClient into its own client gets the limit for each hop); the addresses of
//     one name share the time left, so one that never answers leaves time for the next. A body
//     larger than egress.max_response_bytes or
//     Options.MaxResponseBytes fails with ErrResponseTooLarge instead of being cut short
//     (bodies the transport decodes count decoded bytes), and egress.max_response_bytes also
//     bounds the response header.
//  8. Each refused attempt is logged once at Warn with purpose, reason and host
//     (bothy.egress.purpose, bothy.reason, server.address), never with path, query, headers or
//     bodies, and counted through Deps.Metrics as bothy.egress.blocked with bothy.reason
//     (docs/observability/signals.md). Errors of Client.Do name scheme and host only.
//
// Callers map the errors to their own kinds, usually 503 unavailable with the internal cause
// dependency_unavailable (docs/spec/02-architecture.md 14.1); errors.Is(err, ErrBlocked) tells a
// refusal from a failure of the target. No outgoing call happens inside a database transaction
// (02 8.5).
//
// NewLoopback is the client of `bothy healthcheck`: loopback addresses only, plain http, no
// redirects.
//
// Example of a consumer that reaches the hosts of its identity providers and follows checked
// redirects:
//
//	policy := func(_ context.Context, scheme, host string, port uint16) bool {
//		return scheme == "https" && port == 443 && slices.Contains(providerHosts, host)
//	}
//	client, err := egress.New(cfg, egress.Options{
//		Purpose:   "oidc",
//		Hosts:     policy,
//		Redirects: egress.RedirectChecked,
//	}, egress.Deps{Logger: logger, Metrics: metrics})
//	if err != nil {
//		return err
//	}
//	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
//	if err != nil {
//		return err
//	}
//	resp, err := client.Do(req)
//	if errors.Is(err, egress.ErrBlocked) {
//		// the target is not allowed: a configuration problem, not an outage
//	}
package egress
