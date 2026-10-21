# SPDX-License-Identifier: Apache-2.0
"""The emulator packages of the Android jobs are installed with retries (tools/ci/android_sdk.py).

A fake `sdkmanager` stands in for the real one: it fails a given number of times without
installing anything, like a download that ends in "Error on ZipFile unknown archive", and then
creates the files a real install leaves.
"""

from __future__ import annotations

import os
import stat
from pathlib import Path

import android_sdk
import pytest


def _fake_sdk(tmp_path: Path, failures: int, exit_code: int = 0) -> Path:
    """An SDK root whose sdkmanager installs nothing for `failures` calls, then everything."""
    root = tmp_path / "sdk"
    bin_dir = root / "cmdline-tools" / "latest" / "bin"
    bin_dir.mkdir(parents=True)
    counter = tmp_path / "calls"
    counter.write_text("0", encoding="utf-8")
    script = bin_dir / "sdkmanager"
    script.write_text(
        f"""#!/bin/sh
cat > /dev/null
n=$(cat "{counter}")
n=$((n + 1))
echo "$n" > "{counter}"
if [ "$n" -le {failures} ]; then
  echo "Warning: preparing SDK package Android Emulator: Error on ZipFile unknown archive."
  exit {exit_code}
fi
for p in "$@"; do
  case "$p" in
    emulator) mkdir -p "{root}/emulator" && touch "{root}/emulator/emulator" ;;
    system-images\\;*)
      d=$(echo "$p" | tr ';' '/')
      mkdir -p "{root}/$d" && touch "{root}/$d/system.img" ;;
  esac
done
""",
        encoding="utf-8",
    )
    script.chmod(script.stat().st_mode | stat.S_IXUSR)
    return root


def _calls(tmp_path: Path) -> int:
    return int((tmp_path / "calls").read_text(encoding="utf-8"))


def test_packages_name_the_emulator_and_the_image_of_the_level() -> None:
    assert android_sdk.packages(26) == [
        "emulator",
        "platform-tools",
        "platforms;android-26",
        "system-images;android-26;google_apis;x86_64",
    ]


def test_first_install_that_leaves_the_files_passes(tmp_path: Path) -> None:
    root = _fake_sdk(tmp_path, failures=0)
    assert android_sdk.install(root, 36, attempts=3, pause=0) == 0
    assert _calls(tmp_path) == 1
    assert (
        root / "system-images" / "android-36" / "google_apis" / "x86_64" / "system.img"
    ).exists()


@pytest.mark.parametrize("exit_code", [0, 1])
def test_a_failed_download_is_retried(tmp_path: Path, exit_code: int) -> None:
    # sdkmanager reports a broken archive only as a warning and may still exit 0, so the files
    # decide, not the exit code.
    root = _fake_sdk(tmp_path, failures=2, exit_code=exit_code)
    assert android_sdk.install(root, 26, attempts=3, pause=0) == 0
    assert _calls(tmp_path) == 3


def test_gives_up_after_the_attempts(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _fake_sdk(tmp_path, failures=5)
    assert android_sdk.install(root, 26, attempts=3, pause=0) == 1
    assert _calls(tmp_path) == 3
    assert "emulator" in capsys.readouterr().err


def test_main_reads_the_sdk_root_from_the_environment(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    root = _fake_sdk(tmp_path, failures=0)
    monkeypatch.setenv("ANDROID_HOME", str(root))
    monkeypatch.setattr(android_sdk, "PAUSE_SECONDS", 0)
    assert android_sdk.main(["--api-level", "26"]) == 0
    monkeypatch.delenv("ANDROID_HOME")
    monkeypatch.delenv("ANDROID_SDK_ROOT", raising=False)
    assert android_sdk.main(["--api-level", "26"]) == 2
    assert os.environ.get("ANDROID_HOME") is None
