# SPDX-License-Identifier: Apache-2.0
"""Action catalogue: the single source of actions, classes, event types and ops (ADR 0049).

`docs/spec/actions.yaml` holds every authorization action of chapter 05 with its columns, the
rate-limit classes, the event types with their receive action and the WebSocket ops with their
action. This module loads and validates it and writes its generated copies: the tables of
chapter 05 between `<!-- BEGIN GENERATED: <name> -->` markers and the lists of
`docs/api/redocly.yaml` between `# BEGIN GENERATED: <name> (make catalog)` markers.

Commands (run from anywhere; `--root` defaults to the repository of this file):

    action_catalog.py check     # exit 0 clean, 1 with one finding per line on stderr
    action_catalog.py generate  # rewrite the generated blocks; exit 1 if the catalogue is invalid

Later tickets import this module (M1-03 contract checks, M1-06 module register, M1-25 Go
constants); its public names carry docstrings and stay stable.
"""

from __future__ import annotations

import argparse
import bisect
import os
import posixpath
import re
import reprlib
import stat
import sys
import tempfile
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from functools import cached_property
from pathlib import Path
from types import MappingProxyType
from typing import Any, Literal

import yaml

REPO = Path(__file__).resolve().parents[2]
CATALOG_PATH = Path("docs/spec/actions.yaml")
CHAPTER_PATH = Path("docs/spec/05-actions.md")
REDOCLY_PATH = Path("docs/api/redocly.yaml")
CATALOG_LABEL = "actions.yaml"
# Rule names in findings (`<file>: <rule>: <detail>`), one per rule of tools/catalog/README.md.
RULE_SCHEMA = "schema"
RULE_UNIQUE = "unique"
RULE_SECTION = "section"
RULE_CLASS = "rate-limit-class"
RULE_CONFIG = "rate-limit-config"
RULE_EVENT = "event"
RULE_OP = "op"
RULE_CHANNEL_OP = "channel-op"
RULE_APP_SCOPE = "app-scope"
RULE_EFFECT = "effect"
RULE_BLOCK = "generated-block"
CHANNELS = ("terminal_stream", "host_control", "host_data")
_MD_NAME = r"[a-z][a-z0-9_]*(?:\.[a-z0-9_]+)*"
_MD_BEGIN = re.compile(rf"^<!-- BEGIN GENERATED: (?P<name>{_MD_NAME}) -->$")
_MD_END = re.compile(rf"^<!-- END GENERATED: (?P<name>{_MD_NAME}) -->$")
_MD_PREFIXES = ("<!-- BEGIN GENERATED", "<!-- END GENERATED")
_YAML_NAME = r"[a-z][a-z0-9_]*"
_YAML_BEGIN = re.compile(
    rf"^(?P<indent> *)# BEGIN GENERATED: (?P<name>{_YAML_NAME}) \(make catalog\)$"
)
_YAML_END = re.compile(rf"^(?P<indent> *)# END GENERATED: (?P<name>{_YAML_NAME})$")
_YAML_PREFIXES = ("# BEGIN GENERATED", "# END GENERATED")


def _one_line(text: str) -> str:
    """`text` with its line breaks (every break `str.splitlines` knows) turned into spaces."""
    return " ".join(part.strip() for part in text.splitlines() if part.strip())


class CatalogError(Exception):
    """The catalogue or a generated block is invalid.

    Args:
        findings: One problem each, in the form `<file>: <rule>: <detail>` where a file is
            known; a line break inside one becomes a space, so that each is one line.
            `str(error)` joins them with newlines.

    Attributes:
        findings: The same lines as a list, for callers that report them one by one.
    """

    def __init__(self, findings: Sequence[str]) -> None:
        """Keep the findings, each reduced to one line."""
        self.findings = [_one_line(f) for f in findings]
        super().__init__("\n".join(self.findings))


class CatalogWriteError(CatalogError):
    """`generate` could not write a file; the catalogue and the blocks were valid.

    Args:
        findings: The write error, then one line `old text not restored` for each file that
            kept its new text.
        restored: Whether every file has its old text again.

    Attributes:
        restored: As above.
    """

    def __init__(self, findings: Sequence[str], *, restored: bool) -> None:
        """Keep the findings and whether the files were restored."""
        super().__init__(findings)
        self.restored = restored


# ---- generated-block markers -------------------------------------------------------------


@dataclass(frozen=True)
class _Span:
    begin: int  # index of the BEGIN line
    end: int  # index of the END line
    indent: str


def _spans(
    lines: list[str], begin: re.Pattern[str], end: re.Pattern[str], prefixes: tuple[str, ...]
) -> dict[str, _Span]:
    """Locate every block; any line that looks like a marker must be a well-formed one."""
    spans: dict[str, _Span] = {}
    open_name: str | None = None
    open_at = 0
    open_indent = ""
    for index, raw in enumerate(lines):
        line = raw.rstrip("\n")
        if not line.lstrip().startswith(prefixes):
            continue
        where = f"line {index + 1}"
        if (m := begin.match(line)) is not None:
            name = m.group("name")
            if open_name is not None:
                raise CatalogError([f"{where}: BEGIN of {name!r} nested in {open_name!r}"])
            if name in spans:
                raise CatalogError([f"{where}: duplicate block {name!r}"])
            open_name, open_at, open_indent = name, index, m.groupdict().get("indent") or ""
        elif (m := end.match(line)) is not None:
            name = m.group("name")
            if open_name is None:
                raise CatalogError([f"{where}: END without BEGIN for {name!r}"])
            if name != open_name:
                raise CatalogError(
                    [f"{where}: END of {name!r} does not match BEGIN of {open_name!r}"]
                )
            spans[name] = _Span(open_at, index, open_indent)
            open_name = None
        else:
            raise CatalogError([f"{where}: malformed marker {line.strip()!r}"])
    if open_name is not None:
        raise CatalogError([f"line {open_at + 1}: BEGIN without END for {open_name!r}"])
    return spans


def generated_blocks(text: str) -> dict[str, str]:
    r"""Return the generated blocks of a Markdown text.

    Args:
        text: Markdown with blocks between `<!-- BEGIN GENERATED: <name> -->` and
            `<!-- END GENERATED: <name> -->` lines, the form of docs/spec/06-api.md.

    Returns:
        Block name to body; the body is everything between the two marker lines (each line with
        its newline), the empty string for an empty block.

    Raises:
        CatalogError: A marker is unbalanced, nested, duplicated, mismatched or malformed.

    Example:
        >>> generated_blocks("<!-- BEGIN GENERATED: a -->\nx\n<!-- END GENERATED: a -->\n")
        {'a': 'x\n'}
    """
    lines = text.splitlines(keepends=True)
    spans = _spans(lines, _MD_BEGIN, _MD_END, _MD_PREFIXES)
    return {name: "".join(lines[s.begin + 1 : s.end]) for name, s in spans.items()}


def replace_generated_block(text: str, name: str, body: str) -> str:
    """Replace the body of one Markdown block and keep every other byte of the text.

    Args:
        text: Markdown with generated blocks (see `generated_blocks`).
        name: Name of the block to replace.
        body: New content between the marker lines; empty or ending with a newline.

    Returns:
        The text with the new body.

    Raises:
        CatalogError: The markers are broken, the block does not exist, or the body does not
            end with a newline.
    """
    if body and not body.endswith("\n"):
        raise CatalogError([f"body of block {name!r} must end with a newline"])
    lines = text.splitlines(keepends=True)
    spans = _spans(lines, _MD_BEGIN, _MD_END, _MD_PREFIXES)
    if name not in spans:
        raise CatalogError([f"no generated block {name!r}"])
    span = spans[name]
    return "".join([*lines[: span.begin + 1], body, *lines[span.end :]])


def yaml_generated_blocks(text: str) -> dict[str, tuple[str, list[str]]]:
    """Return the generated list blocks of a YAML text such as docs/api/redocly.yaml.

    Args:
        text: YAML with blocks between `# BEGIN GENERATED: <name> (make catalog)` and
            `# END GENERATED: <name>` comment lines at the indentation of the list items.

    Returns:
        Block name to `(indentation, items)`, items in file order without the `- ` prefix.

    Raises:
        CatalogError: A marker is broken (as in `generated_blocks`) or a line inside a block is
            not a list item at the marker indentation.
    """
    lines = text.splitlines(keepends=True)
    spans = _spans(lines, _YAML_BEGIN, _YAML_END, _YAML_PREFIXES)
    blocks: dict[str, tuple[str, list[str]]] = {}
    for name, span in spans.items():
        items: list[str] = []
        for index in range(span.begin + 1, span.end):
            line = lines[index].rstrip("\n")
            prefix = f"{span.indent}- "
            if not line.startswith(prefix) or line[len(prefix) :].strip() != line[len(prefix) :]:
                raise CatalogError([f"line {index + 1}: not a list item of block {name!r}"])
            items.append(line[len(prefix) :])
        blocks[name] = (span.indent, items)
    return blocks


def replace_yaml_block(text: str, name: str, items: Sequence[str]) -> str:
    """Replace the items of one YAML list block and keep every other byte of the text.

    Args:
        text: YAML with generated list blocks (see `yaml_generated_blocks`).
        name: Name of the block to replace.
        items: New list items, written as `- <item>` at the indentation of the BEGIN marker.

    Returns:
        The text with the new items.

    Raises:
        CatalogError: The markers are broken or the block does not exist.
    """
    lines = text.splitlines(keepends=True)
    spans = _spans(lines, _YAML_BEGIN, _YAML_END, _YAML_PREFIXES)
    if name not in spans:
        raise CatalogError([f"no generated block {name!r}"])
    span = spans[name]
    body = [f"{span.indent}- {item}\n" for item in items]
    return "".join([*lines[: span.begin + 1], *body, *lines[span.end :]])


# ---- data model --------------------------------------------------------------------------

Owner = bool | Literal["as_target"]
StepUp = bool | Literal["conditional", "fresh"]
Audit = Literal["always", "denials", "never", "conditional"]


@dataclass(frozen=True)
class RateLimitClass:
    """One row of 05 "Rate-limit classes".

    Attributes:
        id: Class id, `rl.<name>`.
        key: What the bucket counts by (cell "Key"), verbatim Markdown.
        purpose: Cell "Purpose", verbatim Markdown.
    """

    id: str
    key: str
    purpose: str


@dataclass(frozen=True)
class Action:
    """One action of 05 section 4 with every column and the machine-only fields.

    Attributes:
        id: Action id, for example `message.read`.
        section: Key of the table of section 4 it belongs to (one of `Catalog.sections`).
        description: Cell "Description", verbatim Markdown.
        resource: Cell "Resource", verbatim.
        right: Cell "Permission", verbatim; `system` marks a system action (05 rule 10).
        owner: `True` (`yes`), `False` (`no`) or `"as_target"` (`as target action`).
        step_up: `True`, `False`, `"conditional"` (`conditional`) or `"fresh"` (`fresh`).
        audit: `"always"` (`yes`), `"denials"` (`denials`), `"never"` (`no`) or `"conditional"`.
        audit_text: Verbatim audit cell, set exactly when `audit` is `"conditional"`.
        rights_change: Column "Permissions": the action raises `permission_version` (05 rule 7).
        rl_class: Rate-limit class id, or `"none"` (cell `` `none` ``).
        delivery_check: Internal delivery check (`push.deliver`, `webhook_out.deliver`).
        effect: `"read"` or `"write"` (05 2.7); the maintenance mode uses only this.
        app_scope: Scope an app needs (05 2.6), `"none"` for apps without a scope, `None` when
            2.6 names no scope for the action.
    """

    id: str
    section: str
    description: str
    resource: str
    right: str
    owner: Owner
    step_up: StepUp
    audit: Audit
    audit_text: str | None
    rights_change: bool
    rl_class: str
    delivery_check: bool
    effect: Literal["read", "write"]
    app_scope: str | None

    @property
    def system(self) -> bool:
        """Whether only the server itself runs the action (right `system`, 05 rule 10)."""
        return self.right == SYSTEM_RIGHT


