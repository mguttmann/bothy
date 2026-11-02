### Fixed

- A deleted project forum no longer blocks its folder: its binding stays during the undo window,
  so a restore gets it back, and is released once the deletion is final (at the purge, or with
  the runner's outcome of a requested folder move). The folder can then be bound to a new forum.
  The job `messaging.purge_deleted` also releases bindings of forums purged before this fix on
  its first run. Migrations `00090` (column `project_bindings.released_at`) and `00091` (the
  unique index on `lower(workspace_ref)` leaves released bindings out).
- `deleteProjectBinding` works on an archived or deleted forum, so an owner can always give a
  folder free.
