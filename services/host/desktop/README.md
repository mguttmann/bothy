# Computer helpers

"Bothy Computer Helper": the only process of a computer session that captures the screen and
injects input, one per platform, running in the owner's desktop session
([27-computer-use.md](../../../docs/spec/27-computer-use.md) section 9, brief
[CU-02](../../../docs/briefs/CU-02-computer-helpers.md), rulings
[CU-02](../../../docs/decisions.d/CU-02.md)). The host service `bothy-host` drives it over the local
channel of 27 9.4; the helper itself never talks to the server.

| Folder | Content |
|---|---|
| [`macos/`](macos/README.md) | Swift package: `HelperCore` (logic behind interfaces), `HelperMac` (ScreenCaptureKit, `CGEvent`, event tap, indicator), the app bundle `bothy-computer-helper.app` |
| [`windows/`](windows/README.md) | CMake project (C++20): `helper_core` (portable logic), the Win32 layer (DXGI, `SendInput`, keyboard hook, indicator, tray) and `bothy-computer-helper.exe` with `uiAccess` |

Both share the logic of 27: the enablement switch with its generation (a click at the machine is
the only way on), `show` and `hide` with the indicator, the Esc tap and the display assertion, a
release of every key and button on every end and at start, `screen_locked` and `secure_desktop`,
capture scaled into `max_edge`, and the key names of D-CU-02-7. No app, key or setting is refused.

## Tests

```sh
(cd services/host/desktop/macos && swift test)
cmake -S services/host/desktop/windows -B <temporary folder> -DHELPER_CORE_ONLY=ON
cmake --build <temporary folder> && ctest --test-dir <temporary folder>
```

The native behaviour (capture, input, the indicator outside the image, an injected Esc that stops
nothing) runs only in CI: the jobs `helper-macos` and `helper-windows` of
`.github/workflows/host-platform.yml` probe the hosted runner with `tools/ci/computer_probe.py`
and a fixture window, and the native jobs test it where the probe passed (27 section 16). Never
run the helpers against a real desktop of a maintainer for a test.

## Signing and packages (not published)

The helpers are never published and `release.yml` neither builds nor signs them (D-CU-01-29). The
owner signs and packages them on the machines they run on:

macOS, on the owner's Mac, with the owner's Developer ID Application identity:

```sh
make -C services/host helper-macos OUT=<folder>
make -C services/host helper-sign-macos APP="<folder>/bothy-computer-helper.app" \
  SIGN_IDENTITY="Developer ID Application: <name> (<team>)" REQUIREMENT_FILE=<file kept between builds>
make -C services/host helper-package-macos APP="<folder>/bothy-computer-helper.app" OUT=<folder>
sudo installer -pkg <folder>/bothy-host-computer_<version>_darwin_<arch>.pkg -target /
```

The signing key lives in the login keychain, which is open only in the owner's logged-in GUI
session: when the build runs over ssh, start `helper-sign-macos` as a one-shot job in that session
(a LaunchAgent property list with the command, loaded with `launchctl bootstrap gui/<uid>`), never
by unlocking the keychain. The step refuses a build whose designated requirement differs from the
one in `REQUIREMENT_FILE`, because screen recording and accessibility grants follow that
requirement. No notarization: a package built and installed on the same Mac carries no quarantine
flag. In Installer, select "Computer helper"; then grant screen recording and accessibility to the
helper in the system settings (its menu opens the panes) and switch "Allow computer control" on in
its menu bar item.

Windows, on the owner's Windows computer, elevated:

```sh
uv run --project tools --locked python tools/distribution/hostdist.py helper-cert-windows
uv run --project tools --locked python tools/distribution/hostdist.py helper-sign-windows --exe <bothy-computer-helper.exe> --thumbprint <thumbprint>
uv run --project tools --locked python tools/distribution/hostdist.py helper-package-windows --version <x.y.z> --out <folder> --helper <bothy-computer-helper.exe> --thumbprint <thumbprint>
msiexec /i <folder>\bothy-host-computer_<version>_windows_amd64.msi ADDLOCAL=ComputerHelper
```

`helper-cert-windows` runs once: it creates a self-signed code signing certificate whose key cannot
be exported and which only this machine trusts (`LocalMachine\Root` and `TrustedPublisher`).
`uiAccess` needs that trust and the folder under `Program Files`. The helper starts at the next
login; switch "Allow computer control" on in its tray menu.

Selecting the helper component sets `host_service.computer_use: true` in the host service's
configuration file (`infra/host/set-computer-use.sh`, `infra/host/windows/set-computer-use.ps1`);
`bothy-host restart` makes the host service read it. A build without a signature that verifies
reports `signature: development`.
