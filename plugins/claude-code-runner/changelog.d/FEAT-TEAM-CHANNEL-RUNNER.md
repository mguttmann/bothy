### Added

- Team channels: the runner starts runs with the trigger kind `team`. The run's first message
  tells the session the team channel, its own handle, every member, who wrote the input and
  whether it mentions the member, and how to hand work to another member (`@handle` in the
  final answer). A member's input is labelled as that member's, a colleague's request that is
  never the owner's word, in the first message and as a follow-up (`Input from team member
  @<handle> ...`); it never gets an owner code. The owner's follow-up into the owner's own team
  run gets an owner code as before, and a follow-up whose text names a code the runner wrote is
  not passed on (`followup_carries_owner_code`). Answers keep going out with the invocation
  token of exactly their run.
