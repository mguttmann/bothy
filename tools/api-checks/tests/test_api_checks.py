# SPDX-License-Identifier: Apache-2.0
"""Tests for the API contract checks (06 G2 and G3, ticket M1-03).

Every negative test copies the real inputs into `tmp_path`, applies one mutation and asserts
the finding of the named sub-check; the repository itself must stay free of findings.
"""

from __future__ import annotations

import datetime
import os
import shutil
import subprocess
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any

import api_checks
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
CATALOG = Path("docs/spec/actions.yaml")
SYNC = Path("docs/spec/04-sync-protocol.md")
ACTIONS = Path("docs/spec/05-actions.md")
CHAPTER = Path("docs/spec/06-api.md")
OPENAPI = Path("docs/api/openapi.yaml")
ASYNCAPI = Path("docs/api/asyncapi.yaml")
WEBHOOKS = Path("docs/api/webhooks.md")
CONFIG = Path("docs/operations/configuration.md")
INPUTS = (CATALOG, SYNC, ACTIONS, CHAPTER, OPENAPI, ASYNCAPI, WEBHOOKS, CONFIG)
SYNC_RULE = "api-catalogue-sync"
_DUMPER = getattr(yaml, "CSafeDumper", yaml.SafeDumper)
_LOADER = getattr(yaml, "CSafeLoader", yaml.SafeLoader)
# A German word with an umlaut, built from its code point so that the language gate of this
# repository (tools/docs/language_check.py) does not flag the test file itself.
_UMLAUT_TITLE = "Gel" + chr(0xF6) + "scht"


def _copy_inputs(tmp_path: Path) -> Path:
    """Copy the inputs of both rules into tmp_path with the same relative paths."""
    for rel in INPUTS:
        target = tmp_path / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes((REPO / rel).read_bytes())
    return tmp_path


def _mutate(root: Path, rel: Path, change: Callable[[dict[str, Any]], object]) -> None:
    """Load a copied YAML file, apply `change` to the data, write it back."""
    path = root / rel
    data = yaml.load(path.read_text(encoding="utf-8"), Loader=_LOADER)  # noqa: S506 (safe loader)
    change(data)
    path.write_text(
        yaml.dump(data, Dumper=_DUMPER, sort_keys=False, allow_unicode=True, width=100),
        encoding="utf-8",
    )


def _edit(root: Path, rel: Path, old: str, new: str) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    assert old in text, old
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


def _sync(root: Path, sub_check: str) -> list[str]:
    """Findings of one sub-check of `api-catalogue-sync`."""
    prefix = f"{SYNC_RULE}: {sub_check}: "
    return [f for f in api_checks.check_catalogue_sync(root) if f.startswith(prefix)]


def _operation(data: dict[str, Any], path: str, method: str) -> dict[str, Any]:
    operation: dict[str, Any] = data["paths"][path][method]
    return operation


# ---- source files ------------------------------------------------------------------------


def test_invalid_yaml_is_a_source_finding(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, OPENAPI, "openapi: 3.1.0\n", "openapi: [3.1.0\n")
    findings = api_checks.check_catalogue_sync(root)
    assert findings, "a broken source must not pass"
    assert all(f.startswith(f"{SYNC_RULE}: source: docs/api/openapi.yaml: ") for f in findings)


def test_duplicate_key_is_a_source_finding(tmp_path: Path) -> None:
    # PyYAML keeps the last of two equal keys; the gate must not read another value than lint.
    root = _copy_inputs(tmp_path)
    _edit(
        root,
        OPENAPI,
        "      x-authz-action: self.read\n",
        "      x-authz-action: self.read\n      x-authz-action: user.read\n",
    )
    [finding] = api_checks.check_catalogue_sync(root)
    assert finding.startswith(f"{SYNC_RULE}: source: docs/api/openapi.yaml: line ")
    assert finding.endswith("duplicate key 'x-authz-action'")


def test_wrong_container_is_a_source_finding(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: d.update(operations=["receive.identify"]))
    assert api_checks.check_catalogue_sync(root) == [
        f"{SYNC_RULE}: source: docs/api/asyncapi.yaml: operations: not a mapping"
    ]


# ---- rest-action ---------------------------------------------------------------------------


def test_unknown_action_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root, OPENAPI, lambda d: _operation(d, "/me", "get").update({"x-authz-action": "self.peek"})
    )
    assert _sync(root, "rest-action") == [
        f"{SYNC_RULE}: rest-action: getMe: x-authz-action 'self.peek' is not an action of "
        "docs/spec/actions.yaml"
    ]


def test_system_action_on_route_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _operation(d, "/me", "get").update({"x-authz-action": "grant.expire"}),
    )
    assert _sync(root, "rest-action") == [
        f"{SYNC_RULE}: rest-action: getMe: x-authz-action 'grant.expire' is a system action "
        "(05 rule 10)"
    ]


def test_rate_limit_class_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _operation(d, "/me", "get").update({"x-rate-limit-class": "rl.write"}),
    )
    assert _sync(root, "rest-action") == [
        f"{SYNC_RULE}: rest-action: getMe: x-rate-limit-class 'rl.write' differs from 'rl.read', "
        "the class of 'self.read' in docs/spec/actions.yaml"
    ]


def test_object_action_fails(tmp_path: Path) -> None:
    # The Redocly enum skips an object value; the gate must not.
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _operation(d, "/me", "get").update({"x-authz-action": {"id": "self.read"}}),
    )
    assert _sync(root, "rest-action") == [
        f"{SYNC_RULE}: rest-action: getMe: x-authz-action must name exactly one action, "
        "not a mapping"
    ]


def test_operation_without_action_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, lambda d: _operation(d, "/me", "get").pop("x-authz-action"))
    assert _sync(root, "rest-action") == [
        f"{SYNC_RULE}: rest-action: getMe: x-authz-action is missing"
    ]


# ---- webhook-events ------------------------------------------------------------------------


def test_webhook_receive_action_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: d["webhooks"]["file.updated"]["post"].update(
            {"x-receive-action": "message.read"}
        ),
    )
    assert _sync(root, "webhook-events") == [
        f"{SYNC_RULE}: webhook-events: file.updated: x-receive-action 'message.read' differs from "
        "'file.read' in docs/spec/actions.yaml"
    ]


def test_webhook_for_unknown_event_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def rename(d: dict[str, Any]) -> None:
        d["webhooks"]["file.renamed"] = d["webhooks"].pop("file.updated")

    _mutate(root, OPENAPI, rename)
    assert _sync(root, "webhook-events") == [
        f"{SYNC_RULE}: webhook-events: file.renamed: not an event type of docs/spec/actions.yaml"
    ]


def test_webhook_delivery_action_and_class_are_fixed(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        d["webhooks"]["file.updated"]["post"].update(
            {"x-authz-action": "file.read", "x-rate-limit-class": "rl.read"}
        )

    _mutate(root, OPENAPI, change)
    assert _sync(root, "webhook-events") == [
        f"{SYNC_RULE}: webhook-events: file.updated: x-authz-action 'file.read' must be "
        "'webhook_out.deliver'",
        f"{SYNC_RULE}: webhook-events: file.updated: x-rate-limit-class 'rl.read' must be 'none'",
    ]


# ---- ws-ops --------------------------------------------------------------------------------


def _async_op(data: dict[str, Any], key: str) -> dict[str, Any]:
    operation: dict[str, Any] = data["operations"][key]
    return operation


def test_ws_op_action_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "receive.focus").update({"x-authz-action": "gateway.connect"}),
    )
    assert _sync(root, "ws-ops") == [
        f"{SYNC_RULE}: ws-ops: receive.focus: x-authz-action 'gateway.connect' differs from "
        "'gateway.focus', the action of op 'focus' in docs/spec/actions.yaml"
    ]


def test_ws_op_class_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "receive.focus").update({"x-rate-limit-class": "rl.write"}),
    )
    assert _sync(root, "ws-ops") == [
        f"{SYNC_RULE}: ws-ops: receive.focus: x-rate-limit-class 'rl.write' differs from "
        "'rl.read', the class of 'gateway.focus' in docs/spec/actions.yaml"
    ]


def test_ws_op_missing_in_asyncapi_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: d["operations"].pop("receive.reauth"))
    assert _sync(root, "ws-ops") == [
        f"{SYNC_RULE}: ws-ops: reauth: no operation on channel gateway describes this op of "
        "docs/spec/actions.yaml"
    ]


def test_ws_direction_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: _async_op(d, "receive.focus").update({"action": "send"}))
    assert _sync(root, "ws-ops") == [
        f"{SYNC_RULE}: ws-ops: receive.focus: action 'send' does not match direction "
        "'client_to_server' of op 'focus' in docs/spec/actions.yaml"
    ]


def test_ws_unknown_op_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: d["components"]["messages"]["op.focus"].update(name="blur"))
    assert _sync(root, "ws-ops") == [
        f"{SYNC_RULE}: ws-ops: receive.focus: op 'blur' is not a gateway op of "
        "docs/spec/actions.yaml",
        f"{SYNC_RULE}: ws-ops: focus: no operation on channel gateway describes this op of "
        "docs/spec/actions.yaml",
    ]


def test_ws_unresolved_message_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "receive.focus")["messages"][0].update(
            {"$ref": "#/channels/gateway/messages/op.blur"}
        ),
    )
    assert _sync(root, "ws-ops") == [
        f"{SYNC_RULE}: ws-ops: receive.focus: message reference "
        "'#/channels/gateway/messages/op.blur' does not resolve",
        f"{SYNC_RULE}: ws-ops: focus: no operation on channel gateway describes this op of "
        "docs/spec/actions.yaml",
    ]


# ---- channel-ops ---------------------------------------------------------------------------


def test_channel_op_action_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "hostData.send.hello").update({"x-authz-action": "terminal.stream"}),
    )
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: hostData.send.hello: x-authz-action 'terminal.stream' differs "
        "from 'host.connect', the action of op 'hello' on channel 'host_data' in "
        "docs/spec/actions.yaml"
    ]


def test_channel_op_class_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "terminal.send.frame").update({"x-rate-limit-class": "rl.host"}),
    )
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: terminal.send.frame: x-rate-limit-class 'rl.host' differs "
        "from 'rl.terminal', the class of 'terminal.stream' in docs/spec/actions.yaml"
    ]


