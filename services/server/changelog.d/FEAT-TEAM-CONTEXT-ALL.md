### Changed

- Team channels (09 3.9.4): a member's mirrored reply now reaches every other member as a context
  input in its work thread, which starts no run, is no follow-up and is not counted by the brake;
  only the members it mentions are triggered, as before. The next run of any member reads the
  whole conversation in its run context.
- A member added to a team channel gets the channel's history (the newest 200 messages of its
  main thread: the owners' messages and the mirrors of accepted replies) as context inputs.
