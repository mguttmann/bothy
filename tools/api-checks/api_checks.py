# SPDX-License-Identifier: Apache-2.0
"""API contract checks: every copy of the contract equals its source (06 G2 and G3).

Two rules of docs/spec/06-api.md section 15, and the fixture check of chapter 25:

- `api-catalogue-sync` (G2) compares docs/api/openapi.yaml and docs/api/asyncapi.yaml with the
  action catalogue docs/spec/actions.yaml (actions, rate-limit classes, ops, events, receive
  actions) and the close codes with 04 "Close codes", and holds the machine-readable fields
  of chapter 06: `x-idempotency-exempt` (section 7), the deprecation dates (13.4) against
  `api.deprecation_min_notice` of docs/operations/configuration.md, `x-problem-codes` and
  `x-status` (section 5).
- `api-tables` (G3) renders the generated blocks of chapter 06 and docs/api/webhooks.md from
  the YAML files and fails on any difference.
- `viz-fixtures` (25 7.2, VZ-T11) validates every fixture of packages/bothy_viz/fixtures/
  against its schema in docs/api/openapi.yaml (module `viz_fixtures`).
- `gdpr-export` checks the schemas of the export package (module `gdpr_export`).
- `client-routes` holds every operation the Python SDK and the Python plugins call to
  openapi.yaml, the server's enable list and its route table (module `client_routes`).

Commands (run from anywhere; `--root` defaults to the repository of this file):

    api_checks.py check [--rule <rule>]                         # exit 0 clean, 1 findings
    api_checks.py generate                                      # rewrite the generated blocks

Every finding is one line `<rule>: <sub-check>: <subject>: <detail>`; the sub-check `source`
reports an input that cannot be read or does not have the shape the checks walk.
"""

from __future__ import annotations

import argparse
import datetime
import os
import re
import shutil
import sys
import tempfile
from collections import Counter
from collections.abc import Callable, Iterator, Mapping, Sequence
from dataclasses import dataclass
from pathlib import Path
from types import MappingProxyType
from typing import Any

import yaml

# The Makefile runs this file as a script, so Python puts only tools/api-checks on sys.path;
# the catalogue module lives in tools/catalog (pytest finds it through its pythonpath setting).
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "catalog"))

import action_catalog
import client_routes
import gdpr_export
import viz_fixtures
from action_catalog import Catalog, CatalogError

REPO = Path(__file__).resolve().parents[2]
CATALOG_PATH = action_catalog.CATALOG_PATH
SYNC_PATH = Path("docs/spec/04-sync-protocol.md")
CHAPTER_PATH = Path("docs/spec/06-api.md")
OPENAPI_PATH = Path("docs/api/openapi.yaml")
ASYNCAPI_PATH = Path("docs/api/asyncapi.yaml")
WEBHOOKS_PATH = Path("docs/api/webhooks.md")
CONFIG_PATH = Path("docs/operations/configuration.md")

RULE_SYNC = "api-catalogue-sync"
RULE_TABLES = "api-tables"
RULE_VIZ = viz_fixtures.RULE
RULE_GDPR = gdpr_export.RULE
RULE_CLIENTS = client_routes.RULE
RULES = (RULE_SYNC, RULE_TABLES, RULE_VIZ, RULE_GDPR, RULE_CLIENTS)
SOURCE = "source"

HTTP_METHODS = ("get", "put", "post", "delete", "options", "head", "patch", "trace")
"""Operation keys of an OpenAPI 3.1 path item, in the order of the specification."""

WEBHOOK_ACTION = "webhook_out.deliver"
"""The action of every entry under `webhooks:` (06 section 4, item 7)."""

GATEWAY = "gateway"
"""Key of the gateway channel in asyncapi.yaml; the other channels are the terminal ones."""

DISPATCH = "dispatch"
"""Gateway op of the operation with many messages, one per event type."""

RECEIVE_DIRECTIONS = frozenset({"device_to_server", "host_to_server", "both"})
"""Catalogue directions of a terminal channel row that an asyncapi `receive` operation fits."""

SEND_DIRECTIONS = frozenset({"server_to_device", "server_to_host", "both"})
"""Catalogue directions of a terminal channel row that an asyncapi `send` operation fits."""

CLOSE_CODES_TITLE = "Close codes"
"""Title of the section of 04 whose table lists the gateway close codes (6.4)."""

TERMINAL_CLOSE_CODES_FROM = 4100
"""First close code of the terminal channels' own range (04 6.4, 24 section 5); a lower code on
a terminal channel is one of the table of 04 and must keep its name."""

NO_APP_SCOPE = "none"
"""`app_scope` of an action that apps reach without a scope (05 section 2.6)."""

APP_SCOPE_SCHEMA = "AppScope"
"""Schema of openapi.yaml that lists the scopes an app can hold (05 section 2.6)."""

EVENT_UNION = "Event"
"""Schema of openapi.yaml: union of every durable event type (06 section 11, item 3)."""

IDEMPOTENCY_HEADER = "Idempotency-Key"
"""Header of 04 section 5.2; HTTP header names compare case-insensitively."""

EXEMPTION_REASONS: Mapping[str, str] = MappingProxyType(
    {
        "auth_flow": (
            "login or session flow: one-time challenge or code, refresh rotation, "
            "replacement of the child session"
        ),
        "read_only": "reads only, changes no durable state",
        "agent_runtime": "agent runtime: monotonic or compare-and-swap",
        "client_id": "client ID in the body (table above)",
        "origin_key": (
            "origin key of the import ([22-import-migration.md](22-import-migration.md))"
        ),
        "telemetry": "anonymous measurements; a retry counts a sample twice",
    }
)
"""Codes of `x-idempotency-exempt` (06 section 7, item 3, R-API-24) with the text of the
block `idempotency_exempt` of chapter 06, in the order of that block. The texts render the
chapter; the link of `origin_key` is relative to docs/spec/."""

DEPRECATION_KEYS = ("x-deprecated-since", "x-deprecated-at", "x-sunset")
"""Fields every `deprecated: true` needs (06 13.4, R-API-25); none of them stands alone."""

MIN_NOTICE_KEY = "api.deprecation_min_notice"
"""Named configuration value: least distance from `x-deprecated-at` to `x-sunset`."""

CONFIG_SECTION = "Named values of all chapters"
"""Section of docs/operations/configuration.md that lists every named value."""

DURATION_UNITS: Mapping[str, datetime.timedelta] = MappingProxyType(
    {
        "ms": datetime.timedelta(milliseconds=1),
        "s": datetime.timedelta(seconds=1),
        "m": datetime.timedelta(minutes=1),
        "h": datetime.timedelta(hours=1),
        "d": datetime.timedelta(days=1),
    }
)
"""Units of a `duration` (docs/operations/configuration.md section 2, `d` = 24 h)."""

_CATALOG = CATALOG_PATH.as_posix()
_ASYNCAPI = ASYNCAPI_PATH.as_posix()
_SYNC_TABLE = f'{SYNC_PATH.as_posix()} "{CLOSE_CODES_TITLE}"'
_ASYNC_CODES = f"{_ASYNCAPI} channels.{GATEWAY}.x-close-codes"
_CONFIG_TABLE = f'{CONFIG_PATH.as_posix()} "{CONFIG_SECTION}"'


class ApiCheckError(Exception):
    """An input cannot be read, has the wrong shape, cannot be rendered or cannot be written.

    Args:
        findings: One finding line per problem, each starting with the rule id.
        written: Files `generate` replaced before a later replacement failed; empty when
            nothing was written.

    Attributes:
        findings: The same lines as a list, for callers that report them one by one.
        written: The same paths as a list.
    """

    def __init__(self, findings: Sequence[str], written: Sequence[Path] = ()) -> None:
        """Keep the findings and the files already written."""
        self.findings = list(findings)
        self.written = list(written)
        super().__init__("\n".join(self.findings))


# ---- reading the inputs --------------------------------------------------------------------

# The C composer when PyYAML has libyaml; nodes only, objects come from the safe constructor.
_COMPOSER = getattr(yaml, "CSafeLoader", yaml.SafeLoader)


def _duplicate_keys(root: yaml.Node | None) -> list[str]:
    """Keys given twice in one mapping; the safe loader would keep the last one silently.

    Walks the node graph once per node, so recursive or shared aliases cannot loop or fan out.
    """
    found: list[tuple[int, str]] = []
    stack = [root]
    visited: set[int] = set()
    while stack:
        node = stack.pop()
        if node is None or id(node) in visited:
            continue
        visited.add(id(node))
        if isinstance(node, yaml.MappingNode):
            keys: set[str] = set()
            for key, value in node.value:
                if isinstance(key, yaml.ScalarNode):
                    if key.value in keys:
                        found.append((key.start_mark.line + 1, key.value))
                    keys.add(key.value)
                stack.append(value)
        elif isinstance(node, yaml.SequenceNode):
            stack.extend(node.value)
    return [f"line {line}: duplicate key {key!r}" for line, key in sorted(found)]


def _one_line(error: BaseException) -> str:
    try:
        text = str(error)
    except Exception:  # str() of an error about a huge integer key raises ValueError itself
        return "message cannot be shown"
    return " ".join(text.split())


def _failure(text: str, error: BaseException) -> str:
    """Finding detail for an unexpected error, for example `input not checkable (...)`."""
    return f"{text} ({type(error).__name__}: {_one_line(error)})"


# PyYAML resolves some scalars (an unquoted date that is no real day, for example) and then
# fails to build them with plain Python errors, not with yaml.YAMLError; deep nesting can
# exhaust the stack. Each of them is a finding, never a traceback.
_CONSTRUCTOR_ERRORS = (
    ValueError,
    TypeError,
    KeyError,
    AttributeError,
    OverflowError,
    RecursionError,
)


def _guarded(run: Callable[[], list[str]], failure: str) -> list[str]:
    """Run one guarded part; an unexpected error becomes its finding instead of a traceback.

    It covers exactly the code inside `run`: each sub-check of `api-catalogue-sync`, each block
    of `api-tables`, and the shape stage of each API document in `_load_contracts`. They walk
    contract values of any type, so a value of an unforeseen type must not abort the run and
    hide the findings of the other parts; the parts still report the cases they know with
    precise findings. Outside them, reading an input reports the errors it expects as `source`
    findings (`_Inputs`), `generate` reports a file it cannot write (`_write_all`), and `main`
    turns any other error into one finding line, so the command line never ends in a
    traceback.

    Args:
        run: The guarded part, returning finding details.
        failure: Detail prefix for an unexpected error, for example `input not checkable`.

    Returns:
        The details of `run`, or one detail naming the error.
    """
    try:
        return run()
    except Exception as exc:  # deliberately broad, see the docstring
        return [_failure(failure, exc)]


