### Fixed

- Tests: the phone integration tests open a thread through one helper (`openThread`) that taps
  the forum and the thread row only once each stands still on the screen. The post list slides
  in on iOS, and a tap during the slide missed the row (`apps (iOS)`, voice test "a recording
  over a low limit stops by itself with the notice").
