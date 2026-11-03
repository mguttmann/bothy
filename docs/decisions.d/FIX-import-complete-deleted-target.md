# FIX-import-complete-deleted-target A deleted or archived target channel does not block completeImport

| No | Date | Ruling | Reason |
|---|---|---|---|
| FICD/1 | 2026-11-03 | `completeImport` skips a mapped channel (target not `skip`, channel item `posted`, lead `discord`) whose channel here is not live, as `ImportStructure.LiveChannel` reports it: deleted, archived, in an archived space, or gone after the purge. A live channel that the source still leads keeps `409 conflict` with `reason` `lead_not_bothy`. This narrows the completion rule of D-M8-03-3 and the console rule of D-M8-08-4. The channel keeps its read-only flag; after a restore the owner lifts it with `setChannelReadOnly`, which is open once the source is `completed`. | `setImportLead` refuses such a channel with `404` (`SetReadOnly` refuses an archived channel, and a deletion archives it), so its lead could never become `bothy` and the source stayed `running` or `paused` for good. `LiveChannel` is the same liveness test that the mapping validation of `createImport` uses, so the two rules agree. |
| FICD/2 | 2026-11-03 | The console (`ImportSourceDetails.allImportedLedHere`) applies the same rule: a row whose channel reads with `archived_at` or `deleted_at`, or whose `getChannel` answers `404`, does not block offering completion. Another failure of `getChannel` still blocks it, and the server decides in the end. | The console offered completion only when every imported channel was led here, so it would have hidden the button for the same channel. A channel in an archived space reads as live through `getChannel`; for it the console keeps the button hidden (backlog entry below), which errs on the safe side. |

## Backlog (minors)

- The console shows no hint that an imported channel was deleted or archived here, still offers
  "lead here" for it (the server answers `404`), and keeps the complete button hidden for a live
  channel in an archived space. A text and a hidden action need new strings in both languages.