class _Inputs:
    """Reads the inputs of one rule and collects every problem as a `source` finding."""

    def __init__(self, root: Path, rule: str) -> None:
        self.root = root
        self.rule = rule
        self.findings: list[str] = []

    def fail(self, rel: Path, detail: str) -> None:
        """Record a `source` finding for the input `rel`."""
        self.findings.append(f"{self.rule}: {SOURCE}: {rel.as_posix()}: {detail}")

    def text(self, rel: Path) -> str | None:
        """The UTF-8 text of `rel`, or None after recording why it cannot be read."""
        try:
            return (self.root / rel).read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError) as exc:
            self.fail(rel, f"cannot read: {_one_line(exc)}")
            return None

    def yaml(self, rel: Path) -> Mapping[str, Any]:
        """The YAML mapping of `rel`, or an empty mapping after recording the problems."""
        text = self.text(rel)
        if text is None:
            return {}
        try:
            node = yaml.compose(text, Loader=_COMPOSER)
            duplicates = _duplicate_keys(node)
            data = None if node is None else yaml.SafeLoader("").construct_document(node)
        except yaml.YAMLError as exc:
            self.fail(rel, f"invalid YAML: {_one_line(exc)}")
            return {}
        except _CONSTRUCTOR_ERRORS as exc:
            self.fail(rel, f"cannot be loaded: {_one_line(exc)}")
            return {}
        for duplicate in duplicates:
            self.fail(rel, duplicate)
        if not isinstance(data, dict):
            self.fail(rel, "not a mapping")
            return {}
        return data

    def catalog(self) -> Catalog | None:
        """The action catalogue, or None after recording why it cannot be loaded."""
        try:
            return action_catalog.load_catalog(self.root / CATALOG_PATH)
        except CatalogError as err:
            for finding in err.findings:
                self.fail(CATALOG_PATH, finding)
        except _CONSTRUCTOR_ERRORS as exc:
            self.fail(CATALOG_PATH, f"cannot be loaded: {_one_line(exc)}")
        return None


class _Shape:
    """Container checks: every mapping and list the checks walk has its type."""

    def __init__(self) -> None:
        self.problems: list[str] = []

    def mapping(self, value: object, where: str) -> Mapping[Any, Any]:
        """`value` as a mapping; missing or another type is a problem and gives `{}`."""
        if value is None:
            self.problems.append(f"{where}: missing")
        elif not isinstance(value, Mapping):
            self.problems.append(f"{where}: not a mapping")
        else:
            return value
        return {}

    def optional_mapping(
        self, container: Mapping[Any, Any], key: str, where: str
    ) -> Mapping[Any, Any]:
        """`container[key]` as a mapping; absent is empty, an explicit null is a problem."""
        if key not in container:
            return {}
        if container[key] is None:
            self.problems.append(f"{where}: null; remove the key or give a mapping")
            return {}
        return self.mapping(container[key], where)

    def optional_list(
        self, container: Mapping[Any, Any], key: str, where: str
    ) -> list[Mapping[Any, Any]]:
        """`container[key]` as a list of mappings; absent is empty, null is a problem."""
        if key not in container:
            return []
        value = container[key]
        if value is None:
            self.problems.append(f"{where}: null; remove the key or give a list")
            return []
        if not isinstance(value, list):
            self.problems.append(f"{where}: not a list")
            return []
        return [self.mapping(item, f"{where}[{i}]") for i, item in enumerate(value)]

    def mapping_values(self, value: object, where: str) -> None:
        """`value` is a mapping whose every value is a mapping."""
        for key, item in self.mapping(value, where).items():
            self.mapping(item, f"{where}.{key}")


def _openapi_shape(doc: Mapping[str, Any]) -> list[str]:
    shape = _Shape()
    for section in ("paths", "webhooks"):
        for key, raw in shape.mapping(doc.get(section), section).items():
            item = shape.mapping(raw, f"{section}.{key}")
            if "$ref" in item:
                # Its operations would escape every check and drop out of the tables.
                shape.problems.append(
                    f"{section}.{key}: a path item given by $ref is not supported; write its "
                    "operations inline"
                )
            shape.optional_list(item, "parameters", f"{section}.{key}.parameters")
            for method in HTTP_METHODS:
                if method not in item:
                    continue
                where = f"{section}.{key}.{method}"
                operation = shape.mapping(item[method], where)
                shape.optional_list(operation, "parameters", f"{where}.parameters")
                responses = shape.optional_mapping(operation, "responses", f"{where}.responses")
                for status, response in responses.items():
                    shape.mapping(response, f"{where}.responses.{status}")
    components = shape.mapping(doc.get("components"), "components")
    for name in ("responses", "parameters", "schemas"):
        shape.mapping_values(components.get(name), f"components.{name}")
    shape.optional_list(doc, "tags", "tags")
    return shape.problems


def _asyncapi_shape(doc: Mapping[str, Any]) -> list[str]:
    shape = _Shape()
    for key, raw in shape.mapping(doc.get("channels"), "channels").items():
        channel = shape.mapping(raw, f"channels.{key}")
        messages = shape.optional_mapping(channel, "messages", f"channels.{key}.messages")
        for name, message in messages.items():
            shape.mapping(message, f"channels.{key}.messages.{name}")
    for key, raw in shape.mapping(doc.get("operations"), "operations").items():
        operation = shape.mapping(raw, f"operations.{key}")
        shape.mapping(operation.get("channel"), f"operations.{key}.channel")
        shape.optional_list(operation, "messages", f"operations.{key}.messages")
        reply = shape.optional_mapping(operation, "reply", f"operations.{key}.reply")
        shape.optional_list(reply, "messages", f"operations.{key}.reply.messages")
    components = shape.mapping(doc.get("components"), "components")
    shape.mapping_values(components.get("messages"), "components.messages")
    return shape.problems


@dataclass(frozen=True)
class _Contracts:
    """The catalogue and both API documents, read and with the shape the checks walk."""

    catalog: Catalog
    openapi: Mapping[str, Any]
    asyncapi: Mapping[str, Any]


def _load_contracts(inputs: _Inputs) -> _Contracts | None:
    catalog = inputs.catalog()
    openapi = inputs.yaml(OPENAPI_PATH)
    asyncapi = inputs.yaml(ASYNCAPI_PATH)
    if inputs.findings or catalog is None:
        return None
    # The shape stage prints keys of any type (str() of a huge integer raises), so it runs
    # guarded like a sub-check.
    for rel, shape in (
        (OPENAPI_PATH, lambda: _openapi_shape(openapi)),
        (ASYNCAPI_PATH, lambda: _asyncapi_shape(asyncapi)),
    ):
        for problem in _guarded(shape, "input not checkable"):
            inputs.fail(rel, problem)
    if inputs.findings:
        return None
    return _Contracts(catalog, openapi, asyncapi)


# ---- Markdown sections and tables ----------------------------------------------------------

_ATX_HEADING = re.compile(r"^ {0,3}(#{1,6})(?:[ \t]+(.*?))?[ \t]*$")
_CLOSING_HASHES = re.compile(r"(?:^|[ \t]+)#+$")
_SECTION_NUMBER = re.compile(r"^[0-9]+(?:\.[0-9]+)*\.?[ \t]+")
_FENCE = re.compile(r"^ {0,3}(`{3,}|~{3,})(.*)$")
_SEPARATOR_ROW = re.compile(r"^\|(?:[ \t]*:?-+:?[ \t]*\|)+[ \t]*$")
_UNESCAPED_PIPE = re.compile(r"(?<!\\)\|")


def _heading_title(raw: str) -> str:
    """Heading text without closing hashes and without a leading section number."""
    return _SECTION_NUMBER.sub("", _CLOSING_HASHES.sub("", raw.strip()))


def _sections(text: str, title: str) -> list[list[str]]:
    """Bodies of every ATX section titled `title` (number ignored), headings in fences skipped.

    A body runs to the next heading of the same or a higher level, as in Markdown.
    """
    lines = text.splitlines()
    headings: list[tuple[int, int, str]] = []
    fence: str | None = None
    for index, line in enumerate(lines):
        fence_match = _FENCE.match(line)
        if fence is not None:
            marker = fence_match.group(1) if fence_match else ""
            closes = marker[:1] == fence[:1] and len(marker) >= len(fence)
            if fence_match and closes and not fence_match.group(2).strip():
                fence = None
            continue
        if fence_match:
            fence = fence_match.group(1)
            continue
        if (match := _ATX_HEADING.match(line)) is not None:
            headings.append((index, len(match.group(1)), _heading_title(match.group(2) or "")))
    bodies: list[list[str]] = []
    for position, (index, level, heading) in enumerate(headings):
        if heading != title:
            continue
        end = next((i for i, lvl, _ in headings[position + 1 :] if lvl <= level), len(lines))
        bodies.append(lines[index + 1 : end])
    return bodies


def _cells(line: str) -> list[str]:
    parts = _UNESCAPED_PIPE.split(line.strip())
    if parts and not parts[0].strip():
        parts = parts[1:]
    if parts and not parts[-1].strip():
        parts = parts[:-1]
    return [part.strip() for part in parts]


def _tables(body: Sequence[str]) -> Iterator[tuple[list[str], list[list[str]]]]:
    """Every table of a section body as (header cells, rows of cells); separator rows dropped."""
    block: list[str] = []
    for line in [*body, ""]:
        if line.startswith("|"):
            block.append(line)
            continue
        rows = [_cells(row) for row in block if not _SEPARATOR_ROW.match(row)]
        if rows:
            yield rows[0], rows[1:]
        block = []


# ---- operations of openapi.yaml ------------------------------------------------------------


@dataclass(frozen=True)
class _Operation:
    """One operation of `paths` or `webhooks` in file order."""

    key: str
    method: str
    item: Mapping[str, Any]
    op: Mapping[str, Any]

    @property
    def label(self) -> str:
        """`operationId`, or method and path when it is missing."""
        op_id = self.op.get("operationId")
        return op_id if isinstance(op_id, str) and op_id else f"{self.method.upper()} {self.key}"


def _operations(openapi: Mapping[str, Any], section: str = "paths") -> list[_Operation]:
    """Operations in file order: paths as written, methods as written in each path item."""
    return [
        _Operation(str(key), method, item, item[method])
        for key, item in openapi[section].items()
        for method in item
        if method in HTTP_METHODS
    ]


