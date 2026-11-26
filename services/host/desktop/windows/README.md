# Computer helper for Windows

`bothy-computer-helper.exe` is the only process of a Windows desktop with screen capture and input
rights for computer use ([27-computer-use.md](../../../../docs/spec/27-computer-use.md) section 9.2).
It runs in the desktop session of the person, talks only to the host service over the local channel
of section 9.4, and does what the host service asks: capture the screen, send input, report the
frontmost app, the lock state and the enablement, keep the display awake, show the indicator and
watch for the person's Esc. Rulings: [decisions.d/CU-02.md](../../../../docs/decisions.d/CU-02.md).

## Layout

| Path | Content |
|---|---|
| `include/bothy/helper/` | public headers of the portable core |
| `src/core/` | `helper_core`: JSON, channel codec, key names, geometry, settings files, server rule, Esc decision, `Controller` |
| `src/win32/` | the Win32 implementations of the interfaces, `main.cpp`, the manifest and resource templates |
| `test/` | the test runner `check.hpp` (D-CU-02-2), fakes of every interface, the core tests |

The core is standard C++20 without Windows headers, so it builds and runs its tests with Clang or
GCC on any machine. The Win32 sources are built only on Windows or with a Windows toolchain.

## Interfaces

Every platform call sits behind one interface of `platform.hpp` (brief CU-02 item 13), and the
`Controller` (`controller.hpp`) uses only these:

| Interface | Win32 implementation |
|---|---|
| `Capturer` | DXGI Desktop Duplication of the monitor with the foreground window, GDI `BitBlt` as fallback, WIC encoding in memory (JPEG or PNG) |
| `Injector` | `SendInput` with absolute coordinates over the virtual desktop, `KEYEVENTF_UNICODE` for text, a marker in `dwExtraInfo` |
| `AppInspector` | foreground window, process, executable name, version resource, Authenticode signer (embedded or catalog) |
| `LockMonitor` | `WTSRegisterSessionNotification`, `GUID_CONSOLE_DISPLAY_STATE`, the screen saver, `OpenInputDesktop` for the secure desktop |
| `Waker` | `SetThreadExecutionState(ES_CONTINUOUS \| ES_DISPLAY_REQUIRED)` |
| `EscTap` | `WH_KEYBOARD_LL` hook on its own thread; a real Esc (no `LLKHF_INJECTED`, no marker) is swallowed and ends the session |
| `Indicator` | click-through topmost border in `#FF6A00` and a stop button, both with `WDA_EXCLUDEFROMCAPTURE`; tray icon with the menu "Allow computer control" |
| `EnablementStore` | `%LOCALAPPDATA%\bothy-computer-helper\state.json`, written atomically |
| `HostChannel` | named pipe client of `\\.\pipe\bothy-host-computer` with reconnect and the server check of D-CU-02-5 |

The pipe server must run as LocalSystem or `NT SERVICE\bothy-host` and never as the helper's own
user; a server process whose token cannot be read is refused (`trust.hpp`). A development build
(no verified signature) takes another pipe name from `BOTHY_COMPUTER_HELPER_CHANNEL`, for example
`\\.\pipe\bothy-ci-1234`, and skips that check; a verified build ignores the variable.

A long action runs step by step: between two steps the `Controller` checks the Esc flag that the
keyboard hook raises from its own thread and asks the lock monitor again, so an Esc or a lock stops
it at once and nothing stays pressed. `repeat` is bounded by the default of
`computer.key_repeat_max`, a `type` text by `host_service.computer_frame_max_bytes`.

## Build

With Visual Studio (MSVC) and CMake 3.24 or later:

```sh
cmake -S services/host/desktop/windows -B <build folder> -DHELPER_VERSION=<version>
cmake --build <build folder> --config Release
ctest --test-dir <build folder> -C Release
<build folder>\Release\bothy-computer-helper.exe --version
```

CMake options and cache variables:

| Name | Default | Meaning |
|---|---|---|
| `HELPER_VERSION` | `0.0.0-dev` | the version `--version` prints and `hello` reports |
| `HELPER_CORE_ONLY` | `OFF` | build only the core and its tests, also on Windows |
| `HELPER_UIACCESS` | `ON` | `uiAccess="true"` in the manifest; Windows starts such a program only when it is signed and installed under `Program Files`, so unsigned development and CI builds set it `OFF` |
| `HELPER_BUILD_TESTS` | `ON` | build `helper_core_tests` and register one CTest test per `CHECK_TEST` |

## Tests

The core tests run on every platform:

```sh
cmake -S services/host/desktop/windows -B <temporary folder> -DHELPER_CORE_ONLY=ON
cmake --build <temporary folder>
ctest --test-dir <temporary folder>
```

They read every vector of `services/host/testdata/computer/channel-vectors.json` (path passed by
CMake as `CHANNEL_VECTORS_PATH`) and drive the `Controller` against the fakes of `test/fakes.hpp`:
enablement and generation, show and hide, real and injected Esc (CU-T14), the shortcut table
(CU-T15), release on every end, lock and secure desktop gating, scaling and the `screen` member,
`invalid_action` for unknown names and actions outside a session. Run one test with
`helper_core_tests <name>`, list them with `helper_core_tests --list`. The native behaviour on a
real desktop is checked by the CI jobs of `host-platform.yml`, not here.

## Signing and packages

The helper is never published (D-CU-01-29). On the owner's Windows computer the steps of
`tools/distribution/hostdist.py` (D-CU-02-10) create the self-signed code signing certificate once
(`helper-cert-windows`), sign `bothy-computer-helper.exe` and check it with `signtool verify /pa`
(`helper-sign-windows`), and build the host MSI with the optional feature `ComputerHelper`
(`helper-package-windows`), which installs the helper under `%ProgramFiles%\bothy-host\desktop\`,
writes its start entry at login and sets `host_service.computer_use`. A build without a signature
that verifies reports `signature: development` in `hello`.
