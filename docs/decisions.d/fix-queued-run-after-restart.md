# Fix: a run queued while the adapter was down

| No | Date | Ruling | Reason |
|---|---|---|---|
| FIX-QR/1 | 2026-10-27 | The fix is in the Fake-Agent of the Python SDK (`bothy_sdk.testing.fake_agent`, which the demo agent runs on), not in the server or the SDK adapter: its REST `connect` op now keeps the events of the poll it sends. The conformance scenario AG-10 gets the case `queued while offline` (crash, a run queued while no consumer runs, relaunch, receive and start). | Reproduced against the real server: the first poll of the restarted process delivers the invocation from `acked_seq + 1` as 09 4.3 says, but `connect` discarded the page. The server does not repeat an unacknowledged event within `agent.ack_timeout`, and the acknowledgement of the next settled event covers the dropped one, so the run stayed `queued` until `run.queued_max_age`. The SDK adapter's polling source handles every page and was not affected. |
