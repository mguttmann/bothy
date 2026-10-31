### Added

- The test `FakeServer` attaches files to a follow-up (`deliver_followup(..., files=...)`) and
  changes the check of a stored file (`set_file_status`); `getMessage` shows each attachment with
  its state at the time of the read, as the server does.
