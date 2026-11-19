### Fixed

- The real-CLI contract test of the stream field table no longer fails a tested newer Claude
  Code version for members it adds: such members are named in a warning, while a lost or
  renamed recorded member, a changed status or command state value and a changed workflow type
  still fail. At the pin the table stays exactly equal to the recording, which now names the
  members only the recorded turn wrote (`written_only`).
