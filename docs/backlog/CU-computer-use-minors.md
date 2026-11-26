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
- FIX-windows-data-dir (review of 3825c8bd7, 26.11.):
  - A failed copy of the old data folder (`legacy_data_dir::Migrate` returns `kFailed`) is not
    logged; path_provider then creates the empty new folder and the copy is never tried again.
  - `SanitizedDirectoryName` trims ASCII white space only; Dart's `trimRight` in path_provider also
    trims Unicode white space (only matters for a brand name ending in such a character).
  - `BRANDING_APP_NAME_W` with non-ASCII characters depends on MSVC's source character set (no
    `/utf-8`), while `Runner.rc` reads its narrow name as UTF-8; the two folder names could differ.
- (26.11., review b4151427e) `start-computer-helper.sh`: no test for the exit 1 path (bootstrap keeps failing, service gone); the `kickstart -k` fallback keeps the loaded plist definition.
- (26.11., review b4151427e) macOS permission item: during a running action the pane opens only after the action; `CGRequestScreenCaptureAccess` and the pane can show two dialogs at once.
- (26.11., live) Windows tray icon of the helper is the generic application icon and lands in the overflow ("^"); the owner did not find the switch "Allow computer control". An own icon would make it findable.
