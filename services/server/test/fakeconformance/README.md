# Package `test/fakeconformance`

Status: built (AUD-pkg-sync-engine-01). Component `testsupport` (M1-05), never imported by
production code.

The server side of the behaviour conformance between the real server and the fake server of the
client tests (`packages/sync_engine/lib/src/testing`). The fake copies rules of the server by
hand; this package and the Dart test `packages/sync_engine/test/fake_server/conformance_test.dart`
read the same scenario files, so a rule cannot differ between the two without a failing test
([ruling AUD-SE1/1](../../../../docs/decisions.d/AUD-pkg-sync-engine-01.md)).

## What it proves

`TestFakeConformance` runs every file of `packages/sync_engine/test/fixtures/conformance/`
against the server of this tree, started through [`test/harness`](../harness/README.md) (the
real wire graph on a disposable PostgreSQL, real HTTP with DPoP):

- It builds the world of the scenarios through the public API: a space with one text channel,
  Alex and Sam with a grant of level `write` on it, and the main thread of the channel.
- It sends the requests of the file in order. A step without `observe` prepares the world and
  must answer `2xx`. For an observed step it records the status, the code and field errors of a
  problem, the members of the body the step names, and every event of the rule's types that the
  request appended, read from `event_log` as the envelope the gateway delivers
  (`sync.Event.Envelope`). IDs of the world appear as placeholders.
- It fails when the file differs from what the server showed, also in its form: the file is
  the output of the test.

The format of a file is described in the README of `packages/sync_engine`, section "Conformance
with the real server".

## Running

```sh
cd services/server
go test -tags contract_validate ./test/fakeconformance/
```

After a deliberate change of the server, of a scenario, or for a new scenario:

```sh
go test -tags contract_validate ./test/fakeconformance/ -update-conformance
```

Then run the test of the fake (`dart test test/fake_server/conformance_test.dart` in
`packages/sync_engine`) and make the fake follow. The test needs a Docker-compatible runtime
for PostgreSQL, like every test on the harness.
