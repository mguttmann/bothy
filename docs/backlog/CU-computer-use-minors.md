# Open minors of computer use (CU)

Collected minors without a reproduced harm; the next run in this area reads and takes them along.

- Review of the helper connection fixes (2026-11-26):
  - `services/host/internal/computer/requirementformat_test.go` checks the source text only; a
    Windows test with a real certificate would check `signerSubjectIs` itself.
  - The ACL of `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\LogonUI\SessionData`
    (users read, administrators and SYSTEM write, seen on a Windows 11 test machine) has no test.
  - The MSI action `StartComputerHelper` has no rollback action: after a later rollback the started
    helper keeps running.
  - On an MSI upgrade a running helper is not stopped before its file is replaced; the start action
    then meets the old helper's single-instance lock (Restart Manager decides).
  - `note()` in `pipe_channel.cpp` looks up the folder on every line (only at channel changes).
  - No Windows test runs the peer rule across accounts (service account against a person's process);
    the three cross-account defects of 26.11. were found live only.