def test_channel_op_missing_in_asyncapi_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: d["operations"].pop("terminal.receive.identify"))
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: terminal_stream/identify: no operation on channel "
        "terminalStream describes this op of docs/spec/actions.yaml"
    ]


def test_channel_op_direction_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root, ASYNCAPI, lambda d: _async_op(d, "terminal.receive.identify").update(action="send")
    )
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: terminal.receive.identify: action 'send' does not match "
        "direction 'device_to_server' of op 'identify' on channel 'terminal_stream' in "
        "docs/spec/actions.yaml",
        f"{SYNC_RULE}: channel-ops: terminal_stream/identify: no operation on channel "
        "terminalStream describes this op of docs/spec/actions.yaml",
    ]


def test_channel_op_unknown_op_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root, ASYNCAPI, lambda d: d["components"]["messages"]["host.report"].update(name="poll")
    )
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: hostControl.receive.report: message 'poll' frames op "
        "['report'], expected ['poll']",
        f"{SYNC_RULE}: channel-ops: hostControl.receive.report: op 'poll' is not an op of "
        "channel 'host_control' in docs/spec/actions.yaml",
        f"{SYNC_RULE}: channel-ops: host_control/report: no operation on channel hostControl "
        "describes this op of docs/spec/actions.yaml",
    ]


def test_operation_on_unknown_channel_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def add(d: dict[str, Any]) -> None:
        d["channels"]["auditStream"] = dict(d["channels"]["hostData"], address="/audit/stream")
        d["operations"]["audit.send.error"] = dict(
            _async_op(d, "hostData.send.error"), channel={"$ref": "#/channels/auditStream"}
        )

    _mutate(root, ASYNCAPI, add)
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: audit.send.error: channel 'auditStream' is neither the "
        "gateway nor a channel of docs/spec/actions.yaml"
    ]


def test_operation_with_unresolved_channel_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "hostData.send.error").update(channel={"$ref": "#/channels/none"}),
    )
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: hostData.send.error: channel reference '#/channels/none' "
        "does not resolve",
        f"{SYNC_RULE}: channel-ops: host_data/error: no operation on channel hostData describes "
        "this op of docs/spec/actions.yaml",
    ]


# ---- events --------------------------------------------------------------------------------


def _message(data: dict[str, Any], key: str) -> dict[str, Any]:
    message: dict[str, Any] = data["components"]["messages"][key]
    return message


def test_event_receive_action_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _message(d, "event.file.updated").update({"x-receive-action": "message.read"}),
    )
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: file.updated: x-receive-action 'message.read' differs from "
        "'file.read' in docs/spec/actions.yaml"
    ]


def test_event_audience_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        _message(d, "event.access.changed").pop("x-audience")
        _message(d, "event.file.updated")["x-audience"] = "self"
        _message(d, "event.agent.invocation")["x-audience"] = "agent"

    _mutate(root, ASYNCAPI, change)
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: file.updated: x-audience 'self', but docs/spec/actions.yaml has "
        "no audience",
        f"{SYNC_RULE}: events: access.changed: x-audience is missing; docs/spec/actions.yaml has "
        "audience 'self'",
        f"{SYNC_RULE}: events: agent.invocation: x-audience 'agent' differs from 'self' in "
        "docs/spec/actions.yaml",
    ]


def test_event_kind_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _message(d, "event.typing.started").update({"x-event-kind": "durable"}),
    )
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: typing.started: x-event-kind 'durable' differs from 'ephemeral' "
        "in docs/spec/actions.yaml"
    ]


def test_event_missing_from_dispatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def drop(d: dict[str, Any]) -> None:
        messages = _async_op(d, "send.dispatch")["messages"]
        messages.remove({"$ref": "#/channels/gateway/messages/event.stats.live"})

    _mutate(root, ASYNCAPI, drop)
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: send.dispatch: does not list the event message 'stats.live'"
    ]


def test_event_without_kind_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: _message(d, "event.stats.live").pop("x-event-kind"))
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: event.stats.live: x-receive-action or x-audience without "
        "x-event-kind",
        f"{SYNC_RULE}: events: stats.live: event type of docs/spec/actions.yaml has no message "
        "with x-event-kind in docs/api/asyncapi.yaml",
        f"{SYNC_RULE}: events: send.dispatch: lists 'stats.live', which is not a message with "
        "x-event-kind",
    ]


def test_unknown_event_message_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: _message(d, "event.stats.live").update(name="stats.dead"))
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: stats.dead: message with x-event-kind is not an event type of "
        "docs/spec/actions.yaml",
        f"{SYNC_RULE}: events: stats.live: event type of docs/spec/actions.yaml has no message "
        "with x-event-kind in docs/api/asyncapi.yaml",
    ]


# ---- close-codes ---------------------------------------------------------------------------

SYNC_TABLE = 'docs/spec/04-sync-protocol.md "Close codes"'
ASYNC_CODES = "docs/api/asyncapi.yaml channels.gateway.x-close-codes"


def _close_codes(data: dict[str, Any]) -> list[dict[str, Any]]:
    codes: list[dict[str, Any]] = data["channels"]["gateway"]["x-close-codes"]
    return codes


def test_close_code_name_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def rename(d: dict[str, Any]) -> None:
        next(c for c in _close_codes(d) if c["code"] == 4001)["name"] = "unknown_opcode"

    _mutate(root, ASYNCAPI, rename)
    assert _sync(root, "close-codes") == [
        f"{SYNC_RULE}: close-codes: 4001 unknown_op: in {SYNC_TABLE} but not in {ASYNC_CODES}",
        f"{SYNC_RULE}: close-codes: 4001 unknown_opcode: in {ASYNC_CODES} but not in {SYNC_TABLE}",
    ]


def test_close_code_missing_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: _close_codes(d).pop())
    assert _sync(root, "close-codes") == [
        f"{SYNC_RULE}: close-codes: 4018 forbidden: in {SYNC_TABLE} but not in {ASYNC_CODES}"
    ]


def test_close_code_order_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def swap(d: dict[str, Any]) -> None:
        codes = _close_codes(d)
        codes[0], codes[1] = codes[1], codes[0]

    _mutate(root, ASYNCAPI, swap)
    assert _sync(root, "close-codes") == [
        f"{SYNC_RULE}: close-codes: x-close-codes: the order differs from {SYNC_TABLE}"
    ]


def test_close_code_changed_in_chapter_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, SYNC, "| `4015` | consumer_replaced |", "| `4018` | consumer_replaced |")
    assert _sync(root, "close-codes") == [
        f"{SYNC_RULE}: close-codes: 4018 consumer_replaced: in {SYNC_TABLE} but not in "
        f"{ASYNC_CODES}",
        f"{SYNC_RULE}: close-codes: 4015 consumer_replaced: in {ASYNC_CODES} but not in "
        f"{SYNC_TABLE}",
    ]


def test_close_code_table_missing_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, SYNC, "### 6.4 Close codes\n", "### 6.4 Codes on closing\n")
    assert _sync(root, "close-codes") == [
        f'{SYNC_RULE}: close-codes: docs/spec/04-sync-protocol.md: 0 sections "Close codes", '
        "expected 1"
    ]


def test_close_code_heading_in_code_block_is_ignored(tmp_path: Path) -> None:
    # A heading inside a fenced block is no heading; the real section must still be found.
    root = _copy_inputs(tmp_path)
    _edit(
        root,
        SYNC,
        "### 6.4 Close codes\n",
        "```\n### Close codes\n```\n\n### 6.4 Close codes\n",
    )
    assert _sync(root, "close-codes") == []


# ---- idempotency ---------------------------------------------------------------------------

KEY_REF = {"$ref": "#/components/parameters/IdempotencyKey"}


def test_post_without_key_or_exemption_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root, OPENAPI, lambda d: _operation(d, "/auth/logout", "post").pop("x-idempotency-exempt")
    )
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: logout: POST has neither an Idempotency-Key header parameter "
        "nor x-idempotency-exempt"
    ]


def test_unknown_exemption_reason_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _operation(d, "/auth/logout", "post").update({"x-idempotency-exempt": "session"}),
    )
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: logout: x-idempotency-exempt 'session' is not a known reason "
        "(auth_flow, read_only, agent_runtime, client_id, origin_key, telemetry)"
    ]


def test_key_and_exemption_together_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: (
            _operation(d, "/auth/logout", "post").setdefault("parameters", []).append(KEY_REF)
        ),
    )
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: logout: has both an Idempotency-Key header parameter and "
        "x-idempotency-exempt"
    ]


def test_exemption_on_get_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _operation(d, "/me", "get").update({"x-idempotency-exempt": "read_only"}),
    )
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: getMe: x-idempotency-exempt on GET; only POST and PATCH "
        "carry it"
    ]


def test_exemption_on_path_item_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root, OPENAPI, lambda d: d["paths"]["/me"].update({"x-idempotency-exempt": "read_only"})
    )
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: /me: x-idempotency-exempt belongs on an operation, not on the "
        "path item"
    ]


def test_path_level_key_counts(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def move(d: dict[str, Any]) -> None:
        _operation(d, "/users", "post")["parameters"].remove(KEY_REF)
        d["paths"]["/users"]["parameters"] = [KEY_REF]

    _mutate(root, OPENAPI, move)
    assert _sync(root, "idempotency") == []
    _mutate(root, OPENAPI, lambda d: d["paths"]["/users"].pop("parameters"))
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: createUser: POST has neither an Idempotency-Key header "
        "parameter nor x-idempotency-exempt"
    ]


def test_inline_key_header_counts_in_any_case(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def inline(d: dict[str, Any]) -> None:
        parameters = _operation(d, "/users", "post")["parameters"]
        parameters[parameters.index(KEY_REF)] = {
            "name": "idempotency-key",
            "in": "header",
            "schema": {"type": "string"},
        }

    _mutate(root, OPENAPI, inline)
    assert _sync(root, "idempotency") == []


def test_key_component_must_be_header(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: d["components"]["parameters"]["IdempotencyKey"].update({"in": "query"}),
    )
    findings = _sync(root, "idempotency")
    assert (
        f"{SYNC_RULE}: idempotency: createUser: POST has neither an Idempotency-Key header "
        "parameter nor x-idempotency-exempt"
    ) in findings
    assert all("has neither an Idempotency-Key header parameter" in f for f in findings)


# ---- deprecation ---------------------------------------------------------------------------

GET_ME = "docs/api/openapi.yaml#/paths/~1me/get"


def _deprecate(**fields: object) -> Callable[[dict[str, Any]], object]:
    return lambda d: _operation(d, "/me", "get").update(fields)


def test_deprecated_without_sunset_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        _deprecate(
            deprecated=True, **{"x-deprecated-since": "0.2.0", "x-deprecated-at": "2026-11-01"}
        ),
    )
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: {GET_ME}: deprecated: true without x-sunset"
    ]


