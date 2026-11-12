### Security

- A replayed thread item of an import can no longer switch the mirror of its thread on. The server
  read the lead from the `channel_origin_key` of the request, so an item that named a channel
  Discord leads, or any channel the mapping does not name, renamed, closed or reopened a thread in a
  channel that Bothy leads, with `import.apply` alone (no step-up). The lead now comes from the
  channel the stored thread lies in and the stored mapping. A posted thread item that names another
  channel than the one its thread lies in is `409 conflict` with `reason` `target_outside_source`
  and is recorded as a denial.

### Changed

- A thread that was moved into another channel in Bothy follows the channel it lies in now: an
  import run no longer renames, closes or reopens it in a channel outside the import or in one that
  Bothy leads. Its new Discord messages are still taken over.