@dataclass(frozen=True)
class Event:
    """One event type of 05 section 6.

    Attributes:
        type: Event type, for example `message.created`.
        kind: `"durable"` or `"ephemeral"` (which of the two tables).
        resource: Cell "Resource".
        receive_action: Machine receive action: a non-system action id or `"self"`; equals
            `x-receive-action` in asyncapi.yaml.
        audience: `"self"` when only the named principal receives it, else `None`.
        receivers: Who can be that principal (`"principal"`, `"agent"`, `"agent_or_app"`);
            `None` without `audience`.
        receive_text: Verbatim receive cell when the cell says more than the action.
        note: Cell "Note" (durable) or "Additional condition" (ephemeral); empty when absent.
        resource_version: Whether the envelope carries `resource.version`: the table of the
            resource has a `version` column and the event reports its new value (04
            "Envelope" and "Delivery guarantee" point 3); required for every event.
    """

    type: str
    kind: Literal["durable", "ephemeral"]
    resource: str
    receive_action: str
    audience: Literal["self"] | None
    receivers: Literal["principal", "agent", "agent_or_app"] | None
    receive_text: str | None
    note: str
    resource_version: bool


@dataclass(frozen=True)
class WsOp:
    """One gateway WebSocket op of 05 section 7 (first table).

    Attributes:
        op: Op name, for example `identify`.
        direction: `"client_to_server"`, `"agent_to_server"`, `"agent_or_app_to_server"` or
            `"server_to_client"`.
        action: Non-system action checked for the op (`x-authz-action` in asyncapi.yaml).
        also_actions: Further actions checked in some cases (`agent.heartbeat` for agents).
        action_text: Verbatim action cell when it says more than the action.
        row_group: Consecutive ops with the same group share one table row.
        per_event_receive_action: Only on `dispatch`: delivery checks the receive action of each
            event type.
    """

    op: str
    direction: str
    action: str
    also_actions: tuple[str, ...]
    action_text: str | None
    row_group: str | None
    per_event_receive_action: bool


@dataclass(frozen=True)
class ChannelOp:
    """One row of the table "Terminal channels" of 05 section 7 (not the gateway).

    Attributes:
        channel: `"terminal_stream"`, `"host_control"` or `"host_data"` (asyncapi channels
            `terminalStream`, `hostControl`, `hostData`).
        channel_text: Verbatim cell "Channel" on the first row of a channel, else `None`.
        ops: Op names of the row; `noise_frame` stands for binary Noise frames.
        direction: `"server_to_device"`, `"device_to_server"`, `"server_to_host"`,
            `"host_to_server"` or `"both"`.
        action: Non-system action (`x-authz-action` of the matching asyncapi operations).
        action_text: Verbatim action cell when it says more than the action.
    """

    channel: str
    channel_text: str | None
    ops: tuple[str, ...]
    direction: str
    action: str
    action_text: str | None


@dataclass(frozen=True)
class Catalog:
    """The whole catalogue in file order.

    Attributes:
        version: Schema version, always 1.
        rate_limit_classes: Classes in the row order of 05 "Rate-limit classes".
        sections: Keys of the action tables of 05 section 4, in order.
        actions: Actions in the row order of 05 section 4.
        events: Durable then ephemeral event types in 05 row order.
        ws_ops: Gateway ops in the row order of the first table of 05 section 7.
        channel_ops: Rows of the table "Terminal channels" of 05 section 7.
    """

    version: int
    rate_limit_classes: tuple[RateLimitClass, ...]
    sections: tuple[str, ...]
    actions: tuple[Action, ...]
    events: tuple[Event, ...]
    ws_ops: tuple[WsOp, ...]
    channel_ops: tuple[ChannelOp, ...]

    @cached_property
    def actions_by_id(self) -> Mapping[str, Action]:
        """Read-only mapping from action id to action."""
        return MappingProxyType({a.id: a for a in self.actions})

    @cached_property
    def events_by_type(self) -> Mapping[str, Event]:
        """Read-only mapping from event type to event."""
        return MappingProxyType({e.type: e for e in self.events})

    @cached_property
    def ws_ops_by_name(self) -> Mapping[str, WsOp]:
        """Read-only mapping from gateway op name to op."""
        return MappingProxyType({o.op: o for o in self.ws_ops})

    @cached_property
    def classes_by_id(self) -> Mapping[str, RateLimitClass]:
        """Read-only mapping from rate-limit class id to class."""
        return MappingProxyType({c.id: c for c in self.rate_limit_classes})


TOP_KEYS = (
    "version",
    "rate_limit_classes",
    "sections",
    "actions",
    "events",
    "ws_ops",
    "channel_ops",
)
APP_SCOPES = (
    "messages:read",
    "messages:write",
    "files:read",
    "files:write",
    "commands:write",
    "components:write",
    "users:read",
    "events:subscribe",
    "costs:reconcile",
    "none",
)
SELF = "self"
NONE_CLASS = "none"
SYSTEM_RIGHT = "system"
# `\Z`, not `$`: `$` also matches before a final line break, which would end the table row.
_ID_PATTERN = re.compile(r"^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+\Z")
_CLASS_PATTERN = re.compile(r"^rl\.[a-z][a-z0-9_]*\Z")
_KEY_PATTERN = re.compile(r"^[a-z][a-z0-9_]*\Z")
_OP_PATTERN = re.compile(r"^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*\Z")
# The pieces Python-Markdown's `tables` reads in a row: an escaped backslash, a backtick run
# escaped by one backslash, a backtick run, an escaped `|` and a `|`.
_ROW_PIECES = re.compile(r"(\\\\)|(\\`+)|(`+)|(\\\|)|(\|)")


def _cell_problem(text: str) -> str | None:
    """Why `text` would not stay one cell of a generated table on the site, else `None`.

    The model of how Python-Markdown's `tables` splits a row (`TableProcessor._split`): it reads
    an escaped backslash, an escaped `|`, and a backslash before backticks as escaping one of
    them; it pairs each backtick run with the next later run of the same length (an escaped run
    opens with one backtick less) and splits at every `|` outside a pair. A cell stays one cell
    in any row exactly when every run it opens is closed inside it and no `|` stands outside a
    pair, because the row then pairs and splits it as it does the cell alone. A test compares
    this model with the renderer, and one compares every generated table cell by cell with the
    catalogue. Runs are looked up by length, not searched again, so the check stays linear.
    A cell that stays one cell also holds no markup the site does not show as its text: no
    HTML and no brace pair outside a code span (an attribute list), read like the chapter
    outside the blocks (`_without_code`, `_HTML_START`, `_ATTRIBUTE_LIST`).
    """
    if text.splitlines() != [text]:
        return "holds a line break, but a table cell has one line"
    starts: list[int] = []
    ends: list[int] = []
    opening: list[int] = []
    by_length: dict[int, list[int]] = {}
    pipes: list[int] = []
    for m in _ROW_PIECES.finditer(text):
        if m.group(2) or m.group(3):
            escaped = 1 if m.group(2) else 0
            by_length.setdefault(m.end() - m.start() - escaped, []).append(len(starts))
            opening.append(m.end() - m.start() - 2 * escaped)
            starts.append(m.start() + escaped)
            ends.append(m.end())
        elif m.group(5):
            pipes.append(m.start())
    pairs: list[tuple[int, int]] = []
    problems: list[tuple[int, str]] = []
    n = 0
    while n < len(opening):
        same = by_length.get(opening[n], []) if opening[n] else []
        if (k := bisect.bisect_right(same, n)) < len(same):
            pairs.append((starts[n], ends[same[k]]))
            n = same[k] + 1
            continue
        if opening[n] and not problems:
            problems.append(
                (
                    starts[n],
                    f"the backtick run at character {starts[n] + 1} opens a code span that no"
                    " run of the same length in the cell closes, so on the site it would close"
                    " in a later cell",
                )
            )
        n += 1
    k = 0
    for pipe in pipes:  # pipes and pairs are both in text order
        while k < len(pairs) and pairs[k][1] <= pipe:
            k += 1
        if k == len(pairs) or pipe < pairs[k][0]:
            problems.append(
                (
                    pipe,
                    f"the | at character {pipe + 1} ends the cell on the site (escape it with an"
                    " odd number of backslashes or put it into a code span)",
                )
            )
            break
    if problems:
        return "; ".join(message for _, message in sorted(problems))
    bare = _without_code(text)
    markup = [
        message
        for pattern, message in (
            (
                _HTML_START,
                "holds HTML outside a code span (a < before a letter, !, ? or /), which the"
                " site does not show as text of the cell",
            ),
            (
                _ATTRIBUTE_LIST,
                "holds a brace pair outside a code span, which the site reads as an attribute"
                " list of the cell",
            ),
        )
        if pattern.search(bare)
    ]
    return "; ".join(markup) or None


class _Entry:
    """Typed, schema-checked access to one YAML mapping; problems become schema findings."""

    def __init__(
        self,
        value: object,
        where: str,
        required: Sequence[str],
        optional: Sequence[str],
        findings: list[str],
    ) -> None:
        self.where = where
        self.findings = findings
        self.data: dict[str, Any] = {}
        if not isinstance(value, dict):
            self.fail("must be a mapping")
            return
        self.data = value
        for key in value:
            if key not in required and key not in optional:
                self.fail(f"unknown key {key!r}")
        for key in required:
            if key not in value:
                self.fail(f"missing key {key!r}")

    def fail(self, message: str) -> None:
        self.findings.append(f"{CATALOG_LABEL}: {RULE_SCHEMA}: {self.where}: {message}")

    def text(self, key: str, *, column: str | None = None) -> str | None:
        """A non-empty string, or with `column` one cell of a table in 05.

        With `column`, the value is a cell of that column of a table in 05, which must stay one
        cell on the site (`_cell_problem`).
        """
        if key not in self.data:
            return None
        value = self.data[key]
        if not isinstance(value, str) or not value.strip():
            self.fail(f"{key} must be a non-empty string")
            return None
        if value != value.strip():
            self.fail(f"{key} must not start or end with whitespace")
        if column is not None and (problem := _cell_problem(value)) is not None:
            self.fail(f"{key} (column {column}): {problem}")
        return value

    def name(self, key: str, pattern: re.Pattern[str], label: str) -> str | None:
        value = self.text(key)
        if value is not None and not pattern.match(value):
            self.fail(f"{label} {value!r} does not match {pattern.pattern}")
        return value

    def flag(self, key: str, *, default: bool = False) -> bool:
        if key not in self.data:
            return default
        value = self.data[key]
        if type(value) is not bool:
            self.fail(f"{key} must be a boolean")
            return default
        return value

    def choice(self, key: str, options: Sequence[object]) -> Any:
        """One of `options`; booleans compare by identity, so `1` never passes for `true`."""
        if key not in self.data:
            return None
        value = self.data[key]
        for option in options:
            if (
                (value is option)
                if isinstance(option, bool)
                else (type(value) is str and value == option)
            ):
                return value
        self.fail(f"{key} must be one of {', '.join(map(str, options))}")
        return None

    def names(self, key: str, pattern: re.Pattern[str]) -> tuple[str, ...]:
        if key not in self.data:
            return ()
        value = self.data[key]
        if not isinstance(value, list) or not value:
            self.fail(f"{key} must be a non-empty list")
            return ()
        out: list[str] = []
        for item in value:
            if not isinstance(item, str) or not pattern.match(item):
                self.fail(f"{key} item {_short_repr(item)} does not match {pattern.pattern}")
            else:
                out.append(item)
        return tuple(out)


# The repr of a YAML value in a finding: bounded in depth and length, because aliases can nest
# a value exponentially deep and wide.
_short_repr = reprlib.Repr(maxlevel=3, maxstring=120, maxother=120).repr


def _label(value: object, key: str, fallback: str) -> str:
    """Name of an entry in findings: its id if it has a usable one, else its list position."""
    if isinstance(value, dict) and isinstance(value.get(key), str) and value[key]:
        return str(value[key])
    return fallback


def _items(top: Mapping[str, Any], key: str, findings: list[str]) -> list[Any]:
    if key not in top:
        return []  # reported as a missing key by _parse_top
    value = top[key]
    if not isinstance(value, list):
        findings.append(f"{CATALOG_LABEL}: {RULE_SCHEMA}: {key} must be a list")
        return []
    return value


def _parse_class(value: object, where: str, findings: list[str]) -> RateLimitClass | None:
    e = _Entry(value, where, ("id", "key", "purpose"), (), findings)
    ident = e.name("id", _CLASS_PATTERN, "class id")
    key, purpose = e.text("key", column="Key"), e.text("purpose", column="Purpose")
    if ident is None or key is None or purpose is None:
        return None
    return RateLimitClass(ident, key, purpose)


_ACTION_REQUIRED = (
    "id",
    "section",
    "description",
    "resource",
    "right",
    "owner",
    "step_up",
    "audit",
    "rights_change",
    "rl_class",
    "effect",
)


