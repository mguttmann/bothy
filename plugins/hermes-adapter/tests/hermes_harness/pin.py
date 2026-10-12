# SPDX-License-Identifier: Apache-2.0
"""The pinned Hermes Agent version (``tests/hermes_pin.toml``).

The pin names a public release of Hermes Agent by tag and full commit SHA. It is older than
the Renovate cooldown ``deps.min_release_age`` on the day it was recorded (ADR 0041), and a
Renovate rule asks for a new recording when a newer release appears.

Example:
    >>> pin = load_pin(PIN_FILE)
    >>> len(pin.commit)
    40
"""

from __future__ import annotations

import datetime
import re
import tomllib
from dataclasses import dataclass
from pathlib import Path
from typing import Final

#: The checked-in pin of the recording.
PIN_FILE: Final = Path(__file__).resolve().parents[1] / "hermes_pin.toml"

INSTALL_MODES: Final = frozenset({"uv-lock", "requirements"})
_COMMIT: Final = re.compile(r"[0-9a-f]{40}")
# A release tag is a version: an optional "v", dot-separated numbers and an optional suffix.
# Branch names ("main", "release/2026.9") never match, so a moving ref cannot be pinned.
_TAG: Final = re.compile(r"v?[0-9]+(\.[0-9]+)*([.-][0-9A-Za-z]+)*")
_PYTHON: Final = re.compile(r"3\.[0-9]+")
_FIELDS: Final = frozenset(
    {"repository", "tag", "commit", "python", "install", "extras", "recorded_on"}
)


class PinError(ValueError):
    """The pin file is missing, is no TOML or holds an invalid or unknown field."""


@dataclass(frozen=True)
class HermesPin:
    """One pinned Hermes Agent version.

    Attributes:
        repository: Git URL of the public repository (``https://...`` or, in tests,
            ``file://...``).
        tag: Release tag, or ``None`` for a tag-less pin of a default-branch commit.
        commit: Full commit SHA in lower-case hex.
        python: CPython line of the throwaway environment, for example ``"3.13"``.
        install: ``"uv-lock"`` (Hermes' own lock) or ``"requirements"`` (hash-pinned list).
        extras: Optional dependency groups of Hermes that the recording needs.
        recorded_on: Day of the recording that the fixtures hold.
    """

    repository: str
    tag: str | None
    commit: str
    python: str
    install: str
    extras: tuple[str, ...]
    recorded_on: datetime.date


def _string(data: dict[str, object], name: str) -> str:
    value = data.get(name)
    if value is None:
        raise PinError(f"pin: field {name!r} is missing")
    if not isinstance(value, str) or not value.strip():
        raise PinError(f"pin: field {name!r} must be a non-empty string")
    return value


def load_pin(path: Path) -> HermesPin:
    """Read and validate a pin file.

    Args:
        path: The TOML file, normally :data:`PIN_FILE`.

    Returns:
        The validated pin.

    Raises:
        PinError: The file is missing or no TOML, a field is missing, unknown or invalid:
            a commit that is not 40 lower-case hex digits, a tag that looks like a branch
            name, an unknown install mode, a Python value without minor version, extras that
            are not a list of strings or a date that is not a TOML date.
    """
    try:
        data: dict[str, object] = tomllib.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise PinError(f"pin: {path} is missing") from exc
    except tomllib.TOMLDecodeError as exc:
        raise PinError(f"pin: {path} is no valid TOML: {exc}") from exc
    unknown = sorted(set(data) - _FIELDS)
    if unknown:
        raise PinError(f"pin: unknown field(s) {', '.join(unknown)}")
    repository = _string(data, "repository")
    commit = _string(data, "commit")
    if not _COMMIT.fullmatch(commit):
        raise PinError("pin: commit must be the full SHA in 40 lower-case hex digits")
    tag: str | None = None
    if "tag" in data:
        tag = _string(data, "tag")
        if not _TAG.fullmatch(tag):
            raise PinError(f"pin: tag {tag!r} is no release tag (a branch name cannot be pinned)")
    python = _string(data, "python")
    if not _PYTHON.fullmatch(python):
        raise PinError(f"pin: python {python!r} must name a CPython line such as 3.13")
    install = _string(data, "install")
    if install not in INSTALL_MODES:
        raise PinError(f"pin: install {install!r} is not one of {sorted(INSTALL_MODES)}")
    extras_raw = data.get("extras", [])
    if not isinstance(extras_raw, list) or not all(isinstance(e, str) and e for e in extras_raw):
        raise PinError("pin: extras must be a list of non-empty strings")
    recorded_on = data.get("recorded_on")
    if recorded_on is None:
        raise PinError("pin: field 'recorded_on' is missing")
    if not isinstance(recorded_on, datetime.date) or isinstance(recorded_on, datetime.datetime):
        raise PinError("pin: recorded_on must be a TOML date (YYYY-MM-DD)")
    return HermesPin(
        repository=repository,
        tag=tag,
        commit=commit,
        python=python,
        install=install,
        extras=tuple(extras_raw),
        recorded_on=recorded_on,
    )