# ---- operations of asyncapi.yaml -----------------------------------------------------------

_MAX_REF_HOPS = 16
_LIST_INDEX = re.compile(r"[0-9]{1,9}")


def _unescape(segment: str) -> str:
    """One JSON pointer segment (RFC 6901): `~1` is `/`, `~0` is `~`."""
    return segment.replace("~1", "/").replace("~0", "~")


def _pointer(doc: Mapping[str, Any], ref: str) -> object:
    """The node a local JSON pointer `#/...` names, or None."""
    if not ref.startswith("#/"):
        return None
    node: object = doc
    for raw in ref[2:].split("/"):
        part = _unescape(raw)
        if isinstance(node, Mapping) and part in node:
            node = node[part]
        elif isinstance(node, list) and _LIST_INDEX.fullmatch(part) and int(part) < len(node):
            node = node[int(part)]
        else:
            return None
    return node


def _resolve(doc: Mapping[str, Any], node: object) -> Mapping[str, Any] | None:
    """Follow local `$ref` chains to a mapping; None when a reference does not resolve."""
    for _ in range(_MAX_REF_HOPS):
        if not isinstance(node, Mapping):
            return None
        ref = node.get("$ref")
        if ref is None:
            return node
        if not isinstance(ref, str):
            return None
        node = _pointer(doc, ref)
    return None


def _camel(name: str) -> str:
    """Catalogue channel name to asyncapi channel key: `terminal_stream` -> `terminalStream`."""
    first, *rest = name.split("_")
    return first + "".join(part[:1].upper() + part[1:] for part in rest)


@dataclass(frozen=True)
class _AsyncOperation:
    """One entry of `operations` in asyncapi.yaml with its channel and message names."""

    key: str
    op: Mapping[str, Any]
    channel: str | None  # channel key, None when the reference does not resolve
    channel_ref: object
    count: int  # message references, resolved or not
    names: tuple[str, ...]  # `name` of every resolved message
    problems: tuple[str, ...]  # message references that do not resolve
    messages: tuple[Mapping[str, Any], ...] = ()  # the resolved messages, in `names` order

    @property
    def action(self) -> object:
        """The AsyncAPI `action`, `receive` or `send` in a valid document."""
        return self.op.get("action")


def _async_operations(asyncapi: Mapping[str, Any]) -> list[_AsyncOperation]:
    out: list[_AsyncOperation] = []
    channels = asyncapi["channels"]
    for raw_key, op in asyncapi["operations"].items():
        ref = op["channel"].get("$ref")
        channel = None
        if isinstance(ref, str) and ref.startswith("#/channels/"):
            key = _unescape(ref.removeprefix("#/channels/"))
            channel = key if key in channels else None
        names: list[str] = []
        problems: list[str] = []
        messages: list[Mapping[str, Any]] = []
        items = op.get("messages") or []
        for item in items:
            if item.get("$ref") is None:
                # AsyncAPI 3 names the messages of an operation by reference into its channel;
                # an inline copy would reach the tables without the checks of its message.
                problems.append(f"message {item.get('name')!r} {INLINE_MESSAGE}")
            message = _resolve(asyncapi, item)
            name = None if message is None else message.get("name")
            if message is None:
                problems.append(f"message reference {item.get('$ref')!r} does not resolve")
            elif not isinstance(name, str) or not name:
                problems.append(f"message {item.get('$ref')!r} has no name")
            else:
                names.append(name)
                messages.append(message)
        out.append(
            _AsyncOperation(
                str(raw_key),
                op,
                channel,
                ref,
                len(items),
                tuple(names),
                tuple(problems),
                tuple(messages),
            )
        )
    return out


INLINE_MESSAGE = "is written inline; AsyncAPI 3 lists the messages of an operation by $ref"
"""Detail of an operation message without `$ref`; the operation counts as broken."""


def _dispatch(operations: Sequence[_AsyncOperation]) -> _AsyncOperation | None:
    """The operation with many messages on the gateway channel: the op `dispatch`."""
    return next((o for o in operations if o.channel == GATEWAY and o.count > 1), None)


BINARY_CONTENT_TYPE = "application/octet-stream"
"""Content type of the binary Noise frame (asyncapi.yaml `noise.frame`) of the terminal
channels, the only frame without `op`; every other frame is JSON `{"op": ..., "d": ...}`, and
the gateway has JSON frames only (04 2.3, 24 section 5)."""


def _frame_op_findings(
    operation: _AsyncOperation, expected: str | None, *, binary_frames: bool
) -> list[str]:
    """The `op` of each frame (`payload.properties.op.enum`) names its op.

    Args:
        operation: The asyncapi operation.
        expected: The op every message frames; None means the message's own `name`.
        binary_frames: Whether the channel carries the binary Noise frame (a terminal channel);
            False for the gateway.

    Returns:
        One finding per frame whose `op` enum is not exactly that op. Only a frame of type
        `application/octet-stream` that declares no `op`, on a channel with binary frames, is
        exempt; a frame that declares `op` is checked whatever its type says.
    """
    out: list[str] = []
    for name, message in zip(operation.names, operation.messages, strict=True):
        op = _get(message, "payload", "properties", "op")
        if binary_frames and op is None and message.get("contentType") == BINARY_CONTENT_TYPE:
            continue
        enum = _get(op, "enum")
        wanted = [expected or name]
        if enum != wanted:
            out.append(f"{operation.key}: message {name!r} frames op {enum!r}, expected {wanted!r}")
    return out


def _async_action_findings(
    catalog: Catalog, operation: _AsyncOperation, action: str, whose: str
) -> list[str]:
    """`x-authz-action` equals the catalogue action, `x-rate-limit-class` its class."""
    out: list[str] = []
    key, op = operation.key, operation.op
    if op.get("x-authz-action") != action:
        out.append(
            f"{key}: x-authz-action {op.get('x-authz-action')!r} differs from {action!r}, {whose}"
        )
    rl_class = catalog.actions_by_id[action].rl_class
    if op.get("x-rate-limit-class") != rl_class:
        out.append(
            f"{key}: x-rate-limit-class {op.get('x-rate-limit-class')!r} differs from "
            f"{rl_class!r}, the class of {action!r} in {_CATALOG}"
        )
    return out


def _get(node: object, *keys: str) -> object:
    """`node[k1][k2]...`, or None as soon as a level is missing or not a mapping."""
    for key in keys:
        node = node.get(key) if isinstance(node, Mapping) else None
    return node


def _mapping_or_none(value: object) -> Mapping[str, Any] | None:
    return value if isinstance(value, Mapping) else None


def _kind(value: object) -> str:
    if isinstance(value, Mapping):
        return "a mapping"
    if isinstance(value, list):
        return "a list"
    if isinstance(value, bool):
        return "a boolean"
    return f"a {type(value).__name__}"


# ---- api-catalogue-sync --------------------------------------------------------------------


_OPENAPI_FILE = "openapi.yaml"


def _schema_ref(openapi: Mapping[str, Any], ref: object) -> tuple[str, Mapping[str, Any]] | None:
    """Name and schema behind a `$ref` into openapi.yaml (local, or `./openapi.yaml#/...`)."""
    if not isinstance(ref, str) or "#" not in ref:
        return None
    file, pointer = ref.split("#", 1)
    if file and Path(file).name != _OPENAPI_FILE:
        return None
    schema = _resolve(openapi, {"$ref": f"#{pointer}"})
    return None if schema is None else (_unescape(pointer.rsplit("/", 1)[-1]), schema)


def _type_enum(openapi: Mapping[str, Any], schema: Mapping[str, Any]) -> object:
    """`properties.type.enum` of an event schema, directly or in one of its `allOf` parts."""
    parts = schema.get("allOf")
    for part in [schema, *(parts if isinstance(parts, list) else [])]:
        enum = _get(_resolve(openapi, part), "properties", "type", "enum")
        if enum is not None:
            return enum
    return None


def _event_schema_findings(
    openapi: Mapping[str, Any], subject: str, ref: object, event_type: str
) -> list[str]:
    """The schema behind `ref` is the one of `event_type`: its `type` is exactly that type."""
    found = _schema_ref(openapi, ref)
    if found is None:
        return [f"{subject}: schema reference {ref!r} does not resolve in {_OPENAPI_FILE}"]
    name, schema = found
    enum = _type_enum(openapi, schema)
    if enum != [event_type]:
        return [f"{subject}: schema {name} has type {enum!r}, expected {[event_type]!r}"]
    return []


def _app_scope_findings(catalog: Catalog, openapi: Mapping[str, Any]) -> list[str]:
    """Sub-check `app-scopes`: the enum `AppScope` lists exactly the scopes of 05 section 2.6."""
    subject = APP_SCOPE_SCHEMA
    schema = _resolve(openapi, openapi["components"]["schemas"].get(APP_SCOPE_SCHEMA))
    enum = _get(schema, "enum")
    if not isinstance(enum, list):
        return [f"{subject}: the enum is missing"]
    scopes = {a.app_scope for a in catalog.actions if a.app_scope not in (None, NO_APP_SCOPE)}
    out = [
        f"{subject}: scope {scope!r} of {_CATALOG} (05 section 2.6) is missing from the enum"
        for scope in sorted(scopes)
        if scope not in enum
    ]
    seen: set[str] = set()
    for value in enum:
        if not isinstance(value, str) or value not in scopes:
            out.append(f"{subject}: {value!r} is not a scope of {_CATALOG}")
        elif value in seen:
            out.append(f"{subject}: {value!r} is listed twice")
        else:
            seen.add(value)
    return out


def _rest_action_findings(catalog: Catalog, openapi: Mapping[str, Any]) -> list[str]:
    """Sub-check `rest-action`: each operation names one known, non-system action and its class."""
    out: list[str] = []
    for operation in _operations(openapi):
        label, op = operation.label, operation.op
        action = op.get("x-authz-action")
        if action is None:
            out.append(f"{label}: x-authz-action is missing")
            continue
        if not isinstance(action, str):
            out.append(f"{label}: x-authz-action must name exactly one action, not {_kind(action)}")
            continue
        known = catalog.actions_by_id.get(action)
        if known is None:
            out.append(f"{label}: x-authz-action {action!r} is not an action of {_CATALOG}")
        elif known.system:
            out.append(f"{label}: x-authz-action {action!r} is a system action (05 rule 10)")
        elif op.get("x-rate-limit-class") != known.rl_class:
            out.append(
                f"{label}: x-rate-limit-class {op.get('x-rate-limit-class')!r} differs from "
                f"{known.rl_class!r}, the class of {action!r} in {_CATALOG}"
            )
    return out