def _parse_action(value: object, where: str, findings: list[str]) -> Action | None:
    e = _Entry(
        value, where, _ACTION_REQUIRED, ("audit_text", "delivery_check", "app_scope"), findings
    )
    before = len(findings)
    audit = e.choice("audit", ("always", "denials", "never", "conditional"))
    audit_text = e.text("audit_text", column="Audit")
    if audit == "conditional" and audit_text is None:
        e.fail("audit conditional needs audit_text")
    if audit not in (None, "conditional") and "audit_text" in e.data:
        e.fail("audit_text only with audit conditional")
    right = e.text("right", column="Permission")
    if right is not None and right != SYSTEM_RIGHT and re.search(r"\bsystem\b", right, re.I):
        e.fail("right names system but is not exactly system")
    action = Action(
        id=e.name("id", _ID_PATTERN, "id") or "",
        section=e.name("section", _KEY_PATTERN, "section") or "",
        description=e.text("description", column="Description") or "",
        resource=e.text("resource", column="Resource") or "",
        right=right or "",
        owner=e.choice("owner", (True, False, "as_target")),
        step_up=e.choice("step_up", (True, False, "conditional", "fresh")),
        audit=audit,
        audit_text=audit_text,
        rights_change=e.flag("rights_change"),
        rl_class=e.text("rl_class") or "",
        delivery_check=e.flag("delivery_check"),
        effect=e.choice("effect", ("read", "write")),
        app_scope=e.choice("app_scope", APP_SCOPES),
    )
    return action if len(findings) == before and e.data else None


def _parse_event(value: object, where: str, findings: list[str]) -> Event | None:
    optional = ("receive_action", "audience", "receivers", "receive_text", "note")
    e = _Entry(
        value, where, ("type", "kind", "resource"), (*optional, "resource_version"), findings
    )
    before = len(findings)
    kind = e.choice("kind", ("durable", "ephemeral"))
    if e.data and "resource_version" not in e.data:
        e.fail(f"{kind or 'every'} event needs resource_version")
    audience = e.choice("audience", (SELF,))
    receivers = e.choice("receivers", ("principal", "agent", "agent_or_app"))
    event = Event(
        type=e.name("type", _ID_PATTERN, "type") or "",
        kind=kind,
        resource=e.text("resource", column="Resource") or "",
        receive_action=e.text("receive_action") or "",
        audience=audience,
        receivers=receivers or ("principal" if audience == SELF else None),
        receive_text=e.text("receive_text", column="Receive action"),
        note=e.text("note", column="Additional condition" if kind == "ephemeral" else "Note") or "",
        resource_version=e.flag("resource_version"),
    )
    return event if len(findings) == before and e.data else None


def _parse_ws_op(value: object, where: str, findings: list[str]) -> WsOp | None:
    optional = ("action", "also_actions", "action_text", "row_group", "per_event_receive_action")
    e = _Entry(value, where, ("op", "direction"), optional, findings)
    before = len(findings)
    op = WsOp(
        op=e.name("op", _OP_PATTERN, "op") or "",
        direction=e.choice("direction", tuple(DIRECTION_TEXT)) or "",
        action=e.text("action") or "",
        also_actions=e.names("also_actions", _ID_PATTERN),
        action_text=e.text("action_text", column="Action"),
        row_group=e.name("row_group", _KEY_PATTERN, "row_group"),
        per_event_receive_action=e.flag("per_event_receive_action"),
    )
    return op if len(findings) == before and e.data else None


def _parse_channel_op(value: object, where: str, findings: list[str]) -> ChannelOp | None:
    e = _Entry(
        value,
        where,
        ("channel", "ops", "direction"),
        ("channel_text", "action", "action_text"),
        findings,
    )
    before = len(findings)
    row = ChannelOp(
        channel=e.choice("channel", CHANNELS) or "",
        channel_text=e.text("channel_text", column="Channel"),
        ops=e.names("ops", _OP_PATTERN),
        direction=e.choice("direction", tuple(CHANNEL_DIRECTION_TEXT)) or "",
        action=e.text("action") or "",
        action_text=e.text("action_text", column="Action"),
    )
    return row if len(findings) == before and e.data else None


def _parse_top(data: object, findings: list[str]) -> dict[str, Any]:
    if not isinstance(data, dict):
        findings.append(f"{CATALOG_LABEL}: {RULE_SCHEMA}: top level must be a mapping")
        return {}
    prefix = f"{CATALOG_LABEL}: {RULE_SCHEMA}:"
    findings += [f"{prefix} unknown key {k!r}" for k in data if k not in TOP_KEYS]
    findings += [f"{prefix} missing key {k!r}" for k in TOP_KEYS if k not in data]
    if list(data) != list(TOP_KEYS) and set(data) == set(TOP_KEYS):
        findings.append(f"{prefix} top-level keys must be in the order {', '.join(TOP_KEYS)}")
    version = data.get("version")
    if type(version) is not int or version != 1:
        findings.append(f"{prefix} version must be 1")
    return data


def _parse_sections(top: Mapping[str, Any], findings: list[str]) -> tuple[str, ...]:
    if "sections" not in top:
        return ()  # reported as a missing key by _parse_top
    value = top["sections"]
    prefix = f"{CATALOG_LABEL}: {RULE_SCHEMA}: sections"
    if not isinstance(value, list) or not value:
        findings.append(f"{prefix} must be a non-empty list")
        return ()
    bad = [s for s in value if not isinstance(s, str) or not _KEY_PATTERN.match(s)]
    findings += [f"{prefix}: {_short_repr(s)} does not match {_KEY_PATTERN.pattern}" for s in bad]
    return tuple(s for s in value if s not in bad)


def catalog_from_data(data: object) -> Catalog:
    """Build a `Catalog` from the parsed YAML structure of actions.yaml.

    Schema findings come first; the relational rules (uniqueness, sections, classes, events,
    ops) run only on a schema-clean structure, because they would mostly repeat its problems.

    Args:
        data: The value `yaml.safe_load` returns for the file.

    Returns:
        The catalogue.

    Raises:
        CatalogError: With every finding of the first failing stage, each in the form
            `actions.yaml: <rule>: <detail>`.
    """
    findings: list[str] = []
    top = _parse_top(data, findings)
    sections = _parse_sections(top, findings)

    def parse(key: str, ident: str, parser: Any) -> tuple[Any, ...]:
        out = []
        for n, value in enumerate(_items(top, key, findings)):
            parsed = parser(value, _label(value, ident, f"{key}[{n}]"), findings)
            if parsed is not None:
                out.append(parsed)
        return tuple(out)

    catalog = Catalog(
        version=1,
        rate_limit_classes=parse("rate_limit_classes", "id", _parse_class),
        sections=sections,
        actions=parse("actions", "id", _parse_action),
        events=parse("events", "type", _parse_event),
        ws_ops=parse("ws_ops", "op", _parse_ws_op),
        channel_ops=parse("channel_ops", "channel", _parse_channel_op),
    )
    if findings:
        raise CatalogError(findings)
    findings = relational_findings(catalog)
    if findings:
        raise CatalogError(findings)
    return catalog


def _duplicates(values: Sequence[str]) -> list[str]:
    seen: set[str] = set()
    dupes: list[str] = []
    for value in values:
        if value in seen and value not in dupes:
            dupes.append(value)
        seen.add(value)
    return dupes


def _finding(rule: str, detail: str) -> str:
    return f"{CATALOG_LABEL}: {rule}: {detail}"


def _unique_findings(c: Catalog) -> list[str]:
    checks = (
        ("class id", [x.id for x in c.rate_limit_classes]),
        ("action id", [x.id for x in c.actions]),
        ("event type", [x.type for x in c.events]),
        ("op", [x.op for x in c.ws_ops]),
    )
    return [
        _finding(RULE_UNIQUE, f"duplicate {what} {value!r}")
        for what, values in checks
        for value in _duplicates(values)
    ]


def _section_findings(c: Catalog) -> list[str]:
    out = [
        _finding(RULE_SECTION, f"section {s!r} is listed twice") for s in _duplicates(c.sections)
    ]
    order = {s: n for n, s in reversed(list(enumerate(c.sections)))}
    used = {a.section for a in c.actions}
    out += [_finding(RULE_SECTION, f"section {s!r} has no action") for s in order if s not in used]
    previous: Action | None = None
    for a in c.actions:
        if a.section not in order:
            out.append(_finding(RULE_SECTION, f"{a.id}: section {a.section!r} is not listed"))
            continue
        if previous is not None and order[a.section] < order[previous.section]:
            out.append(
                _finding(
                    RULE_SECTION,
                    f"{a.id}: section {a.section!r} after {previous.section!r} breaks the order"
                    " of sections",
                )
            )
        previous = a
    return out


def _class_findings(c: Catalog) -> list[str]:
    out: list[str] = []
    known = {x.id for x in c.rate_limit_classes}
    for a in c.actions:
        if a.rl_class != NONE_CLASS and a.rl_class not in known:
            out.append(_finding(RULE_CLASS, f"{a.id}: unknown class {a.rl_class!r}"))
        if a.system and a.delivery_check:
            out.append(_finding(RULE_CLASS, f"{a.id}: delivery_check on a system action"))
        exempt = a.system or a.delivery_check
        if a.rl_class == NONE_CLASS and not exempt:
            out.append(
                _finding(
                    RULE_CLASS, f"{a.id}: class none only for system actions and delivery checks"
                )
            )
        if a.rl_class != NONE_CLASS and exempt:
            out.append(
                _finding(RULE_CLASS, f"{a.id}: needs class none (system action or delivery check)")
            )
    used = {a.rl_class for a in c.actions}
    out += [
        _finding(RULE_CLASS, f"class {x.id!r} is used by no action")
        for x in c.rate_limit_classes
        if x.id not in used
    ]
    return out


def _names(text: str | None, action: str) -> bool:
    """Whether a verbatim cell names `action` (backticked) or, for `self`, the word self."""
    if text is None:
        return True
    if action == SELF:
        return re.search(r"\bself\b", text) is not None
    return f"`{action}`" in text


def _action_ref(c: Catalog, rule: str, where: str, action: str) -> list[str]:
    known = c.actions_by_id.get(action)
    if known is None:
        return [_finding(rule, f"{where}: unknown action {action!r}")]
    if known.system:
        return [_finding(rule, f"{where}: action {action!r} is a system action")]
    return []


def _event_findings(c: Catalog) -> list[str]:
    out: list[str] = []
    kinds = [e.kind for e in c.events]
    if "ephemeral" in kinds and "durable" in kinds[kinds.index("ephemeral") :]:
        out.append(_finding(RULE_EVENT, "durable event types must come before ephemeral ones"))
    for e in c.events:
        where = e.type
        if not e.receive_action:
            out.append(_finding(RULE_EVENT, f"{where}: event type without receive action"))
            continue
        if e.receive_action != SELF:
            known = c.actions_by_id.get(e.receive_action)
            if known is None:
                out.append(
                    _finding(RULE_EVENT, f"{where}: unknown receive action {e.receive_action!r}")
                )
            elif known.system:
                out.append(
                    _finding(
                        RULE_EVENT,
                        f"{where}: receive action {e.receive_action!r} is a system action",
                    )
                )
        if e.receive_action == SELF and e.audience != SELF:
            out.append(_finding(RULE_EVENT, f"{where}: receive action self needs audience self"))
        if e.audience != SELF and e.receivers is not None:
            out.append(_finding(RULE_EVENT, f"{where}: receivers only with audience self"))
        if not _names(e.receive_text, e.receive_action):
            out.append(
                _finding(RULE_EVENT, f"{where}: receive_text does not name {e.receive_action}")
            )
        starts_self = receive_cell(e).startswith(SELF)
        if starts_self and e.audience != SELF:
            out.append(
                _finding(
                    RULE_EVENT, f"{where}: receive cell starts with self but audience is not self"
                )
            )
        if not starts_self and e.audience == SELF:
            out.append(
                _finding(
                    RULE_EVENT, f"{where}: audience self needs a receive cell starting with self"
                )
            )
    return out


