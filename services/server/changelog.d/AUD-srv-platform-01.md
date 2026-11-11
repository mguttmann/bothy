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
- A `Forwarded` header of a client no longer hides the element that a proxy appends to it. The
  server ended a quoted string at an escaped quote (`\"`) and accepted a line that ends inside a
  quoted string, so behind a proxy that sets `Forwarded` it read the address the client had sent
  instead of the one the proxy added. Quoted strings are now read as RFC 7230 defines them, and a
  line that ends inside a quoted string is a malformed entry, at which the walk stops.
