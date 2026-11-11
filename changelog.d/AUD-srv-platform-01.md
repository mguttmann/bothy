### Security

- Behind a reverse proxy that sets only `X-Forwarded-For`, a client could choose the address that
  the login rate limit and the audit see by sending a `Forwarded` header. The server now counts
  the proxy itself for a request with both headers, and the example edge
  (`infra/compose/edge/nginx.conf`) removes `Forwarded`. If you run your own reverse proxy, let it
  remove `Forwarded` too (`docs/operations/install-compose.md`, section 5.1).