def _ws_op_findings(c: Catalog) -> list[str]:
    out: list[str] = []
    for o in c.ws_ops:
        if not o.action:
            out.append(_finding(RULE_OP, f"{o.op}: op without action"))
            continue
        for action in (o.action, *o.also_actions):
            out += _action_ref(c, RULE_OP, o.op, action)
        if o.per_event_receive_action and o.op != "dispatch":
            out.append(_finding(RULE_OP, f"{o.op}: per_event_receive_action only on dispatch"))
        if o.also_actions and o.action_text is None:
            out.append(_finding(RULE_OP, f"{o.op}: also_actions need action_text"))
        if o.per_event_receive_action and o.action_text is None:
            out.append(_finding(RULE_OP, f"{o.op}: per_event_receive_action needs action_text"))
        named = o.also_actions if o.per_event_receive_action else (o.action, *o.also_actions)
        for action in named:
            if not _names(o.action_text, action):
                out.append(_finding(RULE_OP, f"{o.op}: action_text does not name {action!r}"))
    groups: dict[str, list[int]] = {}
    for n, o in enumerate(c.ws_ops):
        if o.row_group is not None:
            groups.setdefault(o.row_group, []).append(n)
    for group, indexes in groups.items():
        if indexes != list(range(indexes[0], indexes[0] + len(indexes))):
            out.append(_finding(RULE_OP, f"row group {group!r} is not consecutive"))
        shared = {
            (o.direction, o.action, o.action_text, o.also_actions, o.per_event_receive_action)
            for o in (c.ws_ops[n] for n in indexes)
        }
        if len(shared) > 1:
            out.append(
                _finding(
                    RULE_OP,
                    f"row group {group!r} mixes direction, action, action_text, also_actions or"
                    " per_event_receive_action",
                )
            )
    return out


def _channel_op_findings(c: Catalog) -> list[str]:
    out: list[str] = []
    seen_channels: list[str] = []
    seen_ops: set[tuple[str, str, str]] = set()
    for row in c.channel_ops:
        where = row.channel
        first = not seen_channels or seen_channels[-1] != row.channel
        if first and row.channel in seen_channels:
            out.append(
                _finding(RULE_CHANNEL_OP, f"rows of channel {row.channel!r} are not consecutive")
            )
        if first and row.channel_text is None:
            out.append(
                _finding(RULE_CHANNEL_OP, f"{where}: first row of a channel needs channel_text")
            )
        if not first and row.channel_text is not None:
            out.append(
                _finding(
                    RULE_CHANNEL_OP, f"{where}: channel_text only on the first row of a channel"
                )
            )
        if first:
            seen_channels.append(row.channel)
        for op in row.ops:
            key = (row.channel, op, row.direction)
            if key in seen_ops:
                out.append(
                    _finding(
                        RULE_CHANNEL_OP, f"{where}: op {op!r} twice for direction {row.direction}"
                    )
                )
            seen_ops.add(key)
        if not row.action:
            out.append(_finding(RULE_CHANNEL_OP, f"{where}: channel op without action"))
            continue
        out += _action_ref(c, RULE_CHANNEL_OP, where, row.action)
        if not _names(row.action_text, row.action):
            out.append(
                _finding(RULE_CHANNEL_OP, f"{where}: action_text does not name {row.action!r}")
            )
    return out


def relational_findings(catalog: Catalog) -> list[str]:
    """Rules `unique`, `section`, `rate-limit-class`, `event`, `op` and `channel-op`.

    These are the relational rules of tools/catalog/README.md; they run on a schema-clean
    catalogue.

    Args:
        catalog: The catalogue as built from the YAML.

    Returns:
        One finding per problem, in the form `actions.yaml: <rule>: <detail>`.
    """
    return [
        *_unique_findings(catalog),
        *_section_findings(catalog),
        *_class_findings(catalog),
        *_event_findings(catalog),
        *_ws_op_findings(catalog),
        *_channel_op_findings(catalog),
    ]


# ---- hand-written tables of chapter 05 ---------------------------------------------------

_TICKED = re.compile(r"`([^`]+)`")


def _section(chapter: str, title: str) -> str:
    """Body of the heading `<number> <title>` up to the next heading of its level or higher.

    The body ends at the next line starting with as many `#` or fewer (like Markdown, also
    without a space after them).

    Sections are found by title, never by number, because numbers move (PLAN "Notation").
    No other heading of any level has a title with the same letters (`_title_keys`), so that no
    copy can hold what the site shows under the real one. A setext heading or a heading in a list
    or quote does not end the section here; the test of the rendered page sees those.
    """
    heading = re.compile(
        rf"^(?P<hashes>#{{2,4}}) [0-9]+(?:\.[0-9]+)*\.? {re.escape(title)}[ \t]*$", re.MULTILINE
    )
    m = heading.search(chapter)
    if m is None:
        raise CatalogError([f"section {title!r} not found in chapter 05"])
    keys = _title_keys(title)
    alike = [
        (n + 1, h[1])
        for n, h in enumerate(_headings(chapter.splitlines()))
        if h is not None and _title_keys(h[1]) & keys
    ]
    if len(alike) > 1 and all(found == title for _, found in alike):
        raise CatalogError([f"section {title!r} appears {len(alike)} times in chapter 05"])
    if len(alike) > 1:
        # Titles that differ only in what does not count would read as one title twice.
        shown = ", ".join(f"{found!r} (line {line})" for line, found in alike)
        raise CatalogError(
            [
                f"section {title!r}: headings {shown} in chapter 05 look the same (only the"
                " letters of a title count, without case, with I as l, 1 as l and 0 as o)"
            ]
        )
    body = chapter[m.end() :]
    nxt = re.search(rf"^#{{1,{len(m.group('hashes'))}}}(?!#)", body, re.MULTILINE)
    return body[: nxt.start()] if nxt else body


# Header rows of the hand-written tables the checks read.
SCOPE_HEADER = ("Actions", "Scope")
CONFIG_HEADER = ("Name", "Default", "Meaning")
_SEPARATOR_ROW = re.compile(r"^\|(?:[ \t]*:?-+:?[ \t]*\|)+[ \t]*$")


def _table_rows(
    body: str, label: str, header: tuple[str, ...]
) -> tuple[list[list[str]], list[str]]:
    """Cells of the data rows of the one Markdown table in `body`, plus problems of its form.

    Fail closed: the section holds exactly one run of lines starting with `|`, its first line
    is `header` and its second line a separator row with as many columns (surrounding
    whitespace removed as Markdown does: tabs expanded, then spaces); otherwise the site could
    show another table, or a data row as the header, than the one read here. A test compares
    the whole table with the rendered one.
    """
    runs: list[list[str]] = []
    in_table = False
    for line in body.splitlines():
        if line.startswith("|"):
            if not in_table:
                runs.append([])
            runs[-1].append(line)
        in_table = line.startswith("|")
    problems = [] if len(runs) == 1 else [f"05 {label} must hold one table, found {len(runs)}"]
    if not runs:
        return [], problems
    lines = runs[0]
    cells = tuple(c.strip() for c in lines[0].expandtabs(4).strip(" ").strip("|").split("|"))
    if cells != header:
        problems.append(f"05 {label}: header {list(cells)} differs from {list(header)}")
    rule = lines[1].expandtabs(4).strip(" ") if len(lines) > 1 else ""
    if not _SEPARATOR_ROW.match(rule) or rule.count("|") - 1 != len(header):
        problems.append(
            f"05 {label}: the second line of the table is no separator row for {len(header)}"
            " columns"
        )
    return [[c.strip() for c in line.strip().strip("|").split("|")] for line in lines[2:]], problems


def _scope_table(chapter: str, action_ids: Sequence[str]) -> tuple[dict[str, str], list[str]]:
    """Action id to scope from 05 2.6, plus the problems of the table itself."""
    known = set(action_ids)
    result: dict[str, str] = {}
    rows, problems = _table_rows(_section(chapter, "Scopes for apps"), "2.6", SCOPE_HEADER)
    for cells in rows:
        scopes = _TICKED.findall(cells[1]) if len(cells) > 1 else []
        scope = scopes[0] if scopes else NONE_CLASS
        if scope not in APP_SCOPES:
            problems.append(f"05 2.6 names unknown scope {scope!r}")
        for token in _TICKED.findall(cells[0]):
            if token.endswith(".*"):
                matched = [i for i in action_ids if i.startswith(token[:-1])]
                if not matched:
                    problems.append(f"05 2.6 pattern {token!r} matches no action")
            elif token in known:
                matched = [token]
            else:
                problems.append(f"05 2.6 names unknown action {token!r}")
                matched = []
            for action_id in matched:
                if action_id in result:
                    problems.append(f"05 2.6 names {action_id!r} twice")
                result[action_id] = scope
    return result, problems


def scope_assignments(chapter: str, action_ids: Sequence[str]) -> dict[str, str]:
    """Read 05 "Scopes for apps" (2.6): which action needs which app scope.

    Column one: every backticked token is an action id or a pattern ending in `.*` (prefix
    match); prose without backticks is ignored. Column two: the first backticked token is the
    scope; a cell without one means `none`.

    Args:
        chapter: Text of docs/spec/05-actions.md.
        action_ids: Every action id of the catalogue (to expand patterns).

    Returns:
        Action id to scope for every action the table names.

    Raises:
        CatalogError: The section is missing, another heading shows the letters of its title,
            or it does
            not hold exactly one table with the header `SCOPE_HEADER` and a separator row of
            as many columns; or
            the table names an unknown action or scope, a pattern that matches nothing, or one
            action in two rows.
    """
    result, problems = _scope_table(chapter, action_ids)
    if problems:
        raise CatalogError(problems)
    return result


# Any list item Markdown sees, also nested (`*`, `+`, `-` or a number after any indentation,
# then a space or tab); the reader takes only `- ` in the first column.
_LIST_ITEM = re.compile(r"^[ \t]*(?:[*+-]|[0-9]+[.)])[ \t]")


def _bullets(body: str, label: str) -> tuple[list[str], list[str]]:
    """Top-level list items of `body` (continuation lines joined with a space), plus problems.

    Fail closed: every list item starts with `- ` in the first column and holds no heading;
    another marker, an indented or nested item would show an item on the site that this reader
    reads otherwise or not at all. Items after a blank line belong to the same loose list on
    the site and are read too. A test compares the items with the rendered list.
    """
    items: list[str] = []
    problems: list[str] = []
    for line in body.splitlines():
        if line.startswith("- "):
            items.append(line[2:].strip())
            if items[-1].startswith("#"):
                problems.append(f"05 {label}: a list item holds a heading: {line.strip()!r}")
        elif _LIST_ITEM.match(line):
            problems.append(f"05 {label} uses a list marker other than '- ': {line.strip()!r}")
        elif items and line.startswith("  ") and line.strip():
            items[-1] += " " + line.strip()
    return items, problems


def _effect_list(chapter: str, action_ids: Sequence[str]) -> tuple[set[str], list[str]]:
    """Ids with effect `read` from 05 2.7, plus the unknown ids the list names."""
    known = set(action_ids)
    suffixes: list[str] = []
    named: set[str] = set()
    excluded: set[str] = set()
    items, problems = _bullets(_section(chapter, "Effect (`effect`)"), "2.7")
    for item in items:
        head, *rest = re.split(r"\bexcept\b", item, maxsplit=1)
        tail = rest[0] if rest else ""
        for token in _TICKED.findall(head):
            if token.startswith("."):
                suffixes.append(token)
            elif _ID_PATTERN.match(token):
                named.add(token)
        excluded |= {t for t in _TICKED.findall(tail) if _ID_PATTERN.match(t)}
    problems += [f"05 2.7 names unknown action {i!r}" for i in sorted((named | excluded) - known)]
    by_suffix = {i for i in action_ids if suffixes and i.endswith(tuple(suffixes))}
    return ((by_suffix | named) - excluded) & known, problems


def read_effect_ids(chapter: str, action_ids: Sequence[str]) -> set[str]:
    """Derive the actions with effect `read` from 05 "Effect (`effect`)" (2.7).

    In each list item, a backticked token starting with `.` is an id suffix (`.read`, `.list`),
    a backticked action id is named explicitly, and action ids after the word `except` are
    excluded. Every other action has effect `write`.

    Args:
        chapter: Text of docs/spec/05-actions.md.
        action_ids: Every action id of the catalogue.

    Returns:
        The ids of the actions whose effect is `read`.

    Raises:
        CatalogError: The section is missing, another heading shows the letters of its title, it
            holds a
            list item that does not start with `- ` in the first column or holds a heading, or
            it names an action that does not exist.
    """
    read, problems = _effect_list(chapter, action_ids)
    if problems:
        raise CatalogError(problems)
    return read


_CONFIG_ROW = re.compile(r"`ratelimit\.(?P<cls>rl\.[a-z][a-z0-9_]*)\.per_minute` / `\.burst`")