def _webhook_findings(catalog: Catalog, openapi: Mapping[str, Any]) -> list[str]:
    """Sub-check `webhook-events`: each entry of `webhooks` is a catalogue event type."""
    out: list[str] = []
    operations: dict[str, list[_Operation]] = {}
    for operation in _operations(openapi, "webhooks"):
        operations.setdefault(operation.key, []).append(operation)
    for raw_type in openapi["webhooks"]:
        event_type = str(raw_type)
        event = catalog.events_by_type.get(event_type)
        if event is None:
            out.append(f"{event_type}: not an event type of {_CATALOG}")
            continue
        if not operations.get(event_type):
            out.append(f"{event_type}: has no operation")
        for operation in operations.get(event_type, []):
            op = operation.op
            if op.get("x-receive-action") != event.receive_action:
                out.append(
                    f"{event_type}: x-receive-action {op.get('x-receive-action')!r} differs from "
                    f"{event.receive_action!r} in {_CATALOG}"
                )
            if op.get("x-authz-action") != WEBHOOK_ACTION:
                out.append(
                    f"{event_type}: x-authz-action {op.get('x-authz-action')!r} must be "
                    f"{WEBHOOK_ACTION!r}"
                )
            if op.get("x-rate-limit-class") != action_catalog.NONE_CLASS:
                out.append(
                    f"{event_type}: x-rate-limit-class {op.get('x-rate-limit-class')!r} must be "
                    f"{action_catalog.NONE_CLASS!r}"
                )
            out += _event_schema_findings(
                openapi,
                event_type,
                _get(op, "requestBody", "content", "application/json", "schema", "$ref"),
                event_type,
            )
    return out


def _ws_op_findings(catalog: Catalog, operations: Sequence[_AsyncOperation]) -> list[str]:
    """Sub-check `ws-ops`: the operations on the gateway channel equal the catalogue ops."""
    out: list[str] = []
    described: dict[str, str] = {}
    for operation in operations:
        if operation.channel != GATEWAY:
            continue
        key = operation.key
        if operation.problems or not operation.count:
            out += [f"{key}: {problem}" for problem in operation.problems] or [
                f"{key}: has no message"
            ]
            continue
        name = DISPATCH if operation.count > 1 else operation.names[0]
        ws_op = catalog.ws_ops_by_name.get(name)
        if name in described:
            out.append(f"{key}: op {name!r} is already described by operation {described[name]}")
            continue
        if ws_op is None:
            out.append(f"{key}: op {name!r} is not a gateway op of {_CATALOG}")
            continue
        described[name] = key
        out += _frame_op_findings(
            operation, DISPATCH if name == DISPATCH else None, binary_frames=False
        )
        expected = "receive" if ws_op.direction.endswith("_to_server") else "send"
        if operation.action != expected:
            out.append(
                f"{key}: action {operation.action!r} does not match direction "
                f"{ws_op.direction!r} of op {name!r} in {_CATALOG}"
            )
        out += _async_action_findings(
            catalog, operation, ws_op.action, f"the action of op {name!r} in {_CATALOG}"
        )
    out += [
        f"{name}: no operation on channel {GATEWAY} describes this op of {_CATALOG}"
        for name in catalog.ws_ops_by_name
        if name not in described
    ]
    return out


def _channel_op_findings(catalog: Catalog, operations: Sequence[_AsyncOperation]) -> list[str]:
    """Sub-check `channel-ops`: operations on the terminal channels equal `channel_ops`."""
    out: list[str] = []
    channels = {_camel(row.channel): row.channel for row in catalog.channel_ops}
    covered: set[tuple[int, str]] = set()
    described: dict[tuple[str, str, str], str] = {}
    for operation in operations:
        key = operation.key
        if operation.channel == GATEWAY:
            continue
        if operation.channel is None:
            out.append(f"{key}: channel reference {operation.channel_ref!r} does not resolve")
            continue
        channel = channels.get(operation.channel)
        if channel is None:
            out.append(
                f"{key}: channel {operation.channel!r} is neither the gateway nor a channel of "
                f"{_CATALOG}"
            )
            continue
        if operation.problems or not operation.count:
            out += [f"{key}: {problem}" for problem in operation.problems] or [
                f"{key}: has no message"
            ]
            continue
        out += _frame_op_findings(operation, None, binary_frames=True)
        allowed = {"receive": RECEIVE_DIRECTIONS, "send": SEND_DIRECTIONS}.get(
            str(operation.action), frozenset()
        )
        for name in operation.names:
            first = described.setdefault((channel, name, str(operation.action)), key)
            if first != key:
                out.append(
                    f"{key}: op {name!r} ({operation.action}) on channel {channel!r} is already "
                    f"described by operation {first}"
                )
                continue
            rows = [
                (index, row)
                for index, row in enumerate(catalog.channel_ops)
                if row.channel == channel and name in row.ops
            ]
            fitting = [(index, row) for index, row in rows if row.direction in allowed]
            if not rows:
                out.append(f"{key}: op {name!r} is not an op of channel {channel!r} in {_CATALOG}")
                continue
            if not fitting:
                directions = " or ".join(sorted({repr(row.direction) for _, row in rows}))
                out.append(
                    f"{key}: action {operation.action!r} does not match direction {directions} "
                    f"of op {name!r} on channel {channel!r} in {_CATALOG}"
                )
                continue
            covered.update((index, name) for index, _ in fitting)
            actions = [row.action for _, row in fitting]
            action = next((a for a in actions if a == operation.op.get("x-authz-action")), None)
            out += _async_action_findings(
                catalog,
                operation,
                action or actions[0],
                f"the action of op {name!r} on channel {channel!r} in {_CATALOG}",
            )
    for index, row in enumerate(catalog.channel_ops):
        out += [
            f"{row.channel}/{name}: no operation on channel {_camel(row.channel)} describes "
            f"this op of {_CATALOG}"
            for name in row.ops
            if (index, name) not in covered
        ]
    return out


def _message_objects(asyncapi: Mapping[str, Any]) -> list[tuple[str, Mapping[str, Any]]]:
    """Every distinct message object: components first, then messages written in a channel.

    `dispatch` may point at a channel message that is not a reference to a component; it must
    be checked like one. Each object counts once, however many references lead to it.
    """
    found = [(str(key), message) for key, message in asyncapi["components"]["messages"].items()]
    seen = {id(message) for _, message in found}
    for channel_key, channel in asyncapi["channels"].items():
        for message_key, raw in (channel.get("messages") or {}).items():
            message = _resolve(asyncapi, raw)
            if message is not None and id(message) not in seen:
                seen.add(id(message))
                found.append((f"channels.{channel_key}.messages.{message_key}", message))
    return found


def _event_messages(asyncapi: Mapping[str, Any], out: list[str]) -> dict[str, Mapping[str, Any]]:
    """Messages with `x-event-kind` by `name`; problems of the message list go to `out`."""
    by_name: dict[str, Mapping[str, Any]] = {}
    for key, message in _message_objects(asyncapi):
        if "x-event-kind" not in message:
            if "x-receive-action" in message or "x-audience" in message:
                out.append(f"{key}: x-receive-action or x-audience without x-event-kind")
            continue
        name = message.get("name")
        if not isinstance(name, str) or not name:
            out.append(f"{key}: message with x-event-kind has no name")
        elif name in by_name:
            out.append(f"{key}: event message {name!r} is described twice")
        else:
            by_name[name] = message
    return by_name


def _union_findings(catalog: Catalog, openapi: Mapping[str, Any]) -> list[str]:
    """The union `Event` covers exactly the durable event types, each with its own schema."""
    subject = EVENT_UNION
    union = openapi["components"]["schemas"].get(EVENT_UNION)
    mapping = _get(union, "discriminator", "mapping")
    one_of = _get(union, "oneOf")
    if not isinstance(mapping, Mapping) or not isinstance(one_of, list):
        return [f"{subject}: oneOf or discriminator.mapping is missing"]
    durable = [event.type for event in catalog.events if event.kind == "durable"]
    out = [
        f"{subject}: durable event type {event_type!r} is missing from discriminator.mapping"
        for event_type in durable
        if event_type not in mapping
    ]
    out += [
        f"{subject}: discriminator.mapping names {event_type!r}, which is not a durable event "
        f"type of {_CATALOG}"
        for event_type in mapping
        if event_type not in durable
    ]
    refs = [_get(item, "$ref") for item in one_of]
    out += [
        f"{subject}: oneOf lists {ref!r} twice"
        for ref, count in Counter(str(r) for r in refs).items()
        if count > 1
    ]
    out += [
        f"{subject}: discriminator.mapping names {ref!r}, which oneOf does not list"
        for ref in mapping.values()
        if ref not in refs
    ]
    out += [
        f"{subject}: oneOf lists {ref!r}, which discriminator.mapping does not name"
        for ref in refs
        if ref not in mapping.values()
    ]
    for event_type, ref in mapping.items():
        found = _schema_ref(openapi, ref)
        enum = None if found is None else _type_enum(openapi, found[1])
        if found is None or enum != [event_type]:
            target = ref if found is None else found[0]
            out.append(
                f"{subject}: discriminator.mapping sends {event_type!r} to {target}, whose type "
                f"is {enum!r}"
            )
    return out


