### Changed

- The frame `error` of `command.autocomplete_result` names the first field error as
  `<pointer>: <code>` in `detail`, the form of `run.update`; before, the two were separated by a
  space only. 04 section 6.2 now lists every form of `detail`.

### Fixed

- `notification.created` carries its preview fields (`space_name`, `channel_name`,
  `thread_title`, `sender_display_name`, `excerpt`, `attachment_count`) again when the hint
  crosses processes: with separate `api` and `worker`, and with several api nodes, the signal
  between the processes carries IDs only, and the node that holds the recipient's connection now
  builds the fields of the recipient's `notify.preview_mode` before its hub checks `push.deliver`.
  Before, every such hint arrived with its IDs alone. The fields are built only of a message that
  lies in the thread the hint names; a hint without a thread, or with the thread of another
  message, keeps its IDs alone.
- The gateway op `focus` honours `background_thread_ids`: the frame codec dropped the member, so
  a thread named only there got neither the run document nor `run.updated`.

### Security

- A failed check of the credentials of the gateway ops `identify`, `resume` and `reauth` now
  counts against `rl.public_auth` of the client address, like a failed one of a REST request.
  While that bucket is empty, `identify` and `resume` close with `4008` and
  `retry_after=<seconds>` before any check, and `reauth` answers `error` with `rate_limited`.
  Valid credentials cost nothing.
- The upgrade of `GET /api/v1/gateway` no longer reads the database for the sync epoch of `hello`
  once the process has read it.