def _config_findings(catalog: Catalog, chapter: str) -> list[str]:
    """Rule `rate-limit-config`: the configuration table of 05 has exactly one row per class."""
    named: list[str] = []
    rows, problems = _table_rows(
        _section(chapter, "Configuration values of this chapter"),
        "configuration table",
        CONFIG_HEADER,
    )
    out = [_finding(RULE_CONFIG, p) for p in problems]
    for cells in rows:
        if "ratelimit." not in cells[0]:
            continue
        m = _CONFIG_ROW.fullmatch(cells[0])
        if m is None:
            out.append(
                _finding(
                    RULE_CONFIG,
                    f"malformed rate-limit row {cells[0]!r}; expected "
                    "`ratelimit.rl.<name>.per_minute` / `.burst`",
                )
            )
        else:
            named.append(m.group("cls"))
    classes = [c.id for c in catalog.rate_limit_classes]
    out += [_finding(RULE_CONFIG, f"class {n!r} has more than one row") for n in _duplicates(named)]
    out += [
        _finding(RULE_CONFIG, f"class {c!r} has no row `ratelimit.{c}.per_minute` / `.burst`")
        for c in classes
        if c not in named
    ]
    out += [
        _finding(RULE_CONFIG, f"05 configuration table names unknown class {n!r}")
        for n in dict.fromkeys(named)
        if n not in classes
    ]
    return out


def _scope_findings(catalog: Catalog, chapter: str) -> list[str]:
    """Rule `app-scope`: every app_scope equals 05 2.6 and every action 2.6 names has it."""
    table, problems = _scope_table(chapter, [a.id for a in catalog.actions])
    out = [_finding(RULE_APP_SCOPE, p) for p in problems]
    for a in catalog.actions:
        wanted = table.get(a.id)
        if a.app_scope == wanted:
            continue
        if wanted is None:
            detail = f"{a.id}: app_scope {a.app_scope!r} but 05 2.6 names no scope for it"
        else:
            detail = f"{a.id}: app_scope {a.app_scope!r} differs from 05 2.6 ({wanted!r})"
        out.append(_finding(RULE_APP_SCOPE, detail))
    return out


def _effect_findings(catalog: Catalog, chapter: str) -> list[str]:
    """Every effect equals the closed list of 05 2.7."""
    read, problems = _effect_list(chapter, [a.id for a in catalog.actions])
    out = [_finding(RULE_EFFECT, p) for p in problems]
    for a in catalog.actions:
        wanted = "read" if a.id in read else "write"
        if a.effect != wanted:
            out.append(
                _finding(RULE_EFFECT, f"{a.id}: effect {a.effect} differs from 05 2.7 ({wanted})")
            )
    return out


def chapter_findings(catalog: Catalog, chapter: str) -> list[str]:
    """Compare the catalogue with the hand-written parts of chapter 05.

    Covers the rules `rate-limit-config` (configuration rows of the classes), `app-scope`
    (scope table 2.6) and `effect` (effect list 2.7) of tools/catalog/README.md; the generated
    blocks are compared by `check`.

    Args:
        catalog: A valid catalogue.
        chapter: Text of docs/spec/05-actions.md.

    Returns:
        One finding per problem, in the form `actions.yaml: <rule>: <detail>`.
    """
    out: list[str] = []
    for rule, compare in (
        (RULE_CONFIG, _config_findings),
        (RULE_APP_SCOPE, _scope_findings),
        (RULE_EFFECT, _effect_findings),
    ):
        try:
            out += compare(catalog, chapter)
        except CatalogError as err:
            out += [_finding(rule, f) for f in err.findings]
    return out


# ---- rendering ---------------------------------------------------------------------------

OWNER_TEXT: Mapping[Owner, str] = {True: "yes", False: "no", "as_target": "as target action"}
STEP_UP_TEXT: Mapping[StepUp, str] = {
    True: "yes",
    False: "no",
    "conditional": "conditional",
    "fresh": "fresh",
}
AUDIT_TEXT: Mapping[str, str] = {"always": "yes", "denials": "denials", "never": "no"}
DIRECTION_TEXT: Mapping[str, str] = {
    "client_to_server": "client to server",
    "agent_to_server": "agent to server",
    "agent_or_app_to_server": "agent or app to server",
    "server_to_client": "server to client",
}
CHANNEL_DIRECTION_TEXT: Mapping[str, str] = {
    "server_to_device": "server to device",
    "device_to_server": "device to server",
    "server_to_host": "server to host",
    "host_to_server": "host to server",
    "both": "both",
}
NOISE_FRAME = "noise_frame"
NOISE_FRAME_TEXT = "binary frames (Noise)"
_ACTION_HEADER = ("ID", "Description", "Resource", "Permission", "Owner", "Step-up", "Audit")
_HEADERS: Mapping[str, tuple[str, ...]] = {
    "rate_limit_classes": ("Class", "Key", "Purpose"),
    "actions": (*_ACTION_HEADER, "Permissions", "RL class"),
    "events.durable": ("Event type", "Resource", "Receive action", "Note"),
    "events.ephemeral": ("Event type", "Resource", "Receive action", "Additional condition"),
    "ws_ops": ("Op", "Direction", "Action"),
    "channel_ops": ("Channel", "Op", "Direction", "Action"),
}


def _row(cells: Sequence[str]) -> str:
    return "|" + "".join(f" {cell} |" if cell else " |" for cell in cells) + "\n"


def _table(header: Sequence[str], rows: Sequence[Sequence[str]]) -> str:
    rule = "|" + "---|" * len(header) + "\n"
    return _row(header) + rule + "".join(_row(r) for r in rows)


def _ticked(name: str) -> str:
    return f"`{name}`"


def _action_cells(a: Action) -> list[str]:
    audit = (a.audit_text or "") if a.audit == "conditional" else AUDIT_TEXT[a.audit]
    return [
        _ticked(a.id),
        a.description,
        a.resource,
        a.right,
        OWNER_TEXT[a.owner],
        STEP_UP_TEXT[a.step_up],
        audit,
        "yes" if a.rights_change else "no",
        _ticked(a.rl_class),
    ]


def receive_cell(event: Event) -> str:
    """Text of the cell "Receive action" of an event type.

    Args:
        event: The event type.

    Returns:
        `receive_text` if set; else `self` (no backticks) for `self`; else the backticked id.
    """
    if event.receive_text is not None:
        return event.receive_text
    return "self" if event.receive_action == "self" else _ticked(event.receive_action)


def _ws_rows(ops: Sequence[WsOp]) -> list[list[str]]:
    rows: list[list[str]] = []
    group: list[WsOp] = []
    for op in [*ops, None]:
        if group and (op is None or op.row_group is None or op.row_group != group[0].row_group):
            first = group[0]
            rows.append(
                [
                    ", ".join(_ticked(o.op) for o in group),
                    DIRECTION_TEXT[first.direction],
                    first.action_text or _ticked(first.action),
                ]
            )
            group = []
        if op is not None:
            group.append(op)
    return rows


def _channel_op_name(name: str) -> str:
    return NOISE_FRAME_TEXT if name == NOISE_FRAME else _ticked(name)


def chapter_table_cells(catalog: Catalog) -> dict[str, list[list[str]]]:
    """The cells of every generated table of chapter 05, as Markdown.

    A test renders each generated table on the docs site and compares it cell by cell with
    these cells, so a cell that the site would split or merge is caught even where the static
    cell rule of the schema would miss it.

    Args:
        catalog: The catalogue.

    Returns:
        Block name (as in `render_chapter_blocks`) to the rows of its table, the header row
        first, each row a list of cell texts; an empty cell is the empty string.
    """
    cells = {
        "rate_limit_classes": [
            list(_HEADERS["rate_limit_classes"]),
            *([_ticked(c.id), c.key, c.purpose] for c in catalog.rate_limit_classes),
        ]
    }
    for section in catalog.sections:
        cells[f"actions.{section}"] = [
            list(_HEADERS["actions"]),
            *(_action_cells(a) for a in catalog.actions if a.section == section),
        ]
    for kind in ("durable", "ephemeral"):
        cells[f"events.{kind}"] = [
            list(_HEADERS[f"events.{kind}"]),
            *(
                [_ticked(e.type), e.resource, receive_cell(e), e.note]
                for e in catalog.events
                if e.kind == kind
            ),
        ]
    cells["ws_ops"] = [list(_HEADERS["ws_ops"]), *_ws_rows(catalog.ws_ops)]
    cells["channel_ops"] = [
        list(_HEADERS["channel_ops"]),
        *(
            [
                c.channel_text or "",
                ", ".join(_channel_op_name(n) for n in c.ops),
                CHANNEL_DIRECTION_TEXT[c.direction],
                c.action_text or _ticked(c.action),
            ]
            for c in catalog.channel_ops
        ),
    ]
    return cells


def render_chapter_blocks(catalog: Catalog) -> dict[str, str]:
    """Render every generated table of chapter 05.

    Args:
        catalog: The catalogue.

    Returns:
        Block name to block body (header line, rule line and rows, each ending with a newline):
        `rate_limit_classes`, `actions.<section>` for every section, `events.durable`,
        `events.ephemeral`, `ws_ops` and `channel_ops`.
    """
    return {name: _table(rows[0], rows[1:]) for name, rows in chapter_table_cells(catalog).items()}


def redocly_lists(catalog: Catalog) -> dict[str, list[str]]:
    """Render the generated lists of docs/api/redocly.yaml.

    System actions are left out on purpose: no route, op or event may name one (05 rule 10,
    test 7), so the lint rejects them.

    Args:
        catalog: The catalogue.

    Returns:
        `authz_actions` (non-system action ids, sorted), `rate_limit_classes` (`none`, then the
        class ids, sorted) and `receive_actions` (non-system action ids plus `self`, sorted).
    """
    ids = sorted(a.id for a in catalog.actions if not a.system)
    return {
        "authz_actions": ids,
        "rate_limit_classes": ["none", *sorted(c.id for c in catalog.rate_limit_classes)],
        "receive_actions": sorted([*ids, "self"]),
    }


# ---- load, check, generate ---------------------------------------------------------------


# The C composer when PyYAML has libyaml; nodes only, objects come from the safe constructor.
_COMPOSER = getattr(yaml, "CSafeLoader", yaml.SafeLoader)
# What reading YAML can raise besides OSError: a syntax or construction error, a value Python
# cannot build (an invalid date, an integer beyond the digit limit) and nesting deeper than the
# recursion limit.
_YAML_ERRORS = (yaml.YAMLError, ValueError, RecursionError)


def _yaml_problem(exc: BaseException) -> str:
    """One line for an error in `_YAML_ERRORS`: where and what, as far as PyYAML says."""
    if isinstance(exc, RecursionError):
        return "nested too deeply"
    if isinstance(exc, yaml.MarkedYAMLError) and exc.problem_mark is not None:
        mark = exc.problem_mark
        context = f" ({exc.context})" if exc.context else ""
        return f"line {mark.line + 1}, column {mark.column + 1}: {exc.problem}{context}"
    return _one_line(str(exc))


def _node_findings(root: yaml.Node | None) -> list[str]:
    """Duplicate keys and recursive aliases of a composed YAML document.

    `safe_load` would keep the last of two equal keys silently, and an alias inside the node it
    names makes a recursive structure that no check can walk. Aliases share their node, so
    every node is visited once, however deeply aliases nest, and an explicit stack keeps deep
    nesting within the recursion limit.

    Returns:
        `line <n>: duplicate key <key>` and `line <n>: recursive alias`, in line order.
    """
    out: list[tuple[int, str]] = []
    done: set[int] = set()
    open_nodes: set[int] = set()
    stack: list[tuple[yaml.Node, bool]] = [] if root is None else [(root, False)]
    while stack:
        node, leaving = stack.pop()
        line = node.start_mark.line + 1
        if leaving:
            open_nodes.discard(id(node))
            done.add(id(node))
            continue
        if id(node) in open_nodes:
            out.append((line, f"line {line}: recursive alias"))
            continue
        if id(node) in done:
            continue
        open_nodes.add(id(node))
        stack.append((node, True))
        children: list[yaml.Node] = []
        if isinstance(node, yaml.MappingNode):
            seen: set[str] = set()
            for key, value in node.value:
                if isinstance(key, yaml.ScalarNode):
                    if key.value in seen:
                        where = key.start_mark.line + 1
                        out.append((where, f"line {where}: duplicate key {key.value!r}"))
                    seen.add(key.value)
                children.append(value)
        elif isinstance(node, yaml.SequenceNode):
            children = list(node.value)
        stack += [(child, False) for child in reversed(children)]
    return [finding for _, finding in sorted(out, key=lambda item: item[0])]


