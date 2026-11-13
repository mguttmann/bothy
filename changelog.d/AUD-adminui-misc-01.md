### Added

- "Create access grant" of the administration (desktop and web console) offers the installed apps
  next to people, groups and agents. A second forum for an app, a target left out when the app was
  installed and a target after an update are created there; until now that took a request to the
  API. The list of access grants names the grant of an app by the name of the app instead of its
  ID.

### Fixed

- An export of the audit log that fails before it begins, because the confirmation was cancelled,
  the server refused it or there was no connection, no longer leaves a file: until now the web
  console saved an empty file or a CSV with only its header under the regular name. The export
  says the reason; a CSV export said "The export broke off" for every failure.
- Where the administration lets you pick a person ("Add owner", "Add member", the grant wizard,
  "View as", the person filter of the audit log), the handle stands next to the display name, and
  the confirmation with passkey or password names it too. Two people with the same display name
  could not be told apart before. The grant wizard finds a person by the handle as well.
- An error of the app itself in a screen of the administration no longer reads "Something went
  wrong on the server". It reads "Something went wrong in the app" and reaches the error handling
  of the app, so a build with crash reports reports it. A server error shows its correlation ID,
  which finds the request in the server's log.
- The page of an import source in the web console no longer reads every imported message, file
  and reaction each time it refreshes. It reads the failed items, and the imported items only
  until it has the channels of the mapping, which an import writes first; while a channel of the
  mapping is not imported yet it still reads all imported items. A refresh that is asked for while
  one runs waits for it instead of reading at the same time.
