# Integration int/m1-wave21

| No | Date | Ruling | Reason |
|---|---|---|---|
| INT-W21/1 | 2026-10-15 | M6-10 and M1-60 enter as one squash each onto `int/m1-wave20`, in this order; M1-60 is the squash of `b380a4e..081ae4d`. The only conflicts were in this file, resolved as a union (the M6-10 and M1-60 sections after the wave 20 section). The base added no migration, so `00036_message_templates.sql` kept its number there; sqlc, `go generate`, `make tokens`, the config reference and the module register had nothing to regenerate or reconcile. No semantic conflict, no fix. On main only the M1-60 part applies: its section above; M6-10 is not on main and is not ported with this ticket. | The tickets were built on `int/m1-wave19` and touch no common file besides the append-only ones; a union keeps what each side added. |
