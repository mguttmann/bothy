### Added

- Personal archive of a channel in the sidebar (PROJ-ARCHIVE, 01 6.22, 06 R-API-31):
  `putMySidebarChannel` takes the optional member `hidden` (true archives the channel for the
  caller, false restores it) and `SidebarChannelState` carries `hidden_at` while it is archived,
  in the answer, in `sidebar_channel.updated`, in the snapshot member `sidebar_channels` and in
  `getMySidebar`. Migration 00102 adds `user_channel_states.hidden_at`; the personal data export
  writes it in the section `sidebar_channels`.

### Changed

- `putMySidebarChannel`: `thread_order` is optional; an absent member keeps the stored value, and a
  body without any member is 400 `validation_failed`. A write that changes nothing (the same order,
  archiving an archived channel, restoring a shown one) returns the stored state without a new
  version and without an event.
