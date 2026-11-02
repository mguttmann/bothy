### Fixed

- Terminal tabs (desktop on macOS and Windows, phone): a tab whose session ended shows one "Close"
  instead of the disabled "Detach" and "End", and closing it leaves no tab behind; "End" ends the
  session on the host and closes its tab; a session the server ended while the app slept ends its
  tab when the list is loaded again.

### Added

- "Sessions" lists the own sessions on a host with their start, "connected" or "detached since"
  and "Reattach". A tab whose stream dropped while it was attached attaches again once when the
  app is online again or back in front; a detach by the person stays. The header counts down to
  the maximum of the owner account mode for a session in it.

### Changed

- The German term for "Reattach" is "Wieder verbinden".
