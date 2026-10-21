# Integration int/m1-wave19

| No | Date | Ruling | Reason |
|---|---|---|---|
| INT-W19/1 | 2026-10-15 | M1-56 enters as one squash of `e2f9eb4..e90d1b9` onto the integration base. Append-only files are unions: DECISIONS, the server CHANGELOG, the enabled list of `api/operations.yaml` (`createMessage` and `createThread` enabled, neither in `later`), the route table of `api/routes/real_test.go`, `sqlc.yaml`, the privilege table of the migration tests, the expected configuration of the wire test, the module READMEs and the start step `chat` of `wire`, which also builds messaging. sqlc and `go generate` regenerated the models, the REST and route packages and `config-schema.json`. | The ticket was built on an older base; a union keeps what each side added, and generated files come only from their generators. |
| INT-W19/6 | 2026-10-15 | M1-56's only overlaps are the unions of INT-W19/1 and the migration numbers. The other rulings of the integration (INT-W19/2 to INT-W19/5 and INT-W19/7 to INT-W19/9) resolve the merge with M2-24 and are not ported with this ticket. | Order of the integration; M2-24 is not on main. |
