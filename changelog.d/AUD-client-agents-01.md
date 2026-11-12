### Fixed

- The line of a run that ended before a device signed in, or before the device loaded
  everything again, shows again at its place in the thread, with its duration, cost and steps
  and with "Retry" or "Send again". The apps load the runs of a thread when one of them is
  missing on the device.
- `/resume` and `/status` say that the sessions could not be loaded when that read fails,
  instead of "No earlier sessions" or no session at all, and their dialog no longer waits for
  the retries of that read when the instance does not answer.
- `/Stop`, `/New` and the other built-in commands work in any letter case and are no longer
  sent to the agent as a message.
- A refused read of the details of an ended run is no longer repeated many times over by two
  retry layers.
- "Retry" on a run, the cancel of a wake-up and the switch "Let agents continue on their own"
  no longer leave a failed entry behind when the server refuses them after their view was
  closed.
