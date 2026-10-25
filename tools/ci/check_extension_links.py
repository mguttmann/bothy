# SPDX-License-Identifier: Apache-2.0
r"""Check that the app extensions of the Apple builds link no Flutter framework.

ADR 0056 ("Confirmation") and brief M5-04 Design note 7: the Notification Service Extension
(and later the Share Extension) runs without the Flutter engine. This check inspects every
`.appex` inside the given app bundles (`PlugIns/` on iOS, `Contents/PlugIns/` on macOS), reads
its executable from its Info.plist and runs `otool -L` on it; a load command of
`Flutter.framework`, `FlutterMacOS.framework` or `App.framework` (the compiled Dart code)
fails the check. Every app must contain each extension named with `--expect`, so a build that
silently dropped the extension does not pass.

Usage (from apps/client, after the iOS simulator and macOS builds of `make apps-platform`):
    check_extension_links.py --app build/ios/iphonesimulator/Runner.app \
        --app 'build/macos/Build/Products/Debug/*.app' --expect NotificationService

`--app` accepts a glob in its last path segment. Without Xcode (no `xcodebuild`, so no app was
built) or without `otool` the check prints a SKIP line and exits 0, like the other platform tests
of a host without Xcode.

Exit codes: 0 ok or skipped; 1 an extension links Flutter, an app or an expected extension is
missing, or `otool` failed; 2 usage error.
"""

from __future__ import annotations

import argparse
import plistlib
import re
import shutil
import subprocess
import sys
from collections.abc import Callable
from pathlib import Path

# A load command of a Flutter framework or of the compiled Dart code, by framework folder name.
FORBIDDEN = re.compile(r"(?:^|/)(Flutter|FlutterMacOS|App)\.framework/")

Otool = Callable[[Path], str]


def forbidden_links(otool_output: str) -> list[str]:
    """The load commands of `otool -L` output that name a Flutter framework.

    Args:
        otool_output: the text `otool -L <binary>` printed.

    Returns:
        The offending lines, stripped; empty when the binary is clean.
    """
    lines = otool_output.splitlines()[1:]  # the first line names the binary itself
    return [line.strip() for line in lines if FORBIDDEN.search(line.strip().split(" (")[0])]


def extensions(app: Path) -> list[Path]:
    """The `.appex` bundles of an app bundle, iOS or macOS layout.

    Args:
        app: the `.app` bundle.

    Returns:
        The extension bundles, sorted.
    """
    found: list[Path] = []
    for plugins in (app / "PlugIns", app / "Contents" / "PlugIns"):
        if plugins.is_dir():
            found += sorted(plugins.glob("*.appex"))
    return found


def executable(appex: Path) -> Path:
    """The executable of an extension bundle from its Info.plist.

    Args:
        appex: the `.appex` bundle.

    Returns:
        The path of the binary.

    Raises:
        OSError: the Info.plist cannot be read.
        KeyError: it names no `CFBundleExecutable`.
    """
    for info, binaries in (
        (appex / "Info.plist", appex),
        (appex / "Contents" / "Info.plist", appex / "Contents" / "MacOS"),
    ):
        if info.is_file():
            with info.open("rb") as handle:
                name = plistlib.load(handle)["CFBundleExecutable"]
            return binaries / name
    raise OSError(f"{appex} has no Info.plist")


def run_otool(binary: Path) -> str:
    """`otool -L` of a binary.

    Raises:
        subprocess.CalledProcessError: otool failed.
    """
    return subprocess.run(
        ["otool", "-L", str(binary)], capture_output=True, text=True, check=True
    ).stdout


def check(apps: list[Path], expect: list[str], otool: Otool = run_otool) -> list[str]:
    """Check every extension of `apps`.

    Args:
        apps: the app bundles.
        expect: extension names (without `.appex`) every app must contain.
        otool: runs `otool -L`; injected in the tests.

    Returns:
        One message per problem; empty when everything is clean.
    """
    problems: list[str] = []
    if not apps:
        return ["no app bundle found"]
    for app in apps:
        found = extensions(app)
        names = {appex.stem for appex in found}
        problems += [
            f"{app.name}: extension {name}.appex is missing" for name in expect if name not in names
        ]
        for appex in found:
            try:
                output = otool(executable(appex))
            except (OSError, KeyError, subprocess.CalledProcessError) as exc:
                problems.append(f"{app.name}/{appex.name}: cannot inspect ({exc})")
                continue
            problems += [
                f"{app.name}/{appex.name} links {line}" for line in forbidden_links(output)
            ]
    return problems


def expand(pattern: str) -> list[Path]:
    """The paths a pattern with a glob in its last segment matches, sorted."""
    path = Path(pattern)
    return sorted(path.parent.glob(path.name))


def xcode_available() -> bool:
    """Whether Xcode itself is installed: `xcrun --find xcodebuild` succeeds."""
    if shutil.which("xcrun") is None:
        return False
    found = subprocess.run(["xcrun", "--find", "xcodebuild"], capture_output=True, check=False)
    return found.returncode == 0


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    parser = argparse.ArgumentParser(prog="check_extension_links.py")
    parser.add_argument("--app", action="append", default=[], help="app bundle or glob")
    parser.add_argument("--expect", action="append", default=[], help="extension every app has")
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:  # argparse ends on usage errors (2) and on --help (0)
        return exc.code if isinstance(exc.code, int) else 2
    if shutil.which("otool") is None or not xcode_available():
        print("SKIP extension-links: Xcode is not installed on this host")
        return 0
    apps = [p for pattern in args.app for p in expand(pattern)]
    missing = [pattern for pattern in args.app if not expand(pattern)]
    problems = [f"no app bundle matches {pattern}" for pattern in missing] + check(
        apps, args.expect
    )
    for problem in problems:
        print(f"extension-links: {problem}", file=sys.stderr)
    if not problems:
        print(f"extension-links: ok ({len(apps)} apps, no extension links Flutter)")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