def test_sunset_too_early_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    fields = {"x-deprecated-since": "0.2.0", "x-deprecated-at": "2026-11-01"}
    _mutate(root, OPENAPI, _deprecate(deprecated=True, **fields, **{"x-sunset": "2027-04-29"}))
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: {GET_ME}: x-sunset 2027-04-29 is less than "
        "api.deprecation_min_notice (180d) after x-deprecated-at 2026-11-01"
    ]


def test_sunset_exactly_min_notice_passes(tmp_path: Path) -> None:
    # Unquoted YAML dates load as date objects; they count like the quoted form.
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        _deprecate(
            deprecated=True,
            **{
                "x-deprecated-since": "0.2.0",
                "x-deprecated-at": datetime.date(2026, 11, 1),
                "x-sunset": datetime.date(2027, 4, 30),
            },
        ),
    )
    assert "x-deprecated-at: 2026-11-01\n" in (root / OPENAPI).read_text(encoding="utf-8")
    assert _sync(root, "deprecation") == []


def test_sunset_without_deprecated_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, _deprecate(**{"x-sunset": "2027-04-30"}))
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: {GET_ME}: x-sunset without deprecated: true"
    ]


def test_deprecation_values_must_parse(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    fields = {"x-deprecated-since": "next", "x-deprecated-at": "2026-13-01", "x-sunset": "soon"}
    _mutate(root, OPENAPI, _deprecate(deprecated=True, **fields))
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: {GET_ME}: x-deprecated-since 'next' is not a SemVer version",
        f"{SYNC_RULE}: deprecation: {GET_ME}: x-deprecated-at '2026-13-01' is not a date "
        "YYYY-MM-DD",
        f"{SYNC_RULE}: deprecation: {GET_ME}: x-sunset 'soon' is not a date YYYY-MM-DD",
    ]


def test_deprecation_in_asyncapi_is_checked(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: _message(d, "op.focus").update(deprecated=True))
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: docs/api/asyncapi.yaml#/components/messages/op.focus: "
        f"deprecated: true without {key}"
        for key in ("x-deprecated-since", "x-deprecated-at", "x-sunset")
    ]


def test_example_data_is_not_a_deprecation(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        example = _operation(d, "/me", "get")["responses"]["200"]["content"]["application/json"]
        example["examples"]["default"]["value"]["deprecated"] = True
        properties = d["components"]["schemas"]["Problem"]["properties"]
        properties["examples"] = {"type": "string", "deprecated": True}
        # Property names are names, not deprecation fields.
        properties["deprecated"] = {"type": "boolean"}
        properties["x-sunset"] = {"type": "string"}

    _mutate(root, OPENAPI, change)
    # Example values are data; a property named `examples` is a schema and is checked.
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: docs/api/openapi.yaml#/components/schemas/Problem/properties/"
        f"examples: deprecated: true without {key}"
        for key in ("x-deprecated-since", "x-deprecated-at", "x-sunset")
    ]


CONFIG_ROW = (
    "| `api.deprecation_min_notice` | `BOTHY_API__DEPRECATION_MIN_NOTICE` | Server | `180d` | "
    "minimum gap from `Deprecation` to `Sunset` |\n"
)


def test_missing_config_value_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CONFIG, CONFIG_ROW, "")
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: api.deprecation_min_notice: no row in "
        'docs/operations/configuration.md "Named values of all chapters"'
    ]


def test_duplicate_config_value_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CONFIG, CONFIG_ROW, CONFIG_ROW + CONFIG_ROW)
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: api.deprecation_min_notice: 2 rows in "
        'docs/operations/configuration.md "Named values of all chapters"'
    ]


def test_config_value_must_be_a_duration(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CONFIG, CONFIG_ROW, CONFIG_ROW.replace("`180d`", "halbes Jahr"))
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: api.deprecation_min_notice: standard value 'halbes Jahr' is "
        "not a duration (number with unit ms, s, m, h or d)"
    ]


def test_config_value_is_read_by_column_name(tmp_path: Path) -> None:
    # Another column before "Default" (M1-08 may add one) must not shift the value read.
    root = _copy_inputs(tmp_path)
    _edit(
        root,
        CONFIG,
        "### 06 Public API\n\nSource: [06-api.md](../spec/06-api.md), section 20.\n\n"
        "| Key | Environment | Place | Default | Meaning |\n|---|---|---|---|---|\n",
        "### 06 Public API\n\nSource: [06-api.md](../spec/06-api.md), section 20.\n\n"
        "| Key | Type | Environment | Place | Default | Meaning |\n|---|---|---|---|---|---|\n",
    )
    _edit(
        root,
        CONFIG,
        CONFIG_ROW,
        CONFIG_ROW.replace("`180d`", "`4320h`").replace(
            "| `BOTHY_API__DEPRECATION_MIN_NOTICE`",
            "| duration | `BOTHY_API__DEPRECATION_MIN_NOTICE`",
        ),
    )
    fields = {"x-deprecated-since": "0.2.0", "x-deprecated-at": "2026-11-01"}
    _mutate(root, OPENAPI, _deprecate(deprecated=True, **fields, **{"x-sunset": "2027-04-29"}))
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: {GET_ME}: x-sunset 2027-04-29 is less than "
        "api.deprecation_min_notice (4320h) after x-deprecated-at 2026-11-01"
    ]
    _mutate(root, OPENAPI, _deprecate(**{"x-sunset": "2027-04-30"}))
    assert _sync(root, "deprecation") == []


# ---- problem-codes -------------------------------------------------------------------------


def test_repository_is_clean() -> None:
    assert api_checks.check_catalogue_sync(REPO) == []
    assert api_checks.check_tables(REPO) == []


def _response(data: dict[str, Any], name: str) -> dict[str, Any]:
    response: dict[str, Any] = data["components"]["responses"][name]
    return response


def _problem_codes(data: dict[str, Any]) -> list[dict[str, Any]]:
    codes: list[dict[str, Any]] = data["components"]["schemas"]["Problem"]["x-problem-codes"]
    return codes


def _append_code(name: str, code: str) -> Callable[[dict[str, Any]], object]:
    def change(d: dict[str, Any]) -> None:
        response = _response(d, name)
        response["description"] = response["description"].rstrip(".") + f", `{code}`."

    return change


def test_code_in_response_missing_from_source_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, _append_code("Conflict", "name_taken"))
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: Conflict: description names 'name_taken', which is not in "
        "x-problem-codes"
    ]


def test_problem_status_mismatch_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, _append_code("Gone", "conflict"))
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: Gone: description names 'conflict' with status 409, but "
        "the response has x-status 410"
    ]


def test_unused_problem_code_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def drop(d: dict[str, Any]) -> None:
        response = _response(d, "Conflict")
        response["description"] = response["description"].replace(" `title_manual`,", "")

    _mutate(root, OPENAPI, drop)
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: title_manual: no response of components.responses names "
        "it; name it in a description or mark it router: true"
    ]


def test_router_code_may_be_unused(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    assert _sync(root, "problem-codes") == []  # method_not_allowed carries router: true

    def drop(d: dict[str, Any]) -> None:
        next(c for c in _problem_codes(d) if c["code"] == "method_not_allowed").pop("router")

    _mutate(root, OPENAPI, drop)
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: method_not_allowed: no response of components.responses "
        "names it; name it in a description or mark it router: true"
    ]


def test_response_without_x_status_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, lambda d: _response(d, "NotFound").pop("x-status"))
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: NotFound: x-status is missing"
    ]


def test_response_referenced_under_other_status_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    not_found = {"$ref": "#/components/responses/NotFound"}
    _mutate(
        root, OPENAPI, lambda d: _operation(d, "/me", "get")["responses"].update({"400": not_found})
    )
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: getMe: response 400 refers to NotFound with x-status 404"
    ]


def test_default_response_needs_a_server_error(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        responses = _operation(d, "/me", "get")["responses"]
        responses["default"] = {"$ref": "#/components/responses/NotFound"}
        _operation(d, "/users", "get")["responses"]["default"] = {
            "$ref": "#/components/responses/ServiceUnavailable"
        }

    _mutate(root, OPENAPI, change)
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: getMe: response default refers to NotFound with x-status 404"
    ]


def test_unknown_response_reference_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _operation(d, "/me", "get")["responses"].update(
            {"429": {"$ref": "#/components/responses/Nope"}}
        ),
    )
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: getMe: response 429 refers to "
        "'#/components/responses/Nope', which is not in components.responses"
    ]


def test_problem_codes_are_sorted_by_status(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def swap(d: dict[str, Any]) -> None:
        codes = _problem_codes(d)
        codes[3], codes[4] = codes[4], codes[3]  # webhook_host_not_allowed (400), unauthenticated

    _mutate(root, OPENAPI, swap)
    assert _sync(root, "problem-codes") == [
        f"{SYNC_RULE}: problem-codes: webhook_host_not_allowed: status 400 comes after status "
        "401; x-problem-codes is sorted by status"
    ]


def _entry(d: dict[str, Any], code: str) -> dict[str, Any]:
    return next(c for c in _problem_codes(d) if c["code"] == code)


@pytest.mark.parametrize(
    ("change", "finding"),
    [
        (
            lambda d: _problem_codes(d).append(dict(_entry(d, "setup_required"))),
            "setup_required: listed twice in x-problem-codes",
        ),
        (
            lambda d: _entry(d, "gone").update(title=_UMLAUT_TITLE),
            f"gone: title is not English (R-API-15): {_UMLAUT_TITLE!r}",
        ),
        (lambda d: _entry(d, "gone").update(client_action=" "), "gone: client_action is empty"),
        (lambda d: _entry(d, "gone").pop("description"), "gone: description is empty"),
        (lambda d: _entry(d, "gone").update(hint="x"), "gone: unknown key 'hint'"),
        (
            lambda d: _entry(d, "gone").update(router="yes"),
            "gone: router must be true when present",
        ),
        (
            lambda d: _entry(d, "setup_required").update(status=5030),
            "setup_required: status 5030 is not a three-digit HTTP status",
        ),
        (
            lambda d: _entry(d, "setup_required").update(code="Setup Required"),
            "x-problem-codes[50]: code 'Setup Required' is not a snake_case name",
        ),
    ],
)
def test_problem_code_entries_are_complete(
    tmp_path: Path, change: Callable[[dict[str, Any]], object], finding: str
) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, change)
    assert f"{SYNC_RULE}: problem-codes: {finding}" in _sync(root, "problem-codes")


