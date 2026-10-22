# Integration int/m1-wave40

| No | Date | Ruling | Reason |
|---|---|---|---|
| INT-W40/1 | 2026-10-18 | `test/harness/gateway.go`, which M1-72 and M1-70 both added, keeps the `GatewayConn` of M1-72 (fields `Hello` and `Ready`, `Next` returning a `Frame`, own heartbeats, reauth with a refreshed token, `GatewayFrom`) and takes the identify with the API token of an agent from M1-70; the M1-70 callers read `g.Ready` and `g.Hello` as fields. An agent connection answers `reauth_required` with nothing, as M1-70/7 has it, because an agent client holds no refresh token. | Semantic conflict of two independent versions of one harness type. The perf and chaos tests of M1-72 need the frame receive times, the heartbeat counts and the cursor; the leak tests of M1-70 need only `ready`, `Send` and `Close` and the agent identify, so the smallest union is the M1-72 type plus the agent branch, checked by `TestGateway` (person and agent) and `TestGatewayConn`. |
