### Fixed

- A long job a Hermes agent starts outside Hermes' own background tracking (for example a script
  that detaches its supervisor) can now be shown. The script registers the job with
  `python -m bothy_hermes_adapter job register`; the run then stays running with a background job
  step and its stop button until the job ends, and stop ends the job's process group after
  checking that the process is still the registered one. A registered job whose run already ended
  (for example after a restart) is posted once in its thread, with the command that stops it.