def test_missing_problem_code_list_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, lambda d: d["components"]["schemas"]["Problem"].pop("x-problem-codes"))
    assert _sync(root, "problem-codes")[0] == (
        f"{SYNC_RULE}: problem-codes: Problem: x-problem-codes is missing or empty"
    )


# ---- api-tables ----------------------------------------------------------------------------

TABLES_RULE = "api-tables"
AUTH_FLOW = (
    "login or session flow: one-time challenge or code, refresh rotation, replacement of the "
    "child session"
)


def _tables(root: Path) -> list[str]:
    return api_checks.check_tables(root)


def _stale(block: str, rel: Path) -> str:
    return (
        f"{TABLES_RULE}: {block}: {rel.as_posix()}: generated block differs from its source; "
        "run make api-tables"
    )


def test_render_blocks_names_every_block() -> None:
    blocks = api_checks.render_blocks(REPO)
    assert {rel: set(names) for rel, names in blocks.items()} == {
        CHAPTER: {"problems", "idempotency_exempt", "endpoints", "ws_ops", "events"},
        WEBHOOKS: {"webhook_events"},
    }


def test_rendered_rows_follow_the_sources() -> None:
    blocks = api_checks.render_blocks(REPO)
    chapter, webhooks = blocks[CHAPTER], blocks[WEBHOOKS]
    assert chapter["problems"].startswith(
        "| Code | Status | Meaning (from openapi.yaml) | Client reaction (from openapi.yaml) |\n"
        "|---|---|---|---|\n"
        "| `validation_failed` | 400 | The request violates the schema or a rule; `errors[]` "
        "names the fields. | Correct the input; do not retry. |\n"
    )
    assert (
        f"| `pairHost` | POST | `/api/v1/hosts/pair` | {AUTH_FLOW} |\n"
        "| `openTerminalSession` | POST | `/api/v1/hosts/{host_id}/terminal/sessions` | "
        f"{AUTH_FLOW} |\n"
    ) in chapter["idempotency_exempt"]
    assert "`createHost`" not in chapter["idempotency_exempt"]  # it takes the header (06, 7)
    assert chapter["idempotency_exempt"].endswith(
        "| `submitClientMetrics` | POST | `/api/v1/client-metrics` | anonymous measurements; a "
        "retry counts a sample twice |\n"
    )
    assert chapter["endpoints"].startswith(
        "#### Discovery\n\n"
        "| Method | Path | operationId | Action | Purpose (from openapi.yaml) |\n"
        "|---|---|---|---|---|\n"
        "| GET | `/api/v1/instance/discovery` | `getDiscovery` | `instance.discovery.read` | "
        "Public instance metadata |\n"
    )
    assert "|\n\n#### Auth\n\n| Method |" in chapter["endpoints"]
    assert (
        "| `reauth` | Client to server | `gateway.connect` | `GatewayReauth` |  |\n"
        in (chapter["ws_ops"])
    )
    assert chapter["ws_ops"].endswith(
        "| `dispatch` | Server to client | Receive action of the event type | `<Type>Event` | |\n"
    )
    assert (
        "| `access.changed` | durable | `self` | self | `AccessChangedEvent` | yes |\n"
        in chapter["events"]
    )
    assert (
        "| `stats.live` | ephemeral | `stats.read` |  | `StatsLiveEvent` |  |\n"
        in (chapter["events"])
    )
    assert (
        "| `agent.invocation` | `agent.events.consume` | "
        "agent subscription only (agent is the receiver) | "
        "`AgentInvocationEvent` |\n" in webhooks["webhook_events"]
    )
    assert (
        "| `command.changed` | `command.read` | none (metadata, 05 section 2.6) | "
        "`CommandChangedEvent` |\n" in webhooks["webhook_events"]
    )


@pytest.mark.parametrize(
    ("rel", "block", "old", "new"),
    [
        (CHAPTER, "problems", "| `gone` | 410 |", "| `gone` | 409 |"),
        (CHAPTER, "idempotency_exempt", "| `logout` | POST |", "| `logout` | PATCH |"),
        (
            CHAPTER,
            "endpoints",
            "| GET | `/api/v1/me` | `getMe` |",
            "| GET | `/api/v1/me/` | `getMe` |",
        ),
        (
            CHAPTER,
            "ws_ops",
            "| `focus` | Client to server | `gateway.focus` |",
            "| `focus` | Client to server | `gateway.connect` |",
        ),
        (CHAPTER, "events", "| `stats.live` | ephemeral |", "| `stats.live` | durable |"),
        (
            WEBHOOKS,
            "webhook_events",
            "| `file.updated` | `file.read` | `files:read` |",
            "| `file.updated` | `file.read` | `files:write` |",
        ),
    ],
)
def test_edited_row_fails(tmp_path: Path, rel: Path, block: str, old: str, new: str) -> None:
    root = _copy_inputs(tmp_path)
    assert _tables(root) == []
    _edit(root, rel, old, new)
    assert _tables(root) == [_stale(block, rel)]


def test_unknown_block_name_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(
        root,
        WEBHOOKS,
        "## 8. Incoming webhooks\n",
        "<!-- BEGIN GENERATED: webhook_scopes -->\n<!-- END GENERATED: webhook_scopes -->\n\n"
        "## 8. Incoming webhooks\n",
    )
    assert _tables(root) == [
        f"{TABLES_RULE}: markers: docs/api/webhooks.md: unknown block 'webhook_scopes'; this "
        "tool owns webhook_events"
    ]


def test_missing_block_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CHAPTER, "<!-- BEGIN GENERATED: ws_ops -->\n", "")
    _edit(root, CHAPTER, "<!-- END GENERATED: ws_ops -->\n", "")
    assert _tables(root) == [f"{TABLES_RULE}: markers: docs/spec/06-api.md: missing block 'ws_ops'"]


def test_broken_markers_fail(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CHAPTER, "<!-- END GENERATED: events -->\n", "")
    [finding] = _tables(root)
    assert finding.startswith(f"{TABLES_RULE}: markers: docs/spec/06-api.md: line ")
    assert "BEGIN without END for 'events'" in finding


def test_generate_is_idempotent(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    assert api_checks.generate(root) == []  # the repository is current
    _edit(root, CHAPTER, "| `gone` | 410 |", "| `gone` | 409 |")
    _edit(root, WEBHOOKS, "| `file.updated` | `file.read` | `files:read` |", "| x |")
    assert api_checks.generate(root) == [CHAPTER, WEBHOOKS]
    assert api_checks.generate(root) == []
    for rel in (CHAPTER, WEBHOOKS):
        assert (root / rel).read_bytes() == (REPO / rel).read_bytes()


def test_generate_refuses_invalid_source(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CHAPTER, "| `gone` | 410 |", "| `gone` | 409 |")
    stale = (root / CHAPTER).read_bytes()
    _edit(root, ASYNCAPI, "asyncapi: 3.0.0\n", "asyncapi: [3.0.0\n")
    with pytest.raises(api_checks.ApiCheckError) as err:
        api_checks.generate(root)
    assert all(
        f.startswith(f"{TABLES_RULE}: source: docs/api/asyncapi.yaml: ") for f in err.value.findings
    )
    assert (root / CHAPTER).read_bytes() == stale


def test_generate_refuses_unknown_block(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CHAPTER, "| `gone` | 410 |", "| `gone` | 409 |")
    _edit(
        root,
        WEBHOOKS,
        "## 8. Incoming webhooks\n",
        "<!-- BEGIN GENERATED: extra -->\n<!-- END GENERATED: extra -->\n\n"
        "## 8. Incoming webhooks\n",
    )
    stale = (root / CHAPTER).read_bytes()
    with pytest.raises(api_checks.ApiCheckError, match="unknown block 'extra'"):
        api_checks.generate(root)
    assert (root / CHAPTER).read_bytes() == stale


def test_operation_with_undeclared_tag_fails(tmp_path: Path) -> None:
    # Without this check the operation would silently drop out of the endpoints table.
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        _operation(d, "/me", "get")["tags"] = ["Myself"]
        _operation(d, "/me/profile", "patch").pop("tags")

    _mutate(root, OPENAPI, change)
    assert _tables(root) == [
        f"{TABLES_RULE}: endpoints: getMe: tag 'Myself' is not declared in tags",
        f"{TABLES_RULE}: endpoints: updateMyProfile: has no tag",
    ]


def test_cell_with_pipe_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, lambda d: _operation(d, "/me", "get").update(summary="Read | write"))
    expected = [
        f"{TABLES_RULE}: endpoints: getMe: summary 'Read | write' contains a pipe or a line break"
    ]
    assert _tables(root) == expected
    with pytest.raises(api_checks.ApiCheckError) as err:
        api_checks.generate(root)
    assert err.value.findings == expected


def test_missing_render_field_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: _message(d, "op.focus")["payload"]["properties"].pop("d"))
    assert _tables(root) == [
        f"{TABLES_RULE}: ws_ops: receive.focus: message 'focus' has no payload.properties.d.$ref"
    ]


def test_methods_render_in_file_order(tmp_path: Path) -> None:
    # The table follows the path item as written (GET, PATCH, DELETE), not the OpenAPI key order.
    root = _copy_inputs(tmp_path)
    assert _tables(root) == []

    def move_patch_last(d: dict[str, Any]) -> None:
        item = d["paths"]["/groups/{group_id}"]
        item["patch"] = item.pop("patch")

    _mutate(root, OPENAPI, move_patch_last)
    assert _tables(root) == [_stale("endpoints", CHAPTER)]