def _safe_load(text: str) -> Any:
    """`yaml.safe_load` through the C composer when available (same result, much faster).

    Raises:
        An error in `_YAML_ERRORS`.
    """
    node = yaml.compose(text, Loader=_COMPOSER)
    return None if node is None else yaml.SafeLoader("").construct_document(node)


def _parse_yaml(text: str) -> tuple[Any, list[str]]:
    """Parse YAML like `yaml.safe_load`, through the C composer when available (much faster).

    Returns:
        The data (`None` when the node graph has a problem) and the `_node_findings`.

    Raises:
        An error in `_YAML_ERRORS`.
    """
    node = yaml.compose(text, Loader=_COMPOSER)
    problems = _node_findings(node)
    if node is None or problems:
        return None, problems
    return yaml.SafeLoader("").construct_document(node), []


def load_catalog(path: Path) -> Catalog:
    """Load and validate docs/spec/actions.yaml.

    Validates the schema and every rule that needs only the YAML: `schema`, `unique`,
    `section`, `rate-limit-class`, `event`, `op` and `channel-op` of tools/catalog/README.md.
    The findings come in stages, and a stage runs only when the stages before it found
    nothing: the file is read and parsed, then duplicate keys and recursive aliases, then the
    schema, then the relational rules.

    Args:
        path: Path of the catalogue file.

    Returns:
        The catalogue.

    Raises:
        CatalogError: With every finding of the first stage that has one, each in the form
            `actions.yaml: <rule>: <detail>`.
    """
    prefix = f"{CATALOG_LABEL}: {RULE_SCHEMA}:"
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        raise CatalogError([f"{prefix} cannot read: {exc}"]) from exc
    try:
        data, problems = _parse_yaml(text)
    except _YAML_ERRORS as exc:
        raise CatalogError([f"{prefix} cannot parse: {_yaml_problem(exc)}"]) from exc
    if problems:
        raise CatalogError([f"{prefix} {p}" for p in problems])
    return catalog_from_data(data)


def _read(root: Path, rel: Path) -> str:
    try:
        return (root / rel).read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        raise CatalogError([f"{rel.as_posix()}: {RULE_BLOCK}: cannot read: {exc}"]) from exc


def _compare(
    label: str, current: Mapping[str, object], wanted: Mapping[str, object], *, ordered: bool
) -> list[str]:
    """Block findings of one file: unknown and missing names, order if `ordered`, stale bodies."""
    prefix = f"{label}: {RULE_BLOCK}:"
    findings = [f"{prefix} unknown block {name!r}" for name in sorted(set(current) - set(wanted))]
    if ordered and set(current) == set(wanted) and list(current) != list(wanted):
        findings.append(
            f"{prefix} blocks are not in the order of the catalogue: expected {', '.join(wanted)}"
        )
    findings += [f"{prefix} missing block {name!r}" for name in sorted(set(wanted) - set(current))]
    findings += [
        f"{prefix} {name}: generated block differs from docs/spec/actions.yaml; run make catalog"
        for name in sorted(set(current) & set(wanted))
        if current[name] != wanted[name]
    ]
    return findings


def _block_findings(catalog: Catalog, chapter: str, redocly: str) -> list[str]:
    findings: list[str] = []
    chapter_label, redocly_label = CHAPTER_PATH.as_posix(), REDOCLY_PATH.as_posix()
    try:
        # The tables of 05 follow the catalogue; in redocly.yaml the order of the lint rules
        # does not change what Redocly enforces.
        findings += _compare(
            chapter_label, generated_blocks(chapter), render_chapter_blocks(catalog), ordered=True
        )
    except CatalogError as err:
        findings += [f"{chapter_label}: {RULE_BLOCK}: {f}" for f in err.findings]
    try:
        current = {name: items for name, (_, items) in yaml_generated_blocks(redocly).items()}
        findings += _compare(redocly_label, current, redocly_lists(catalog), ordered=False)
    except CatalogError as err:
        findings += [f"{redocly_label}: {RULE_BLOCK}: {f}" for f in err.findings]
    return findings


# The lint rules that enforce the generated lists (rule `generated-block`), with the shape each
# must keep: subject type, subject property and generated list. Any other change to such a rule
# (an entry next to the markers, a replaced anchor, `severity`, `where`, another subject or
# assertion) would weaken the lint while the blocks stay current, so the whole rule is checked.
REDOCLY_RULES: Mapping[str, tuple[str, str, str]] = {
    "rule/authz-action-known": ("Operation", "x-authz-action", "authz_actions"),
    "rule/rate-limit-class-known": ("Operation", "x-rate-limit-class", "rate_limit_classes"),
    "rule/receive-action-known-message": ("Message", "x-receive-action", "receive_actions"),
    "rule/receive-action-known-webhook": ("Operation", "x-receive-action", "receive_actions"),
}
_RULE_KEYS = frozenset({"subject", "assertions", "message"})
# Positive list for docs/api/redocly.yaml: every other key (preprocessors, plugins, decorators,
# a local `extends`, `<type>Rules`, ...) could change what the lint enforces, so it is a finding
# until this list and its review allow it.
REDOCLY_TOP_KEYS = ("extends", "apis", "rules")
REDOCLY_EXTENDS = ["recommended"]
REDOCLY_API_KEYS = ("root", "rules")
LINT_IGNORE_PATH = Path("docs/api/.redocly.lint-ignore.yaml")
# Level-1 and level-2 sections of chapter 05 whose tables stay hand-written. Outside the
# generated blocks every other section holds no line with `|` or `<table`, not even in a code
# block (fail closed: no Markdown parser decides what renders as a table).
HAND_WRITTEN_SECTIONS = (
    "Terms of the tables",
    "Agent invocation `agent.invoke`",
    "Decisions of this chapter",
    "Configuration values of this chapter",
)
# Level-2 section of chapter 05 that holds each generated table (by the part of the block name
# before the dot), and the level-3 heading above each action table; the catalogue names its
# sections only by id, so a new section needs its heading here.
BLOCK_SECTIONS: Mapping[str, str] = {
    "rate_limit_classes": "Rate-limit classes",
    "actions": "Actions",
    "events": "Durable event types and receive action",
    "ws_ops": "WebSocket ops and action",
    "channel_ops": "WebSocket ops and action",
}
ACTION_SECTION_TITLES: Mapping[str, str] = {
    "public": "Public and login",
    "instance": "Instance and administration",
    "identity": "People, identity, devices, tokens",
    "grants": "Groups and grants",
    "structure": "Spaces, channels, sidebar",
    "threads": "Threads and agent sessions",
    "messages": "Messages, reactions, pins, templates",
    "files": "Files and search",
    "personal": "Personal data, notifications, sync",
    "agents": "Agents, commands, runs, interactions",
    "apps": "Apps and webhooks",
    "operations": "Import, costs, privacy, status",
    "dev": "Dev environments (milestone)",
    "terminal": "Hosts and terminal",
}
# An ATX heading as Python-Markdown renders it: `#` in the first column (with leading spaces the
# line is text). The title may be empty and drops a leading number and trailing `#`; possessive
# runs and a greedy title keep the match linear on long lines.
_ATX = re.compile(
    r"^(?P<level>#{1,6})"
    r"(?:[ \t]++(?:[0-9]+(?:\.[0-9]+)*\.?[ \t]++)?(?P<title>.*[^ \t#]))?[ \t#]*$"
)
# A heading title is visible only with a letter or digit of this positive list; zero-width,
# soft-hyphen, blank or filler characters show nothing on the page. A title holds no character
# outside `_TITLE_CHARS`, so that no other script, entity or invisible character can make a
# heading look like another; titles compare by the letters a reader sees (`_title_keys`).
_VISIBLE = re.compile("[0-9A-Za-z]")
_LETTER = re.compile("[A-Za-z]")
_TITLE_CHARS = re.compile(r'[0-9A-Za-z "(),\-.:/_`]*')


# Digits that the sans-serif fonts of the site draw like a letter.
_LETTER_DIGITS = str.maketrans({"1": "l", "0": "o"})


def _title_keys(title: str) -> frozenset[str]:
    """The letters a reader can see in a heading title, case folded, `I` and `l` as one letter.

    Two keys: the letters alone (a number can be written in many ways, so digits do not count),
    and the letters with `1` read as `l` and `0` as `o`. Two titles look alike when they share
    a key.
    """
    return frozenset(
        "".join(_LETTER.findall(text)).casefold().replace("i", "l")
        for text in (title, title.translate(_LETTER_DIGITS))
    )


# Characters that the checks and Markdown read differently: str.splitlines breaks a line at VT,
# FF, U+001C to U+001E, U+0085, U+2028 and U+2029 and Markdown does not; Markdown drops STX and
# ETX. Either way the static view would read other lines than the site renders.
_FOREIGN_BREAK = re.compile("[\x02\x03\x0b\x0c\x1c-\x1e\x85\u2028\u2029]")
_DROPPED = "\x02\x03"
# A setext underline, like Python-Markdown in the first column (tabs become spaces there), and a
# horizontal rule (Python-Markdown's HRProcessor), after which a new block starts.
_SETEXT_UNDERLINE = re.compile(r"^(?:-+|=+)[ \t]*$")
_RULE_LINE = re.compile(r"^ {0,3}(?>(?:-+ {0,2}){3,}|(?:_+ {0,2}){3,}|(?:\*+ {0,2}){3,}) *$")
# An HTML comment on its own line (the markers): Python-Markdown keeps it as a block of its own;
# so does a link reference definition (its ReferenceProcessor puts the rest into a new block).
_COMMENT_LINE = re.compile(r"^<!--.*-->[ \t]*$")
_REFERENCE_LINE = re.compile(r"^ {0,3}\[[^\[\]]*\]:[ \t]*\S")
# Outside the blocks, chapter 05 holds no HTML at all (any `<` before a letter, `!`, `?` or `/`
# outside a code span) and no ATX heading without a space after `#` (the docs site renders it
# as a heading); both could hide or invent a section.
# The backslash escapes of the docs site: Python-Markdown's own and `|` from `tables`. A test
# compares the set with the renderer. `\<` is no escape: the backslash stays text and the `<`
# still starts raw HTML.
MARKDOWN_ESCAPES = frozenset("\\`*_{}[]()>#+-.!|")
_BACKTICKS = re.compile(r"`+")
_HTML_START = re.compile(r"<[A-Za-z!?/]")
_ATX_WITHOUT_SPACE = re.compile(r"^ {0,3}#{1,6}[^ #\t]")
_LEADING_NUMBER = re.compile(r"^[0-9]+(?:\.[0-9]+)*\.?[ \t]+")
# Admonitions (`!!!`), collapsible blocks (`???`) and footnote definitions (`[^x]:`) can carry a
# heading where the static view does not see one.
_BLOCK_WRAPPER = re.compile(r"^ {0,3}(?:!!!|\?\?\?|\[\^)")
# An attribute list (`{: hidden }`, `{ #id }`) closes its brace on the same line.
_ATTRIBUTE_LIST = re.compile(r"\{[^{}\n]*\}")


def _without_code(line: str) -> str:
    """`line` without its code spans and backslash escapes, found as the docs site does.

    One left-to-right pass: an escape outside a span is dropped; a backtick that no escape takes
    opens a span with the rest of its run. The span ends with the next later run of the same
    length, else with the first longest later run (Markdown's fallback, which can even render
    an empty span); inside a span a backslash is a literal character. Without a later run the
    backticks are text. This follows `find_code_spans` of pymdownx.inlinehilite (the version
    pinned in tools/uv.lock), which renders backticks on the site like Python-Markdown; a test
    compares the result with the renderer. Table rows outside the generated blocks (where
    `tables` first pairs the backtick runs of the whole row, splits at the other `|` and then
    reads each cell on its own, see `_cell_problem`) and spans over several lines are left to
    the test of the rendered page. Runs are looked up, not rescanned, so a long line of
    backticks cannot stall the check.
    """
    runs = [(m.start(), m.end()) for m in _BACKTICKS.finditer(line)]
    # For each run, the first longest run from it on; per length, the runs in line order.
    longest = list(range(len(runs)))
    for j in range(len(runs) - 2, -1, -1):
        k = longest[j + 1]
        if runs[k][1] - runs[k][0] > runs[j][1] - runs[j][0]:
            longest[j] = k
    by_length: dict[int, list[int]] = {}
    for j, (begin, end) in enumerate(runs):
        by_length.setdefault(end - begin, []).append(j)
    out: list[str] = []
    i = j = 0
    while i < len(line):
        if line[i] == "\\" and i + 1 < len(line) and line[i + 1] in MARKDOWN_ESCAPES:
            i += 2
        elif line[i] != "`":
            out.append(line[i])
            i += 1
        else:
            while runs[j][1] <= i:
                j += 1
            # After an escaped backtick the opening is only the rest of its run.
            same = by_length.get(runs[j][1] - i, [])
            if (n := bisect.bisect_right(same, j)) < len(same):
                i = runs[same[n]][1]
            elif j + 1 < len(runs):
                i = runs[longest[j + 1]][1]
            else:
                out.append(line[i : runs[j][1]])
                i = runs[j][1]
    return "".join(out)


