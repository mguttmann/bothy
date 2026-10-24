# Fixtures of the cost API

Pages of `GET /v1/organizations/cost_report`, written after the provider documentation cited in
docs/spec/10-backend-policy.md 12.4 (usage and cost API, daily buckets grouped by
`workspace_id` and `description`). They hold no real data: the workspace IDs, the model name
and the amounts are invented.

| File | Content |
|---|---|
| `cost_report_page1.json` | first page: bucket 2026-10-20 with two lines for `wrkspc_01e2ew1`, one for `wrkspc_01e2ew2` and one for `wrkspc_01e2ew3`; `has_more` `true`, `next_page` `page_2` |
| `cost_report_page2.json` | last page: bucket 2026-10-19 with one line for `wrkspc_01e2ew1`; `has_more` `false`, `next_page` `null` |
| `cost_report_missing_amount.json` | a malformed page: a line without `amount` |

The first two pages are also served by the end-to-end test of the server
(`services/server/test/reconcile`), so their content stays as it is.

Fields the app relies on (everything else, such as `description`, `cost_type`, `model` and
`token_type`, is ignored):

| Place | Field | Use |
|---|---|---|
| page | `data` | list of buckets |
| page | `has_more` | `true` while another page follows |
| page | `next_page` | the value of the query parameter `page` for the next page; `null` on the last page |
| bucket | `starting_at`, `ending_at` | RFC 3339 bounds of the daily bucket; they become `period_start` and `period_end` of the submission |
| bucket | `results` | list of cost lines |
| line | `amount` | decimal string in cents (the lowest unit of the currency) |
| line | `currency` | ISO 4217 code, `USD` |
| line | `workspace_id` | workspace of the line; `null` for the default workspace, which is never in the map |
