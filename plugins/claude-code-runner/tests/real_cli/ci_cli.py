# SPDX-License-Identifier: Apache-2.0
"""Run the background part of the real-CLI suite in CI (make runner-real-cli).

The tests of ``tests/real_cli`` that start background sessions (``claude --bg``) must not run on
a machine where a runner of the operator runs: the supervisor of the CLI is shared by the host
user's sessions. A GitHub-hosted runner is such a fresh machine, so this program runs only there
(``GITHUB_ACTIONS=true``). It downloads the Linux x64 package of `VERSION` from the npm registry,
checks it against the `INTEGRITY` recorded here, unpacks the program and runs `TESTS` with the
switch of the suite. Free like the rest of level one: the CLI talks only to the local stand-in
for the API, with no login and no model call (FIX-RUNNER-BG-NO-PROMPT).

`VERSION` is the CLI the runner is run with; while it differs from the pin
(`PINNED_CLAUDE_CODE_VERSION`) the tests take it as the tested version
(``CLAUDE_CODE_RUNNER_TESTED_VERSION``, README "Following new Claude Code versions"). A new
version changes `VERSION` and `INTEGRITY` together; the integrity is the ``dist.integrity`` of the
package in the registry.
"""

from __future__ import annotations

import base64
import hashlib
import os
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
from pathlib import Path

from claude_code_runner.heartbeat import PINNED_CLAUDE_CODE_VERSION

VERSION = "2.1.332"
PACKAGE = "@anthropic-ai/claude-code-linux-x64"
INTEGRITY = "sha512-" + (
    "f4JWhzwYutJIHScC7/dYOPX6s28tt7ZADhsOi8zwOHq5fFPEuUeChh2NMfinSg0GrUckYxaeebC4h0DqiKzLcw=="
)
TARBALL = (
    "https://registry.npmjs.org/@anthropic-ai/claude-code-linux-x64/-/"
    f"claude-code-linux-x64-{VERSION}.tgz"
)
TESTS = (
    "tests/real_cli/test_pinned_cli.py::test_pinned_cli_knows_error_texts_and_events",
    "tests/real_cli/test_pinned_cli.py::test_pinned_cli_knows_the_names_of_background_mode",
    "tests/real_cli/test_background.py::test_launch_list_stop_and_remove",
    "tests/real_cli/test_background.py::"
    "test_a_call_that_would_wait_for_a_person_is_refused_and_the_turn_goes_on",
    # The probes of docs/spec/27-computer-use.md 16 (CU-06): the computer MCP file in print mode
    # and in a background session, and the files of the session that hold the image.
    "tests/real_cli/test_computer_probe.py",
)
PLUGIN = Path(__file__).resolve().parents[2]


def integrity_of(data: bytes) -> str:
    """Return the npm integrity text (``sha512-<base64>``) of a package's bytes."""
    return "sha512-" + base64.b64encode(hashlib.sha512(data).digest()).decode("ascii")


def unpack(data: bytes, folder: Path) -> Path:
    """Write the program ``package/claude`` of the package into ``folder``; its path."""
    archive = folder / "package.tgz"
    archive.write_bytes(data)
    with tarfile.open(archive) as tar:
        member = tar.getmember("package/claude")
        source = tar.extractfile(member)
        assert source is not None, "the package has no program"
        program = folder / "claude"
        program.write_bytes(source.read())
    program.chmod(0o755)
    archive.unlink()
    return program


def main() -> int:
    """Install the CLI and run the tests; the exit status of pytest."""
    if os.environ.get("GITHUB_ACTIONS") != "true":
        print("runner-real-cli starts background sessions and runs only in CI (GITHUB_ACTIONS)")
        return 2
    with urllib.request.urlopen(TARBALL, timeout=300) as response:  # noqa: S310 - fixed https
        data = response.read()
    if integrity_of(data) != INTEGRITY:
        print(f"{PACKAGE} {VERSION}: the download does not match the recorded integrity")
        return 1
    folder = Path(tempfile.mkdtemp(prefix="cli", dir="/tmp"))
    program = unpack(data, folder)
    env = {
        **os.environ,
        "CLAUDE_CODE_RUNNER_REAL_CLI": "1",
        "CLAUDE_CODE_RUNNER_CLAUDE_BIN": str(program),
        "TMPDIR": "/tmp",
    }
    if VERSION != PINNED_CLAUDE_CODE_VERSION:
        env["CLAUDE_CODE_RUNNER_TESTED_VERSION"] = VERSION
    command = [sys.executable, "-m", "pytest", "-p", "no:cacheprovider", "-v", *TESTS]
    return subprocess.run(command, cwd=PLUGIN, env=env, check=False).returncode


if __name__ == "__main__":
    sys.exit(main())
