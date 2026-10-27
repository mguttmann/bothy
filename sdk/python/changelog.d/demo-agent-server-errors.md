### Fixed

- Demo agent: a `5xx` answer without a problem body (for example a `502` of the TLS proxy while
  the API restarts) no longer stops the demo agents. Every `5xx`, `429` and a request without an
  HTTP answer is waited out; only another `4xx` ends the agent it hit.
- Fake-Agent: a server error the SDK raises (a body that is no problem) is a reply with its
  `status` and `code` like any other refused answer, no longer an `internal` error.
- FakeServer: `Fault.proxy_error(status)` answers a status with a plain text body that is no
  problem, as a proxy in front of a stopped server does.
