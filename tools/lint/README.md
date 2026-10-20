# tools/lint

Repository linters that hold a rule of the specification over source files. Gate: `make
sql-visibility`, part of `make gates`.

## `sql_visibility.py` (AZ-P12)

07 "Query pattern" rule 1: every query over content, threads, channels, spaces, runs, files or
mentions carries `channel_id = ANY($visible)` or `space_id = ANY($visible_spaces)`, unless it is on
a reviewed exception list. The SQL filter is the performance, `authorize` the truth (rule 3); the
lint makes sure that no new reading query forgets the filter.

What it reads:

- every file `services/server/internal/<module>/adapters/postgres/queries/*.sql`; each block that
  starts with a sqlc header `-- name: <Name> :<cmd>` is one query `<module>.<Name>`;
- the configuration [`sql_visibility.yaml`](sql_visibility.yaml): `data_model` (the chapter whose
  table headings name every table), `content_tables` (final state, also tables of later
  milestones) and `exceptions`.

Rules (comments and string literals never count):

| Rule id | Finding |
|---|---|
| `missing-predicate` | a read (first keyword `SELECT` or `WITH`, no `INSERT`, `UPDATE` or `DELETE` at the top level; `FOR UPDATE` is a row lock of a read) names a content table after `FROM` or `JOIN` and has no predicate `(alias.)?(channel_id\|space_id) = ANY(<param>)`; `<param>` is `$n`, `@name`, `sqlc.arg(name)` or `sqlc.narg(name)` with an optional cast, never a subquery; on `spaces` and `channels` their key `id` counts as the column (`id = ANY(@visible)`) |
| `stale-exception` | an exception names a query that no longer exists |
| `unknown-category` | the category is not one of `authorize_resolution` (resolves a resource path for `authorize`), `after_authorize` (loads a single resource, or the rows of one resource, after `authorize` allowed it), `system_job` (job without principal), `log_tail` (the log tail reads every event; delivery checks `authorize` per receiver), `thread_scope` (a list over a thread-scoped table without `channel_id`, such as `runs`: reads the active runs, resolves their threads through the `chat` port, keeps only threads whose channel is in `Scope.ChannelIDs` and authorizes each row) |
| `duplicate-exception` | a query stands twice on the list |
| `exception-reason` | the reason is empty |
| `unknown-content-table` | a content table is no table heading of the data model chapter |

Output: one line per finding `file:line: <module>.<Name>: <rule>`. Exit codes: 0 clean, 1 findings,
2 usage error (unreadable or malformed configuration, missing chapter).

```sh
python tools/lint/sql_visibility.py [--repo DIR]
make sql-visibility
```

### Adding an exception

An exception says why the query cannot carry the predicate, never only that it does not. A list
over several channels or spaces always can: pass the visible lists of `Authorizer.Scope` to the
query, as `ListVisibleSpaces`, `ListVisibleChannels` and `ListSidebarStates` do. Single-resource
reads, locks and the rows of one resource `authorize` allowed (`after_authorize`), the structure and
invariant facts `authz` resolves (`authorize_resolution`), jobs, the log tail and the lists over
thread-scoped tables without `channel_id` (`thread_scope`, docs/DECISIONS.md D-M2-05-9) are the
cases the list holds (docs/DECISIONS.md M1-70).

### Limits

The lint checks presence, not meaning: a predicate on a parameter that is not the visible set of
`Scope` passes. `authorize` per element (07 7.4 rule 3) and the leak tests of
[`services/server/test/leak`](../../services/server/test/leak/README.md) (AZ-P4, AZ-P5) hold the
meaning.
