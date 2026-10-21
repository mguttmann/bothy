# SPDX-License-Identifier: Apache-2.0
"""Install the emulator packages of an Android CI job with retries (jobs apps-android-*).

The emulator action downloads the emulator and the system image itself and gives up on the
first broken download ("Error on ZipFile unknown archive", then "could not connect to TCP port
5554"). This step installs the same packages before the action with `sdkmanager`, checks that the
files are there, and tries again after a pause when they are not: `sdkmanager` reports a broken
archive only as a warning and may still exit 0, so the files decide. The action then finds the
packages installed and downloads nothing.

Usage: android_sdk.py --api-level <level>
The SDK root is `ANDROID_HOME` (or `ANDROID_SDK_ROOT`). Exit 0 when the packages are installed,
1 when they are still missing after the attempts, 2 without an SDK root.
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import time
from pathlib import Path

# How often a CI job tries the install and how long it waits between two tries.
ATTEMPTS = 3
PAUSE_SECONDS = 20
# The image the emulator jobs run (ci.yml and build-verify.yml: target google_apis, x86_64).
TARGET = "google_apis"
ARCH = "x86_64"


def packages(api_level: int) -> list[str]:
    """The sdkmanager packages an emulator of `api_level` needs."""
    return [
        "emulator",
        "platform-tools",
        f"platforms;android-{api_level}",
        f"system-images;android-{api_level};{TARGET};{ARCH}",
    ]


def missing(root: Path, api_level: int) -> list[str]:
    """The packages whose files are not under `root`."""
    out = []
    if not (root / "emulator" / "emulator").exists():
        out.append("emulator")
    image = root / "system-images" / f"android-{api_level}" / TARGET / ARCH / "system.img"
    if not image.exists():
        out.append(f"system-images;android-{api_level};{TARGET};{ARCH}")
    return out


def install(
    root: Path, api_level: int, attempts: int = ATTEMPTS, pause: float = PAUSE_SECONDS
) -> int:
    """Install the packages of `api_level` into `root`, trying up to `attempts` times."""
    sdkmanager = root / "cmdline-tools" / "latest" / "bin" / "sdkmanager"
    for attempt in range(1, attempts + 1):
        # The license prompts read "y" from standard input; the SDK program runs with fixed
        # arguments and no shell.
        subprocess.run(  # nosec B603
            [str(sdkmanager), "--install", *packages(api_level)],
            input="y\n" * 20,
            text=True,
            check=False,
        )
        left = missing(root, api_level)
        if not left:
            return 0
        print(
            f"android_sdk: attempt {attempt} of {attempts} left {', '.join(left)} missing",
            file=sys.stderr,
        )
        if attempt < attempts:
            time.sleep(pause)
    return 1


def main(argv: list[str] | None = None) -> int:
    """Command line entry point."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--api-level", type=int, required=True)
    args = parser.parse_args(argv)
    home = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT")
    if not home:
        print("android_sdk: neither ANDROID_HOME nor ANDROID_SDK_ROOT is set", file=sys.stderr)
        return 2
    return install(Path(home), args.api_level, ATTEMPTS, PAUSE_SECONDS)


if __name__ == "__main__":
    sys.exit(main())
