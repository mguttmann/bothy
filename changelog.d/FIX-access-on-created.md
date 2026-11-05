### Fixed

- Client: a space or channel that a device first learned through `space.created` or
  `channel.created`, such as a project forum created after the last full snapshot, now gets the
  person's level and capabilities. The engine loads its view through REST after the event, and on
  the next connection for rows stored earlier without access, so the owner sees "Rename project"
  and "Delete project..." in the sidebar menu again without a resync.
