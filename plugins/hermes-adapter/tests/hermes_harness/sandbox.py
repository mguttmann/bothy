# SPDX-License-Identifier: Apache-2.0
"""The macOS sandbox of the Hermes process tree.

The Hermes process tree runs under ``sandbox-exec`` with a profile that denies outbound network
except the loopback ports of the harness' own servers and the temporary root's own sockets,
signals and process information outside the sandbox, reading file contents of the real home
except the temporary root and the interpreter, writing below the real home except the
temporary root, and running the service and keychain tools. The profile does not block the
keychain API of macOS; the keychain stays out of reach because ``HOME`` is a throwaway home
without a default keychain, the ``security`` tool runs into a refusing shim and its real binary
may not be executed. On Linux there is no such sandbox: the allowlisted environment already
makes every stored credential unreachable, so a leaked request could carry only fake
credentials, which cost nothing at a real provider.

Example:
    >>> profile.write_text(sandbox_profile(root, readable=interpreter_dirs(), ports=[4711]))
    >>> subprocess.Popen(wrap_command(["hermes", "gateway", "run"], profile), ...)
"""

from __future__ import annotations

import os
import platform
import pwd
from collections.abc import Iterable
from pathlib import Path
from typing import Final

SANDBOX_EXEC: Final = "/usr/bin/sandbox-exec"
#: Executables the Hermes tree may never run, even by absolute path.
DENIED_EXECUTABLES: Final = ("/bin/launchctl", "/usr/bin/security")


def account_home() -> Path:
    """The account home of the current user from the password database, not from ``HOME``."""
    return Path(pwd.getpwuid(os.getuid()).pw_dir)


def _quoted(path: Path) -> str:
    text = str(path.resolve())
    if '"' in text or "\\" in text:
        raise ValueError(f"sandbox: path {text!r} cannot be quoted in a profile")
    return f'"{text}"'


def sandbox_profile(
    root: Path,
    *,
    readable: Iterable[Path],
    ports: Iterable[int],
    real_home: Path | None = None,
) -> str:
    """Render the sandbox profile for one run.

    Args:
        root: The temporary root; the only place below the real home that the tree may read,
            write and connect to (its own Unix sockets).
        readable: Further directories whose file contents the tree may read (the managed
            Python of the throwaway venv).
        ports: The loopback ports the tree may connect to (the harness' own servers); every
            other loopback service of the machine, such as a local model or another gateway,
            stays unreachable.
        real_home: The account home to protect; the password database entry by default.

    Returns:
        The profile text (SBPL). Later rules take precedence over earlier ones.

    Raises:
        ValueError: A path contains a character that cannot be quoted.
    """
    home = real_home if real_home is not None else account_home()
    lines = [
        "(version 1)",
        "(allow default)",
        "(deny network-outbound)",
        *(f'(allow network-outbound (remote ip "localhost:{int(port)}"))' for port in ports),
        f"(allow network-outbound (subpath {_quoted(root)}))",
        "(deny signal)",
        "(allow signal (target same-sandbox))",
        "(deny process-info*)",
        "(allow process-info* (target same-sandbox))",
        f"(deny file-read-data (subpath {_quoted(home)}))",
        f"(deny file-write* (subpath {_quoted(home)}))",
        f"(allow file-read-data (subpath {_quoted(root)}))",
        f"(allow file-write* (subpath {_quoted(root)}))",
    ]
    lines += [f"(allow file-read-data (subpath {_quoted(path)}))" for path in readable]
    lines += [f'(deny process-exec (literal "{exe}"))' for exe in DENIED_EXECUTABLES]
    return "\n".join(lines) + "\n"


def wrap_command(command: list[str], profile: Path) -> list[str]:
    """Wrap a command in the sandbox on macOS; unchanged elsewhere.

    Args:
        command: The command line.
        profile: The profile file written from :func:`sandbox_profile`.

    Returns:
        The command line to run.
    """
    if platform.system() == "Darwin":
        return [SANDBOX_EXEC, "-f", str(profile), *command]
    return list(command)
