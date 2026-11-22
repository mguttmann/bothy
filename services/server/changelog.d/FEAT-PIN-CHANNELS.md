### Added

- Personal pins in the sidebar (FEAT-PIN-CHANNELS, 01 6.24, 06 R-API-31): `putMySidebarChannel`
  takes the optional members `pinned` (true pins the channel to the top of its space for the
  caller, false unpins it) and `pinned_threads` (the pinned posts of a forum, in pin order, under
  the list rule of `thread_order`); `SidebarChannelState` carries `pinned_at` while the channel is
  pinned and `pinned_threads`, in the answer, in `sidebar_channel.updated`, in the snapshot member
  `sidebar_channels` and in `getMySidebar`. Migration 00111 adds `user_channel_states.pinned_at`
  and `pinned_threads`; the personal data export writes both in the section `sidebar_channels`.
