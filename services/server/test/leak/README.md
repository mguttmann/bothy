# Package `test/leak`

Status: built (M1-70). Component `testsupport` (M1-05), never imported by production code. The
tests behind threat T-AZ-02 (`docs/security/threat-model.md`): no reading path of the server core
reveals a foreign channel, thread, message, space or agent.

## Tests

| Test | Rule | What it proves |
|---|---|---|
| `TestAZ_P4_RouteCoverage` | AZ-P4 | every enabled GET operation with a uuid path parameter has a fixture per parameter; the push routes `unregisterPush` (Sam against a registration of Alex) and `getPushPreview` (Sam's valid preview token for a message of Engineering/Werkzeuge) give the same normalized `404 not_found` as a random ID (M3-22); prints the table of routes, requesters (Alex, Lotse, Sam for the push routes) and outcome; `ok=false` needs a reason from 05 |
| `TestAZ_P4_NotFoundEquality` | AZ-P4, R-AZ-01 | subtests `AZ-P4/<operationId>/<requester>`: a foreign and a fresh random UUIDv7 give the same normalized status, headers, body and length; without read on a content resource the status is `404 not_found` |
| `TestAZ_P4_TimingMedian` | AZ-P4 | same subtests: after a warm-up, interleaved samples of both IDs have medians within `authz.not_found_timing_tolerance` (read from `docs/operations/configuration.md`) |
| `TestAZ_M52_ForeignMessageEqualsUnknown` | AZ-M52 | Alex reading a message of Marketing/Kampagnen gets the answer of a random ID |
| `TestAZ_P5_ListsAndCountersMatchAuthorize` | AZ-P5 property 1 | on random worlds, for a person and an agent: every ID a surface names is readable by the per-item truth, events have a visible scope or `audience` self, unread and mention counts equal the model, no totals |
| `TestAZ_P5_NoiseInvariance` | AZ-P5 property 2 | threads, messages, mentions, reactions and pins in unreadable channels leave every surface equal (without `head_seq`, `next_cursor`, cursor frames; for the search the presence of `next_cursor` and `candidates_capped` stay compared, with `search.max_candidates` set low) |

Unit tests: `TestNormalize*`, `TestSelectRoutes*`, `TestToleranceFromReference`,
`TestWorldPlanReproducible`.

## Files

- `routes.go`: `SelectRoutes` reads `api/operations.yaml` (documented format) and the contract
  (`rest.Spec`, byte-equal to `docs/api/openapi.yaml`).
- `fixtures.go`: `ParamFixture` per path parameter name, `PrepareExtras` (a login driver, a
  host, an app and a webhook subscription of the app, created by the owner, and an upload of Sam
  in Marketing/Kampagnen for `file_id` of `getFile` and `getUpload`, M3-03; a later ticket that
  enables a route with a new path parameter adds its fixture, M1-70/9, M1-70/10, M1-70/11).
- `push_test.go`: the push routes of M3-22, which `SelectRoutes` does not select (a DELETE and a
  query parameter): the P4 server enables `apns` with a Fake-APNs and a generated key (role all
  builds the adapter; `EgressDenyAll` still refuses every outgoing request), Alex and Sam register
  through `registerPush`, and the preview request carries `Authorization: DPoP <preview token>`
  with a proof of Sam's device key whose `ath` binds the token.
- `normalize.go`: drops `Correlation-Id`, `Date`, `DPoP-Nonce`, keeps the policy name of
  `RateLimit`, drops `correlation_id` and replaces the requested IDs in `instance` of a problem
  body; `Diff` lists every remaining difference.
- `world.go`: `NewPlan(seed)` (pure, reproducible), `BuildWorld` through the public API, sizes and
  `WorldsPerRun` as named constants.

## Running

```sh
cd services/server
go test -race -count=1 ./test/leak/... ./test/harness/...
LEAK_SEED=<printed seed> go test -count=1 -run TestAZ_P5 ./test/leak/   # replay a seed
```

Every AZ-P5 run logs its seeds (`AZ-P5 seed <n> (replay: LEAK_SEED=<n>)`); with `LEAK_SEED` the
run uses that seed and the following ones.

## Not proven here

- Timing on hosts other than the build host; the medians hold on the host that ran the test.
- Facets, suggestions and "Running now" (AZ-P5 joins them with M2 and M3). The search (M3-14) is a
  surface of both properties for messages and thread titles; the random worlds hold no attached
  files, so the type `files` is collected but always empty there (files are covered by
  `test/search`).
- Routes with a `user_id` for a person as requester: every person is readable for a person
  (05 `user.read`), so there is no foreign instance; the agent requester covers them.
- Mentions of the observed agent and thread-level operators in lists (07 7.2 "Limit").