def _event_findings(
    catalog: Catalog,
    openapi: Mapping[str, Any],
    asyncapi: Mapping[str, Any],
    operations: Sequence[_AsyncOperation],
) -> list[str]:
    """Sub-check `events`: event messages, schemas, union `Event` and `dispatch` list.

    All of them equal the catalogue events.
    """
    out = _union_findings(catalog, openapi)
    by_name = _event_messages(asyncapi, out)
    out += [
        f"{name}: message with x-event-kind is not an event type of {_CATALOG}"
        for name in by_name
        if name not in catalog.events_by_type
    ]
    for event in catalog.events:
        message = by_name.get(event.type)
        if message is None:
            out.append(
                f"{event.type}: event type of {_CATALOG} has no message with x-event-kind in "
                f"{_ASYNCAPI}"
            )
            continue
        for field, wanted in (
            ("x-event-kind", event.kind),
            ("x-receive-action", event.receive_action),
        ):
            if message.get(field) != wanted:
                out.append(
                    f"{event.type}: {field} {message.get(field)!r} differs from {wanted!r} in "
                    f"{_CATALOG}"
                )
        audience = message.get("x-audience")
        if event.audience == action_catalog.SELF and audience is None:
            out.append(
                f"{event.type}: x-audience is missing; {_CATALOG} has audience "
                f"{action_catalog.SELF!r}"
            )
        elif event.audience == action_catalog.SELF and audience != action_catalog.SELF:
            out.append(
                f"{event.type}: x-audience {audience!r} differs from {action_catalog.SELF!r} in "
                f"{_CATALOG}"
            )
        elif event.audience is None and audience is not None:
            out.append(f"{event.type}: x-audience {audience!r}, but {_CATALOG} has no audience")
        out += _event_schema_findings(
            openapi, event.type, _get(message, "payload", "properties", "d", "$ref"), event.type
        )
    dispatch = _dispatch(operations)
    if dispatch is None:
        out.append(f"{DISPATCH}: no operation on channel {GATEWAY} lists the event messages")
        return out
    listed = Counter(dispatch.names)
    out += [
        f"{dispatch.key}: does not list the event message {name!r}"
        for name in by_name
        if name not in listed
    ]
    for name, count in listed.items():
        if name not in by_name:
            out.append(f"{dispatch.key}: lists {name!r}, which is not a message with x-event-kind")
        elif count > 1:
            out.append(f"{dispatch.key}: lists {name!r} {count} times")
    return out


_CLOSE_CODE = re.compile(r"`?([0-9]{1,5})`?")


def _chapter_close_codes(sync_text: str) -> tuple[list[tuple[int, str]] | None, list[str]]:
    """(code, name) pairs of the table in 04 "Close codes", in order, and the findings."""
    sections = _sections(sync_text, CLOSE_CODES_TITLE)
    if len(sections) != 1:
        return None, [
            f'{SYNC_PATH.as_posix()}: {len(sections)} sections "{CLOSE_CODES_TITLE}", expected 1'
        ]
    tables = [table for table in _tables(sections[0]) if {"Code", "Name"} <= set(table[0])]
    if len(tables) != 1:
        return None, [f"{_SYNC_TABLE}: {len(tables)} tables with columns Code and Name, expected 1"]
    header, rows = tables[0]
    code_at, name_at = header.index("Code"), header.index("Name")
    out: list[str] = []
    pairs: list[tuple[int, str]] = []
    for row in rows:
        cells = row + [""] * (max(code_at, name_at) + 1 - len(row))
        code = _CLOSE_CODE.fullmatch(cells[code_at])
        if code is None:
            out.append(f"{_SYNC_TABLE}: code {cells[code_at]!r} is not a number")
            continue
        pairs.append((int(code.group(1)), cells[name_at]))
    return pairs, out


def _contract_close_codes(
    asyncapi: Mapping[str, Any], channel: str
) -> tuple[list[tuple[int, str]] | None, list[str]]:
    """(code, name) pairs of `x-close-codes` of one asyncapi channel, and the findings."""
    label = f"{_ASYNCAPI} channels.{channel}.x-close-codes"
    entries = _get(asyncapi["channels"].get(channel), "x-close-codes")
    if not isinstance(entries, list):
        return None, [f"{label}: not a list"]
    out: list[str] = []
    pairs: list[tuple[int, str]] = []
    for index, entry in enumerate(entries):
        code, name = _get(entry, "code"), _get(entry, "name")
        if type(code) is not int or not isinstance(name, str):
            out.append(f"{label}[{index}]: needs an integer code and a name")
        else:
            pairs.append((code, name))
    return pairs, out


def _gateway_close_code_findings(
    chapter: Sequence[tuple[int, str]], contract: Sequence[tuple[int, str]]
) -> list[str]:
    """Pairs of the gateway that differ from 04, or one finding when only the order differs."""
    in_chapter, in_contract = Counter(chapter), Counter(contract)
    if list(chapter) == list(contract):
        return []
    if in_chapter == in_contract:
        return [f"x-close-codes: the order differs from {_SYNC_TABLE}"]
    out: list[str] = []
    for pair in dict.fromkeys([*chapter, *contract]):
        code, name = pair
        here, there = in_chapter[pair], in_contract[pair]
        if here and not there:
            out.append(f"{code} {name}: in {_SYNC_TABLE} but not in {_ASYNC_CODES}")
        elif there and not here:
            out.append(f"{code} {name}: in {_ASYNC_CODES} but not in {_SYNC_TABLE}")
        elif here != there:
            out.append(
                f"{code} {name}: {here} times in {_SYNC_TABLE}, {there} times in {_ASYNC_CODES}"
            )
    return out


def _close_code_findings(
    catalog: Catalog, sync_text: str, asyncapi: Mapping[str, Any]
) -> list[str]:
    """Sub-check `close-codes`: the close codes of 04 "Close codes" on the channels.

    The gateway lists them by code and name in the same order; a terminal channel uses a code of
    that table (below `TERMINAL_CLOSE_CODES_FROM`) only with its name.
    """
    chapter, out = _chapter_close_codes(sync_text)
    if chapter is None:
        return out
    contract, problems = _contract_close_codes(asyncapi, GATEWAY)
    out += problems
    if contract is not None:
        out += _gateway_close_code_findings(chapter, contract)
    for channel in dict.fromkeys(_camel(row.channel) for row in catalog.channel_ops):
        if channel not in asyncapi["channels"]:
            continue  # channel-ops reports the missing channel
        pairs, problems = _contract_close_codes(asyncapi, channel)
        out += problems
        label = f"{_ASYNCAPI} channels.{channel}.x-close-codes"
        for (code, name), count in Counter(pairs or []).items():
            if count > 1:
                times = "twice" if count == 2 else f"{count} times"
                out.append(f"{channel} {code} {name}: listed {times} in {label}")
            if code < TERMINAL_CLOSE_CODES_FROM and (code, name) not in chapter:
                out.append(f"{channel} {code} {name}: in {label} but not in {_SYNC_TABLE}")
    return out


def _has_idempotency_key(openapi: Mapping[str, Any], operation: _Operation) -> bool:
    """Whether a path- or operation-level parameter is the header `Idempotency-Key`."""
    for parameter in [*operation.item.get("parameters", []), *operation.op.get("parameters", [])]:
        resolved = _resolve(openapi, parameter)
        if resolved is None or resolved.get("in") != "header":
            continue
        name = resolved.get("name")
        if isinstance(name, str) and name.casefold() == IDEMPOTENCY_HEADER.casefold():
            return True
    return False


def _idempotency_findings(openapi: Mapping[str, Any]) -> list[str]:
    """Sub-check `idempotency`: POST and PATCH take the key or name a known exemption."""
    out = [
        f"{key}: x-idempotency-exempt belongs on an operation, not on the path item"
        for section in ("paths", "webhooks")
        for key, item in openapi[section].items()
        if "x-idempotency-exempt" in item
    ]
    reasons = ", ".join(EXEMPTION_REASONS)
    for operation in _operations(openapi):
        label, method = operation.label, operation.method.upper()
        # The key counts as carried even with an empty value, as in the table of chapter 06.
        carried = "x-idempotency-exempt" in operation.op
        exempt = operation.op.get("x-idempotency-exempt")
        if operation.method not in ("post", "patch"):
            if carried:
                out.append(
                    f"{label}: x-idempotency-exempt on {method}; only POST and PATCH carry it"
                )
            continue
        has_key = _has_idempotency_key(openapi, operation)
        if not carried and not has_key:
            out.append(
                f"{label}: {method} has neither an {IDEMPOTENCY_HEADER} header parameter nor "
                "x-idempotency-exempt"
            )
        elif carried and has_key:
            out.append(
                f"{label}: has both an {IDEMPOTENCY_HEADER} header parameter and "
                "x-idempotency-exempt"
            )
        if carried and not (isinstance(exempt, str) and exempt in EXEMPTION_REASONS):
            out.append(
                f"{label}: x-idempotency-exempt {exempt!r} is not a known reason ({reasons})"
            )
    out += [
        f"{operation.key}: webhook operations carry no x-idempotency-exempt"
        for operation in _operations(openapi, "webhooks")
        if "x-idempotency-exempt" in operation.op
    ]
    return out


_DURATION = re.compile(r"^([0-9]{1,6})(ms|s|m|h|d)$")
_DATE = re.compile(r"^[0-9]{4}-[0-9]{2}-[0-9]{2}$")
_SEMVER = re.compile(
    r"^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?:-(?:0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)"
    r"(?:\.(?:0|[1-9][0-9]*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*))*)?"
    r"(?:\+[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*)?$"
)
_EXAMPLE_KEYS = frozenset({"example", "examples"})
_NAME_MAPS = frozenset({"properties", "patternProperties"})


def _min_notice(config_text: str) -> tuple[tuple[datetime.timedelta, str] | None, list[str]]:
    """`api.deprecation_min_notice` from configuration.md: (value, text as written), findings."""
    subject = f"{MIN_NOTICE_KEY}:"
    sections = _sections(config_text, CONFIG_SECTION)
    if len(sections) != 1:
        return None, [
            f'{subject} {len(sections)} sections "{CONFIG_SECTION}" in {CONFIG_PATH.as_posix()}, '
            "expected 1"
        ]
    values: list[str] = []
    for header, rows in _tables(sections[0]):
        if "Key" not in header or "Default" not in header:
            continue
        key_at, value_at = header.index("Key"), header.index("Default")
        values += [
            row[value_at]
            for row in rows
            if len(row) > max(key_at, value_at) and row[key_at] == f"`{MIN_NOTICE_KEY}`"
        ]
    if len(values) != 1:
        count = "no row" if not values else f"{len(values)} rows"
        return None, [f"{subject} {count} in {_CONFIG_TABLE}"]
    raw = values[0].replace("`", "").strip()
    match = _DURATION.match(raw)
    if match is None:
        return None, [
            f"{subject} standard value {raw!r} is not a duration (number with unit ms, s, m, h "
            "or d)"
        ]
    return (int(match.group(1)) * DURATION_UNITS[match.group(2)], raw), []


def _date(value: object) -> datetime.date | None:
    """A `full-date`: a YAML date or a string `YYYY-MM-DD` naming a real day."""
    if isinstance(value, datetime.date) and not isinstance(value, datetime.datetime):
        return value
    if isinstance(value, str) and _DATE.match(value):
        try:
            return datetime.date.fromisoformat(value)
        except ValueError:
            return None
    return None


def _escape(key: object) -> str:
    return str(key).replace("~", "~0").replace("/", "~1")


