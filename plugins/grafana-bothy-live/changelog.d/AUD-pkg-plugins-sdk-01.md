### Fixed

- A failed query shows "Query failed" with Grafana's message and a first load shows "Loading",
  instead of the empty text of a quiet period; frames of the queries that did return stay.
- The backend view rounds the extrapolated counts of `increase()` before it leaves out zero
  counts, so no row shows a count of 0.
- The live topology counts a run state outside the catalogue as "other state" in its lane and in
  "Active runs".
- Play at the end of the timeline plays from the start again, as in the apps; series and totals
  are computed once per data change instead of on every tick.
