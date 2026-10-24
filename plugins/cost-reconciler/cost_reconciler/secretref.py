# SPDX-License-Identifier: Apache-2.0
"""Secret references after docs/spec/16-operations.md 5.2 and a type that never prints its value.

A reference has the form `<scheme>:<target>`. The app supports the schemes `file` (a file with
mode `0400` or `0600` owned by the process user, SEC-SEC-1) and `env` (an environment variable;
allowed, not recommended, because child processes and crash reports inherit it). Values are read
again at the start of every pass, so a rotation needs no restart.

Example:
    >>> ref = parse_secret_ref("env:ADMIN_KEY")
    >>> secret = read_secret(ref, environ={"ADMIN_KEY": "value"})
    >>> str(secret)
    '[redacted]'
"""

from __future__ import annotations

import os
import re
import stat
from collections.abc import Mapping
from dataclasses import dataclass

REDACTED = "[redacted]"
"""The mark that stands in logs, reprs and messages instead of a secret value."""

_ENV_NAME = re.compile(r"[A-Z_][A-Z0-9_]*")
_PRIVATE_MODES = frozenset({0o400, 0o600})


class SecretRefError(Exception):
    """A reference that is invalid or whose value cannot be read safely.

    The message names the violated rule, never the reference text or a value, because a value
    may stand in a mistyped reference.

    Attributes:
        code: One of `invalid_ref`, `unsupported_scheme`, `not_found`, `insecure_file`,
            `read_failed` (the codes of 16 5.2 rule 6).
    """

    def __init__(self, code: str, rule: str) -> None:
        """Create the error.

        Args:
            code: The error code.
            rule: A fixed text that names the violated rule.
        """
        super().__init__(f"{code}: {rule}")
        self.code = code


class Secret:
    """A secret value whose `str`, `repr` and formatting show only `REDACTED`.

    Only the HTTP clients call `reveal` to build a header.

    Example:
        >>> f"{Secret('value')}"
        '[redacted]'
    """

    __slots__ = ("_value",)

    def __init__(self, value: str) -> None:
        """Wrap a value.

        Args:
            value: The secret text; never empty.

        Raises:
            ValueError: The value is empty.
        """
        if not value:
            raise ValueError("a secret is never empty")
        self._value = value

    def reveal(self) -> str:
        """Return the value; only for building a request header."""
        return self._value

    def __repr__(self) -> str:
        """Return the redaction mark."""
        return REDACTED

    def __str__(self) -> str:
        """Return the redaction mark."""
        return REDACTED

    def __format__(self, format_spec: str) -> str:
        """Return the redaction mark whatever the format specification."""
        del format_spec
        return REDACTED


@dataclass(frozen=True)
class SecretRef:
    """A parsed reference.

    Attributes:
        scheme: `file` or `env`.
        target: The absolute path or the variable name.
    """

    scheme: str
    target: str


def _clean_absolute(path: str) -> bool:
    if not path.startswith("/") or path == "/" or path.endswith("/") or "\x00" in path:
        return False
    return all(part not in {"", ".", ".."} for part in path[1:].split("/"))


def parse_secret_ref(text: str) -> SecretRef:
    """Parse a reference after the grammar of 16 5.2 rule 1.

    Args:
        text: The reference, for example `file:/run/secrets/admin_key` or `env:ADMIN_KEY`.

    Returns:
        The parsed reference.

    Raises:
        SecretRefError: `invalid_ref` for a malformed reference, `unsupported_scheme` for any
            scheme other than `file` and `env`.
    """
    scheme, sep, target = text.partition(":")
    if not sep or not scheme:
        raise SecretRefError("invalid_ref", "the reference has no scheme")
    if scheme == "file":
        if not _clean_absolute(target):
            raise SecretRefError(
                "invalid_ref", "the file scheme needs an absolute path in clean form"
            )
        return SecretRef(scheme, target)
    if scheme == "env":
        if not _ENV_NAME.fullmatch(target):
            raise SecretRefError("invalid_ref", "the env scheme needs a name of [A-Z_][A-Z0-9_]*")
        return SecretRef(scheme, target)
    raise SecretRefError("unsupported_scheme", "only the schemes file and env are supported")


def _strip_line_end(value: str) -> str:
    # `echo value > file` adds one line end; a line end is never part of a secret (16 5.2 rule 2).
    if value.endswith("\r\n"):
        return value[:-2]
    if value.endswith("\n"):
        return value[:-1]
    return value


def _read_file(path: str) -> str:
    try:
        fd = os.open(path, os.O_RDONLY | getattr(os, "O_CLOEXEC", 0))
    except FileNotFoundError:
        raise SecretRefError("not_found", "the file does not exist") from None
    except OSError:
        raise SecretRefError("read_failed", "the file cannot be opened") from None
    try:
        # The check runs on the opened file, so the file cannot change between check and read.
        info = os.fstat(fd)
        if not stat.S_ISREG(info.st_mode):
            raise SecretRefError("insecure_file", "the reference is not a regular file")
        if stat.S_IMODE(info.st_mode) not in _PRIVATE_MODES:
            raise SecretRefError("insecure_file", "the file mode must be 0400 or 0600")
        if info.st_uid != os.geteuid():
            raise SecretRefError("insecure_file", "the file must be owned by the process user")
        chunks = []
        while chunk := os.read(fd, 65536):
            chunks.append(chunk)
    except OSError:
        raise SecretRefError("read_failed", "the file cannot be read") from None
    finally:
        os.close(fd)
    try:
        return b"".join(chunks).decode("utf-8")
    except UnicodeDecodeError:
        raise SecretRefError("read_failed", "the file is no UTF-8 text") from None


def read_secret(ref: SecretRef, environ: Mapping[str, str] | None = None) -> Secret:
    """Read the current value of a reference.

    Args:
        ref: The parsed reference.
        environ: The environment for `env:`; the process environment when absent.

    Returns:
        The value, wrapped.

    Raises:
        SecretRefError: `not_found` for a missing file or variable or an empty value,
            `insecure_file` for a file that is not regular, has another mode than `0400` or
            `0600` or another owner than the process user, `read_failed` otherwise.
    """
    if ref.scheme == "file":
        value = _strip_line_end(_read_file(ref.target))
    elif ref.scheme == "env":
        source = os.environ if environ is None else environ
        value = source.get(ref.target, "")
    else:
        raise SecretRefError("unsupported_scheme", "only the schemes file and env are supported")
    if not value:
        raise SecretRefError("not_found", "the secret is missing or empty")
    return Secret(value)
