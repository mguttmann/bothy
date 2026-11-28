### Changed

- The video dialog no longer starts playing by itself; it has Play and Pause, Mute, a position
  slider and the position in words.

### Fixed

- Approval cards lock each button by its own permission: a run starter can deny, and only an
  owner can choose "Always allow".
- A video that stops because its playback link expired goes on where it was.
- "Play" after "Pause" on an audio attachment continues instead of starting again.
- Removing an attachment or uploading it again cleans up the upload on the server.
- Attaching several files is all or none, and a drop and a paste at the same time stay within
  the attachment limit.
- Typing no longer rebuilds the chat history and the sidebar on every key press.
- On iPhone, a share survives when the system ends the share sheet during the upload, and
  copies left by an ended share sheet are cleaned up.