def _headings(lines: list[str]) -> list[tuple[int, str] | None]:
    r"""Level and title of the heading each line ends, approximating Python-Markdown's blocks.

    Lines are read as Markdown reads them: tabs expanded to four columns, a blank line holds
    only spaces. An ATX heading is a line with `#` in the first column. A setext underline makes
    a heading of the line above only if that line is text that starts a block: at the start or
    after a blank line, a line starting with `#`, a horizontal rule, a heading, a comment line
    (the markers), a link reference definition or an indented code line. Leading numbers are
    removed. Each block rule is recognized by the form of a single line, so among others this
    view misses headings inside block quotes, lists or definition lists, what several block
    rules do together in one block (a rule and a reference, a line of no-break spaces after
    indented code), rules that span lines (a reference with trailing text or its address on
    the next line), ATX corner cases (a title ending in `\`) and markup in titles; a test
    compares listed cases with the rendered headings, and the test of the rendered page holds
    the class.
    """
    out: list[tuple[int, str] | None] = []
    at_block_start, previous_starts_block, previous = True, False, ""
    for raw in lines:
        line = raw.expandtabs(4)
        blank = not line.strip(" ")
        code = at_block_start and not blank and line.startswith("    ")
        heading: tuple[int, str] | None = None
        if (m := _ATX.match(line)) is not None:
            heading = (len(m.group("level")), m.group("title") or "")
        elif _SETEXT_UNDERLINE.match(line) and previous_starts_block:
            heading = (1 if line.startswith("=") else 2, _LEADING_NUMBER.sub("", previous.strip()))
        out.append(heading)
        # Lines that form a block of their own and are no text for an underline. A horizontal
        # rule or a link reference also ends its block, but at the start of a block it can be
        # the text of a setext heading (Markdown tries setext first).
        separate = (
            code
            or heading is not None
            or line.startswith("#")
            or _COMMENT_LINE.match(line) is not None
        )
        ends_block = _RULE_LINE.match(line) or _REFERENCE_LINE.match(line)
        previous_starts_block = at_block_start and not blank and not separate
        at_block_start = blank or separate or ends_block is not None
        previous = line
    return out


def section_titles(chapter: str) -> list[str]:
    """Titles of the level-1 and level-2 sections of a chapter, leading numbers removed.

    The rule `generated-block` uses this view of the chapter; a test compares it with the
    headings the docs site renders; where each heading sits is checked on the rendered page
    itself.

    Args:
        chapter: Text of docs/spec/05-actions.md.

    Returns:
        The section titles in document order.
    """
    return [h[1] for h in _headings(chapter.splitlines()) if h is not None and h[0] <= 2]


def _hand_tables_outside_blocks(chapter: str) -> list[str]:
    """Problems of 05 outside the generated blocks.

    The problems are table characters except in hand-written sections, HTML, headings without a
    space, a visible title or with a character outside `_TITLE_CHARS`, brace pairs,
    admonitions, collapsible blocks and footnotes.

    Every ATX heading of level 1 or 2 (with or without a number) and every setext heading
    starts a section; each hand-written title must occur exactly once, so a copy of it (for
    example inside a code block) cannot open a hand-written section elsewhere.
    """
    lines = chapter.splitlines()
    try:
        spans = _spans(lines, _MD_BEGIN, _MD_END, _MD_PREFIXES)
    except CatalogError:
        return []  # reported by the block comparison
    inside = {n for s in spans.values() for n in range(s.begin, s.end + 1)}
    prefix = f"{CHAPTER_PATH.as_posix()}: {RULE_BLOCK}:"
    out: list[str] = []
    counts = dict.fromkeys(HAND_WRITTEN_SECTIONS, 0)
    title = ""
    for n, (line, found) in enumerate(zip(lines, _headings(lines), strict=True)):
        heading = found[1] if found is not None and found[0] <= 2 else None
        if n not in inside and found is not None and not _VISIBLE.search(found[1]):
            out.append(
                f"{prefix} heading without a visible title (line {n + 1}): the site shows what"
                " follows as part of the section before"
            )
        elif n not in inside and found is not None and not _TITLE_CHARS.fullmatch(found[1]):
            out.append(
                f"{prefix} heading title with a character outside the positive list (line"
                f" {n + 1}): it could look like another heading"
            )
        bare = _without_code(line)
        if n not in inside and (_HTML_START.search(bare) or _ATX_WITHOUT_SPACE.match(line)):
            out.append(
                f"{prefix} HTML outside the generated blocks or a heading without a space after"
                f" # (line {n + 1}): either could hide or invent a section"
            )
        if n not in inside and _BLOCK_WRAPPER.match(line):
            out.append(
                f"{prefix} admonition, collapsible block or footnote outside the generated"
                f" blocks (line {n + 1}): it could carry a hidden section"
            )
        if n not in inside and _ATTRIBUTE_LIST.search(bare):
            out.append(
                f"{prefix} brace pair outside a code span outside the generated blocks"
                f" (line {n + 1}): an attribute list such as hidden could hide a section"
            )
        if heading is not None:
            title = heading
            if title in counts:
                counts[title] += 1
        elif (
            n not in inside
            and title not in HAND_WRITTEN_SECTIONS
            and ("|" in line or "<table" in line.lower())
        ):
            out.append(
                f"{prefix} table row outside the generated blocks in section {title!r}"
                f" (line {n + 1})"
            )
    out += [
        f"{prefix} hand-written section {t!r} appears {c} times; expected once"
        for t, c in counts.items()
        if c != 1
    ]
    return out


def _misplaced(name: str, section: str, subsection: str) -> list[str]:
    """Why block `name` does not belong under level-2 `section` and level-3 `subsection`."""
    family, _, part = name.partition(".")
    out: list[str] = []
    want = BLOCK_SECTIONS.get(family)
    if want is None:
        out.append("has no section in BLOCK_SECTIONS")
    elif section != want:
        out.append(f"is in section {section!r}; expected {want!r}")
    if family == "actions":
        title = ACTION_SECTION_TITLES.get(part)
        if title is None:
            out.append("has no heading in ACTION_SECTION_TITLES")
        elif subsection != title:
            out.append(f"is under heading {subsection!r}; expected {title!r}")
    return out


def _block_section_findings(chapter: str) -> list[str]:
    """Generated tables of 05 that sit in the wrong place.

    These are tables outside their level-2 section (`BLOCK_SECTIONS`), action tables not under
    the level-3 heading of their catalogue section (`ACTION_SECTION_TITLES`), and tables below a
    heading of level 4 or deeper, which would show a reader another title right above the table.

    Block order alone does not catch a table moved under the next heading; a test checks the
    same placement on the rendered page.
    """
    lines = chapter.splitlines()
    try:
        spans = _spans(lines, _MD_BEGIN, _MD_END, _MD_PREFIXES)
    except CatalogError:
        return []  # reported by the block comparison
    begins = {span.begin: name for name, span in spans.items()}
    prefix = f"{CHAPTER_PATH.as_posix()}: {RULE_BLOCK}:"
    out: list[str] = []
    section = subsection = minor = ""
    for n, heading in enumerate(_headings(lines)):
        if heading is not None and heading[0] <= 2:
            section, subsection, minor = heading[1], "", ""
        elif heading is not None and heading[0] == 3:
            subsection, minor = heading[1], ""
        elif heading is not None:
            minor = heading[1]
        elif n in begins:
            problems = _misplaced(begins[n], section, subsection)
            if minor:
                problems.append(f"is under heading {minor!r} of level 4 or deeper")
            out += [
                f"{prefix} block {begins[n]!r} {problem} (line {n + 1})" for problem in problems
            ]
    return out


def _line_break_findings(chapter: str) -> list[str]:
    """Characters of 05 that the checks and Markdown read differently (`_FOREIGN_BREAK`)."""
    prefix = f"{CHAPTER_PATH.as_posix()}: {RULE_BLOCK}:"
    out: list[str] = []
    line, counted = 1, 0
    for m in _FOREIGN_BREAK.finditer(chapter):
        line += chapter.count("\n", counted, m.start())
        counted = m.start()
        why = (
            "Markdown drops it"
            if m.group() in _DROPPED
            else "str.splitlines breaks the line there and Markdown does not"
        )
        out.append(
            f"{prefix} character U+{ord(m.group()):04X} (line {line}): {why}, so the checks"
            " would read other lines than the site renders"
        )
    return out


def _chapter_start_findings(chapter: str) -> list[str]:
    """Chapter 05 starts with its `# ` heading.

    Page metadata or front matter would let the docs site render something else than the text
    the checks read (for example another template).
    """
    if chapter.startswith("# "):
        return []
    return [
        f"{CHAPTER_PATH.as_posix()}: {RULE_BLOCK}: chapter must start with its '# ' heading"
        " (line 1); page metadata or front matter would change how the site renders it"
    ]


def _regenerate_redocly(catalog: Catalog, redocly: str) -> str:
    for name, items in redocly_lists(catalog).items():
        redocly = replace_yaml_block(redocly, name, items)
    return redocly


def _override_rule_sets(data: Mapping[str, Any]) -> list[tuple[str, Mapping[str, Any]]]:
    """Every rule mapping that Redocly merges over the top-level `rules`.

    Redocly reads `<type>Rules` (for example `oas3_1Rules`, `async3Rules`) at the top level and
    `rules` plus `<type>Rules` in each entry of `apis`; any of them can switch a rule off.
    """
    found: list[tuple[str, Mapping[str, Any]]] = [
        (key, value)
        for key, value in data.items()
        if key != "rules" and str(key).endswith("Rules") and isinstance(value, dict)
    ]
    apis = data.get("apis")
    for api, conf in apis.items() if isinstance(apis, dict) else ():
        if isinstance(conf, dict):
            found += [
                (f"apis[{api}].{key}", value)
                for key, value in conf.items()
                if (key == "rules" or str(key).endswith("Rules")) and isinstance(value, dict)
            ]
    return found


def _dollar_keys(data: object) -> list[str]:
    """Keys starting with `$` anywhere: Redocly resolves `$ref` in its configuration.

    Each mapping and list is visited once (aliases share them, also recursively), and an
    explicit stack keeps deep nesting within the recursion limit.
    """
    out: list[str] = []
    seen: set[int] = set()
    stack: list[tuple[object, str]] = [(data, "")]
    while stack:
        node, where = stack.pop()
        if not isinstance(node, (dict, list)) or id(node) in seen:
            continue
        seen.add(id(node))
        children: list[tuple[object, str]] = []
        if isinstance(node, dict):
            for key, value in node.items():
                if str(key).startswith("$"):
                    out.append(f"{where or 'top level'}: key {key!r} is not allowed")
                path = (
                    f"{where}[{key}]"
                    if where == "apis"
                    else f"{where}.{key}"
                    if where
                    else str(key)
                )
                children.append((value, path))
        else:
            children = [(item, f"{where}[{n}]") for n, item in enumerate(node)]
        stack += reversed(children)
    return out


def _positive_list_findings(data: Mapping[str, Any], prefix: str) -> list[str]:
    """Findings of the positive list of redocly.yaml.

    These are keys outside the positive list or starting with `$` anywhere, a changed
    `extends`, a missing or absolute `root`, and two `apis` entries for one file.
    """
    out = [f"{prefix} {d}" for d in _dollar_keys(data)]
    out += [
        f"{prefix} key {k!r} is not allowed (allowed: {', '.join(REDOCLY_TOP_KEYS)})"
        for k in data
        if k not in REDOCLY_TOP_KEYS
    ]
    if data.get("extends") != REDOCLY_EXTENDS:
        out.append(f"{prefix} extends must be exactly [{', '.join(REDOCLY_EXTENDS)}]")
    apis = data.get("apis")
    roots: dict[str, str] = {}
    for api, conf in apis.items() if isinstance(apis, dict) else ():
        if not isinstance(conf, dict):
            out.append(f"{prefix} apis[{api}] must be a mapping")
            continue
        out += [
            f"{prefix} apis[{api}]: key {k!r} is not allowed"
            f" (allowed: {', '.join(REDOCLY_API_KEYS)})"
            for k in conf
            if k not in REDOCLY_API_KEYS
        ]
        raw = conf.get("root")
        if not isinstance(raw, str) or not raw.strip() or posixpath.isabs(raw):
            out.append(f"{prefix} apis[{api}]: root must be a relative path")
            continue
        root = posixpath.normpath(posixpath.join(REDOCLY_PATH.parent.as_posix(), raw))
        if root in roots:
            out.append(f"{prefix} apis[{api}] lints {root} like apis[{roots[root]}]")
        roots.setdefault(root, str(api))
    return out


