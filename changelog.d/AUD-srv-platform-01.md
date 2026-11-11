### Security

- Behind a reverse proxy that sets only `X-Forwarded-For`, a client could choose the address that
  the login rate limit and the audit see by sending a `Forwarded` header. The server now counts
  the proxy itself for a request with both headers, and the example edge
  (`infra/compose/edge/nginx.conf`) removes `Forwarded`. If you run your own reverse proxy, let it
  send the address of the client in one of the two headers and remove the other
  (`docs/operations/install-compose.md`, section 5.1). In front of the example edge of the
  multi-node overlay that header is `X-Forwarded-For`: a proxy there that sends only `Forwarded`
  has to send `X-Forwarded-For` from this version on (section 5.3).