def test_events_render_in_dispatch_order(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def move_first_last(d: dict[str, Any]) -> None:
        messages = _async_op(d, "send.dispatch")["messages"]
        messages.append(messages.pop(0))

    _mutate(root, ASYNCAPI, move_first_last)
    assert _sync(root, "events") == []  # the order of the list carries no meaning for G2
    assert _tables(root) == [_stale("events", CHAPTER)]


# ---- command line and gate wiring ----------------------------------------------------------


def test_cli_exit_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _copy_inputs(tmp_path)
    assert api_checks.main(["check", "--root", str(root)]) == 0
    _edit(root, CHAPTER, "| `gone` | 410 |", "| `gone` | 409 |")
    assert api_checks.main(["check", "--root", str(root)]) == 1
    assert capsys.readouterr().err == _stale("problems", CHAPTER) + "\n"
    assert api_checks.main(["check", "--root", str(root), "--rule", SYNC_RULE]) == 0
    assert api_checks.main(["check", "--root", str(root), "--rule", TABLES_RULE]) == 1
    capsys.readouterr()
    _edit(root, OPENAPI, "openapi: 3.1.0\n", "openapi: [3.1.0\n")
    stale = (root / CHAPTER).read_bytes()
    assert api_checks.main(["generate", "--root", str(root)]) == 1
    assert "nothing written" in capsys.readouterr().err
    assert (root / CHAPTER).read_bytes() == stale
    _edit(root, OPENAPI, "openapi: [3.1.0\n", "openapi: 3.1.0\n")
    assert api_checks.main(["generate", "--root", str(root)]) == 0
    assert capsys.readouterr().out == "  rewrote docs/spec/06-api.md\n"
    assert api_checks.main(["check", "--root", str(root)]) == 0
    with pytest.raises(SystemExit) as exit_info:
        api_checks.main(["generate", "--root", str(root), "--rule", SYNC_RULE])
    assert exit_info.value.code == 2


def _make_rule(makefile: str, target: str) -> list[str]:
    """Prerequisites of every `target:` line of the Makefile, continuation lines joined."""
    joined = makefile.replace("\\\n", " ")
    prerequisites: list[str] = []
    for line in joined.splitlines():
        if line.startswith(f"{target}:"):
            prerequisites += line.split(":", 1)[1].split("##", 1)[0].split()
    return prerequisites


def test_make_gates_run_api_checks_after_api_lint() -> None:
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    gates = _make_rule(makefile, "gates")
    assert gates.index("api-lint") + 1 == gates.index("api-checks")
    assert {"api-checks", "api-tables"} <= set(_make_rule(makefile, ".PHONY"))
    bandit = next(line for line in makefile.splitlines() if "bandit -q -c" in line)
    assert " tools/api-checks " in bandit


API_RECIPES = (
    "api-checks: ## API contract copies equal their sources (06 G2, G3)\n"
    "\t$(UVRUN) python tools/api-checks/api_checks.py check\n"
    "\n"
    "api-tables: ## Regenerate the generated tables of chapter 06 and webhooks.md\n"
    "\t$(UVRUN) python tools/api-checks/api_checks.py generate\n"
)


def test_make_api_recipes_are_pinned() -> None:
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    start = makefile.index("api-checks: ##")
    end = makefile.index("\n\n", makefile.index("api-tables: ##")) + 1
    assert makefile[start:end] == API_RECIPES


def _make(root: Path, target: str) -> tuple[int, str]:
    env = {k: v for k, v in os.environ.items() if k not in ("MAKEFLAGS", "MAKELEVEL", "MFLAGS")}
    # The project environment of the repository (as `make` itself would use it), never the
    # interpreter that happens to run pytest, so that uv neither builds an environment in
    # tmp_path nor rewrites a foreign one.
    env.setdefault("UV_PROJECT_ENVIRONMENT", str(REPO / "tools" / ".venv"))
    proc = subprocess.run(
        ["make", target],
        cwd=root,
        env=env,
        capture_output=True,
        text=True,
        timeout=600,
        check=False,
    )
    return proc.returncode, proc.stdout + proc.stderr


def test_make_api_checks_runs_the_real_tool_on_a_copy(tmp_path: Path) -> None:
    # Through the real Makefile on a copy of the inputs: green, red on a stale block and on a
    # catalogue difference, and `make api-tables` repairs only the stale block.
    root = _copy_inputs(tmp_path)
    shutil.copy(REPO / "Makefile", root / "Makefile")
    for rel in (
        ".python-version",
        "pyproject.toml",
        "uv.lock",
        "catalog/action_catalog.py",
        "api-checks/api_checks.py",
    ):
        (root / "tools" / rel).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy(REPO / "tools" / rel, root / "tools" / rel)
    code, output = _make(root, "api-checks")
    assert code == 0, output
    _edit(root, CHAPTER, "| `gone` | 410 |", "| `gone` | 409 |")
    _mutate(root, OPENAPI, lambda d: _operation(d, "/me", "get").update({"x-authz-action": "x.y"}))
    code, output = _make(root, "api-checks")
    assert code != 0, output
    assert "api-tables: problems:" in output, output
    assert "api-catalogue-sync: rest-action: getMe:" in output, output
    code, output = _make(root, "api-tables")
    assert code == 0, output
    code, output = _make(root, "api-checks")
    assert code != 0, output
    assert "api-tables:" not in output, output


# ---- closing review: fail closed on every malformed input ----------------------------------


def test_path_item_ref_is_a_source_finding(tmp_path: Path) -> None:
    # A path item given by $ref would hide its operations from G2 and drop them from G3.
    root = _copy_inputs(tmp_path)

    def move(d: dict[str, Any]) -> None:
        d["components"]["pathItems"] = {"Me": d["paths"]["/me"]}
        d["paths"]["/me"] = {"$ref": "#/components/pathItems/Me"}

    _mutate(root, OPENAPI, move)
    detail = "paths./me: a path item given by $ref is not supported; write its operations inline"
    assert api_checks.check_catalogue_sync(root) == [
        f"{SYNC_RULE}: source: docs/api/openapi.yaml: {detail}"
    ]
    assert _tables(root) == [f"{TABLES_RULE}: source: docs/api/openapi.yaml: {detail}"]


def test_ws_ops_row_with_unresolved_channel_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "receive.focus").update(channel={"$ref": "#/channels/none"}),
    )
    assert _tables(root) == [
        f"{TABLES_RULE}: ws_ops: receive.focus: channel reference '#/channels/none' does not "
        "resolve"
    ]


def test_missing_tags_fail_the_endpoints_table(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, lambda d: d.pop("tags"))
    assert _tables(root) == [
        f"{TABLES_RULE}: endpoints: tags: missing; the table groups the operations by tag"
    ]


@pytest.mark.parametrize(
    ("change", "detail"),
    [
        (lambda d: d.update(tags=None), "tags: null; remove the key or give a list"),
        (
            lambda d: _operation(d, "/users", "post").update(parameters=None),
            "paths./users.post.parameters: null; remove the key or give a list",
        ),
        (
            lambda d: d["paths"]["/users"].update(parameters=None),
            "paths./users.parameters: null; remove the key or give a list",
        ),
        (
            lambda d: _operation(d, "/me", "get").update(responses=None),
            "paths./me.get.responses: null; remove the key or give a mapping",
        ),
    ],
)
def test_null_container_is_a_source_finding(
    tmp_path: Path, change: Callable[[dict[str, Any]], object], detail: str
) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, change)
    expected = f"source: docs/api/openapi.yaml: {detail}"
    assert api_checks.check_catalogue_sync(root) == [f"{SYNC_RULE}: {expected}"]
    assert _tables(root) == [f"{TABLES_RULE}: {expected}"]


def test_unloadable_yaml_scalar_is_a_source_finding(tmp_path: Path) -> None:
    # An unquoted date that is no real day passes the YAML parser and fails its constructor.
    root = _copy_inputs(tmp_path)
    _edit(
        root,
        OPENAPI,
        "      x-authz-action: self.read\n",
        "      x-authz-action: self.read\n      x-deprecated-at: 2026-13-01\n",
    )
    _edit(root, CATALOG, "version: 1\n", "version: 1\nplanned: 2026-13-01\n")
    findings = api_checks.check_catalogue_sync(root)
    assert findings[0].startswith(f"{SYNC_RULE}: source: docs/spec/actions.yaml: ")
    assert "month must be in 1..12" in findings[0]
    assert findings[1:] == [
        f"{SYNC_RULE}: source: docs/api/openapi.yaml: cannot be loaded: month must be in 1..12, "
        "not 13"
    ]


def test_recursive_alias_does_not_crash(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(
        root, OPENAPI, "info:\n", "info:\n  x-loop: &loop [*loop]\n  x-self: &self {next: *self}\n"
    )
    assert api_checks.check_catalogue_sync(root) == []
    assert _tables(root) == []


def test_huge_numbers_are_findings(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, CONFIG, CONFIG_ROW, CONFIG_ROW.replace("`180d`", f"`{'9' * 30}d`"))
    _edit(root, SYNC, "| `4015` | consumer_replaced |", f"| `{'4' * 30}` | consumer_replaced |")
    findings = api_checks.check_catalogue_sync(root)
    assert (
        f"{SYNC_RULE}: deprecation: api.deprecation_min_notice: standard value '{'9' * 30}d' is "
        "not a duration (number with unit ms, s, m, h or d)"
    ) in findings
    assert (
        f"{SYNC_RULE}: close-codes: {SYNC_TABLE}: code '`{'4' * 30}`' is not a number"
    ) in findings


def test_unicode_digit_in_a_reference_does_not_resolve(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: _async_op(d, "receive.focus")["messages"][0].update(
            {"$ref": "#/operations/receive.focus/messages/²"}
        ),
    )
    assert _sync(root, "ws-ops")[0] == (
        f"{SYNC_RULE}: ws-ops: receive.focus: message reference "
        "'#/operations/receive.focus/messages/²' does not resolve"
    )


def test_separator_only_table_is_ignored(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, SYNC, "### 6.4 Close codes\n", "### 6.4 Close codes\n\n|---|---|\n")
    assert _sync(root, "close-codes") == []


def test_null_exemption_counts_as_carried(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        _operation(d, "/me", "get")["x-idempotency-exempt"] = None
        _operation(d, "/users", "post")["x-idempotency-exempt"] = None

    _mutate(root, OPENAPI, change)
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: getMe: x-idempotency-exempt on GET; only POST and PATCH "
        "carry it",
        f"{SYNC_RULE}: idempotency: createUser: has both an Idempotency-Key header parameter and "
        "x-idempotency-exempt",
        f"{SYNC_RULE}: idempotency: createUser: x-idempotency-exempt None is not a known reason "
        "(auth_flow, read_only, agent_runtime, client_id, origin_key, telemetry)",
    ]


def test_patch_without_key_or_exemption_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root, OPENAPI, lambda d: _operation(d, "/me/profile", "patch")["parameters"].remove(KEY_REF)
    )
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: updateMyProfile: PATCH has neither an Idempotency-Key header "
        "parameter nor x-idempotency-exempt"
    ]