def _redocly_rule_findings(catalog: Catalog, redocly: str) -> list[str]:
    """Each catalogue lint rule keeps its generated shape and nothing else changes the lint.

    Checked: the rule has only `subject` (generated type and property), `assertions` with
    exactly `enum` equal to its generated list, and `message`; no rule set (`rules` or
    `<type>Rules` under `apis`, top-level `<type>Rules`) names it; the file stays within the
    positive list (`REDOCLY_TOP_KEYS`, `REDOCLY_EXTENDS`, `REDOCLY_API_KEYS`) and has no key
    starting with `$`; no two `apis` entries lint the same file.
    """
    prefix = f"{REDOCLY_PATH.as_posix()}: {RULE_BLOCK}:"
    try:
        loaded = _safe_load(redocly)
    except _YAML_ERRORS as exc:
        return [f"{prefix} cannot parse: {_yaml_problem(exc)}"]
    data: dict[str, Any] = loaded if isinstance(loaded, dict) else {}
    rules: dict[str, Any] = data["rules"] if isinstance(data.get("rules"), dict) else {}
    wanted = redocly_lists(catalog)
    out: list[str] = []
    for rule, (subject_type, subject_property, block) in REDOCLY_RULES.items():
        body = rules.get(rule)
        if not isinstance(body, dict):
            out.append(f"{prefix} {rule}: rule is missing or not a mapping")
            continue
        out += [f"{prefix} {rule}: unexpected key {k!r}" for k in sorted(set(body) - _RULE_KEYS)]
        if body.get("subject") != {"type": subject_type, "property": subject_property}:
            out.append(
                f"{prefix} {rule}: subject must be type {subject_type}, property {subject_property}"
            )
        assertions = body.get("assertions")
        if not isinstance(assertions, dict) or set(assertions) != {"enum"}:
            out.append(f"{prefix} {rule}: assertions must hold only enum")
        elif assertions["enum"] != wanted[block]:
            out.append(
                f"{prefix} {rule}: enum differs from the generated list {block!r} (an entry"
                " outside the generated block or a replaced anchor)"
            )
    for where, rule_set in _override_rule_sets(data):
        out += [f"{prefix} {where} overrides {r}" for r in REDOCLY_RULES if r in rule_set]
    return out + _positive_list_findings(data, prefix)


def _redocly_node_findings(redocly: str) -> list[str]:
    """Duplicate keys and recursive aliases with line numbers of the file itself.

    Redocly rejects both.
    """
    try:
        node = yaml.compose(redocly, Loader=_COMPOSER)
    except _YAML_ERRORS:
        return []  # reported by _redocly_rule_findings
    return [f"{REDOCLY_PATH.as_posix()}: {RULE_BLOCK}: {d}" for d in _node_findings(node)]


def _lint_ignore_findings(root: Path) -> list[str]:
    """A lint-ignore file next to redocly.yaml must not suppress a catalogue rule anywhere."""
    path = root / LINT_IGNORE_PATH
    if not path.exists():
        return []
    label = f"{LINT_IGNORE_PATH.as_posix()}: {RULE_BLOCK}:"
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError) as exc:
        return [f"{label} cannot read: {exc}"]
    named = {rule for rule in REDOCLY_RULES if rule in text}  # also catches comments
    try:
        data = _safe_load(text)
    except _YAML_ERRORS as exc:
        return [f"{label} cannot parse: {_yaml_problem(exc)}"]
    for per_file in data.values() if isinstance(data, dict) else ():
        if isinstance(per_file, dict):
            named |= {str(rule) for rule in per_file if rule in REDOCLY_RULES}
    return [
        f"{label} suppresses {rule}; a value outside the catalogue would pass the lint there"
        for rule in REDOCLY_RULES
        if rule in named
    ]


def _outside_findings(root: Path, catalog: Catalog, chapter: str, redocly: str) -> list[str]:
    """Problems next to the generated blocks, which regenerating cannot repair."""
    out = [
        *_chapter_start_findings(chapter),
        *_line_break_findings(chapter),
        *_hand_tables_outside_blocks(chapter),
        *_block_section_findings(chapter),
        *_redocly_node_findings(redocly),
        *_lint_ignore_findings(root),
    ]
    try:
        regenerated = _regenerate_redocly(catalog, redocly)
    except CatalogError:
        return out  # broken markers are reported by the block comparison
    return out + _redocly_rule_findings(catalog, regenerated)


def check(root: Path) -> list[str]:
    """Run every rule against the catalogue and its generated copies.

    The catalogue is checked first, in the stages of `load_catalog`; chapter 05 and
    docs/api/redocly.yaml are compared with it only when it is valid, and then every rule of
    those two files reports at once.

    Args:
        root: Repository root containing docs/spec/actions.yaml, docs/spec/05-actions.md and
            docs/api/redocly.yaml.

    Returns:
        The findings, each one line; empty when everything is consistent and current.
    """
    try:
        catalog = load_catalog(root / CATALOG_PATH)
        chapter = _read(root, CHAPTER_PATH)
        redocly = _read(root, REDOCLY_PATH)
    except CatalogError as err:
        return err.findings
    findings = [
        *chapter_findings(catalog, chapter),
        *_block_findings(catalog, chapter, redocly),
        *_outside_findings(root, catalog, chapter, redocly),
    ]
    return [_one_line(f) for f in findings]


def _write_all(root: Path, changes: Sequence[tuple[Path, str, str]]) -> list[str]:
    """Write every changed file or none of them.

    Each new text goes into a temporary file next to its target first (mode copied); only when
    all are written do they replace their targets. A target that is not writable stops
    everything before the first write. If anything fails after that, also an interrupt, the
    files replaced before get their old text back and no temporary file stays.

    Args:
        root: Repository root.
        changes: `(relative path, old text, new text)` for each file; unchanged ones are
            skipped.

    Returns:
        The relative paths written, in POSIX form.

    Raises:
        CatalogWriteError: A file could not be written; every file has its old text again
            unless a finding says otherwise (`restored`).
        BaseException: Any other error, such as an interrupt, after the same clean-up; a file
            that kept its new text is named in a note of the error.
    """
    targets = [(rel, Path(os.path.realpath(root / rel)), old, new) for rel, old, new in changes]
    targets = [t for t in targets if t[3] != t[2]]
    findings = [
        f"{rel.as_posix()}: {RULE_BLOCK}: cannot write: not writable"
        for rel, path, _, _ in targets
        if not os.access(path, os.W_OK)
    ]
    if findings:
        raise CatalogWriteError(findings, restored=True)
    temporary: list[Path] = []
    replaced: list[tuple[Path, Path, str]] = []
    current = CHAPTER_PATH
    try:
        for current, path, _, new in targets:  # noqa: B007 (named in the finding below)
            handle, name = tempfile.mkstemp(dir=path.parent, prefix=f".{path.name}.")
            temporary.append(Path(name))
            with os.fdopen(handle, "w", encoding="utf-8") as stream:
                stream.write(new)
            temporary[-1].chmod(stat.S_IMODE(path.stat().st_mode))
        for (current, path, old, _), temp in zip(targets, temporary, strict=True):
            temp.replace(path)
            replaced.append((current, path, old))
    except BaseException as exc:
        unrestored: list[str] = []
        for rel, path, old in reversed(replaced):
            try:
                path.write_text(old, encoding="utf-8")
            except OSError as undo:
                unrestored.append(f"{rel.as_posix()}: {RULE_BLOCK}: old text not restored: {undo}")
        for temp in temporary:
            temp.unlink(missing_ok=True)
        if not isinstance(exc, OSError):
            for line in unrestored:
                exc.add_note(line)
            raise
        raise CatalogWriteError(
            [f"{current.as_posix()}: {RULE_BLOCK}: cannot write: {exc}", *unrestored],
            restored=not unrestored,
        ) from exc
    return [rel.as_posix() for rel, _, _, _ in targets]


def generate(root: Path) -> list[str]:
    """Rewrite the generated blocks of chapter 05 and docs/api/redocly.yaml from the catalogue.

    Nothing is written unless the catalogue is valid, agrees with the hand-written parts of
    chapter 05 and every block of both files can be replaced without leaving drift next to
    the markers; a stale block body is the only finding this function repairs.

    Args:
        root: Repository root (see `check`).

    Returns:
        The repository-relative paths of the files it rewrote, in POSIX form.

    Raises:
        CatalogError: The catalogue is invalid; it contradicts the hand-written parts of
            chapter 05 (scope table 2.6, effect list 2.7, configuration rows); a file has
            broken, unknown or missing blocks; the blocks of chapter 05 are out of the order of
            the catalogue or a table is outside its section (`BLOCK_SECTIONS`, for action
            tables also the heading in `ACTION_SECTION_TITLES`) or below a heading of level 4
            or deeper; chapter 05 has a table
            character outside the generated blocks of a section that is not hand-written,
            HTML, a heading without a space, a brace pair, an admonition, a collapsible block
            or a footnote outside the blocks, a heading without a visible title or with a
            character outside `_TITLE_CHARS`, a hand-written title that does not occur exactly
            once, a character that the checks and Markdown read differently
            (`_FOREIGN_BREAK`), or no `# ` heading on its first line (page metadata); or
            docs/api/redocly.yaml
            no longer enforces the generated lists unchanged (a changed or overridden catalogue
            lint rule, a key outside the positive list `REDOCLY_TOP_KEYS` and
            `REDOCLY_API_KEYS` or starting with `$` anywhere, an `extends` other than
            `REDOCLY_EXTENDS`, a missing, absolute or repeated `root`, duplicate keys, a
            lint-ignore entry for a catalogue rule).
        CatalogWriteError: All of that was valid, but a file could not be written (see
            `_write_all`).
    """
    catalog = load_catalog(root / CATALOG_PATH)
    chapter = _read(root, CHAPTER_PATH)
    redocly = _read(root, REDOCLY_PATH)
    blocking = [
        *chapter_findings(catalog, chapter),
        *(
            f
            for f in _block_findings(catalog, chapter, redocly)
            if "generated block differs" not in f
        ),
        *_outside_findings(root, catalog, chapter, redocly),
    ]
    if blocking:
        raise CatalogError(blocking)
    new_chapter = chapter
    for name, body in render_chapter_blocks(catalog).items():
        new_chapter = replace_generated_block(new_chapter, name, body)
    new_redocly = _regenerate_redocly(catalog, redocly)
    return _write_all(
        root, [(CHAPTER_PATH, chapter, new_chapter), (REDOCLY_PATH, redocly, new_redocly)]
    )


def main(argv: Sequence[str] | None = None) -> int:
    """Command line: `check` or `generate`, optionally with `--root <repository root>`.

    Args:
        argv: Arguments without the program name; `None` reads `sys.argv`.

    Returns:
        Exit code: 0 clean or written, 1 with findings (printed to stderr, one per line).
    """
    parser = argparse.ArgumentParser(
        prog="action_catalog.py",
        description="Check or generate the copies of docs/spec/actions.yaml (ADR 0049).",
    )
    parser.add_argument("command", choices=("check", "generate"))
    parser.add_argument("--root", type=Path, default=REPO, help="repository root")
    args = parser.parse_args(argv)
    if args.command == "check":
        findings = check(args.root)
        for finding in findings:
            print(finding, file=sys.stderr)
        return 1 if findings else 0
    try:
        written = generate(args.root)
    except CatalogError as err:
        for finding in err.findings:
            print(finding, file=sys.stderr)
        if not isinstance(err, CatalogWriteError):
            summary = "catalogue or blocks invalid; nothing written"
        elif err.restored:
            summary = "cannot write the generated blocks; nothing written"
        else:
            summary = (
                "cannot write the generated blocks; the files above whose old text was not"
                " restored hold the new text"
            )
        print(f"action_catalog: {summary}", file=sys.stderr)
        return 1
    for rel in written:
        print(f"  rewrote {rel}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
