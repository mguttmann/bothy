### Security

- The client IP of a request from a trusted proxy no longer comes from a forwarding header that
  the proxy only passed through. The server read `Forwarded` before `X-Forwarded-For`, and a proxy
  that sets only `X-Forwarded-For` (nginx, also the example edge) passes a `Forwarded` of the
  client on. An anonymous client so chose the address that `rl.public_auth` limits and the audit
  records, and could empty the bucket of another address. Now the server reads the one forwarding
  header a request carries and counts the proxy itself for a request with both. A proxy in
  `server.trusted_proxies` sends exactly one of the two headers and removes the other; behind a
  proxy that sends both, all clients share the limit of the proxy until it sends one
  (`docs/spec/16-operations.md`, section 4.3).
