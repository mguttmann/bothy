# Several api nodes

Status: built in M7-17. Rules: [16-operations.md](../spec/16-operations.md) 4.4 and 4.5,
[02-architecture.md](../spec/02-architecture.md) sections 11.6 and 16 (level 2),
[13-push-notifications.md](../spec/13-push-notifications.md) 12 point 4. The Compose example is
the overlay of [install-compose.md](install-compose.md) 5.3.

## 1. When to run more than one api node

One process in role `all` is the default. Add api nodes when one node reaches
`perf.ws_connections_per_node` or its CPU limit, or when an update must not drop connections
(level 2 of the scaling path). Separate `api` and `worker` (level 1) come first; several workers
need nothing besides more `worker` containers (level 3).

## 2. What every process needs

1. `cluster.mode = multi` on every api node and on the worker. With `single`, nodes do not see
   each other's typing, live box and presence, and a device or agent connected to two nodes is
   not closed.
2. One database and one key store of the signing keys (`secrets.signing_key_dir`, 16 5.2): a
   shared volume, never a copy per node. A key rotated on one node is used by all nodes within
   `signing_key.cache_ttl`; with a store per node, tokens of one node fail on the others.
3. The same file storage (`file.*`) and the same secrets as for one node.

## 3. The edge

1. REST and the gateway (`/api/v1/gateway`) need no sticky sessions. A client that reconnects
   on another node identifies again and catches up from its cursor.
2. Terminal streams need the hash rule of 16 4.4: both stream paths of a session go to the same
   node. `infra/compose/edge/nginx.conf` is the example.
3. Rate limits count per node. With `n` nodes, divide the `ratelimit.*` values by `n`, or let
   the edge keep a client on one node.

## 4. What crosses the nodes

| What | How | What you see when it fails |
|---|---|---|
| Typing, live box, agent presence | `NOTIFY ephemeral` with IDs; the receiving node reads the rest from the database | `ephemeral listener lost` in the log; live indicators pause until the connection is back |
| A second connection of the same device | `NOTIFY ephemeral`; the older connection closes with `4017` | both connections stay open until the next reconnect |
| A second connection of the same agent consumer | the cursor row (`holder_session_id`) and `NOTIFY ephemeral`; the older connection closes with `4015` | the older connection closes at its next save instead of at once; no event is delivered twice |
| Desktop hints and the delay of mobile pushes | `gateway_presence`, written by every api node every `cluster.presence_flush_interval`, read by the worker; the hint goes to the node of the connection through `NOTIFY ephemeral` | `gateway presence not written` and `bothy.gateway.presence.write_failures`; after `cluster.presence_ttl` the desktop counts as offline and the phone gets its push at once |
| Command autocomplete | stays on the node of the request | the request ends with an empty result when the agent has no connection on that node |

## 5. Updates and failures

1. Replace one api node after the other. A stopped node deletes its rows of `gateway_presence`;
   its connections reconnect to the other nodes.
2. A crashed node leaves its rows; the worker ignores them after `cluster.presence_ttl`, and the
   next flush of any node deletes them.
3. After a crash of the database the table `gateway_presence` is empty (`UNLOGGED`); every node
   writes its rows again within one `cluster.presence_flush_interval`.
4. Migrations follow expand and contract (16 section 10), because old and new nodes run side by
   side during the update.

## 6. Not proven

The integration tests run two api nodes and one worker in one test process behind an in-process
edge. Behaviour behind a real load balancer and with more than two api nodes is not tested.
