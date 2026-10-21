# SPDX-License-Identifier: Apache-2.0
"""``secret_ref`` values of the runner (docs/spec/16-betrieb.md 5.2, schemes ``file``, ``env``).

A value is read fresh for every use and handed out only as `bothy_sdk.auth.Secret`, whose
``repr`` and ``str`` are ``[redacted]``. Errors carry a code and never the value or the target
(SEC-LOG-2); the caller names the configuration key.

Examples:
    >>> parse_secret_ref("env:RUNNER_KEY").scheme
    'env'
"""

from __future__ import annotations

import os
import re
import stat
from dataclasses import dataclass, field

from bothy_sdk.auth import Secret

__all__ = ["SecretError", "SecretRef", "describe", "parse_secret_ref", "resolve"]

_ENV_NAME = re.compile(r"[A-Z_][A-Z0-9_]*")
_ALLOWED_MODES = (0o400, 0o600)


class SecretError(Exception):
    """A ``secret_ref`` that cannot be parsed or resolved.

    Attributes:
        code: ``unsupported``, ``missing``, ``empty`` or ``unsafe_mode``.
    """

    def __init__(self, code: str) -> None:
        """Create the error with its code only."""
        super().__init__(code)
        self.code = code


@dataclass(frozen=True)
class SecretRef:
    """A parsed ``secret_ref``; it names where a secret lives, never the secret.

    Attributes:
        scheme: ``file`` or ``env``.
        target: The absolute path or the variable name.
    """

    scheme: str
    target: str = field(repr=False)

    def __str__(self) -> str:
        """The scheme only: even the place of a secret stays out of logs."""
        return f"{self.scheme}:[redacted]"


def parse_secret_ref(text: str) -> SecretRef:
    """Parse ``file:<absolute clean path>`` or ``env:<NAME>`` (16 5.2 rule 1).

    Args:
        text: The configured value.

    Returns:
        The reference.

    Raises:
        SecretError: ``unsupported`` for another scheme or a malformed target.
    """
    scheme, sep, target = text.partition(":")
    if not sep:
        raise SecretError("unsupported")
    if scheme == "env" and _ENV_NAME.fullmatch(target):
        return SecretRef("env", target)
    if scheme == "file" and _clean_absolute(target):
        return SecretRef("file", target)
    raise SecretError("unsupported")


def _clean_absolute(path: str) -> bool:
    if not path.startswith("/") or "\x00" in path or path == "/":
        return False
    parts = path[1:].split("/")
    return all(p not in ("", ".", "..") for p in parts)


def resolve(ref: SecretRef) -> Secret:
    """Read the secret now (no cache, so a rotation applies at the next use).

    A file must be a regular file of the process user with mode ``0400`` or ``0600``, checked
    on the open file; one final line ending is not part of the value.

    Args:
        ref: The reference.

    Returns:
        The secret.

    Raises:
        SecretError: ``missing``, ``empty`` or ``unsafe_mode``.
    """
    if ref.scheme == "env":
        value = os.environ.get(ref.target, "")
        if not value:
            raise SecretError("empty" if ref.target in os.environ else "missing")
        return Secret(value)
    try:
        fd = os.open(ref.target, os.O_RDONLY)
    except OSError:
        raise SecretError("missing") from None
    with os.fdopen(fd, "rb") as fh:
        info = os.fstat(fh.fileno())
        if (
            not stat.S_ISREG(info.st_mode)
            or info.st_uid != os.getuid()
            or stat.S_IMODE(info.st_mode) not in _ALLOWED_MODES
        ):
            raise SecretError("unsafe_mode")
        raw = fh.read().decode("utf-8", errors="strict")
    if raw.endswith("\r\n"):
        raw = raw[:-2]
    elif raw.endswith("\n"):
        raw = raw[:-1]
    if not raw:
        raise SecretError("empty")
    return Secret(raw)


def describe(ref: SecretRef) -> str:
    """Return a printable form of ``ref`` without its target (for ``check-config``).

    Args:
        ref: The reference.

    Returns:
        For example ``file:[redacted]``.
    """
    return str(ref)