def _deprecation_marks(document: object) -> Iterator[tuple[str, Mapping[str, Any]]]:
    """Every mapping with a deprecation field, as (JSON pointer, mapping), in document order.

    Example values are data, not markers, and are skipped. The keys of `properties` are
    names: a property called `examples` is still walked, and one called `deprecated` or
    `x-sunset` is no marker. Each list and mapping is visited once, so recursive or shared
    YAML aliases cannot loop or fan out.
    """
    stack: list[tuple[object, str, bool]] = [(document, "", False)]
    visited: set[int] = set()
    while stack:
        node, pointer, names = stack.pop()
        if not isinstance(node, (list, Mapping)) or id(node) in visited:
            continue
        visited.add(id(node))
        if isinstance(node, list):
            children = [(item, f"{pointer}/{index}", False) for index, item in enumerate(node)]
        else:
            if not names and ("deprecated" in node or any(key in node for key in DEPRECATION_KEYS)):
                yield pointer, node
            children = [
                (value, f"{pointer}/{_escape(key)}", not names and key in _NAME_MAPS)
                for key, value in node.items()
                if names or key not in _EXAMPLE_KEYS
            ]
        stack.extend(reversed(children))


def _deprecation_findings(
    documents: Sequence[tuple[Path, Mapping[str, Any]]], config_text: str
) -> list[str]:
    """Sub-check `deprecation`: deprecation fields complete and `x-sunset` late enough."""
    min_notice, out = _min_notice(config_text)
    for rel, document in documents:
        for pointer, node in _deprecation_marks(document):
            where = f"{rel.as_posix()}#{pointer}"
            present = [key for key in DEPRECATION_KEYS if key in node]
            if node.get("deprecated") is not True:
                if present:
                    out.append(f"{where}: {', '.join(present)} without deprecated: true")
                continue
            out += [
                f"{where}: deprecated: true without {key}"
                for key in DEPRECATION_KEYS
                if key not in node
            ]
            since = node.get("x-deprecated-since")
            if "x-deprecated-since" in node and not (
                isinstance(since, str) and _SEMVER.match(since)
            ):
                out.append(f"{where}: x-deprecated-since {since!r} is not a SemVer version")
            dates: dict[str, datetime.date] = {}
            for key in ("x-deprecated-at", "x-sunset"):
                if key not in node:
                    continue
                day = _date(node[key])
                if day is None:
                    out.append(f"{where}: {key} {node[key]!r} is not a date YYYY-MM-DD")
                else:
                    dates[key] = day
            if len(dates) == 2 and min_notice is not None:
                at, sunset = dates["x-deprecated-at"], dates["x-sunset"]
                notice, written = min_notice
                if datetime.timedelta(days=(sunset - at).days) < notice:
                    out.append(
                        f"{where}: x-sunset {sunset} is less than {MIN_NOTICE_KEY} ({written}) "
                        f"after x-deprecated-at {at}"
                    )
    return out


_PROBLEM_TEXTS = ("title", "description", "client_action")
_PROBLEM_KEYS = frozenset({"code", "status", *_PROBLEM_TEXTS, "router"})
_PROBLEM_CODE = re.compile(r"^[a-z][a-z0-9_]*$")
# Umlauts and sharp s (built from code points, so the language gate sees no German letter here).
_GERMAN_LETTERS = re.compile(
    "[" + "".join(map(chr, (0xC4, 0xD6, 0xDC, 0xE4, 0xF6, 0xFC, 0xDF))) + "]"
)
_TICKED = re.compile(r"`([^`]*)`")
_RESPONSE_REF = "#/components/responses/"


def _http_status(value: object) -> int | None:
    """A three-digit HTTP status as an integer, else None."""
    if type(value) is int and 100 <= value <= 599:
        return value
    return None


def _problem_entries(openapi: Mapping[str, Any], out: list[str]) -> dict[str, Mapping[str, Any]]:
    """`x-problem-codes` by code; problems of the list go to `out`."""
    problem = _mapping_or_none(openapi["components"]["schemas"].get("Problem"))
    entries = None if problem is None else problem.get("x-problem-codes")
    if not isinstance(entries, list) or not entries:
        out.append("Problem: x-problem-codes is missing or empty")
        return {}
    by_code: dict[str, Mapping[str, Any]] = {}
    highest: int | None = None
    for index, entry in enumerate(entries):
        code = entry.get("code") if isinstance(entry, Mapping) else None
        if not isinstance(entry, Mapping):
            out.append(f"x-problem-codes[{index}]: not a mapping")
            continue
        if not isinstance(code, str) or not _PROBLEM_CODE.match(code):
            out.append(f"x-problem-codes[{index}]: code {code!r} is not a snake_case name")
            continue
        if code in by_code:
            out.append(f"{code}: listed twice in x-problem-codes")
            continue
        by_code[code] = entry
        out += [f"{code}: unknown key {key!r}" for key in entry if key not in _PROBLEM_KEYS]
        status = _http_status(entry.get("status"))
        if status is None:
            out.append(f"{code}: status {entry.get('status')!r} is not a three-digit HTTP status")
        elif highest is not None and status < highest:
            out.append(
                f"{code}: status {status} comes after status {highest}; x-problem-codes is "
                "sorted by status"
            )
        else:
            highest = status
        for field in _PROBLEM_TEXTS:
            text = entry.get(field)
            if not isinstance(text, str) or not text.strip():
                out.append(f"{code}: {field} is empty")
            elif _GERMAN_LETTERS.search(text):
                out.append(f"{code}: {field} is not English (R-API-15): {text!r}")
        if "router" in entry and entry["router"] is not True:
            out.append(f"{code}: router must be true when present")
    return by_code


def _problem_findings(openapi: Mapping[str, Any]) -> list[str]:
    """Sub-check `problem-codes`: `x-problem-codes` and `x-status` complete and consistent."""
    out: list[str] = []
    by_code = _problem_entries(openapi, out)
    responses = openapi["components"]["responses"]
    statuses: dict[str, int | None] = {}
    named: set[str] = set()
    for raw_name, response in responses.items():
        name = str(raw_name)
        description = response.get("description")
        codes = _TICKED.findall(description) if isinstance(description, str) else []
        named.update(codes)
        status = statuses[name] = _http_status(response.get("x-status"))
        if "x-status" not in response:
            out.append(f"{name}: x-status is missing")
            continue
        if status is None:
            out.append(
                f"{name}: x-status {response['x-status']!r} is not a three-digit HTTP status"
            )
            continue
        for code in codes:
            entry = by_code.get(code)
            if entry is None:
                out.append(f"{name}: description names {code!r}, which is not in x-problem-codes")
            elif _http_status(entry.get("status")) != status:
                out.append(
                    f"{name}: description names {code!r} with status {entry.get('status')}, but "
                    f"the response has x-status {status}"
                )
    out += [
        f"{code}: no response of components.responses names it; name it in a description or "
        "mark it router: true"
        for code, entry in by_code.items()
        if code not in named and entry.get("router") is not True
    ]
    for operation in [*_operations(openapi), *_operations(openapi, "webhooks")]:
        for raw_status, response in operation.op.get("responses", {}).items():
            ref = response.get("$ref")
            if not isinstance(ref, str) or not ref.startswith(_RESPONSE_REF):
                continue
            name = _unescape(ref.removeprefix(_RESPONSE_REF))
            if name not in responses:
                out.append(
                    f"{operation.label}: response {raw_status} refers to {ref!r}, which is not in "
                    "components.responses"
                )
                continue
            status = statuses[name]
            key = str(raw_status)
            if (
                status is not None
                and key != str(status)
                and not (key == "default" and status >= 500)
            ):
                out.append(
                    f"{operation.label}: response {key} refers to {name} with x-status {status}"
                )
    return out


def check_catalogue_sync(root: Path) -> list[str]:
    """Rule `api-catalogue-sync` (06 G2): the contract copies equal their sources.

    Args:
        root: Repository root with docs/spec/actions.yaml, docs/spec/04-sync-protocol.md,
            docs/api/openapi.yaml, docs/api/asyncapi.yaml and docs/operations/configuration.md.

    Returns:
        One finding per line, `api-catalogue-sync: <sub-check>: <subject>: <detail>`; empty
        when every copy agrees. An unreadable or malformed input yields only `source` findings.

    Example:
        >>> check_catalogue_sync(REPO)
        []
    """
    inputs = _Inputs(root, RULE_SYNC)
    contracts = _load_contracts(inputs)
    sync_text = inputs.text(SYNC_PATH)
    config_text = inputs.text(CONFIG_PATH)
    if contracts is None or sync_text is None or config_text is None:
        return inputs.findings
    catalog, openapi, asyncapi = contracts.catalog, contracts.openapi, contracts.asyncapi
    # Each sub-check runs on its own (operations are read inside it, they are cheap), so an
    # error in one cannot hide the findings of another.
    checks: tuple[tuple[str, Callable[[], list[str]]], ...] = (
        ("rest-action", lambda: _rest_action_findings(catalog, openapi)),
        ("webhook-events", lambda: _webhook_findings(catalog, openapi)),
        ("ws-ops", lambda: _ws_op_findings(catalog, _async_operations(asyncapi))),
        ("channel-ops", lambda: _channel_op_findings(catalog, _async_operations(asyncapi))),
        (
            "events",
            lambda: _event_findings(catalog, openapi, asyncapi, _async_operations(asyncapi)),
        ),
        ("app-scopes", lambda: _app_scope_findings(catalog, openapi)),
        ("close-codes", lambda: _close_code_findings(catalog, sync_text, asyncapi)),
        ("idempotency", lambda: _idempotency_findings(openapi)),
        (
            "deprecation",
            lambda: _deprecation_findings(
                ((OPENAPI_PATH, openapi), (ASYNCAPI_PATH, asyncapi)), config_text
            ),
        ),
        ("problem-codes", lambda: _problem_findings(openapi)),
    )
    return [
        f"{RULE_SYNC}: {name}: {detail}"
        for name, run in checks
        for detail in _guarded(run, "input not checkable")
    ]


# ---- api-tables ----------------------------------------------------------------------------

API_PREFIX = "/api/v1"
"""Path prefix of every surface (06 section 1); the tables show full paths."""

BLOCKS: Mapping[Path, tuple[str, ...]] = MappingProxyType(
    {
        CHAPTER_PATH: ("problems", "idempotency_exempt", "endpoints", "ws_ops", "events"),
        WEBHOOKS_PATH: ("webhook_events",),
    }
)
"""The generated blocks this tool owns, per file; any other block name there is a finding."""

