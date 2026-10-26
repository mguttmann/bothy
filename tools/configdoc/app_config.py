# SPDX-License-Identifier: Apache-2.0
"""Generate the key table of `AppConfig` in `packages/domain` from the configuration reference.

`AppConfig` is the client's single source of its configuration values (docs/DECISIONS.md
D-W-115, D-M4-08-5). Its table holds every row of section 4 of
`docs/operations/configuration.md` with place `App`, plus the keys of place `Server` that steer
the client's own behaviour without an API that delivers them (`SERVER_KEYS_READ_BY_APP`). The
kind of each value is derived from the written default, so the Dart parsers read the default
with the same grammar as an override.

Usage:
    app_config.py [--root <repo>]            write OUTPUT (the generated Dart part)
    app_config.py [--root <repo>] --check    fail (exit 1) when OUTPUT differs from a fresh run

Exit codes: 0 ok; 1 the reference is invalid for the table or (with `--check`) the file is
stale; 2 usage error.
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

REFERENCE = Path("docs/operations/configuration.md")
OUTPUT = Path("packages/domain/lib/src/config/app_config_keys.g.dart")
START = "<!-- config-reference:start -->"
END = "<!-- config-reference:end -->"
APP_PLACE = "App"
SERVER_PLACE = "Server"

# Keys of place Server that the apps read for their own behaviour, because no API operation
# delivers them (D-W-115: sync and outbox timing, focus limit, cache age; D-W-116: the expiry of a
# pending device that screen E4 shows and the maximum duration of a terminal session that the
# header of screen D11 counts down, M4-36). Each one must stay in the reference with place Server;
# a key that moves or disappears fails the generation, so the list is reviewed with the change.
SERVER_KEYS_READ_BY_APP: tuple[str, ...] = (
    "sync.reconnect_backoff_base",
    "sync.reconnect_backoff_cap",
    "sync.typing_min_interval",
    "sync.typing_ttl",
    "sync.max_focused_threads",
    "client.cache_max_age",
    "client.outbox_retry_base",
    "client.outbox_retry_cap",
    "auth.device_pending_ttl",
    "terminal.max_session_duration",
)

_KEY = re.compile(r"^[a-z][a-z0-9]*(_[a-z0-9]+)*(\.[a-z][a-z0-9]*(_[a-z0-9]+)*)+$")
_KINDS: tuple[tuple[str, re.Pattern[str]], ...] = (
    ("boolean", re.compile(r"^(true|false)$")),
    ("duration", re.compile(r"^(0|[1-9][0-9]*)(ms|s|m|h|d)$")),
    ("bytes", re.compile(r"^(0|[1-9][0-9]*)(KiB|MiB|GiB)$")),
    ("byteRate", re.compile(r"^(0|[1-9][0-9]*)(KiB|MiB|GiB)/s$")),
    ("bitRate", re.compile(r"^(0|[1-9][0-9]*)kbps$")),
    ("pixels", re.compile(r"^(0|[1-9][0-9]*)px$")),
    ("integer", re.compile(r"^-?(0|[1-9][0-9]*)$")),
    ("decimal", re.compile(r"^-?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][-+]?[0-9]+)?$")),
    ("list", re.compile(r"^[^,]+(,[^,]+)+$")),
)


class ReferenceError(Exception):
    """The reference does not yield a valid key table; the message names the key or row."""


@dataclass(frozen=True)
class Entry:
    """One key of the table.

    Attributes:
        key: the configuration key, for example `client.outbox_stuck_after`.
        place: the place column of the reference (`App` or `Server`).
        kind: the Dart kind derived from the default (see `kind_of`).
        default: the default as written in the reference, without backticks.
        getter: the name of the typed getter on `AppConfig`.
    """

    key: str
    place: str
    kind: str
    default: str
    getter: str


def kind_of(default: str) -> str | None:
    """Derive the kind of a value from its written default.

    Args:
        default: the default text, for example `2m`, `1GiB`, `64kbps` or `60, 300, 900`.

    Returns:
        One of `boolean`, `duration`, `bytes`, `byteRate`, `bitRate`, `pixels`, `integer`,
        `decimal`, `list`, or None when no grammar matches.

    Example:
        >>> kind_of("150ms"), kind_of("1MiB/s"), kind_of("0.01")
        ('duration', 'byteRate', 'decimal')
    """
    for kind, pattern in _KINDS:
        if pattern.match(default):
            return kind
    return None


def getter_name(key: str) -> str:
    """Name the typed getter of a key: the segments after the first, in lowerCamelCase.

    Example:
        >>> getter_name("client.outbox_stuck_after"), getter_name("client.perf.cold_start_p95")
        ('outboxStuckAfter', 'perfColdStartP95')
    """
    words = [w for segment in key.split(".")[1:] for w in segment.split("_")]
    return words[0] + "".join(w[:1].upper() + w[1:] for w in words[1:])


def _cells(line: str) -> list[str]:
    """Split a Markdown table row into stripped cells."""
    return [c.strip() for c in line.strip().strip("|").split("|")]


def _unquote(cell: str) -> str:
    """Remove one pair of backticks around a cell."""
    return cell[1:-1] if len(cell) >= 2 and cell[0] == cell[-1] == "`" else cell


def rows(reference: str) -> dict[str, tuple[str, str]]:
    """Read the rows of the generated block of section 4.

    The columns are positional (key, environment, place, type, default, meaning, status), so
    the table reads the same in every language of the reference.

    Args:
        reference: the text of `docs/operations/configuration.md`.

    Returns:
        Key mapped to (place, default text without backticks).

    Raises:
        ReferenceError: the markers are missing or a key appears twice.
    """
    if START not in reference or END not in reference:
        raise ReferenceError("configuration reference: section 4 markers are missing")
    block = reference.split(START, 1)[1].split(END, 1)[0]
    found: dict[str, tuple[str, str]] = {}
    for line in block.splitlines():
        if not line.startswith("| `"):
            continue
        cells = _cells(line)
        if len(cells) < 5:
            raise ReferenceError(f"configuration reference: short row {cells[0]}")
        key = _unquote(cells[0])
        if key in found:
            raise ReferenceError(f"configuration reference: {key} appears twice")
        found[key] = (cells[2], _unquote(cells[4]))
    return found


def entries(reference: str) -> list[Entry]:
    """Build the table of `AppConfig` from the reference.

    Args:
        reference: the text of `docs/operations/configuration.md`.

    Returns:
        The entries sorted by key: every row of place `App` and every key of
        `SERVER_KEYS_READ_BY_APP`.

    Raises:
        ReferenceError: a listed server key is missing or has another place, a default has no
            known kind, a key has an invalid form, or two keys give the same getter.
    """
    table = rows(reference)
    for key in SERVER_KEYS_READ_BY_APP:
        if key not in table:
            raise ReferenceError(f"{key}: listed for AppConfig (D-W-115) but not in the reference")
        if table[key][0] != SERVER_PLACE:
            raise ReferenceError(f"{key}: listed for AppConfig with place Server, has another")
    chosen = sorted(k for k, (place, _) in table.items() if place == APP_PLACE)
    chosen = sorted({*chosen, *SERVER_KEYS_READ_BY_APP})
    result: list[Entry] = []
    getters: dict[str, str] = {}
    for key in chosen:
        place, default = table[key]
        if not _KEY.match(key):
            raise ReferenceError(f"{key}: not a valid key")
        kind = kind_of(default)
        if kind is None:
            raise ReferenceError(f"{key}: the default has no kind AppConfig can parse")
        getter = getter_name(key)
        if getter in getters:
            raise ReferenceError(f"{key}: getter {getter} is also the getter of {getters[getter]}")
        getters[getter] = key
        result.append(Entry(key, place, kind, default, getter))
    return result


def _dart_string(text: str) -> str:
    """Quote text as a Dart single-quoted raw-free string literal."""
    escaped = text.replace("\\", "\\\\").replace("'", "\\'").replace("$", "\\$")
    return f"'{escaped}'"


_DART_TYPES = {
    "boolean": "bool",
    "duration": "Duration",
    "bytes": "int",
    "byteRate": "int",
    "bitRate": "int",
    "pixels": "int",
    "integer": "int",
    "decimal": "double",
    "list": "List<String>",
}


def render(table: list[Entry]) -> str:
    """Render the Dart library part with the key table and the typed getters.

    Args:
        table: the entries from `entries`.

    Returns:
        The content of `app_config_keys.g.dart`.
    """
    out = [
        "// SPDX-License-Identifier: Apache-2.0",
        "// GENERATED by tools/configdoc/app_config.py from docs/operations/configuration.md.",
        "// Do not edit; run `make app-config` (make codegen-check holds this file).",
        "",
        "part of 'app_config.dart';",
        "",
        "/// Every key of [AppConfig] with its place, kind and the default of the reference.",
        "const List<AppConfigKey> appConfigKeys = <AppConfigKey>[",
    ]
    for e in table:
        out.append(
            f"  AppConfigKey({_dart_string(e.key)}, {_dart_string(e.place)}, "
            f"AppConfigKind.{e.kind}, {_dart_string(e.default)}),"
        )
    out += [
        "];",
        "",
        "/// Keys of place `Server` that [AppConfig] carries (docs/DECISIONS.md D-W-115, D-W-116).",
        "const List<String> serverKeysReadByApp = <String>[",
        *(f"  {_dart_string(k)}," for k in SERVER_KEYS_READ_BY_APP),
        "];",
        "",
        "/// Typed getters, one per key, named after the key without its first segment.",
        "extension AppConfigValues on AppConfig {",
    ]
    for e in table:
        dart_type = _DART_TYPES[e.kind]
        out += [
            f"  /// `{e.key}` ({e.kind}, default `{e.default}`).",
            f"  {dart_type} get {e.getter} => value<{dart_type}>({_dart_string(e.key)});",
        ]
    out += ["}", ""]
    return "\n".join(out)


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; see the module docstring for usage and exit codes."""
    parser = argparse.ArgumentParser(prog="app_config.py")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--check", action="store_true", help="fail when the file is stale")
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:  # argparse ends on usage errors (2) and on --help (0)
        return exc.code if isinstance(exc.code, int) else 2
    root: Path = args.root
    try:
        text = render(entries((root / REFERENCE).read_text(encoding="utf-8")))
    except (OSError, ReferenceError) as exc:
        print(f"app-config: {exc}", file=sys.stderr)
        return 1
    target = root / OUTPUT
    if args.check:
        current = target.read_text(encoding="utf-8") if target.is_file() else None
        if current != text:
            print(
                f"app-config: {OUTPUT} is stale; run `make app-config` and commit the result",
                file=sys.stderr,
            )
            return 1
        return 0
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
