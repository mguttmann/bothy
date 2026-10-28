### Fixed

- The desktop shell keeps the last route selection right after the refresh of the providers in
  which the route selection changed, not inside it. Inside it, a sidebar provider that watches
  the selection (the new chat rows of a forum) could rebuild twice in one frame, and debug
  builds failed with `Tried to rebuild newChatRowsProvider(...) multiple times in the same
  frame` (main run 40691988401, `forum_test.dart` on macOS).
