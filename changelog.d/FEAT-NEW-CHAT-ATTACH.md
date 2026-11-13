### Added

- A new chat takes files before it exists. In the new chat view a person pastes a screenshot,
  picks files with the attachment button or drops them on the composer (macOS and Windows), or
  takes them from the camera, the photo library or the files (iOS and Android), with or without
  text, and sends. The first message of the new chat carries the files, so the agent of the forum
  gets them with the message that starts its run, as in an existing chat. The limits and checks
  are those of an existing chat.

### Changed

- The files of a new chat wait on the device until the send: removing one, leaving the draft or
  quitting the app before the send leaves nothing in the outbox or on the server. The send stores
  the uploads and the post in one step, and the post is created after its uploads, also when the
  device was offline.

### Fixed

- A new post whose upload failed for good no longer reads "Sending" for ever: it shows as not
  sent with the reason and its files, "Try again" sends the upload again, and "Discard" removes
  the post with its uploads and deletes what they had created on the server.