def test_webhook_carries_no_exemption(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        d["webhooks"]["file.updated"]["post"]["x-idempotency-exempt"] = "read_only"
        d["webhooks"]["user.updated"]["x-idempotency-exempt"] = "read_only"

    _mutate(root, OPENAPI, change)
    assert _sync(root, "idempotency") == [
        f"{SYNC_RULE}: idempotency: user.updated: x-idempotency-exempt belongs on an operation, "
        "not on the path item",
        f"{SYNC_RULE}: idempotency: file.updated: webhook operations carry no x-idempotency-exempt",
    ]


def test_list_action_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _operation(d, "/me", "get").update({"x-authz-action": ["self.read"]}),
    )
    assert _sync(root, "rest-action") == [
        f"{SYNC_RULE}: rest-action: getMe: x-authz-action must name exactly one action, not a list"
    ]


def test_inline_event_message_in_a_channel_is_checked(tmp_path: Path) -> None:
    # dispatch may point at a message written inline in the channel instead of a component.
    root = _copy_inputs(tmp_path)

    def inline(d: dict[str, Any]) -> None:
        message = dict(_message(d, "event.file.updated"), **{"x-receive-action": "message.read"})
        d["channels"]["gateway"]["messages"]["event.file.updated"] = message

    _mutate(root, ASYNCAPI, inline)
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: channels.gateway.messages.event.file.updated: event message "
        "'file.updated' is described twice"
    ]


def test_min_notice_is_read_from_the_configuration(tmp_path: Path) -> None:
    # A different value and a column in front of "Key": neither a fixed 180 days nor a
    # positional read passes.
    root = _copy_inputs(tmp_path)
    _edit(
        root,
        CONFIG,
        "| Key | Environment | Place | Default | Meaning |\n|---|---|---|---|---|\n"
        "| `api.page_size_default` |",
        "| No. | Key | Environment | Place | Default | Meaning |\n|---|---|---|---|---|---|\n"
        "| 1 | `api.page_size_default` |",
    )
    _edit(root, CONFIG, CONFIG_ROW, "| 4 " + CONFIG_ROW.replace("`180d`", "`10d`"))
    since = {"x-deprecated-since": "0.2.0", "x-deprecated-at": "2026-11-01"}
    _mutate(root, OPENAPI, _deprecate(deprecated=True, **since, **{"x-sunset": "2026-11-10"}))
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: {GET_ME}: x-sunset 2026-11-10 is less than "
        "api.deprecation_min_notice (10d) after x-deprecated-at 2026-11-01"
    ]
    _mutate(root, OPENAPI, _deprecate(**{"x-sunset": "2026-11-11"}))
    assert _sync(root, "deprecation") == []


def test_channel_op_duplicate_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        ASYNCAPI,
        lambda d: d["operations"].update(
            {"terminal.send.frame2": dict(_async_op(d, "terminal.send.frame"))}
        ),
    )
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: terminal.send.frame2: op 'noise_frame' (send) on channel "
        "'terminal_stream' is already described by operation terminal.send.frame"
    ]


def _first_close_code(d: dict[str, Any]) -> dict[str, Any]:
    return _close_codes(d)[0]


def _drop_dispatch(d: dict[str, Any]) -> None:
    d["operations"].pop("send.dispatch")


def _event_without_name(d: dict[str, Any]) -> None:
    _message(d, "event.stats.live").pop("name")


@pytest.mark.parametrize(
    ("rel", "change", "sub_check", "finding"),
    [
        (
            OPENAPI,
            lambda d: _response(d, "NotFound").update({"x-status": 4040}),
            "problem-codes",
            "NotFound: x-status 4040 is not a three-digit HTTP status",
        ),
        (
            ASYNCAPI,
            lambda d: _async_op(d, "send.dispatch")["messages"].append(
                {"$ref": "#/channels/gateway/messages/event.stats.live"}
            ),
            "events",
            "send.dispatch: lists 'stats.live' 2 times",
        ),
        (
            ASYNCAPI,
            _drop_dispatch,
            "events",
            "dispatch: no operation on channel gateway lists the event messages",
        ),
        (
            ASYNCAPI,
            _event_without_name,
            "events",
            "event.stats.live: message with x-event-kind has no name",
        ),
        (
            ASYNCAPI,
            lambda d: d["operations"].update(
                {"receive.focus2": dict(_async_op(d, "receive.focus"))}
            ),
            "ws-ops",
            "receive.focus2: op 'focus' is already described by operation receive.focus",
        ),
        (
            ASYNCAPI,
            lambda d: _async_op(d, "receive.focus").update(messages=[]),
            "ws-ops",
            "receive.focus: has no message",
        ),
        (
            OPENAPI,
            lambda d: d["webhooks"]["file.updated"].pop("post"),
            "webhook-events",
            "file.updated: has no operation",
        ),
        (
            ASYNCAPI,
            lambda d: d["channels"]["gateway"].update({"x-close-codes": {"1000": "normal"}}),
            "close-codes",
            "docs/api/asyncapi.yaml channels.gateway.x-close-codes: not a list",
        ),
        (
            ASYNCAPI,
            lambda d: _first_close_code(d).update(code="1000"),
            "close-codes",
            "docs/api/asyncapi.yaml channels.gateway.x-close-codes[0]: needs an integer code and "
            "a name",
        ),
        (
            ASYNCAPI,
            lambda d: _close_codes(d).append(dict(_first_close_code(d))),
            "close-codes",
            '1000 normal: 1 times in docs/spec/04-sync-protocol.md "Close codes", 2 times in '
            "docs/api/asyncapi.yaml channels.gateway.x-close-codes",
        ),
    ],
)
def test_guard_findings_of_the_sub_checks(
    tmp_path: Path,
    rel: Path,
    change: Callable[[dict[str, Any]], object],
    sub_check: str,
    finding: str,
) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, rel, change)
    assert f"{SYNC_RULE}: {sub_check}: {finding}" in _sync(root, sub_check)


def test_two_configuration_sections_fail(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(
        root,
        CONFIG,
        "## 5. Secrets\n",
        "## 11. Named values of all chapters\n\n## 5. Secrets\n",
    )
    assert _sync(root, "deprecation") == [
        f"{SYNC_RULE}: deprecation: api.deprecation_min_notice: 2 sections "
        '"Named values of all chapters" in docs/operations/configuration.md, expected 1'
    ]


def test_close_code_cell_that_is_no_number_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _edit(root, SYNC, "| `4015` | consumer_replaced |", "| `40x5` | consumer_replaced |")
    assert f"{SYNC_RULE}: close-codes: {SYNC_TABLE}: code '`40x5`' is not a number" in _sync(
        root, "close-codes"
    )


@pytest.mark.parametrize(
    ("rel", "change", "finding"),
    [
        (
            ASYNCAPI,
            lambda d: _message(d, "event.typing.started").update({"x-event-kind": "sometimes"}),
            "events: typing.started: x-event-kind 'sometimes' is unknown",
        ),
        (
            OPENAPI,
            lambda d: d["webhooks"]["file.updated"].update(
                put=d["webhooks"]["file.updated"]["post"]
            ),
            "webhook_events: file.updated: has 2 operations, expected 1",
        ),
        (
            OPENAPI,
            lambda d: d["webhooks"]["file.updated"]["post"].pop("requestBody"),
            "webhook_events: file.updated: has no requestBody.content.application/json.schema.$ref",
        ),
        (
            OPENAPI,
            lambda d: d["tags"][1].update(name="Discovery"),
            "endpoints: tags[1]: tag 'Discovery' is declared twice",
        ),
        (
            OPENAPI,
            lambda d: _operation(d, "/me", "get").update(operationId="get`Me"),
            "endpoints: get`Me: operationId 'get`Me' contains a backtick",
        ),
        (
            OPENAPI,
            lambda d: _entry(d, "gone").update(status="410"),
            "problems: gone: status '410' is not a three-digit HTTP status",
        ),
    ],
)
def test_render_guard_findings(
    tmp_path: Path, rel: Path, change: Callable[[dict[str, Any]], object], finding: str
) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, rel, change)
    assert f"{TABLES_RULE}: {finding}" in _tables(root)


# ---- red team: catalogue copies inside the contract schemas --------------------------------


def _schema(data: dict[str, Any], name: str) -> dict[str, Any]:
    schema: dict[str, Any] = data["components"]["schemas"][name]
    return schema


def test_app_scope_enum_equals_the_catalogue(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    assert _sync(root, "app-scopes") == []

    def change(d: dict[str, Any]) -> None:
        enum = _schema(d, "AppScope")["enum"]
        enum.remove("users:read")
        enum.append("admin:all")

    _mutate(root, OPENAPI, change)
    assert _sync(root, "app-scopes") == [
        f"{SYNC_RULE}: app-scopes: AppScope: scope 'users:read' of docs/spec/actions.yaml (05 "
        "section 2.6) is missing from the enum",
        f"{SYNC_RULE}: app-scopes: AppScope: 'admin:all' is not a scope of docs/spec/actions.yaml",
    ]


def test_event_union_equals_the_durable_events(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def drop(d: dict[str, Any]) -> None:
        union = _schema(d, "Event")
        union["oneOf"].remove({"$ref": "#/components/schemas/HostUpdatedEvent"})
        union["discriminator"]["mapping"].pop("host.updated")
        union["oneOf"].append({"$ref": "#/components/schemas/StatsLiveEvent"})
        union["discriminator"]["mapping"]["stats.live"] = "#/components/schemas/StatsLiveEvent"

    _mutate(root, OPENAPI, drop)
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: Event: durable event type 'host.updated' is missing from "
        "discriminator.mapping",
        f"{SYNC_RULE}: events: Event: discriminator.mapping names 'stats.live', which is not a "
        "durable event type of docs/spec/actions.yaml",
    ]


def test_event_union_one_of_equals_the_mapping(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _schema(d, "Event")["oneOf"].remove(
            {"$ref": "#/components/schemas/HostUpdatedEvent"}
        ),
    )
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: Event: discriminator.mapping names "
        "'#/components/schemas/HostUpdatedEvent', which oneOf does not list"
    ]


