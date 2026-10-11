# SPDX-License-Identifier: Apache-2.0
"""Test helper: read the pre-migration tables of chapter 05 into the actions.yaml structure.

M1-02 used it once to write docs/spec/actions.yaml; since then it only serves the migration
proof (`test_render_reproduces_pre_migration_tables`), which renders the imported structure and
compares it with the frozen copy of chapter 05 in fixtures/. Everything that the tables do not
say in a machine form sits in the constant tables below, so the proof depends on nothing but
the fixture.
"""

from __future__ import annotations

import re
from typing import Any

import action_catalog

SECTION_TITLES = {
    "Public and login": "public",
    "Instance and administration": "instance",
    "People, identity, devices, tokens": "identity",
    "Groups and grants": "grants",
    "Spaces, channels, sidebar": "structure",
    "Threads and agent sessions": "threads",
    "Messages, reactions, pins, templates": "messages",
    "Files and search": "files",
    "Personal data, notifications, sync": "personal",
    "Agents, commands, runs, interactions": "agents",
    "Apps and webhooks": "apps",
    "Import, costs, privacy, status": "operations",
    "Dev environments (milestone)": "dev",
    "Hosts and terminal": "terminal",
}
# Receive cells that name more than one case: (machine receive action, receivers).
COMPOSITE_RECEIVE = {
    "agent.invocation": ("agent.events.consume", "agent"),
    "agent.followup": ("agent.events.consume", "agent"),
    "run.cancel_requested": ("agent.events.consume", "agent"),
    "interaction.created": ("agent.events.consume", "agent_or_app"),
    "command.invoked": ("agent.events.consume", "agent_or_app"),
    "command.autocomplete_requested": ("agent.events.consume", "agent_or_app"),
    "agent.config_changed": ("agent.config.read", "agent"),
    "notification.created": ("push.deliver", "principal"),
}
# Types whose envelope carries resource.version: the table of the resource has a `version`
# column (03) and the event reports its new value (04 "Envelope", "Delivery guarantee"
# point 3). `run.updated` is the only ephemeral one: it reports the new `runs.version` (04
# "Live box streaming").
RESOURCE_VERSION = frozenset(
    {
        "message.created",
        "message.updated",
        "message.deleted",
        "thread.created",
        "thread.updated",
        "thread.closed",
        "thread.reopened",
        "thread.moved",
        "run.state_changed",
        "space.created",
        "space.updated",
        "space.archived",
        "channel.created",
        "channel.updated",
        "channel.archived",
        "command.changed",
        "agent.updated",
        "draft.updated",
        "host.updated",
        "terminal_session.changed",
        "run.updated",
    }
)
# Gateway rows that list several ops share one row; the group names the row.
ROW_GROUPS = {"hello": "session", "stats.subscribe": "stats"}
# `dispatch` names no action in its cell; delivery checks the receive action per event type.
DISPATCH = "dispatch"
DISPATCH_ACTION = "gateway.connect"
CHANNEL_BY_PATH = {
    "`/terminal/sessions/{session_id}/stream`": "terminal_stream",
    "`/hosts/connect`": "host_control",
    "`/hosts/sessions/{session_id}/stream`": "host_data",
}
_OWNER = {"yes": True, "no": False, "as target action": "as_target"}
_STEP_UP = {"yes": True, "no": False, "conditional": "conditional", "fresh": "fresh"}
_AUDIT = {"yes": "always", "denials": "denials", "no": "never"}
_YES_NO = {"yes": True, "no": False}
_DIRECTION = {v: k for k, v in action_catalog.DIRECTION_TEXT.items()}
_CHANNEL_DIRECTION = {v: k for k, v in action_catalog.CHANNEL_DIRECTION_TEXT.items()}
_TICKED = re.compile(r"`([^`]+)`")


def _sections(text: str, level: int) -> list[tuple[str, str]]:
    """(title without number, body) of every heading of `level` in `text`."""
    pattern = re.compile(rf"^{'#' * level} [0-9]+(?:\.[0-9]+)*\.? (?P<title>.+)$", re.MULTILINE)
    found = list(pattern.finditer(text))
    out = []
    for n, m in enumerate(found):
        end = found[n + 1].start() if n + 1 < len(found) else len(text)
        body = text[m.end() : end]
        stop = re.search(rf"^#{{1,{level}}} ", body, re.MULTILINE)
        out.append((m.group("title").strip(), body[: stop.start()] if stop else body))
    return out


def _section(text: str, title: str, level: int) -> str:
    for found, body in _sections(text, level):
        if found == title:
            return body
    raise KeyError(title)


def _tables(body: str) -> list[str]:
    """Every Markdown table of `body` as text, each line with its newline."""
    tables: list[str] = []
    current: list[str] = []
    for line in body.splitlines(keepends=True):
        if line.startswith("|"):
            current.append(line)
        elif current:
            tables.append("".join(current))
            current = []
    if current:
        tables.append("".join(current))
    return tables


def chapter_tables(chapter: str) -> dict[str, str]:
    """Block name to table text (header, rule and rows) of the pre-migration chapter."""
    tables = {"rate_limit_classes": _tables(_section(chapter, "Rate-limit classes", 2))[0]}
    actions = _section(chapter, "Actions", 2)
    for title, body in _sections(actions, 3):
        tables[f"actions.{SECTION_TITLES[title]}"] = _tables(body)[0]
    durable, ephemeral = _tables(_section(chapter, "Durable event types and receive action", 2))[:2]
    tables["events.durable"] = durable
    tables["events.ephemeral"] = ephemeral
    ws_ops, channel_ops = _tables(_section(chapter, "WebSocket ops and action", 2))[:2]
    tables["ws_ops"] = ws_ops
    tables["channel_ops"] = channel_ops
    return tables


