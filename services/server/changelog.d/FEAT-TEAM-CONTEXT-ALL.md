### Changed

- Team channels (09 3.9.4): a member's mirrored reply now reaches every other member as a context
  input in its work thread, which starts no run, is no follow-up and is not counted by the brake;
  only the members it mentions are triggered, as before. The next run of any member reads the
  whole conversation in its run context.
- A member added to a team channel gets the channel's history (up to the fixed team history cap,
  the newest owners' messages and mirrors of accepted replies of its main thread) as context
  inputs.
