### Fixed

- Expanding or collapsing a space in the sidebar no longer does the same on the other devices
  of the person (01 6.7, R-PR-24; 04 14, R-SY-29). Expanding a space on the phone to open a chat
  collapsed or expanded it on the desktop at the same moment, which looked as if one device
  followed the other. The collapse state of a space is now kept per device in the local table
  `space_view_states` (`local_store` schema version 10, filled from the stored state on the
  upgrade, so nothing changes on screen); the personal order of the spaces still syncs.
