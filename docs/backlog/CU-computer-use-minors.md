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
- (26.11., review of the duplicate-helper fix) `start-computer-helper.sh` matches `ps` `comm`, which is argv[0], not the real executable: a helper started by hand with a relative path or through a symlink is not ended (the `open` and launchd starts are). The comment, README and spec say "exact path".
- (26.11., review of the duplicate-helper fix) The fake `ps` and `_alive` in `test_hostdist_helper.py` use `kill -0`, which also succeeds for a zombie not yet reaped by launchd; a theoretical flake under heavy load (not seen in 9 runs).
- (27.11., red team of the CU branch) `actions.go` `input` with quick discards the result of
  `s.sleep(min(settle, quickPause))`: when the session ends within those 50 ms, the item is still
  reported ok.
- (27.11., red team of the CU branch) `console_windows.go` `sessionDataOwner`: the fallback through
  `LoggedOnUserSID` is not checked against a stale subkey of a reused session id; a Windows test
  would show it.
- (27.11., review of the helper job fix) `host-platform.yml`, Windows helper job: the helper the
  install started is found by name only; a check of its path (`$started.Path`) would log whether
  it runs from Program Files.
- (27.11., review of the server suite fixes) `services/server/test/canary/scenario_terminal.go`: the
  canary step of computer use drives only refusal paths; no computer session opens or carries data
  in the canary run (the content proof stays with CU-T18). A canary computer session needs a fake
  desktop helper.
- (27.11., same review) The comment of `fakehost.DialStream` names the M1-67 rule as "dials out of
  _test.go files"; the rule is about gateway dials through the harness, and these streams are no
  gateway streams.
- (27.11., review of the probe pipe fix) `tools/ci/computer_probe.py` `PipeEndpoint`: the helper
  starts before `ConnectNamedPipe`; a helper that connects first makes Windows answer
  `ERROR_PIPE_CONNECTED` (535), which the probe reports as "the helper did not connect". Treat
  winerror 535 as connected (not reproduced).
- (27.11., review of the Sonar fix round) `services/host/cmd/bothy-host/computer_test.go`: the
  doctor check with computer use on stats the real helper path of the test machine; with an
  installed helper it would run `TrustedPath` with a nil resolver.
- (27.11., same review) `services/host/internal/computer/actions_test.go`: the zoom of an unknown
  scale ID asserts only that it is not ok, not its code.
- (27.11., same review) `plugins/claude-code-runner/tests/unit/test_computer.py`
  `test_a_request_link_is_not_followed`: the fixed wait before the "not answered" check can only
  let the test pass wrongly on a slow machine, never fail it.
- (28.11., red team of FIX-CU-BLACK-CAPTURE) The bound of 8 Desktop Duplication frames
  (`kMaxDuplicationFrames`) is a fixed value in code that D-CU-02-19 does not list yet.
- (28.11., red team of FIX-CU-BLACK-CAPTURE) Only the core loop has a test; that the frame lambda in
  `capturer.cpp` releases a pointer-only frame and its resource is proven on a real desktop only.
- (28.11., red team of FIX-CU-BLACK-CAPTURE) A driver whose first frame has `LastPresentTime` 0 but a
  valid image would cost one 500 ms wait on a still desktop and end in GDI (not seen).
- (28.11., red team of FIX-CU-BLACK-CAPTURE) The comment in `duplication.hpp` mentions four monitors;
  whether the monitor count matters is unproven, only the pointer-only first frame is.
- (28.11., review of FIX-CU-HELPER-NO-ICON) The settings window opens at every login for every
  account of the machine that never switched on (machine Run key, `/Library/LaunchAgents`), not
  only for the owner; on a shared computer the other persons see it each time.
- (28.11., same review) The reopen path on macOS (Finder or Spotlight against the helper launchd
  started) is shown in CI only for a helper started with `open -n`; one manual try on the owner's
  Mac would show it for the LaunchAgent case.
- (28.11., same review) `doctor` prints "open Computer Helper" also when the helper is not installed.
- (28.11., same review) At a start by the installer `SetForegroundWindow` and `NSApp.activate()`
  may leave the settings window behind others.
- (28.11., same review) `visible_windows_windows` in `tools/ci/computer_probe.py` calls `user32`
  without `argtypes`; an HWND above 2^31 would raise.
- (28.11., red team of FIX-CU-HELPER-NO-ICON) macOS: while a helper started by hand runs, the LaunchAgent's own start
  ends at once and launchd restarts it about every 10 s (silent, `KeepAlive`).
- (28.11., same red team) That Spotlight finds the installed bundle is seen on one Mac
  (`mdfind` lists it), not as a search in the Spotlight window.
- (28.11., same red team) Spec 27 section 15 has no CU-T row for R-CU-13; the tests exist (Swift,
  C++ core, CI probe window checks).
- (28.11., delta red team of FIX-CU-HELPER-NO-ICON) That the injection marker reaches the switch's
  click handler end to end (`eventSourceUserData` in `NSApp.currentEvent`, `GetMessageExtraInfo()`)
  is proven by reading and the core tests only; no CI check clicks the switch with injected input.
- (28.11., same delta red team) Windows settings window: Enter and Esc send `IDOK` and `IDCANCEL`,
  which `settings_proc` does not handle, so they do nothing (the comment in `win32.hpp` names them).