def test_event_schema_type_matches_the_event(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        _schema(d, "MessageCreatedEvent")["allOf"][1]["properties"]["type"]["enum"] = [
            "message.updated"
        ]
        _schema(d, "StatsLiveEvent")["allOf"][1]["properties"]["type"]["enum"] = ["stats.dead"]

    _mutate(root, OPENAPI, change)
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: Event: discriminator.mapping sends 'message.created' to "
        "MessageCreatedEvent, whose type is ['message.updated']",
        f"{SYNC_RULE}: events: message.created: schema MessageCreatedEvent has type "
        "['message.updated'], expected ['message.created']",
        f"{SYNC_RULE}: events: stats.live: schema StatsLiveEvent has type ['stats.dead'], "
        "expected ['stats.live']",
    ]
    assert _sync(root, "webhook-events") == [
        f"{SYNC_RULE}: webhook-events: message.created: schema MessageCreatedEvent has type "
        "['message.updated'], expected ['message.created']"
    ]


def test_frame_op_matches_the_message(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        _message(d, "op.identify")["payload"]["properties"]["op"]["enum"] = ["resume"]
        _message(d, "event.file.updated")["payload"]["properties"]["op"]["enum"] = ["identify"]
        _message(d, "terminal.hello")["payload"]["properties"]["op"]["enum"] = ["identify"]

    _mutate(root, ASYNCAPI, change)
    assert _sync(root, "ws-ops") == [
        f"{SYNC_RULE}: ws-ops: receive.identify: message 'identify' frames op ['resume'], "
        "expected ['identify']",
        f"{SYNC_RULE}: ws-ops: send.dispatch: message 'file.updated' frames op ['identify'], "
        "expected ['dispatch']",
    ]
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: terminal.send.hello: message 'hello' frames op ['identify'], "
        "expected ['hello']"
    ]


def test_terminal_close_codes_below_4100_equal_the_chapter(tmp_path: Path) -> None:
    # 04 "Close codes": the codes of the table apply on a terminal channel where it names them.
    root = _copy_inputs(tmp_path)

    def rename(d: dict[str, Any]) -> None:
        codes = d["channels"]["terminalStream"]["x-close-codes"]
        next(c for c in codes if c["code"] == 4002)["name"] = "bad_frame"
        next(c for c in codes if c["code"] == 4100)["name"] = "closed"  # own range, not compared

    _mutate(root, ASYNCAPI, rename)
    assert _sync(root, "close-codes") == [
        f"{SYNC_RULE}: close-codes: terminalStream 4002 bad_frame: in docs/api/asyncapi.yaml "
        f"channels.terminalStream.x-close-codes but not in {SYNC_TABLE}"
    ]


# ---- re-review 2: no traceback from any sub-check or block ---------------------------------


def test_unexpected_error_in_a_sub_check_is_a_finding(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    # The class "malformed input gives a traceback" is closed in one place: the runner turns
    # any error of one sub-check into its finding, and the other sub-checks still report.
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, lambda d: _operation(d, "/me", "get").update({"x-authz-action": "x.y"}))

    def boom(*_: object) -> list[str]:
        raise RuntimeError("boom")

    monkeypatch.setattr(api_checks, "_app_scope_findings", boom)
    findings = api_checks.check_catalogue_sync(root)
    assert f"{SYNC_RULE}: app-scopes: input not checkable (RuntimeError: boom)" in findings
    assert any(f.startswith(f"{SYNC_RULE}: rest-action: getMe: ") for f in findings)