PROBLEMS_COLUMNS = (
    "Code",
    "Status",
    "Meaning (from openapi.yaml)",
    "Client reaction (from openapi.yaml)",
)
IDEMPOTENCY_COLUMNS = ("operationId", "Method", "Path", "Reason")
ENDPOINTS_COLUMNS = ("Method", "Path", "operationId", "Action", "Purpose (from openapi.yaml)")
WS_OPS_COLUMNS = ("Op", "Direction", "Action", "Body schema", "Reply")
EVENTS_COLUMNS = ("Event type", "Kind", "Receive action", "Audience", "Schema", "Webhook")
WEBHOOK_EVENTS_COLUMNS = (
    "Event type",
    "Receive action",
    "Receiver and scope in addition to `events:subscribe`",
    "Schema",
)
DISPATCH_ROW = (
    "| `dispatch` | Server to client | Receive action of the event type | `<Type>Event` | |"
)
"""Fixed row of the operation with many messages in block `ws_ops`."""

DIRECTION_TEXT: Mapping[str, str] = MappingProxyType(
    {"receive": "Client to server", "send": "Server to client"}
)
KIND_TEXT: Mapping[str, str] = MappingProxyType({"durable": "durable", "ephemeral": "ephemeral"})
SCOPE_AGENT = "agent subscription only (agent is the receiver)"
SCOPE_SELF = "own only (app or agent is the receiver)"
SCOPE_NONE = "none (metadata, 05 section 2.6)"


def _row(cells: Sequence[str]) -> str:
    return "| " + " | ".join(cells) + " |\n"


def _table(header: Sequence[str], rows: Sequence[Sequence[str]]) -> str:
    return _row(header) + "|" + "---|" * len(header) + "\n" + "".join(_row(r) for r in rows)


class _Cells:
    """Turns values into the cells of one block; a value that cannot be a cell is a finding.

    Args:
        block: Name of the block, the sub-check of every finding.
        findings: Shared list the findings of all blocks go to.
    """

    def __init__(self, block: str, findings: list[str]) -> None:
        self.block = block
        self.findings = findings

    def fail(self, subject: str, detail: str) -> None:
        """Record a finding of this block about `subject`."""
        self.findings.append(f"{RULE_TABLES}: {self.block}: {subject}: {detail}")

    def text(self, value: object, subject: str, field: str) -> str:
        """A one-line, non-empty string without `|`; a pipe would split the table cell.

        One line as `str.splitlines` counts lines, as in the cell rule of tools/catalog: the
        marker parser splits the file the same way, so a U+2028 in a cell could otherwise turn
        the rest of the row into a marker line.
        """
        if not isinstance(value, str) or not value.strip():
            self.fail(subject, f"{field} is missing or not a string")
            return ""
        if "|" in value or value.splitlines() != [value]:
            self.fail(subject, f"{field} {value!r} contains a pipe or a line break")
            return ""
        return value

    def ticked(self, value: object, subject: str, field: str) -> str:
        """The value as a code span; a backtick inside would end the span early."""
        text = self.text(value, subject, field)
        if "`" in text:
            self.fail(subject, f"{field} {text!r} contains a backtick")
            return ""
        return f"`{text}`" if text else ""

    def schema(self, node: object, subject: str, missing: str) -> str:
        """Last segment of the `$ref` of `node` as a code span; `missing` is the finding text."""
        ref = node.get("$ref") if isinstance(node, Mapping) else None
        if not isinstance(ref, str) or "/" not in ref:
            self.fail(subject, missing)
            return ""
        return self.ticked(ref.rsplit("/", 1)[1], subject, "schema name")


def _problems_block(findings: list[str], openapi: Mapping[str, Any]) -> str:
    cells = _Cells("problems", findings)
    entries = _get(openapi, "components", "schemas", "Problem", "x-problem-codes")
    if not isinstance(entries, list):
        cells.fail("Problem", "x-problem-codes is missing")
        return ""
    rows: list[list[str]] = []
    for index, entry in enumerate(entries):
        subject = str(_get(entry, "code") or f"x-problem-codes[{index}]")
        status = _get(entry, "status")
        if _http_status(status) is None:
            cells.fail(subject, f"status {status!r} is not a three-digit HTTP status")
        rows.append(
            [
                cells.ticked(_get(entry, "code"), subject, "code"),
                str(status),
                cells.text(_get(entry, "description"), subject, "description"),
                cells.text(_get(entry, "client_action"), subject, "client_action"),
            ]
        )
    return _table(PROBLEMS_COLUMNS, rows)


def _idempotency_block(findings: list[str], openapi: Mapping[str, Any]) -> str:
    cells = _Cells("idempotency_exempt", findings)
    rows: dict[str, list[list[str]]] = {reason: [] for reason in EXEMPTION_REASONS}
    for operation in _operations(openapi):
        if "x-idempotency-exempt" not in operation.op:
            continue
        label, reason = operation.label, operation.op["x-idempotency-exempt"]
        if not isinstance(reason, str) or reason not in EXEMPTION_REASONS:
            cells.fail(label, f"x-idempotency-exempt {reason!r} is not a known reason")
            continue
        rows[reason].append(
            [
                cells.ticked(operation.op.get("operationId"), label, "operationId"),
                operation.method.upper(),
                cells.ticked(API_PREFIX + operation.key, label, "path"),
                EXEMPTION_REASONS[reason],
            ]
        )
    return _table(IDEMPOTENCY_COLUMNS, [row for group in rows.values() for row in group])


def _endpoints_block(findings: list[str], openapi: Mapping[str, Any]) -> str:
    cells = _Cells("endpoints", findings)
    rows: dict[str, list[list[str]]] = {}
    if "tags" not in openapi:
        cells.fail("tags", "missing; the table groups the operations by tag")
        return ""
    for index, tag in enumerate(openapi["tags"]):
        name = cells.text(tag.get("name"), f"tags[{index}]", "name")
        if name in rows:
            cells.fail(f"tags[{index}]", f"tag {name!r} is declared twice")
        elif name:
            rows[name] = []
    for operation in _operations(openapi):
        label, op = operation.label, operation.op
        tags = op.get("tags")
        if not isinstance(tags, list) or not tags:
            cells.fail(label, "has no tag")
            continue
        row = [
            operation.method.upper(),
            cells.ticked(API_PREFIX + operation.key, label, "path"),
            cells.ticked(op.get("operationId"), label, "operationId"),
            cells.ticked(op.get("x-authz-action"), label, "x-authz-action"),
            cells.text(op.get("summary"), label, "summary"),
        ]
        for tag in tags:
            if isinstance(tag, str) and tag in rows:
                rows[tag].append(row)
            else:
                cells.fail(label, f"tag {tag!r} is not declared in tags")
    return "\n".join(
        f"#### {tag}\n\n" + _table(ENDPOINTS_COLUMNS, group) for tag, group in rows.items() if group
    )


def _ws_ops_block(
    findings: list[str], asyncapi: Mapping[str, Any], operations: Sequence[_AsyncOperation]
) -> str:
    cells = _Cells("ws_ops", findings)
    lines: list[str] = []
    for operation in operations:
        key = operation.key
        if operation.channel is None:
            # Its channel is unknown, so the row would silently drop out of the table.
            cells.fail(key, f"channel reference {operation.channel_ref!r} does not resolve")
            continue
        if operation.channel != GATEWAY:
            continue
        if operation.problems or not operation.count:
            cells.fail(key, "; ".join(operation.problems) or "has no message")
            continue
        if operation.count > 1:
            lines.append(DISPATCH_ROW + "\n")
            continue
        name = operation.names[0]
        direction = DIRECTION_TEXT.get(str(operation.action), "")
        if not direction:
            cells.fail(key, f"action {operation.action!r} is neither receive nor send")
        message = _resolve(asyncapi, operation.op["messages"][0])
        reply_items = _get(operation.op, "reply", "messages")
        replies = [
            cells.ticked(_get(_resolve(asyncapi, item), "name"), key, "reply message name")
            for item in (reply_items if isinstance(reply_items, list) else [])
        ]
        lines.append(
            _row(
                [
                    cells.ticked(name, key, "message name"),
                    direction,
                    cells.ticked(operation.op.get("x-authz-action"), key, "x-authz-action"),
                    cells.schema(
                        _get(message, "payload", "properties", "d"),
                        key,
                        f"message {name!r} has no payload.properties.d.$ref",
                    ),
                    ", ".join(replies),
                ]
            )
        )
    return _table(WS_OPS_COLUMNS, []) + "".join(lines)


def _events_block(
    findings: list[str],
    openapi: Mapping[str, Any],
    asyncapi: Mapping[str, Any],
    operations: Sequence[_AsyncOperation],
) -> str:
    cells = _Cells("events", findings)
    dispatch = _dispatch(operations)
    if dispatch is None:
        cells.fail(DISPATCH, f"no operation on channel {GATEWAY} lists the event messages")
        return ""
    rows: list[list[str]] = []
    for item in dispatch.op["messages"]:
        message = _resolve(asyncapi, item)
        subject = str(_get(message, "name") or _get(item, "$ref"))
        kind = KIND_TEXT.get(str(_get(message, "x-event-kind")), "")
        if not kind:
            cells.fail(subject, f"x-event-kind {_get(message, 'x-event-kind')!r} is unknown")
        audience = _get(message, "x-audience")
        rows.append(
            [
                cells.ticked(_get(message, "name"), subject, "name"),
                kind,
                cells.ticked(_get(message, "x-receive-action"), subject, "x-receive-action"),
                "" if audience is None else cells.text(audience, subject, "x-audience"),
                cells.schema(
                    _get(message, "payload", "properties", "d"),
                    subject,
                    "has no payload.properties.d.$ref",
                ),
                "yes" if subject in openapi["webhooks"] else "",
            ]
        )
    return _table(EVENTS_COLUMNS, rows)


def _scope_cell(cells: _Cells, catalog: Catalog, event_type: str) -> str:
    """Cell "Receiver and scope" of webhooks.md section 7 from the catalogue."""
    event = catalog.events_by_type.get(event_type)
    if event is None:
        cells.fail(event_type, f"not an event type of {_CATALOG}")
        return ""
    if event.audience == action_catalog.SELF:
        return SCOPE_AGENT if event.receivers == "agent" else SCOPE_SELF
    action = catalog.actions_by_id.get(event.receive_action)
    if action is None or action.app_scope is None:
        cells.fail(
            event_type, f"receive action {event.receive_action!r} has no app_scope in {_CATALOG}"
        )
        return ""
    return SCOPE_NONE if action.app_scope == "none" else f"`{action.app_scope}`"