def _rows(table: str) -> list[list[str]]:
    lines = table.splitlines()[2:]
    return [[c.strip() for c in line.strip()[1:-1].split("|")] for line in lines]


def _untick(cell: str) -> str:
    m = re.fullmatch(r"`([^`]+)`", cell)
    if m is None:
        raise ValueError(f"expected one backticked name, got {cell!r}")
    return m.group(1)


def _action(section: str, cells: list[str]) -> dict[str, Any]:
    ident, description, resource, right, owner, step_up, audit, rights, rl_class = cells
    entry: dict[str, Any] = {
        "id": _untick(ident),
        "section": section,
        "description": description,
        "resource": resource,
        "right": right,
        "owner": _OWNER[owner],
        "step_up": _STEP_UP[step_up],
        "audit": _AUDIT.get(audit, "conditional"),
    }
    if entry["audit"] == "conditional":
        entry["audit_text"] = audit
    entry["rights_change"] = _YES_NO[rights]
    entry["rl_class"] = _untick(rl_class)
    if entry["rl_class"] == "none" and right != "system":
        entry["delivery_check"] = True
    return entry


def _event(kind: str, cells: list[str]) -> dict[str, Any]:
    ident, resource, receive, note = cells
    event_type = _untick(ident)
    entry: dict[str, Any] = {"type": event_type, "kind": kind, "resource": resource}
    if receive == "self":
        entry |= {"receive_action": "self", "audience": "self"}
    elif receive.startswith("self"):
        action, receivers = COMPOSITE_RECEIVE[event_type]
        entry |= {"receive_action": action, "audience": "self"}
        if receivers != "principal":
            entry["receivers"] = receivers
        entry["receive_text"] = receive
    else:
        entry["receive_action"] = _untick(receive)
    if note:
        entry["note"] = note
    entry["resource_version"] = event_type in RESOURCE_VERSION
    return entry


def _ws_ops(table: str, action_ids: set[str]) -> list[dict[str, Any]]:
    ops: list[dict[str, Any]] = []
    for op_cell, direction, action_cell in _rows(table):
        names = [_untick(n) for n in op_cell.split(", ")]
        named = [t for t in _TICKED.findall(action_cell) if t in action_ids]
        action = DISPATCH_ACTION if names == [DISPATCH] else named[0]
        for name in names:
            entry: dict[str, Any] = {
                "op": name,
                "direction": _DIRECTION[direction],
                "action": action,
            }
            if named[1:]:
                entry["also_actions"] = named[1:]
            if action_cell != f"`{action}`":
                entry["action_text"] = action_cell
            if len(names) > 1:
                entry["row_group"] = ROW_GROUPS[names[0]]
            if name == DISPATCH:
                entry["per_event_receive_action"] = True
            ops.append(entry)
    return ops


def _channel_ops(table: str) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    channel = ""
    for channel_cell, op_cell, direction, action_cell in _rows(table):
        entry: dict[str, Any] = {}
        if channel_cell:
            channel = next(c for path, c in CHANNEL_BY_PATH.items() if path in channel_cell)
            entry = {"channel": channel, "channel_text": channel_cell}
        else:
            entry = {"channel": channel}
        entry["ops"] = [
            action_catalog.NOISE_FRAME if n == action_catalog.NOISE_FRAME_TEXT else _untick(n)
            for n in op_cell.split(", ")
        ]
        entry["direction"] = _CHANNEL_DIRECTION[direction]
        entry["action"] = _TICKED.findall(action_cell)[0]
        if action_cell != f"`{entry['action']}`":
            entry["action_text"] = action_cell
        rows.append(entry)
    return rows


def import_chapter(chapter: str) -> dict[str, Any]:
    """Parse the pre-migration chapter 05 into the structure of docs/spec/actions.yaml.

    Cell mapping: `yes`/`no` to booleans, Step-up `conditional`/`fresh` to `conditional`/`fresh`,
    `as target action` to `as_target`, Audit `yes`/`denials`/`no` to `always`/`denials`/`never`
    and any other text to `conditional` plus `audit_text`, the class `` `none` `` to `none`,
    `delivery_check: true` where the class is `none` and the right is not `system`;
    `app_scope` from 2.6 and `effect` from 2.7 of the same text.
    """
    tables = chapter_tables(chapter)
    classes = [
        {"id": _untick(ident), "key": key, "purpose": purpose}
        for ident, key, purpose in _rows(tables["rate_limit_classes"])
    ]
    sections = [SECTION_TITLES[title] for title, _ in _sections(_section(chapter, "Actions", 2), 3)]
    actions = [
        _action(section, cells)
        for section in sections
        for cells in _rows(tables[f"actions.{section}"])
    ]
    ids = [a["id"] for a in actions]
    read = action_catalog.read_effect_ids(chapter, ids)
    scopes = action_catalog.scope_assignments(chapter, ids)
    for entry in actions:
        entry["effect"] = "read" if entry["id"] in read else "write"
        if entry["id"] in scopes:
            entry["app_scope"] = scopes[entry["id"]]
    events = [_event("durable", c) for c in _rows(tables["events.durable"])]
    events += [_event("ephemeral", c) for c in _rows(tables["events.ephemeral"])]
    return {
        "version": 1,
        "rate_limit_classes": classes,
        "sections": sections,
        "actions": actions,
        "events": events,
        "ws_ops": _ws_ops(tables["ws_ops"], set(ids)),
        "channel_ops": _channel_ops(tables["channel_ops"]),
    }