def test_unexpected_error_in_a_block_is_a_finding(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    root = _copy_inputs(tmp_path)

    def boom(*_: object) -> str:
        raise RuntimeError("boom")

    monkeypatch.setattr(api_checks, "_events_block", boom)
    assert _tables(root) == [f"{TABLES_RULE}: events: input not renderable (RuntimeError: boom)"]


def test_scope_enum_with_a_mapping_is_a_finding(tmp_path: Path) -> None:
    # `- costs: reconcile` (a space after the colon) is a YAML mapping, not a scope.
    root = _copy_inputs(tmp_path)
    _edit(root, OPENAPI, "      - costs:reconcile\n", "      - costs: reconcile\n")
    assert _sync(root, "app-scopes") == [
        f"{SYNC_RULE}: app-scopes: AppScope: scope 'costs:reconcile' of docs/spec/actions.yaml "
        "(05 section 2.6) is missing from the enum",
        f"{SYNC_RULE}: app-scopes: AppScope: {{'costs': 'reconcile'}} is not a scope of "
        "docs/spec/actions.yaml",
    ]


def test_all_of_that_is_no_list_is_a_finding(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, lambda d: _schema(d, "HostUpdatedEvent").update(allOf=True))
    findings = _sync(root, "events")
    assert (
        f"{SYNC_RULE}: events: host.updated: schema HostUpdatedEvent has type None, expected "
        "['host.updated']"
    ) in findings
    assert all("not checkable" not in f for f in findings)


def test_event_union_lists_each_schema_once(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(
        root,
        OPENAPI,
        lambda d: _schema(d, "Event")["oneOf"].append(
            {"$ref": "#/components/schemas/MessageCreatedEvent"}
        ),
    )
    assert _sync(root, "events") == [
        f"{SYNC_RULE}: events: Event: oneOf lists '#/components/schemas/MessageCreatedEvent' twice"
    ]


def test_terminal_json_frame_without_op_fails(tmp_path: Path) -> None:
    # Only the binary Noise frame (contentType application/octet-stream) has no `op`.
    root = _copy_inputs(tmp_path)
    _mutate(root, ASYNCAPI, lambda d: _message(d, "host.report")["payload"]["properties"].pop("op"))
    assert _sync(root, "channel-ops") == [
        f"{SYNC_RULE}: channel-ops: hostControl.receive.report: message 'report' frames op None, "
        "expected ['report']"
    ]


def test_terminal_close_code_listed_twice_fails(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def double(d: dict[str, Any]) -> None:
        codes = d["channels"]["hostData"]["x-close-codes"]
        codes.append(dict(next(c for c in codes if c["code"] == 4002)))

    _mutate(root, ASYNCAPI, double)
    assert _sync(root, "close-codes") == [
        f"{SYNC_RULE}: close-codes: hostData 4002 decode_error: listed twice in "
        "docs/api/asyncapi.yaml channels.hostData.x-close-codes"
    ]


def test_app_scope_given_by_ref_is_followed(tmp_path: Path) -> None:
    root = _copy_inputs(tmp_path)

    def move(d: dict[str, Any]) -> None:
        d["components"]["schemas"]["AppScopeValues"] = _schema(d, "AppScope")
        d["components"]["schemas"]["AppScope"] = {"$ref": "#/components/schemas/AppScopeValues"}

    _mutate(root, OPENAPI, move)
    assert _sync(root, "app-scopes") == []


# ---- red team on 1eca876: no traceback, binary frames, inline messages ---------------------

HUGE_KEY = "0x" + "F" * 5000  # a YAML integer with more digits than str() converts
STALE_PROBLEM_ROW = ("| `gone` | 410 |", "| `gone` | 409 |")
STALE_WEBHOOK_ROW = ("| `file.updated` | `file.read` | `files:read` |", "| x |")
NOTHING_WRITTEN = "api_checks: nothing written"


@pytest.mark.parametrize(
    ("rel", "old", "new"),
    [
        (OPENAPI, "\npaths:\n", f"\npaths:\n  ? {HUGE_KEY}\n  : {{}}\n"),
        (ASYNCAPI, "\nchannels:\n", f"\nchannels:\n  ? {HUGE_KEY}\n  : {{}}\n"),
    ],
    ids=["paths", "channels"],
)
def test_key_the_shape_check_cannot_print_is_a_source_finding(
    tmp_path: Path, capsys: pytest.CaptureFixture[str], rel: Path, old: str, new: str
) -> None:
    # The red team's mutant: str() of the key raises in the shape stage, before any sub-check.
    root = _copy_inputs(tmp_path)
    _edit(root, CHAPTER, *STALE_PROBLEM_ROW)
    _edit(root, rel, old, new)
    stale = (root / CHAPTER).read_bytes()
    detail = f"source: {rel.as_posix()}: input not checkable (ValueError: Exceeds the limit"
    assert api_checks.main(["check", "--root", str(root)]) == 1
    lines = capsys.readouterr().err.splitlines()
    assert len(lines) == 2, lines
    assert lines[0].startswith(f"{SYNC_RULE}: {detail}")
    assert lines[1].startswith(f"{TABLES_RULE}: {detail}")
    assert api_checks.main(["generate", "--root", str(root)]) == 1
    lines = capsys.readouterr().err.splitlines()
    assert len(lines) == 2, lines
    assert lines[0].startswith(f"{TABLES_RULE}: {detail}")
    assert lines[1] == NOTHING_WRITTEN
    assert (root / CHAPTER).read_bytes() == stale


def test_read_only_target_is_a_finding_and_nothing_is_written(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    # Every file is opened for writing before the first one is written, so 06 stays as it was.
    root = _copy_inputs(tmp_path)
    _edit(root, CHAPTER, *STALE_PROBLEM_ROW)
    _edit(root, WEBHOOKS, *STALE_WEBHOOK_ROW)
    stale = {rel: (root / rel).read_bytes() for rel in (CHAPTER, WEBHOOKS)}
    (root / WEBHOOKS).chmod(0o444)
    if os.access(root / WEBHOOKS, os.W_OK):
        pytest.skip("this user writes read-only files")
    assert api_checks.main(["generate", "--root", str(root)]) == 1
    lines = capsys.readouterr().err.splitlines()
    assert len(lines) == 2, lines
    assert lines[0].startswith(
        f"{TABLES_RULE}: source: docs/api/webhooks.md: cannot write: [Errno 13] Permission denied"
    )
    assert lines[1] == NOTHING_WRITTEN
    assert {rel: (root / rel).read_bytes() for rel in stale} == stale


EXTRA_PROBLEM_ROW = ("| `gone` | 410 |", "| `extra` | 400 | Extra. | Retry. |\n| `gone` | 410 |")


def _two_stale_files(root: Path) -> dict[Path, bytes]:
    """Make both files stale, 06 with a block longer than its rendering; return their bytes."""
    _edit(root, CHAPTER, *EXTRA_PROBLEM_ROW)
    _edit(root, WEBHOOKS, *STALE_WEBHOOK_ROW)
    with (root / WEBHOOKS).open("a", encoding="utf-8") as stream:
        stream.write("\nPadding outside the blocks. " * 8000)  # larger than 06, about 230 kB
    return {rel: (root / rel).read_bytes() for rel in (CHAPTER, WEBHOOKS)}


def _leftovers(root: Path) -> list[str]:
    return sorted(
        path.name for rel in (CHAPTER, WEBHOOKS) for path in (root / rel).parent.iterdir()
    )


@pytest.mark.parametrize(
    ("limit", "failing"),
    [(50_000, CHAPTER), (150_000, WEBHOOKS)],
    ids=["first-file", "second-file"],
)
def test_write_that_fails_part_way_changes_no_file(
    tmp_path: Path, limit: int, failing: Path
) -> None:
    # A real write error: the process may not write files larger than `limit` (RLIMIT_FSIZE;
    # Python ignores SIGXFSZ, so the write fails with EFBIG). 06 is about 113 kB.
    resource = pytest.importorskip("resource")
    root = _copy_inputs(tmp_path)
    before = _two_stale_files(root)
    files = _leftovers(root)

    def limit_file_size() -> None:
        resource.setrlimit(resource.RLIMIT_FSIZE, (limit, limit))

    command = [sys.executable, "-B", api_checks.__file__, "generate", "--root", str(root)]
    # preexec_fn: the limit must apply to the child alone, never to the test process.
    run = subprocess.run(
        command, capture_output=True, text=True, check=False, preexec_fn=limit_file_size
    )
    assert (run.returncode, run.stdout) == (1, "")
    assert run.stderr == (
        f"{TABLES_RULE}: source: {failing.as_posix()}: cannot write: [Errno 27] File too large\n"
        f"{NOTHING_WRITTEN}\n"
    )
    assert {rel: (root / rel).read_bytes() for rel in before} == before
    assert _leftovers(root) == files
    assert api_checks.generate(root) == [CHAPTER, WEBHOOKS]  # the block of 06 shrinks
    assert (root / CHAPTER).read_bytes() == (REPO / CHAPTER).read_bytes()


def test_failed_rename_names_the_files_already_rewritten(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # Only the final renames can leave one file new and the other old; the message says so.
    root = _copy_inputs(tmp_path)
    before = _two_stale_files(root)
    files = _leftovers(root)
    rename = os.replace

    def fail_for_webhooks(source: str | Path, target: str | Path) -> None:
        if Path(target).name == "webhooks.md":
            raise OSError(16, "Resource busy")
        rename(source, target)

    monkeypatch.setattr(os, "replace", fail_for_webhooks)
    assert api_checks.main(["generate", "--root", str(root)]) == 1
    assert capsys.readouterr().err == (
        f"{TABLES_RULE}: source: docs/api/webhooks.md: cannot write: [Errno 16] Resource busy\n"
        "api_checks: rewrote only docs/spec/06-api.md before the error; run make api-tables "
        "again\n"
    )
    assert (root / CHAPTER).read_bytes() == (REPO / CHAPTER).read_bytes()
    assert (root / WEBHOOKS).read_bytes() == before[WEBHOOKS]
    assert _leftovers(root) == files


@pytest.mark.parametrize("separator", ["\u2028", "\u2029", "\x85", "\x0b", "\x0c", "\x1c"])
def test_cell_with_a_line_separator_fails(tmp_path: Path, separator: str) -> None:
    # str.splitlines breaks the line there, and so does the marker parser of tools/catalog.
    root = _copy_inputs(tmp_path)
    summary = f"Read{separator}me"
    _mutate(root, OPENAPI, lambda d: _operation(d, "/me", "get").update(summary=summary))
    assert _tables(root) == [
        f"{TABLES_RULE}: endpoints: getMe: summary {summary!r} contains a pipe or a line break"
    ]


def _summary(text: str) -> Callable[[dict[str, Any]], object]:
    return lambda d: _operation(d, "/me", "get").update(summary=text)


def _receive_action(text: str) -> Callable[[dict[str, Any]], object]:
    return lambda d: d["webhooks"]["file.updated"]["post"].update({"x-receive-action": text})


@pytest.mark.parametrize(
    "change",
    [
        _summary("Read\u2028<!-- END GENERATED: endpoints -->"),  # a block in the middle of 06
        _receive_action("file.read\u2028<!-- END GENERATED: webhook_events -->"),  # last block
    ],
    ids=["middle-block", "last-block"],
)
def test_generate_never_writes_a_cell_that_breaks_a_marker(
    tmp_path: Path, change: Callable[[dict[str, Any]], object]
) -> None:
    root = _copy_inputs(tmp_path)
    _mutate(root, OPENAPI, change)
    before = {rel: (root / rel).read_bytes() for rel in (CHAPTER, WEBHOOKS)}
    with pytest.raises(api_checks.ApiCheckError, match="contains a pipe or a line break"):
        api_checks.generate(root)
    assert {rel: (root / rel).read_bytes() for rel in before} == before


def test_error_that_cannot_be_printed_is_still_a_finding(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    root = _copy_inputs(tmp_path)

    def boom(*_: object) -> list[str]:
        raise KeyError(1 << 20000)  # str() of this error raises ValueError itself

    monkeypatch.setattr(api_checks, "_app_scope_findings", boom)
    assert _sync(root, "app-scopes") == [
        f"{SYNC_RULE}: app-scopes: input not checkable (KeyError: message cannot be shown)"
    ]


@pytest.mark.parametrize(
    ("target", "rules"),
    [("_load_contracts", (SYNC_RULE, TABLES_RULE)), ("_block_findings", (TABLES_RULE,))],
)
def test_unexpected_error_outside_the_guards_is_one_finding(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
    target: str,
    rules: tuple[str, ...],
) -> None:
    # main is the last place every error ends in: one line per rule, exit 1, never a traceback.
    root = _copy_inputs(tmp_path)
    _edit(root, CHAPTER, *STALE_PROBLEM_ROW)
    stale = (root / CHAPTER).read_bytes()

    def boom(*_: object, **__: object) -> object:
        raise RuntimeError("boom")

    monkeypatch.setattr(api_checks, target, boom)
    assert api_checks.main(["check", "--root", str(root)]) == 1
    assert capsys.readouterr().err == "".join(
        f"{rule}: source: input not checkable (RuntimeError: boom)\n" for rule in rules
    )
    assert api_checks.main(["generate", "--root", str(root)]) == 1
    assert capsys.readouterr().err == (
        f"{TABLES_RULE}: source: input not renderable (RuntimeError: boom)\n{NOTHING_WRITTEN}\n"
    )
    assert (root / CHAPTER).read_bytes() == stale


@pytest.mark.parametrize(
    ("key", "op", "sub_check", "detail"),
    [
        (
            "op.identify",
            ["resume"],
            "ws-ops",
            "receive.identify: message 'identify' frames op ['resume'], expected ['identify']",
        ),
        (
            "op.identify",
            None,
            "ws-ops",
            "receive.identify: message 'identify' frames op None, expected ['identify']",
        ),
        (
            "event.file.updated",
            ["identify"],
            "ws-ops",
            "send.dispatch: message 'file.updated' frames op ['identify'], expected ['dispatch']",
        ),
        (
            "terminal.hello",
            ["identify"],
            "channel-ops",
            "terminal.send.hello: message 'hello' frames op ['identify'], expected ['hello']",
        ),
    ],
)
def test_binary_content_type_exempts_only_a_terminal_frame_without_op(
    tmp_path: Path, key: str, op: list[str] | None, sub_check: str, detail: str
) -> None:
    # The gateway knows JSON frames only (04 2.3); the Noise frame of a terminal channel is the
    # one frame without `op`, and a frame that declares `op` is checked whatever its type says.
    root = _copy_inputs(tmp_path)

    def change(d: dict[str, Any]) -> None:
        message = _message(d, key)
        message["contentType"] = "application/octet-stream"
        if op is None:
            message["payload"]["properties"].pop("op")
        else:
            message["payload"]["properties"]["op"]["enum"] = op

    _mutate(root, ASYNCAPI, change)
    assert _sync(root, sub_check) == [f"{SYNC_RULE}: {sub_check}: {detail}"]


INLINE = "is written inline; AsyncAPI 3 lists the messages of an operation by $ref"


@pytest.mark.parametrize("ref", [{}, {"$ref": None}], ids=["no-ref", "null-ref"])
def test_inline_message_in_an_operation_fails(tmp_path: Path, ref: dict[str, None]) -> None:
    # An inline copy in dispatch would render the events table without the events sub-check.
    root = _copy_inputs(tmp_path)

    def inline(d: dict[str, Any]) -> None:
        messages = _async_op(d, "send.dispatch")["messages"]
        index = messages.index({"$ref": "#/channels/gateway/messages/event.stats.live"})
        live = dict(_message(d, "event.stats.live"), **{"x-receive-action": "thread.read"})
        messages[index] = {**ref, **live}
        hello = {**ref, **_message(d, "terminal.hello")}
        _async_op(d, "terminal.send.hello")["messages"][0] = hello

    _mutate(root, ASYNCAPI, inline)
    findings = api_checks.check_catalogue_sync(root)
    assert f"{SYNC_RULE}: ws-ops: send.dispatch: message 'stats.live' {INLINE}" in findings
    assert f"{SYNC_RULE}: channel-ops: terminal.send.hello: message 'hello' {INLINE}" in findings
    assert _tables(root) == [f"{TABLES_RULE}: ws_ops: send.dispatch: message 'stats.live' {INLINE}"]
    with pytest.raises(api_checks.ApiCheckError, match="written inline"):
        api_checks.generate(root)