def _webhook_events_block(findings: list[str], catalog: Catalog, openapi: Mapping[str, Any]) -> str:
    cells = _Cells("webhook_events", findings)
    rows: list[list[str]] = []
    for raw_type, item in openapi["webhooks"].items():
        event_type = str(raw_type)
        operations = [item[method] for method in HTTP_METHODS if method in item]
        if len(operations) != 1:
            cells.fail(event_type, f"has {len(operations)} operations, expected 1")
            continue
        op = operations[0]
        rows.append(
            [
                cells.ticked(event_type, event_type, "event type"),
                cells.ticked(op.get("x-receive-action"), event_type, "x-receive-action"),
                _scope_cell(cells, catalog, event_type),
                cells.schema(
                    _get(op, "requestBody", "content", "application/json", "schema"),
                    event_type,
                    "has no requestBody.content.application/json.schema.$ref",
                ),
            ]
        )
    return _table(WEBHOOK_EVENTS_COLUMNS, rows)


def render_blocks(root: Path) -> dict[Path, dict[str, str]]:
    """Render every generated block of chapter 06 and docs/api/webhooks.md (06 G3).

    Args:
        root: Repository root with docs/api/openapi.yaml, docs/api/asyncapi.yaml and
            docs/spec/actions.yaml (the scope cell of `webhook_events`).

    Returns:
        Repository-relative file path to block name to block body (lines ending with a
        newline): `problems`, `idempotency_exempt`, `endpoints`, `ws_ops` and `events` for
        docs/spec/06-api.md, `webhook_events` for docs/api/webhooks.md.

    Raises:
        ApiCheckError: An input cannot be read or has the wrong shape (`source` findings), or a
            value cannot become a table cell: missing, not a string, with a pipe, a line break
            or (in a code span) a backtick, an undeclared or missing tag, an unknown reason or
            event kind, a missing `$ref` (findings named after the block).
    """
    inputs = _Inputs(root, RULE_TABLES)
    contracts = _load_contracts(inputs)
    if contracts is None:
        raise ApiCheckError(inputs.findings)
    catalog, openapi, asyncapi = contracts.catalog, contracts.openapi, contracts.asyncapi
    findings: list[str] = []

    def block(name: str, run: Callable[[], str]) -> str:
        """Render one block; an unexpected error becomes its finding (see `_guarded`)."""
        rendered: list[str] = []

        def render() -> list[str]:
            """Keep the rendered body; findings of the block go to `findings` directly."""
            rendered.append(run())
            return []

        for problem in _guarded(render, "input not renderable"):
            findings.append(f"{RULE_TABLES}: {name}: {problem}")
        return rendered[0] if rendered else ""

    chapter = {
        "problems": block("problems", lambda: _problems_block(findings, openapi)),
        "idempotency_exempt": block(
            "idempotency_exempt", lambda: _idempotency_block(findings, openapi)
        ),
        "endpoints": block("endpoints", lambda: _endpoints_block(findings, openapi)),
        "ws_ops": block(
            "ws_ops", lambda: _ws_ops_block(findings, asyncapi, _async_operations(asyncapi))
        ),
        "events": block(
            "events",
            lambda: _events_block(findings, openapi, asyncapi, _async_operations(asyncapi)),
        ),
    }
    webhooks = {
        "webhook_events": block(
            "webhook_events", lambda: _webhook_events_block(findings, catalog, openapi)
        )
    }
    if findings:
        raise ApiCheckError(findings)
    return {CHAPTER_PATH: chapter, WEBHOOKS_PATH: webhooks}


def _block_findings(
    inputs: _Inputs, rendered: Mapping[Path, Mapping[str, str]], *, stale: bool
) -> tuple[dict[Path, str], list[str]]:
    """Read the files of `rendered`; findings for markers and, with `stale`, for old bodies."""
    texts: dict[Path, str] = {}
    out: list[str] = []
    for rel, blocks in rendered.items():
        text = inputs.text(rel)
        if text is None:
            continue
        texts[rel] = text
        prefix = f"{RULE_TABLES}: markers: {rel.as_posix()}:"
        try:
            current = action_catalog.generated_blocks(text)
        except CatalogError as err:
            out += [f"{prefix} {finding}" for finding in err.findings]
            continue
        out += [
            f"{prefix} unknown block {name!r}; this tool owns {', '.join(BLOCKS[rel])}"
            for name in current
            if name not in blocks
        ]
        out += [f"{prefix} missing block {name!r}" for name in blocks if name not in current]
        if stale:
            out += [
                f"{RULE_TABLES}: {name}: {rel.as_posix()}: generated block differs from its "
                "source; run make api-tables"
                for name, body in blocks.items()
                if name in current and current[name] != body
            ]
    return texts, [*inputs.findings, *out]


def check_tables(root: Path) -> list[str]:
    """Rule `api-tables` (06 G3): every generated block equals a fresh rendering.

    Args:
        root: Repository root (see `render_blocks`; also docs/spec/06-api.md and
            docs/api/webhooks.md).

    Returns:
        One finding per line, `api-tables: <block or markers or source>: ...`; empty when
        every block is current and no file holds a block this tool does not own.
    """
    try:
        rendered = render_blocks(root)
    except ApiCheckError as err:
        return err.findings
    return _block_findings(_Inputs(root, RULE_TABLES), rendered, stale=True)[1]


def generate(root: Path) -> list[Path]:
    """Rewrite the generated blocks of chapter 06 and docs/api/webhooks.md (`make api-tables`).

    Nothing is written unless every input is valid, both files have exactly the blocks of
    `BLOCKS` with intact markers and every file to change opens for writing; a stale block body
    is the only thing this repairs.

    Args:
        root: Repository root (see `check_tables`).

    Returns:
        The repository-relative paths of the files it rewrote.

    Raises:
        ApiCheckError: See `render_blocks`; also a file that cannot be read, broken markers,
            an unknown or a missing block, and a file that cannot be written (`written` names
            the files rewritten before that).
    """
    rendered = render_blocks(root)
    texts, blocking = _block_findings(_Inputs(root, RULE_TABLES), rendered, stale=False)
    if blocking:
        raise ApiCheckError(blocking)
    contents: dict[Path, bytes] = {}
    for rel, blocks in rendered.items():
        new = texts[rel]
        for name, body in blocks.items():
            new = action_catalog.replace_generated_block(new, name, body)
        if new != texts[rel]:
            contents[rel] = new.encode("utf-8")
    _write_all(root, contents)
    return list(contents)


def _write_all(root: Path, contents: Mapping[Path, bytes]) -> None:
    """Write every file, or none of them.

    Each file must open for writing, so a read-only file stops the run. The new contents go to
    temporary files next to their targets first; only when all of them are complete does one
    rename per file replace the targets. A failure before that (a full disk, a file-size limit)
    leaves every target as it was and no temporary file behind. Only a failed rename can leave
    the files before it replaced; the error names them.

    Raises:
        ApiCheckError: A file cannot be written (`source` finding); `written` names the files
            replaced before the error.
    """

    def cannot_write(rel: Path, error: BaseException) -> str:
        return f"{RULE_TABLES}: {SOURCE}: {rel.as_posix()}: cannot write: {_one_line(error)}"

    for rel in contents:
        try:
            with (root / rel).open("r+b"):  # changes nothing; fails on a read-only file
                pass
        except OSError as exc:
            raise ApiCheckError([cannot_write(rel, exc)]) from exc
    staged: dict[Path, Path] = {}
    written: list[Path] = []
    try:
        for rel, data in contents.items():
            target = root / rel
            handle, name = tempfile.mkstemp(prefix=f".{target.name}.", dir=target.parent)
            staged[rel] = Path(name)
            with os.fdopen(handle, "wb") as stream:
                stream.write(data)
            shutil.copymode(target, name)
        for rel, temporary in staged.items():
            temporary.replace(root / rel)
            written.append(rel)
    except OSError as exc:
        raise ApiCheckError([cannot_write(rel, exc)], written) from exc
    finally:
        for temporary in staged.values():
            temporary.unlink(missing_ok=True)


NOTHING_WRITTEN = "api_checks: nothing written"


def main(argv: Sequence[str] | None = None) -> int:
    """Command line: `check [--rule <rule>]` or `generate`, both with `--root <repository>`.

    Args:
        argv: Arguments without the program name; `None` reads `sys.argv`.

    Returns:
        Exit code: 0 clean or written, 1 with findings (printed to stderr, one per line). This
        is the last place every error ends in: an error that no guarded part reports becomes
        one finding `<rule>: source: input not checkable (...)` (`input not renderable` for
        `generate`), never a traceback. `generate` changes files only in `_write_all`, which
        reports its own errors, so after any other error nothing was written.
    """
    parser = argparse.ArgumentParser(
        prog="api_checks.py",
        description="Check the API contract copies (06 G2, G3) or regenerate the 06 tables.",
    )
    parser.add_argument("command", choices=("check", "generate"))
    parser.add_argument("--root", type=Path, default=REPO, help="repository root")
    parser.add_argument("--rule", choices=RULES, help="check one rule only (default: all)")
    args = parser.parse_args(argv)
    if args.command == "generate":
        if args.rule is not None:
            parser.error("--rule applies to check only")
        try:
            written = generate(args.root)
        except ApiCheckError as err:
            for finding in err.findings:
                print(finding, file=sys.stderr)
            done = ", ".join(rel.as_posix() for rel in err.written)
            print(
                f"api_checks: rewrote only {done} before the error; run make api-tables again"
                if done
                else NOTHING_WRITTEN,
                file=sys.stderr,
            )
            return 1
        except Exception as exc:  # the last place every error ends in, see the docstring
            print(
                f"{RULE_TABLES}: {SOURCE}: {_failure('input not renderable', exc)}", file=sys.stderr
            )
            print(NOTHING_WRITTEN, file=sys.stderr)
            return 1
        for rel in written:
            print(f"  rewrote {rel.as_posix()}")
        return 0
    findings: list[str] = []
    checks = (
        (RULE_SYNC, check_catalogue_sync),
        (RULE_TABLES, check_tables),
        (RULE_VIZ, viz_fixtures.check),
        (RULE_GDPR, gdpr_export.check),
        (RULE_CLIENTS, client_routes.check),
    )
    for rule, run in checks:
        if args.rule not in (None, rule):
            continue
        try:
            findings += run(args.root)
        except Exception as exc:  # the last place every error ends in, see the docstring
            findings.append(f"{rule}: {SOURCE}: {_failure('input not checkable', exc)}")
    for finding in findings:
        print(finding, file=sys.stderr)
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
