# SPDX-License-Identifier: Apache-2.0
"""Tests for the action catalogue tool (ADR 0049, ticket M1-02)."""

from __future__ import annotations

import datetime
import functools
import itertools
import os
import re
import shutil
import signal
import subprocess
import sys
import time
from collections import Counter
from collections.abc import Callable, Mapping, Sequence
from html.parser import HTMLParser
from pathlib import Path
from typing import Any, SupportsIndex

import action_catalog
import markdown
import pytest
import table_import
import yaml
from action_catalog import CatalogError
from mkdocs.config import load_config
from mkdocs.utils import meta as mkdocs_meta

# ---- generated-block markers ------------------------------------------------------------

MARKDOWN = """# Title

Intro stays.
<!-- BEGIN GENERATED: one -->
| a |
|---|
<!-- END GENERATED: one -->
Between stays.
<!-- BEGIN GENERATED: two.x -->
<!-- END GENERATED: two.x -->
Tail stays.
"""


def test_generated_blocks_returns_body_between_marker_lines() -> None:
    blocks = action_catalog.generated_blocks(MARKDOWN)
    assert blocks == {"one": "| a |\n|---|\n", "two.x": ""}


def test_replace_generated_block_keeps_every_other_byte() -> None:
    out = action_catalog.replace_generated_block(MARKDOWN, "two.x", "| b |\n")
    assert out == MARKDOWN.replace(
        "<!-- BEGIN GENERATED: two.x -->\n",
        "<!-- BEGIN GENERATED: two.x -->\n| b |\n",
    )
    assert action_catalog.replace_generated_block(out, "two.x", "") == MARKDOWN


@pytest.mark.parametrize(
    ("text", "message"),
    [
        ("<!-- END GENERATED: a -->\n", "END without BEGIN"),
        ("<!-- BEGIN GENERATED: a -->\nrow\n", "BEGIN without END"),
        (
            "<!-- BEGIN GENERATED: a -->\n<!-- BEGIN GENERATED: b -->\n"
            "<!-- END GENERATED: b -->\n<!-- END GENERATED: a -->\n",
            "nested",
        ),
        (
            "<!-- BEGIN GENERATED: a -->\n<!-- END GENERATED: a -->\n"
            "<!-- BEGIN GENERATED: a -->\n<!-- END GENERATED: a -->\n",
            "duplicate",
        ),
        ("<!-- BEGIN GENERATED: a -->\n<!-- END GENERATED: b -->\n", "does not match"),
        ("<!-- BEGIN GENERATED: Not A Name -->\n<!-- END GENERATED: a -->\n", "malformed"),
    ],
)
def test_broken_markers_raise_catalog_error(text: str, message: str) -> None:
    with pytest.raises(CatalogError) as err:
        action_catalog.generated_blocks(text)
    assert any(message in f for f in err.value.findings), err.value.findings


def test_replace_generated_block_rejects_unknown_name_and_body_without_newline() -> None:
    with pytest.raises(CatalogError, match="no generated block"):
        action_catalog.replace_generated_block(MARKDOWN, "three", "x\n")
    with pytest.raises(CatalogError, match="end with a newline"):
        action_catalog.replace_generated_block(MARKDOWN, "one", "x")


YAML_TEXT = """rules:
  rule/x:
    assertions:
      enum: &anchor
        # BEGIN GENERATED: names (make catalog)
        - b
        - a
        # END GENERATED: names
    message: stays
"""


def test_yaml_blocks_read_items_and_indentation() -> None:
    assert action_catalog.yaml_generated_blocks(YAML_TEXT) == {"names": ("        ", ["b", "a"])}


def test_replace_yaml_block_writes_items_at_marker_indentation() -> None:
    out = action_catalog.replace_yaml_block(YAML_TEXT, "names", ["c"])
    assert out == YAML_TEXT.replace("        - b\n        - a\n", "        - c\n")


def test_yaml_markers_reject_malformed_and_unbalanced_lines() -> None:
    with pytest.raises(CatalogError, match="malformed"):
        action_catalog.yaml_generated_blocks(
            "  # BEGIN GENERATED: names\n  # END GENERATED: names\n"
        )
    with pytest.raises(CatalogError, match="BEGIN without END"):
        action_catalog.yaml_generated_blocks("  # BEGIN GENERATED: names (make catalog)\n")
    with pytest.raises(CatalogError, match="not a list item"):
        action_catalog.yaml_generated_blocks(
            "  # BEGIN GENERATED: names (make catalog)\n  key: v\n  # END GENERATED: names\n"
        )


# ---- rendering and the migration proof ---------------------------------------------------

FIXTURE = Path(__file__).parent / "fixtures" / "05-aktionen-before.md.txt"
SECTIONS = (
    "public",
    "instance",
    "identity",
    "grants",
    "structure",
    "threads",
    "messages",
    "files",
    "personal",
    "agents",
    "apps",
    "operations",
    "dev",
    "terminal",
)
CHAPTER_BLOCKS = {
    "rate_limit_classes",
    *(f"actions.{key}" for key in SECTIONS),
    "events.durable",
    "events.ephemeral",
    "ws_ops",
    "channel_ops",
}


def _fixture() -> str:
    return FIXTURE.read_text(encoding="utf-8")


def test_render_reproduces_pre_migration_tables() -> None:
    chapter = _fixture()
    catalog = action_catalog.catalog_from_data(table_import.import_chapter(chapter))
    rendered = action_catalog.render_chapter_blocks(catalog)
    tables = table_import.chapter_tables(chapter)
    assert set(rendered) == set(tables) == CHAPTER_BLOCKS
    for name in sorted(CHAPTER_BLOCKS):
        assert rendered[name] == tables[name], name


def test_scope_table_of_the_chapter() -> None:
    ids = [a["id"] for a in table_import.import_chapter(_fixture())["actions"]]
    scopes = action_catalog.scope_assignments(_fixture(), ids)
    assert scopes["reaction.add"] == scopes["reaction.remove"] == "messages:write"
    assert scopes["gateway.connect"] == scopes["events.catch_up"] == "events:subscribe"
    assert scopes["self.read"] == scopes["webhook_in.post"] == "none"
    assert scopes["cost.reconcile.submit"] == "costs:reconcile"
    assert "message.revisions.read" not in scopes
    assert "terminal.open" not in scopes


def test_effect_list_of_the_chapter() -> None:
    ids = [a["id"] for a in table_import.import_chapter(_fixture())["actions"]]
    read = action_catalog.read_effect_ids(_fixture(), ids)
    for action in (
        "cost.read.thread",
        "cost.read.aggregate",
        "device.list",
        "instance.discovery.read",
        "terminal.read_own",
        "typing.start",
        "push.deliver",
        "webhook_out.deliver",
        "command.autocomplete.respond",
    ):
        assert action in read, action
    for action in ("audit.read", "message.create", "access.preview", "terminal.open"):
        assert action not in read, action


# ---- repository files, generate, check, CLI ----------------------------------------------

REPO = Path(__file__).resolve().parents[3]
CATALOG = Path("docs/spec/actions.yaml")
CHAPTER = Path("docs/spec/05-aktionen.md")
REDOCLY = Path("docs/api/redocly.yaml")


def _copy_repo(tmp_path: Path) -> Path:
    """Copy the three files of the catalogue into tmp_path with the same relative paths."""
    for rel in (CATALOG, CHAPTER, REDOCLY):
        target = tmp_path / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes((REPO / rel).read_bytes())
    return tmp_path


def _edit(root: Path, rel: Path, old: str, new: str, count: int = 1) -> None:
    path = root / rel
    text = path.read_text(encoding="utf-8")
    assert old in text, old
    path.write_text(text.replace(old, new, count), encoding="utf-8")


def _findings_for(root: Path, rule: str) -> list[str]:
    return [f for f in action_catalog.check(root) if f": {rule}:" in f]


def test_repository_catalog_loads() -> None:
    catalog = action_catalog.load_catalog(REPO / CATALOG)
    assert catalog.version == 1
    assert catalog.sections == SECTIONS
    assert catalog.actions_by_id["message.read"].rl_class == "rl.read"
    assert catalog.actions_by_id["grant.expire"].system
    assert catalog.events_by_type["access.changed"].audience == "self"
    assert catalog.ws_ops_by_name["dispatch"].per_event_receive_action
    assert set(catalog.classes_by_id) == {
        a.rl_class for a in catalog.actions if a.rl_class != "none"
    }


def test_repository_blocks_are_current() -> None:
    assert action_catalog.check(REPO) == []


def test_generate_is_idempotent(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    assert action_catalog.generate(root) == []  # the repository is current
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    _edit(root, REDOCLY, "        - access.explain\n", "")
    assert action_catalog.generate(root) == [CHAPTER.as_posix(), REDOCLY.as_posix()]
    assert action_catalog.generate(root) == []
    for rel in (CHAPTER, REDOCLY):
        assert (root / rel).read_bytes() == (REPO / rel).read_bytes()


def test_edited_chapter_block_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root, CHAPTER, "| `user.create` | Menschen anlegen", "| `user.create` | Menschen erzeugen"
    )
    [finding] = _findings_for(root, "generated-block")
    assert finding.startswith("docs/spec/05-aktionen.md: generated-block: actions.identity:")
    assert "run make catalog" in finding


def test_edited_redocly_list_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, "        - rl.admin\n", "        - rl.admin\n        - rl.unknown\n")
    [finding] = _findings_for(root, "generated-block")
    assert finding.startswith("docs/api/redocly.yaml: generated-block: rate_limit_classes:")


def test_unknown_block_name_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "## 8. Entscheidungen dieses Kapitels\n",
        "<!-- BEGIN GENERATED: actions.unknown -->\n<!-- END GENERATED: actions.unknown -->\n\n"
        "## 8. Entscheidungen dieses Kapitels\n",
    )
    assert any("unknown block 'actions.unknown'" in f for f in action_catalog.check(root))


def test_missing_block_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "<!-- BEGIN GENERATED: ws_ops -->\n", "")
    _edit(root, CHAPTER, "<!-- END GENERATED: ws_ops -->\n", "")
    assert any("missing block 'ws_ops'" in f for f in action_catalog.check(root))


def test_unbalanced_markers_fail(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "<!-- END GENERATED: actions.files -->\n", "")
    [finding] = _findings_for(root, "generated-block")
    assert finding.startswith("docs/spec/05-aktionen.md: generated-block:")
    assert "nested" in finding or "without END" in finding


def test_redocly_lists_exclude_system_actions() -> None:
    catalog = action_catalog.load_catalog(REPO / CATALOG)
    blocks = action_catalog.yaml_generated_blocks((REPO / REDOCLY).read_text(encoding="utf-8"))
    system = {a.id for a in catalog.actions if a.system}
    others = {a.id for a in catalog.actions if not a.system}
    assert system, "the catalogue has system actions"
    for name in ("authz_actions", "receive_actions"):
        items = set(blocks[name][1])
        assert not items & system, name
        assert others <= items, name
    assert set(blocks["receive_actions"][1]) - others == {"self"}
    assert blocks["rate_limit_classes"][1][0] == "none"


def test_generate_writes_nothing_when_any_block_is_broken(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    _edit(root, REDOCLY, "        # END GENERATED: authz_actions\n", "")
    stale = (root / CHAPTER).read_bytes()
    with pytest.raises(CatalogError):
        action_catalog.generate(root)
    assert (root / CHAPTER).read_bytes() == stale


def test_cli_exit_codes(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    root = _copy_repo(tmp_path)
    assert action_catalog.main(["check", "--root", str(root)]) == 0
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    assert action_catalog.main(["check", "--root", str(root)]) == 1
    assert "generated-block" in capsys.readouterr().err
    _edit(root, CATALOG, "version: 1\n", "version: [\n")
    stale = (root / CHAPTER).read_bytes()
    assert action_catalog.main(["generate", "--root", str(root)]) == 1
    assert (root / CHAPTER).read_bytes() == stale
    assert "nothing written" in capsys.readouterr().err
    _edit(root, CATALOG, "version: [\n", "version: 1\n")
    assert action_catalog.main(["generate", "--root", str(root)]) == 0
    assert "rewrote docs/spec/05-aktionen.md" in capsys.readouterr().out
    assert action_catalog.main(["check", "--root", str(root)]) == 0


# ---- rules that need only the YAML -------------------------------------------------------


def _mutate(root: Path, change: Callable[[dict[str, Any]], object]) -> None:
    """Load the copied actions.yaml, apply `change` to the data, write it back."""
    path = root / CATALOG
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    change(data)
    path.write_text(
        yaml.safe_dump(data, sort_keys=False, allow_unicode=True, width=100), encoding="utf-8"
    )


def _entry(data: dict[str, Any], key: str, field: str, value: str) -> dict[str, Any]:
    return next(e for e in data[key] if e.get(field) == value)


def _catalog_findings(root: Path, rule: str) -> list[str]:
    return [f for f in action_catalog.check(root) if f.startswith(f"actions.yaml: {rule}:")]


def _assert_finding(root: Path, rule: str, needle: str) -> None:
    findings = _catalog_findings(root, rule)
    assert any(needle in f for f in findings), action_catalog.check(root)


def test_duplicate_action_id_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.update").update(id="user.create"))
    _assert_finding(root, "unique", "duplicate action id 'user.create'")


def test_unknown_rate_limit_class_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.read").update(rl_class="rl.unknown"))
    _assert_finding(root, "rate-limit-class", "unknown class 'rl.unknown'")


def test_unused_rate_limit_class_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root,
        lambda d: d["rate_limit_classes"].append(
            {"id": "rl.unused", "key": "Prinzipal", "purpose": "nie benutzt"}
        ),
    )
    _assert_finding(root, "rate-limit-class", "class 'rl.unused' is used by no action")


def test_none_class_only_for_system_or_delivery_check(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.read").update(rl_class="none"))
    _assert_finding(root, "rate-limit-class", "user.read: class none")
    root = _copy_repo(tmp_path / "second")
    _mutate(root, lambda d: _entry(d, "actions", "id", "grant.expire").update(rl_class="rl.admin"))
    _assert_finding(root, "rate-limit-class", "grant.expire: needs class none")


def test_event_without_receive_action_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "events", "type", "message.created").pop("receive_action"))
    _assert_finding(root, "event", "message.created: event type without receive action")


def test_event_receive_system_action_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root,
        lambda d: _entry(d, "events", "type", "grant.changed").update(
            receive_action="grant.expire"
        ),
    )
    _assert_finding(
        root, "event", "grant.changed: receive action 'grant.expire' is a system action"
    )


def test_self_receive_requires_self_audience(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "events", "type", "access.changed").pop("audience"))
    _assert_finding(root, "event", "access.changed: receive action self needs audience self")


def test_receive_text_must_name_receive_action(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root,
        lambda d: _entry(d, "events", "type", "agent.invocation").update(
            receive_text="self (Agent) und `agent.read`"
        ),
    )
    _assert_finding(root, "event", "agent.invocation: receive_text does not name")


def test_op_without_action_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "ws_ops", "op", "ack").pop("action"))
    _assert_finding(root, "op", "ack: op without action")


def test_op_with_system_action_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "ws_ops", "op", "ack").update(action="device.expire"))
    _assert_finding(root, "op", "ack: action 'device.expire' is a system action")


def test_row_group_must_be_consecutive(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root,
        lambda d: _entry(d, "ws_ops", "op", "identify").update(
            row_group="session",
            direction="server_to_client",
            action_text="keine eigene; Teil der Sitzung aus `gateway.connect`",
        ),
    )
    _assert_finding(root, "op", "row group 'session' is not consecutive")


def test_unknown_key_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.read").update(colour="rot"))
    _assert_finding(root, "schema", "unknown key 'colour'")


def test_wrong_version_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: d.update(version=2))
    _assert_finding(root, "schema", "version must be 1")


def test_durable_event_without_resource_version_flag_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "events", "type", "message.created").pop("resource_version"))
    _assert_finding(root, "schema", "message.created: durable event needs resource_version")


def test_duplicate_yaml_key_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CATALOG, "- id: user.read\n", "- id: user.read\n  id: user.read\n")
    _assert_finding(root, "schema", "duplicate key 'id'")


@pytest.mark.parametrize(
    ("field", "value", "needle"),
    [
        ("rights_change", 1, "rights_change must be a boolean"),
        ("owner", "ja", "owner must be one of"),
        ("step_up", "yes", "step_up must be one of"),
        ("effect", "both", "effect must be one of"),
        ("description", 7, "description must be a non-empty string"),
        ("app_scope", "messages:all", "app_scope must be one of"),
    ],
)
def test_wrong_scalar_type_fails(tmp_path: Path, field: str, value: object, needle: str) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.read").update({field: value}))
    _assert_finding(root, "schema", needle)


def test_boolean_version_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: d.update(version=True))
    _assert_finding(root, "schema", "version must be 1")


@pytest.mark.parametrize("value", ["a | b", "eine\nzweite Zeile"])
def test_cell_with_pipe_or_newline_fails(tmp_path: Path, value: str) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.read").update(description=value))
    _assert_finding(root, "schema", "user.read: description (column Beschreibung): ")


@pytest.mark.parametrize(
    ("text", "needle"),
    [("", "top level must be a mapping"), ("- a\n", "top level must be a mapping")],
)
def test_empty_or_non_mapping_yaml_fails(tmp_path: Path, text: str, needle: str) -> None:
    root = _copy_repo(tmp_path)
    (root / CATALOG).write_text(text, encoding="utf-8")
    _assert_finding(root, "schema", needle)


def _move_first_key_last(d: dict[str, Any]) -> None:
    d["version"] = d.pop("version")


def _add_top_key(d: dict[str, Any]) -> None:
    d["extra"] = []


def _drop_top_key(d: dict[str, Any]) -> None:
    del d["channel_ops"]


def _audit_text_without_conditional(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "user.read")["audit_text"] = "ja bei Bedarf"


def _conditional_without_audit_text(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "message.delete").pop("audit_text")


def _bad_id(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "user.read")["id"] = "User.Read"


def _ephemeral_event_with_bad_flag(d: dict[str, Any]) -> None:
    _entry(d, "events", "type", "run.updated")["resource_version"] = "ja"


def _ops_not_a_list(d: dict[str, Any]) -> None:
    _entry(d, "channel_ops", "channel", "host_data")["ops"] = "hello"


def _class_id_without_prefix(d: dict[str, Any]) -> None:
    d["rate_limit_classes"][0]["id"] = "public_auth"


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_move_first_key_last, "top-level keys must be in the order"),
        (_add_top_key, "unknown key 'extra'"),
        (_drop_top_key, "missing key 'channel_ops'"),
        (_audit_text_without_conditional, "audit_text only with audit conditional"),
        (_conditional_without_audit_text, "message.delete: audit conditional needs audit_text"),
        (_bad_id, "id 'User.Read' does not match"),
        (_ephemeral_event_with_bad_flag, "resource_version must be a boolean"),
        (_ops_not_a_list, "ops must be a non-empty list"),
        (_class_id_without_prefix, "class id 'public_auth' does not match"),
    ],
)
def test_schema_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "schema", needle)


def _duplicate_event(d: dict[str, Any]) -> None:
    d["events"].append(dict(_entry(d, "events", "type", "message.created")))


def _duplicate_op(d: dict[str, Any]) -> None:
    d["ws_ops"].append(dict(_entry(d, "ws_ops", "op", "ack")))


def _duplicate_class(d: dict[str, Any]) -> None:
    d["rate_limit_classes"].append(dict(d["rate_limit_classes"][0]))


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_duplicate_event, "duplicate event type 'message.created'"),
        (_duplicate_op, "duplicate op 'ack'"),
        (_duplicate_class, "duplicate class id 'rl.public_auth'"),
    ],
)
def test_uniqueness_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "unique", needle)


def _unknown_section(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "user.read")["section"] = "people"


def _empty_section(d: dict[str, Any]) -> None:
    d["sections"].append("later")


def _section_out_of_order(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "user.read")["section"] = "terminal"


def _duplicate_section(d: dict[str, Any]) -> None:
    d["sections"].append("public")


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_unknown_section, "user.read: section 'people' is not listed"),
        (_empty_section, "section 'later' has no action"),
        (_section_out_of_order, "section 'identity' after 'terminal' breaks the order"),
        (_duplicate_section, "section 'public' is listed twice"),
    ],
)
def test_section_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "section", needle)


def _delivery_check_on_system_action(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "grant.expire")["delivery_check"] = True


@pytest.mark.parametrize(
    ("change", "needle"),
    [(_delivery_check_on_system_action, "grant.expire: delivery_check on a system action")],
)
def test_class_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "rate-limit-class", needle)


def _unknown_receive_action(d: dict[str, Any]) -> None:
    _entry(d, "events", "type", "message.created")["receive_action"] = "message.view"


def _receivers_without_audience(d: dict[str, Any]) -> None:
    _entry(d, "events", "type", "message.created")["receivers"] = "agent"


def _self_cell_without_audience(d: dict[str, Any]) -> None:
    event = _entry(d, "events", "type", "notification.created")
    event.pop("audience")


def _audience_without_self_cell(d: dict[str, Any]) -> None:
    _entry(d, "events", "type", "message.created")["audience"] = "self"


def _self_text_without_self(d: dict[str, Any]) -> None:
    event = _entry(d, "events", "type", "access.changed")
    event["receive_text"] = "nur der Prinzipal"


def _ephemeral_before_durable(d: dict[str, Any]) -> None:
    d["events"].insert(0, d["events"].pop())


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_unknown_receive_action, "message.created: unknown receive action 'message.view'"),
        (_receivers_without_audience, "message.created: receivers only with audience self"),
        (_self_cell_without_audience, "notification.created: receive cell starts with self"),
        (_audience_without_self_cell, "message.created: audience self needs a receive cell"),
        (_self_text_without_self, "access.changed: receive_text does not name"),
        (_ephemeral_before_durable, "durable event types must come before ephemeral ones"),
    ],
)
def test_event_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "event", needle)


def _unknown_op_action(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "ack")["action"] = "agent.ack"


def _unknown_also_action(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "heartbeat")["also_actions"] = ["agent.pulse"]


def _row_group_with_other_action(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "stats.unsubscribe")["action"] = "status.read"


def _per_event_on_other_op(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "ack")["per_event_receive_action"] = True


def _action_text_without_action(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "focus")["action_text"] = "je Thread"


def _action_text_without_also_action(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "heartbeat")["action_text"] = "`gateway.connect` (Sitzung gültig)"


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_unknown_op_action, "ack: unknown action 'agent.ack'"),
        (_unknown_also_action, "heartbeat: unknown action 'agent.pulse'"),
        (
            _row_group_with_other_action,
            "row group 'stats' mixes direction, action, action_text, also_actions or"
            " per_event_receive_action",
        ),
        (_per_event_on_other_op, "ack: per_event_receive_action only on dispatch"),
        (_action_text_without_action, "focus: action_text does not name 'gateway.focus'"),
        (
            _action_text_without_also_action,
            "heartbeat: action_text does not name 'agent.heartbeat'",
        ),
    ],
)
def test_op_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "op", needle)


def _channel_rows(d: dict[str, Any], channel: str) -> list[dict[str, Any]]:
    return [c for c in d["channel_ops"] if c["channel"] == channel]


def _channel_op_without_action(d: dict[str, Any]) -> None:
    _channel_rows(d, "host_data")[0].pop("action")


def _channel_op_system_action(d: dict[str, Any]) -> None:
    _channel_rows(d, "host_data")[0]["action"] = "terminal.terminate"


def _channel_not_consecutive(d: dict[str, Any]) -> None:
    d["channel_ops"].append(d["channel_ops"].pop(1))


def _channel_text_on_continuation(d: dict[str, Any]) -> None:
    _channel_rows(d, "host_control")[1]["channel_text"] = "Steuerung"


def _channel_without_text(d: dict[str, Any]) -> None:
    _channel_rows(d, "host_data")[0].pop("channel_text")


def _channel_op_twice(d: dict[str, Any]) -> None:
    _channel_rows(d, "host_control")[0]["ops"].append("hello")


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_channel_op_without_action, "host_data: channel op without action"),
        (_channel_op_system_action, "host_data: action 'terminal.terminate' is a system action"),
        (_channel_not_consecutive, "rows of channel 'terminal_stream' are not consecutive"),
        (_channel_text_on_continuation, "host_control: channel_text only on the first row"),
        (_channel_without_text, "host_data: first row of a channel needs channel_text"),
        (_channel_op_twice, "host_control: op 'hello' twice for direction server_to_host"),
    ],
)
def test_channel_op_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "channel-op", needle)


# ---- cross-checks with the hand-written parts of chapter 05 -------------------------------

HOST_ROW = "| `ratelimit.rl.host.per_minute` / `.burst` | `120` / `30` | je Rechner |\n"


def test_missing_ratelimit_config_row_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, HOST_ROW, "")
    _assert_finding(root, "rate-limit-config", "class 'rl.host' has no row")


@pytest.mark.parametrize(
    ("row", "needle"),
    [
        ("| `ratelimit.rl.ghost.per_minute` / `.burst` | `1` / `1` | je Geist |\n", "'rl.ghost'"),
        ("| `ratelimit.rl.host.per_minute` | `120` | je Rechner |\n", "malformed rate-limit row"),
        (HOST_ROW, "class 'rl.host' has more than one row"),
    ],
)
def test_ratelimit_config_rows_must_match_classes(tmp_path: Path, row: str, needle: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, HOST_ROW, HOST_ROW + row)
    _assert_finding(root, "rate-limit-config", needle)


def test_app_scope_mismatch_with_scope_table_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "file.read").update(app_scope="files:write"))
    _assert_finding(root, "app-scope", "file.read: app_scope 'files:write' differs from 05 2.6")


def test_scope_table_names_unknown_action_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "| `file.read` | `files:read` |",
        "| `file.read`, `file.view` | `files:read` |",
    )
    _assert_finding(root, "app-scope", "05 2.6 names unknown action 'file.view'")


def test_app_scope_without_scope_row_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root,
        lambda d: _entry(d, "actions", "id", "message.revisions.read").update(
            app_scope="messages:read"
        ),
    )
    _assert_finding(root, "app-scope", "message.revisions.read: app_scope 'messages:read' but")


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        ("| `file.read` | `files:read` |", "| `file.read`, `user.read` | `files:read` |", "twice"),
        ("| `file.read` | `files:read` |", "| `file.read` | `files:all` |", "unknown scope"),
        ("| `file.read` | `files:read` |", "| `nothing.*` | `files:read` |", "matches no action"),
    ],
)
def test_scope_table_rows_must_be_well_formed(
    tmp_path: Path, old: str, new: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, old, new)
    _assert_finding(root, "app-scope", needle)


def test_effect_mismatch_with_effect_list_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "message.read").update(effect="write"))
    _assert_finding(root, "effect", "message.read: effect write differs from 05 2.7 (read)")


def test_effect_list_names_unknown_action_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "`push.deliver`, `webhook_out.deliver`", "`push.deliver`, `push.send`")
    _assert_finding(root, "effect", "05 2.7 names unknown action 'push.send'")


def test_missing_hand_written_section_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "### 2.6 Scopes für Apps\n", "### 2.6 Scopes\n")
    _assert_finding(root, "app-scope", "section 'Scopes für Apps' not found")


def test_generate_refuses_a_catalogue_that_contradicts_chapter_05(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "message.read").update(effect="write"))
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    stale = (root / CHAPTER).read_bytes()
    with pytest.raises(CatalogError, match="effect"):
        action_catalog.generate(root)
    assert (root / CHAPTER).read_bytes() == stale


# ---- error paths of the loader and the public parsers -------------------------------------


def _action_not_a_mapping(d: dict[str, Any]) -> None:
    d["actions"][0] = "user.read"


def _actions_not_a_list(d: dict[str, Any]) -> None:
    d["actions"] = {"user.read": {}}


def _sections_empty(d: dict[str, Any]) -> None:
    d["sections"] = []


def _also_action_bad(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "heartbeat")["also_actions"] = ["Agent Heartbeat"]


def _action_without_right(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "user.read").pop("right")


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_action_not_a_mapping, "actions[0]: must be a mapping"),
        (_actions_not_a_list, "actions must be a list"),
        (_sections_empty, "sections must be a non-empty list"),
        (_also_action_bad, "also_actions item 'Agent Heartbeat' does not match"),
        (_action_without_right, "user.read: missing key 'right'"),
    ],
)
def test_structural_schema_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "schema", needle)


def test_missing_chapter_file_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    (root / CHAPTER).unlink()
    [finding] = action_catalog.check(root)
    assert finding.startswith("docs/spec/05-aktionen.md: generated-block: cannot read:")


def test_public_parsers_raise_on_broken_tables() -> None:
    chapter = _fixture()
    ids = [a["id"] for a in table_import.import_chapter(chapter)["actions"]]
    broken_scope = chapter.replace(
        "| `file.read` | `files:read` |", "| `file.view` | `files:read` |"
    )
    with pytest.raises(CatalogError, match=r"unknown action 'file\.view'"):
        action_catalog.scope_assignments(broken_scope, ids)
    broken_effect = chapter.replace("`push.deliver`, `webhook_out.deliver`", "`push.send`")
    with pytest.raises(CatalogError, match=r"unknown action 'push\.send'"):
        action_catalog.read_effect_ids(broken_effect, ids)
    with pytest.raises(CatalogError, match="no generated block"):
        action_catalog.replace_yaml_block(YAML_TEXT, "other", ["x"])


# ---- gate wiring --------------------------------------------------------------------------


def _make_rule(makefile: str, target: str) -> list[str]:
    """Prerequisites of every `target:` line of the Makefile, continuation lines joined."""
    joined = makefile.replace("\\\n", " ")
    prerequisites: list[str] = []
    for line in joined.splitlines():
        if line.startswith(f"{target}:"):
            prerequisites += line.split(":", 1)[1].split("##", 1)[0].split()
    return prerequisites


def test_make_gates_run_catalog_check_before_api_lint() -> None:
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    gates = _make_rule(makefile, "gates")
    assert gates.index("catalog-check") + 1 == gates.index("api-lint")
    phony = _make_rule(makefile, ".PHONY")
    assert {"catalog", "catalog-check"} <= set(phony)
    assert "tools/catalog/action_catalog.py generate" in makefile
    assert "tools/catalog/action_catalog.py check" in makefile
    bandit = next(line for line in makefile.splitlines() if "bandit -q -c" in line)
    assert " tools/catalog " in bandit


# ---- red team: entries next to the generated blocks ---------------------------------------


@pytest.mark.parametrize(
    ("old", "new", "rule"),
    [
        (
            "        # END GENERATED: authz_actions\n",
            "        # END GENERATED: authz_actions\n        - grant.expire\n",
            "rule/authz-action-known",
        ),
        (
            "        # BEGIN GENERATED: receive_actions (make catalog)\n",
            "        - instance.bootstrap\n"
            "        # BEGIN GENERATED: receive_actions (make catalog)\n",
            "rule/receive-action-known-message",
        ),
        (
            "        # END GENERATED: rate_limit_classes\n",
            "        # END GENERATED: rate_limit_classes\n        - rl.bogus\n",
            "rule/rate-limit-class-known",
        ),
        (
            "      enum: *receive_actions\n",
            "      enum: [grant.expire, self]\n",
            "rule/receive-action-known-webhook",
        ),
    ],
)
def test_redocly_entry_outside_the_generated_block_fails(
    tmp_path: Path, old: str, new: str, rule: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, old, new)
    findings = _findings_for(root, "generated-block")
    assert any(
        f.startswith(f"docs/api/redocly.yaml: generated-block: {rule}:") for f in findings
    ), findings
    with pytest.raises(CatalogError, match=rule):
        action_catalog.generate(root)


def test_redocly_api_rule_override_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        REDOCLY,
        "      security-defined: off\n",
        "      security-defined: off\n      rule/authz-action-known: off\n",
    )
    assert any(
        "apis[gateway@v1].rules overrides rule/authz-action-known" in f
        for f in _findings_for(root, "generated-block")
    )


def test_table_row_outside_the_generated_blocks_of_chapter_05_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: ws_ops -->\n",
        "<!-- END GENERATED: ws_ops -->\n| `shell.exec` | Client an Server | `host.read` |\n",
    )
    assert any(
        "table row outside the generated blocks" in f and "WebSocket-Ops und Aktion" in f
        for f in _findings_for(root, "generated-block")
    )
    with pytest.raises(CatalogError, match="table row outside"):
        action_catalog.generate(root)


def test_cell_with_surrounding_whitespace_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "grant.expire").update(right="system "))
    _assert_finding(root, "schema", "grant.expire: right must not start or end with whitespace")


def test_right_that_mentions_system_but_is_not_system_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "grant.expire").update(right="system (Job)"))
    _assert_finding(root, "schema", "grant.expire: right names system but is not exactly system")


# ---- review findings ----------------------------------------------------------------------


@pytest.mark.parametrize("key", ["events", "ws_ops", "channel_ops", "actions"])
def test_null_list_fails(tmp_path: Path, key: str) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: d.update({key: None}))
    _assert_finding(root, "schema", f"{key} must be a list")


def _also_actions_without_text(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "heartbeat").pop("action_text")


def _per_event_without_text(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "dispatch").pop("action_text")


def _row_group_with_other_also_actions(d: dict[str, Any]) -> None:
    # Same action_text on both members; only also_actions differs.
    for op in ("stats.subscribe", "stats.unsubscribe"):
        _entry(d, "ws_ops", "op", op)["action_text"] = (
            "`stats.read` auf Instanz bzw. Bereich; `agent.heartbeat`"
        )
    _entry(d, "ws_ops", "op", "stats.unsubscribe")["also_actions"] = ["agent.heartbeat"]


def _row_group_with_other_per_event_flag(d: dict[str, Any]) -> None:
    # dispatch joins the session row with the same text; only per_event_receive_action differs.
    dispatch = _entry(d, "ws_ops", "op", "dispatch")
    dispatch["row_group"] = "session"
    dispatch["action_text"] = _entry(d, "ws_ops", "op", "error")["action_text"]


def _action_text_names_action_without_backticks(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "focus")["action_text"] = "gateway.focus je Thread"


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_also_actions_without_text, "heartbeat: also_actions need action_text"),
        (_per_event_without_text, "dispatch: per_event_receive_action needs action_text"),
        (_row_group_with_other_also_actions, "row group 'stats' mixes direction, action,"),
        (_row_group_with_other_per_event_flag, "row group 'session' mixes direction, action,"),
        (_action_text_names_action_without_backticks, "focus: action_text does not name"),
    ],
)
def test_more_op_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "op", needle)


def _receive_text_without_backticks(d: dict[str, Any]) -> None:
    _entry(d, "events", "type", "agent.invocation")["receive_text"] = (
        "self (Agent) und agent.events.consume"
    )


def _self_only_inside_a_word(d: dict[str, Any]) -> None:
    _entry(d, "events", "type", "access.changed")["receive_text"] = "selfish"


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_receive_text_without_backticks, "agent.invocation: receive_text does not name"),
        (_self_only_inside_a_word, "access.changed: receive_text does not name self"),
    ],
)
def test_receive_text_must_name_the_action_exactly(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "event", needle)


def _channel_op_unknown_action(d: dict[str, Any]) -> None:
    _channel_rows(d, "host_data")[1]["action"] = "host.view"


def _channel_action_text_without_action(d: dict[str, Any]) -> None:
    _channel_rows(d, "host_data")[0]["action_text"] = "(Sitzung gehört dem Rechner)"


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_channel_op_unknown_action, "host_data: unknown action 'host.view'"),
        (_channel_action_text_without_action, "host_data: action_text does not name"),
    ],
)
def test_more_channel_op_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "channel-op", needle)


def test_effect_list_matches_the_word_ausser_only(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "- `access.explain`, `search.query`,",
        "- außerhalb der Endungen: `access.explain`, `search.query`,",
    )
    assert _catalog_findings(root, "effect") == []


def test_effect_list_unknown_excluded_action_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "außer `audit.read` (Audit `ja`)", "außer `audit.view` (Audit `ja`)")
    _assert_finding(root, "effect", "05 2.7 names unknown action 'audit.view'")


@pytest.mark.parametrize(
    "value", ["eine\rzweite", "eine\N{LINE SEPARATOR}zweite", "eine\x0czweite"]
)
def test_cell_with_other_line_breaks_fails(tmp_path: Path, value: str) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.read").update(description=value))
    _assert_finding(
        root, "schema", "user.read: description (column Beschreibung): holds a line break"
    )


def _set_class(field: str) -> Callable[[dict[str, Any]], None]:
    return lambda d: d["rate_limit_classes"][0].update({field: "a | b"})


def _set_action(field: str) -> Callable[[dict[str, Any]], None]:
    return lambda d: _entry(d, "actions", "id", "message.delete").update({field: "a | b"})


def _set_event(field: str) -> Callable[[dict[str, Any]], None]:
    return lambda d: _entry(d, "events", "type", "agent.config_changed").update({field: "a | b"})


def _set_ws_op(field: str) -> Callable[[dict[str, Any]], None]:
    return lambda d: _entry(d, "ws_ops", "op", "focus").update({field: "a | b"})


def _set_channel_op(field: str) -> Callable[[dict[str, Any]], None]:
    return lambda d: _channel_rows(d, "host_data")[0].update({field: "a | b"})


@pytest.mark.parametrize(
    ("change", "field"),
    [
        (_set_class("key"), "key (column Schlüssel)"),
        (_set_class("purpose"), "purpose (column Zweck)"),
        (_set_action("description"), "description (column Beschreibung)"),
        (_set_action("resource"), "resource (column Ressource)"),
        (_set_action("right"), "right (column Recht)"),
        (_set_action("audit_text"), "audit_text (column Audit)"),
        (_set_event("resource"), "resource (column Ressource)"),
        (_set_event("receive_text"), "receive_text (column Empfangs-Aktion)"),
        (_set_event("note"), "note (column Hinweis)"),
        (
            lambda d: _entry(d, "events", "type", "typing.started").update(note="a | b"),
            "note (column Zusatzbedingung)",
        ),
        (_set_ws_op("action_text"), "action_text (column Aktion)"),
        (_set_channel_op("channel_text"), "channel_text (column Kanal)"),
        (_set_channel_op("action_text"), "action_text (column Aktion)"),
    ],
)
def test_every_table_cell_rejects_an_unescaped_pipe(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], field: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "schema", f"{field}: the | at character 3 ends the cell on the site")


def _bad_op_name(d: dict[str, Any]) -> None:
    _entry(d, "ws_ops", "op", "identify")["op"] = "Identify Now"


def _bad_row_group(d: dict[str, Any]) -> None:
    for op in ("stats.subscribe", "stats.unsubscribe"):
        _entry(d, "ws_ops", "op", op)["row_group"] = "Stats Group"


def _bad_section_key(d: dict[str, Any]) -> None:
    _entry(d, "actions", "id", "user.read")["section"] = "People"


def _bad_sections_entry(d: dict[str, Any]) -> None:
    d["sections"].append(5)


def _ephemeral_without_flag(d: dict[str, Any]) -> None:
    _entry(d, "events", "type", "typing.started").pop("resource_version")


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (_bad_op_name, "op 'Identify Now' does not match"),
        (_bad_row_group, "row_group 'Stats Group' does not match"),
        (_bad_section_key, "section 'People' does not match"),
        (_bad_sections_entry, "sections: 5 does not match"),
        (_ephemeral_without_flag, "typing.started: ephemeral event needs resource_version"),
    ],
)
def test_more_schema_rule_violations_fail(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "schema", needle)


@pytest.mark.parametrize(("field", "value"), [("owner", 1), ("step_up", 0), ("owner", 0)])
def test_integer_never_passes_for_a_boolean_choice(tmp_path: Path, field: str, value: int) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "user.read").update({field: value}))
    _assert_finding(root, "schema", f"user.read: {field} must be one of")


def test_receivers_default_to_principal_for_audience_self() -> None:
    catalog = action_catalog.load_catalog(REPO / CATALOG)
    assert catalog.events_by_type["access.changed"].receivers == "principal"
    assert catalog.events_by_type["notification.created"].receivers == "principal"
    assert catalog.events_by_type["agent.invocation"].receivers == "agent"
    assert catalog.events_by_type["command.invoked"].receivers == "agent_or_app"
    assert catalog.events_by_type["message.created"].receivers is None


def test_run_updated_carries_resource_version() -> None:
    catalog = action_catalog.load_catalog(REPO / CATALOG)
    assert catalog.events_by_type["run.updated"].resource_version
    assert not catalog.events_by_type["typing.started"].resource_version


def test_generate_refuses_an_unknown_block(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "## 8. Entscheidungen dieses Kapitels\n",
        "<!-- BEGIN GENERATED: actions.unknown -->\n<!-- END GENERATED: actions.unknown -->\n\n"
        "## 8. Entscheidungen dieses Kapitels\n",
    )
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    stale = (root / CHAPTER).read_bytes()
    with pytest.raises(CatalogError, match="unknown block"):
        action_catalog.generate(root)
    assert (root / CHAPTER).read_bytes() == stale


@pytest.mark.parametrize("rel", [CATALOG, CHAPTER, REDOCLY])
def test_invalid_utf8_gives_a_finding(tmp_path: Path, rel: Path) -> None:
    root = _copy_repo(tmp_path)
    (root / rel).write_bytes(b"\xff\xfe" + (root / rel).read_bytes())
    findings = action_catalog.check(root)
    assert any("cannot read" in f for f in findings), findings
    assert action_catalog.main(["generate", "--root", str(root)]) == 1


def test_redocly_lists_are_exactly_the_three_named_blocks() -> None:
    catalog = action_catalog.load_catalog(REPO / CATALOG)
    assert set(action_catalog.redocly_lists(catalog)) == {
        "authz_actions",
        "rate_limit_classes",
        "receive_actions",
    }


# ---- delta review: lint rule shape, table forms, single finding ---------------------------


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        (
            "    message: x-authz-action is not in the action catalogue.\n",
            "    message: x-authz-action is not in the action catalogue.\n    severity: off\n",
            "rule/authz-action-known: unexpected key 'severity'",
        ),
        (
            "      property: x-rate-limit-class\n    assertions:\n      enum:\n",
            "      property: x-rate-limit-class-unused\n    assertions:\n      enum:\n",
            "rule/rate-limit-class-known: subject must be",
        ),
        (
            "  rule/receive-action-known-message:\n    subject:\n      type: Message\n",
            "  rule/receive-action-known-message:\n    subject:\n      type: Schema\n",
            "rule/receive-action-known-message: subject must be",
        ),
        (
            "  rule/receive-action-known-webhook:\n    subject:\n",
            "  rule/receive-action-known-webhook:\n    where:\n      - subject:\n"
            "          type: Operation\n          property: summary\n"
            "        assertions:\n          defined: true\n    subject:\n",
            "rule/receive-action-known-webhook: unexpected key 'where'",
        ),
        (
            "      enum: *receive_actions\n",
            "      enum: *receive_actions\n      notPattern: /x/\n",
            "rule/receive-action-known-webhook: assertions must hold only enum",
        ),
        (
            "  # Conservative schemas for code generators",
            "  rule/authz-action-known: off\n  # Conservative schemas for code generators",
            "duplicate key 'rule/authz-action-known'",
        ),
    ],
)
def test_weakened_catalogue_lint_rule_fails(
    tmp_path: Path, old: str, new: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, old, new)
    findings = _findings_for(root, "generated-block")
    assert any(needle in f for f in findings), findings
    with pytest.raises(CatalogError):
        action_catalog.generate(root)


@pytest.mark.parametrize(
    "table",
    [
        "  | `shell.exec` | Client an Server | `host.read` |\n",
        "Op | Richtung | Aktion\n--- | --- | ---\n`shell.exec` | Client an Server | `host.read`\n",
    ],
)
def test_other_table_forms_outside_the_blocks_fail(tmp_path: Path, table: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: ws_ops -->\n",
        "<!-- END GENERATED: ws_ops -->\n\n" + table,
    )
    assert any(
        "table row outside the generated blocks" in f
        for f in _findings_for(root, "generated-block")
    )


def test_event_entry_that_is_not_a_mapping_gives_one_finding(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: d["events"].__setitem__(0, "message.created"))
    assert action_catalog.check(root) == ["actions.yaml: schema: events[0]: must be a mapping"]


# ---- red team delta: rule overrides in every *Rules key, case, table placement ------------


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        (
            "  rest@v1:\n    root: ./openapi.yaml\n    rules:\n",
            "  rest@v1:\n    root: ./openapi.yaml\n    oas3_1Rules:\n"
            "      rule/authz-action-known: off\n    rules:\n",
            "apis[rest@v1].oas3_1Rules overrides rule/authz-action-known",
        ),
        (
            "  gateway@v1:\n    root: ./asyncapi.yaml\n",
            "  gateway@v1:\n    root: ./asyncapi.yaml\n    async3Rules:\n"
            "      rule/receive-action-known-message: off\n",
            "apis[gateway@v1].async3Rules overrides rule/receive-action-known-message",
        ),
        (
            "extends:\n  - recommended\n",
            "extends:\n  - recommended\noas3_1Rules:\n  rule/rate-limit-class-known: off\n",
            "oas3_1Rules overrides rule/rate-limit-class-known",
        ),
    ],
)
def test_catalogue_lint_rule_overridden_in_any_rules_key_fails(
    tmp_path: Path, old: str, new: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, old, new)
    findings = _findings_for(root, "generated-block")
    assert any(needle in f for f in findings), findings


@pytest.mark.parametrize("right", ["System", "SYSTEM", "system-Job"])
def test_right_that_names_system_in_any_case_fails(tmp_path: Path, right: str) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, lambda d: _entry(d, "actions", "id", "grant.expire").update(right=right))
    _assert_finding(root, "schema", "grant.expire: right names system but is not exactly system")


HAND_TABLE = "| `shell.exec` | Client an Server | `host.read` |\n"


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        # a hand-written title inside a code fence cannot open a hand-written section
        (
            "<!-- END GENERATED: ws_ops -->\n",
            "<!-- END GENERATED: ws_ops -->\n\n```text\n## 99. Konfigurationswerte dieses Kapitels"
            "\n```\n\n" + HAND_TABLE,
            "hand-written section 'Konfigurationswerte dieses Kapitels' appears 2 times;"
            " expected once",
        ),
        # a new level-2 section that is not one of the hand-written ones
        (
            "## 5. Agenten-Aufruf `agent.invoke`\n",
            "## 4.15 Extra\n\n" + HAND_TABLE + "\n## 5. Agenten-Aufruf `agent.invoke`\n",
            "table row outside the generated blocks in section 'Extra'",
        ),
    ],
)
def test_hand_table_outside_the_blocks_cannot_hide(
    tmp_path: Path, old: str, new: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, old, new)
    findings = _findings_for(root, "generated-block")
    assert any(needle in f for f in findings), findings


def test_table_inside_a_code_fence_is_a_finding_too(tmp_path: Path) -> None:
    # Fail closed: outside the blocks, a generated section holds no pipe at all.
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: ws_ops -->\n",
        "<!-- END GENERATED: ws_ops -->\n\n```text\n" + HAND_TABLE + "```\n",
    )
    assert any("outside the generated blocks" in f for f in _findings_for(root, "generated-block"))


# ---- end-to-end: `make api-lint` rejects an excluded value at every location ------------

NODE_MODULES = REPO / "tools" / "node" / "node_modules"
# Redocly's own summary line; quoted source excerpts start with a line number, so a comment in a
# contract cannot pose as it.
_LINT_SUMMARY = re.compile(
    r"^\W*Validation failed with (\d+) errors?(?: and \d+ warnings?)?\.?\s*$", re.MULTILINE
)
REDOCLY_BIN = NODE_MODULES / ".bin" / "redocly"
API_LINT_RECIPE = (
    "api-lint: ## Redocly lint of the OpenAPI and AsyncAPI documents\n"
    "\t@if [ ! -f docs/api/openapi.yaml ]; then"
    " $(call skip,api-lint,docs/api/openapi.yaml does not exist yet); \\\n"
    '\telse cfg=""; [ -f docs/api/redocly.yaml ] && cfg="--config docs/api/redocly.yaml"; \\\n'
    '\t  files="docs/api/openapi.yaml"; [ -f docs/api/asyncapi.yaml ]'
    ' && files="$$files docs/api/asyncapi.yaml"; \\\n'
    "\t  $(call need_node,redocly); REDOCLY_TELEMETRY=off $(NODE_BIN)/redocly lint"
    " $$cfg $$files; fi\n"
)


def test_make_api_lint_recipe_is_pinned() -> None:
    # A changed recipe (an extra argument, a filter, `|| true`) must be a visible test change.
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    start = makefile.index("api-lint: ##")
    assert makefile[start : makefile.index("\n\n", start) + 1] == API_LINT_RECIPE


def test_redocly_accepts_the_real_contracts_without_make(tmp_path: Path) -> None:
    # Independent of the Makefile: the real contracts pass the real configuration as they are.
    if not REDOCLY_BIN.exists():
        pytest.fail("tools/node/node_modules/.bin/redocly not found; run `make bootstrap`")
    shutil.copytree(REPO / "docs" / "api", tmp_path / "docs" / "api")
    proc = subprocess.run(
        [
            str(REDOCLY_BIN),
            "lint",
            "--config",
            "docs/api/redocly.yaml",
            "docs/api/openapi.yaml",
            "docs/api/asyncapi.yaml",
        ],
        cwd=tmp_path,
        env={**os.environ, "REDOCLY_TELEMETRY": "off", "REDOCLY_SUPPRESS_UPDATE_NOTICE": "true"},
        capture_output=True,
        text=True,
        timeout=600,
        check=False,
    )
    assert proc.returncode == 0, proc.stdout + proc.stderr


def _inject(api: Path, rel: str, prop: str, value: str) -> int:
    """Set `prop` to `value` at every location of one contract; return how many it changed.

    Coupling to keep in mind: a rate-limit class `none` exempts a webhook description from the
    4xx rule of redocly.yaml. Today every webhook documents a 4xx response anyway; a future one
    without would add a 4xx error to the count and fail this test loudly (fail closed).
    """
    path = api / rel
    text, count = re.subn(
        rf"^(\s*){re.escape(prop)}: \S+$",
        rf"\g<1>{prop}: {value}",
        path.read_text("utf-8"),
        flags=re.MULTILINE,
    )
    path.write_text(text, encoding="utf-8")
    return count


def _make_api_lint(
    tmp_path: Path, injections: Sequence[tuple[str, str, str]]
) -> tuple[int, int, str]:
    """Run the real `make api-lint` on a copy with the given injections.

    Returns the exit code, the number of changed locations and the output. Going through make
    covers the whole gate (recipe, `.IGNORE`, a second recipe, variables), and changing every
    location covers any weakening that acts only on some of them (a `$ref`, an ignore entry,
    a preprocessor), whatever mechanism it uses.
    """
    if not REDOCLY_BIN.exists():
        pytest.fail("tools/node/node_modules/.bin/redocly not found; run `make bootstrap`")
    shutil.copytree(REPO / "docs" / "api", tmp_path / "docs" / "api")
    shutil.copy(REPO / "Makefile", tmp_path / "Makefile")
    (tmp_path / "tools" / "node").mkdir(parents=True)
    (tmp_path / "tools" / "node" / "node_modules").symlink_to(NODE_MODULES)
    changed = sum(_inject(tmp_path / "docs" / "api", *spec) for spec in injections)
    env = {k: v for k, v in os.environ.items() if k not in ("MAKEFLAGS", "MAKELEVEL", "MFLAGS")}
    env |= {"REDOCLY_TELEMETRY": "off", "REDOCLY_SUPPRESS_UPDATE_NOTICE": "true", "NO_COLOR": "1"}
    proc = subprocess.run(
        ["make", "api-lint"],
        cwd=tmp_path,
        env=env,
        capture_output=True,
        text=True,
        timeout=600,
        check=False,
    )
    return proc.returncode, changed, proc.stdout + proc.stderr


def test_make_api_lint_accepts_the_unchanged_contracts(tmp_path: Path) -> None:
    code, _, output = _make_api_lint(tmp_path, [])
    assert code == 0, output


@pytest.mark.parametrize(
    ("injections", "rule"),
    [
        (
            [
                ("openapi.yaml", "x-authz-action", "grant.expire"),
                ("asyncapi.yaml", "x-authz-action", "grant.expire"),
            ],
            "rule/authz-action-known",
        ),
        (
            [
                ("openapi.yaml", "x-authz-action", "no.such_action"),
                ("asyncapi.yaml", "x-authz-action", "no.such_action"),
            ],
            "rule/authz-action-known",
        ),
        (
            [
                ("openapi.yaml", "x-rate-limit-class", "rl.unknown"),
                ("asyncapi.yaml", "x-rate-limit-class", "rl.unknown"),
            ],
            "rule/rate-limit-class-known",
        ),
        (
            [("asyncapi.yaml", "x-receive-action", "instance.bootstrap")],
            "rule/receive-action-known-message",
        ),
        (
            [("openapi.yaml", "x-receive-action", "owner.recover")],
            "rule/receive-action-known-webhook",
        ),
    ],
)
def test_make_api_lint_rejects_an_excluded_value_at_every_location(
    tmp_path: Path, injections: list[tuple[str, str, str]], rule: str
) -> None:
    code, changed, output = _make_api_lint(tmp_path, injections)
    assert changed > 0
    assert code != 0, output
    assert rule in output, output
    summaries = _LINT_SUMMARY.findall(output)
    assert summaries, output
    assert int(summaries[-1]) == changed, output


def test_dollar_key_in_redocly_configuration_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        REDOCLY,
        "  rest@v1:\n    root: ./openapi.yaml\n    rules:\n",
        "  rest@v1:\n    root: ./openapi.yaml\n    rules:\n      $ref: ./relax.yaml\n",
    )
    assert any(
        "apis[rest@v1].rules: key '$ref' is not allowed" in f
        for f in _findings_for(root, "generated-block")
    )


# ---- second delta: fail closed for hand tables, lint-ignore and preprocessors -------------


@pytest.mark.parametrize(
    "addition",
    [
        "> | Op | Richtung | Aktion |\n> |---|---|---|\n> " + HAND_TABLE,
        "<table><tr><td>shell.exec</td></tr></table>\n",
        "Op | Richtung | Aktion\n---\t| --- | ---\n`shell.exec` | Client an Server | `host.read`\n",
        "```text\n~~~\n```\n\n" + HAND_TABLE,
        "```shell.exec``` folgt.\n\n" + HAND_TABLE,
    ],
)
def test_any_table_form_in_a_generated_section_fails(tmp_path: Path, addition: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: ws_ops -->\n",
        "<!-- END GENERATED: ws_ops -->\n\n" + addition,
    )
    assert any("outside the generated blocks" in f for f in _findings_for(root, "generated-block"))


@pytest.mark.parametrize(
    "heading",
    [
        "## Anhang\n",
        "Anhang\n------\n",
        "# Anhang\n",
    ],
)
def test_section_after_the_last_hand_written_one_is_covered(tmp_path: Path, heading: str) -> None:
    root = _copy_repo(tmp_path)
    chapter = (root / CHAPTER).read_text(encoding="utf-8")
    (root / CHAPTER).write_text(chapter + "\n" + heading + "\n" + HAND_TABLE, encoding="utf-8")
    assert any(
        "outside the generated blocks in section 'Anhang'" in f
        for f in _findings_for(root, "generated-block")
    )


def test_hand_written_title_twice_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: ws_ops -->\n",
        "<!-- END GENERATED: ws_ops -->\n\n## 7.1 Entscheidungen dieses Kapitels\n\n" + HAND_TABLE,
    )
    assert any(
        "hand-written section 'Entscheidungen dieses Kapitels' appears 2 times" in f
        for f in _findings_for(root, "generated-block")
    )


def test_lint_ignore_file_naming_a_catalogue_rule_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    (root / "docs/api/.redocly.lint-ignore.yaml").write_text(
        "openapi.yaml:\n  rule/authz-action-known:\n    - '#/paths/~1me/get/x-authz-action'\n",
        encoding="utf-8",
    )
    assert any(
        f.startswith("docs/api/.redocly.lint-ignore.yaml: generated-block:")
        and "rule/authz-action-known" in f
        for f in action_catalog.check(root)
    )
    with pytest.raises(CatalogError, match="lint-ignore"):
        action_catalog.generate(root)


def test_lint_ignore_file_for_other_rules_is_fine(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    (root / "docs/api/.redocly.lint-ignore.yaml").write_text(
        "openapi.yaml:\n  operation-summary:\n    - '#/paths/~1me/get'\n", encoding="utf-8"
    )
    assert action_catalog.check(root) == []


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        (
            "extends:\n  - recommended\n",
            "extends:\n  - recommended\npreprocessors:\n  remove-x-internal: on\n",
            "key 'preprocessors' is not allowed",
        ),
        (
            "  gateway@v1:\n    root: ./asyncapi.yaml\n",
            "  gateway@v1:\n    root: ./asyncapi.yaml\n    plugins:\n      - ./local.js\n",
            "apis[gateway@v1]: key 'plugins' is not allowed",
        ),
        (
            "apis:\n  rest@v1:\n",
            "apis:\n  shadow@v1:\n    root: ./openapi.yaml\n  rest@v1:\n",
            "apis[rest@v1] lints docs/api/openapi.yaml like apis[shadow@v1]",
        ),
    ],
)
def test_redocly_configuration_that_changes_the_lint_fails(
    tmp_path: Path, old: str, new: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, old, new)
    findings = _findings_for(root, "generated-block")
    assert any(needle in f for f in findings), findings


def test_duplicate_key_line_points_into_the_file(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, "        - access.explain\n", "")  # stale block, regenerated text differs
    _edit(
        root,
        REDOCLY,
        "  # Conservative schemas for code generators",
        "  rule/authz-action-known: off\n  # Conservative schemas for code generators",
    )
    lines = (root / REDOCLY).read_text(encoding="utf-8").splitlines()
    line = lines.index("  rule/authz-action-known: off") + 1
    assert any(
        f"line {line}: duplicate key 'rule/authz-action-known'" in f
        for f in _findings_for(root, "generated-block")
    )


# ---- third delta: positive list for redocly.yaml, HTML in 05, parsed lint-ignore ---------


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        (
            "extends:\n  - recommended\n",
            "extends:\n  - recommended\noas3_1Preprocessors:\n  filter-out:\n"
            "    property: operationId\n    value: getMe\n",
            "key 'oas3_1Preprocessors' is not allowed",
        ),
        (
            "  rest@v1:\n    root: ./openapi.yaml\n",
            "  rest@v1:\n    root: ./openapi.yaml\n    extends:\n      - ./relax.yaml\n",
            "apis[rest@v1]: key 'extends' is not allowed",
        ),
        (
            "extends:\n  - recommended\n",
            "extends:\n  - recommended\n  - ./relax.yaml\n",
            "extends must be exactly [recommended]",
        ),
        (
            "apis:\n  rest@v1:\n",
            "apis:\n  shadow@v1:\n    root: openapi.yaml\n  rest@v1:\n",
            "apis[rest@v1] lints docs/api/openapi.yaml like apis[shadow@v1]",
        ),
    ],
)
def test_redocly_configuration_outside_the_positive_list_fails(
    tmp_path: Path, old: str, new: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, old, new)
    findings = _findings_for(root, "generated-block")
    assert any(needle in f for f in findings), findings


def test_lint_ignore_file_is_read_as_yaml(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    (root / "docs/api/.redocly.lint-ignore.yaml").write_text(
        'openapi.yaml:\n  "rule\\/authz-action-known":\n    - "#/paths/~1me/get/x-authz-action"\n',
        encoding="utf-8",
    )
    assert any(
        "suppresses rule/authz-action-known" in f for f in _findings_for(root, "generated-block")
    )
    (root / "docs/api/.redocly.lint-ignore.yaml").write_text("a: [\n", encoding="utf-8")
    assert any("cannot parse" in f for f in _findings_for(root, "generated-block"))


@pytest.mark.parametrize(
    ("old", "new"),
    [
        (
            "<!-- END GENERATED: channel_ops -->\n",
            "<!-- END GENERATED: channel_ops -->\n\n<!--\n## Entscheidungen dieses Kapitels\n-->\n",
        ),
        ("## 8. Entscheidungen dieses Kapitels\n", "<h2>8. Entscheidungen dieses Kapitels</h2>\n"),
    ],
)
def test_html_comment_or_heading_outside_the_blocks_fails(
    tmp_path: Path, old: str, new: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, old, new)
    assert any(
        "HTML outside the generated blocks" in f for f in _findings_for(root, "generated-block")
    )


def test_missing_hand_written_title_fails(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "## 8. Entscheidungen dieses Kapitels\n", "## 8. Rulings\n")
    assert any(
        "hand-written section 'Entscheidungen dieses Kapitels' appears 0 times" in f
        for f in _findings_for(root, "generated-block")
    )


# ---- fourth delta: the rendered docs site is the artefact; fail-closed HTML; real roots ----


@functools.cache
def _site_markdown_config() -> tuple[list[Any], dict[str, Any]]:
    """Markdown extensions and their options exactly as mkdocs.yml gives them to the site."""
    config = load_config(config_file=str(REPO / "mkdocs.yml"))
    return list(config["markdown_extensions"]), dict(config["mdx_configs"])


# Positive list of the markup the docs site may render for chapter 05: what Markdown itself
# emits for text, lists, links and tables. Any other element (div, span, details, style, ...)
# or attribute (hidden, style, data-*, ...) is a finding; fenced code or admonitions in 05 would
# need this list extended in review.
_ALLOWED_MARKUP: dict[str, frozenset[str]] = {
    **dict.fromkeys(("h1", "h2", "h3", "h4", "h5", "h6"), frozenset({"id"})),
    "a": frozenset({"href", "title", "class"}),
    **dict.fromkeys(
        (
            "p",
            "code",
            "em",
            "strong",
            "ul",
            "ol",
            "li",
            "blockquote",
            "hr",
            "br",
            "table",
            "thead",
            "tbody",
            "tr",
            "th",
            "td",
        ),
        frozenset(),
    ),
}
# Attribute values that are allowed only in this form: the permalink anchors of the toc extension.
_ALLOWED_VALUES: dict[tuple[str, str], frozenset[str]] = {("a", "class"): frozenset({"headerlink"})}
# Like action_catalog's rules: a title shows only with a letter or digit of this positive list,
# holds no other characters than these (code spans render without backticks), and titles
# compare by their visible letters.
_VISIBLE_TITLE = re.compile("[0-9A-Za-zÄÖÜäöüß]")
_TITLE_TEXT = re.compile(r'[0-9A-Za-zÄÖÜäöüß "(),\-.:/_]*')


def _title_keys(title: str) -> frozenset[str]:
    return frozenset(
        "".join(re.findall("[A-Za-zÄÖÜäöüß]", text)).casefold().replace("i", "l")
        for text in (title, title.replace("1", "l").replace("0", "o"))
    )


_MARKER_HTML = re.compile(r"^<!-- (?:BEGIN|END) GENERATED: [a-z0-9_.]+ -->\n?$")


class _Tables(HTMLParser):
    """Level-1/2/3 headings, the cell texts of every table and markup outside the positive list."""

    def __init__(self) -> None:
        super().__init__()
        self.items: list[tuple[str, Any]] = []
        self.foreign: list[str] = []
        self._heading: list[str] | None = None
        self._table: list[list[str]] | None = None
        self._cell: list[str] | None = None
        self._item: list[str] | None = None
        self._lists = 0
        self._containers = 0
        self._any_heading: list[str] | None = None
        self.headings: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        allowed = _ALLOWED_MARKUP.get(tag)
        if allowed is None:
            self.foreign.append(f"<{tag}>")
        else:
            self.foreign += [f"<{tag} {name}>" for name, _ in attrs if name not in allowed]
            self.foreign += [
                f'<{tag} {name}="{value}">'
                for name, value in attrs
                if (tag, name) in _ALLOWED_VALUES and value not in _ALLOWED_VALUES[(tag, name)]
            ]
        if tag == "code":
            # Cells and list items keep their code spans as backticks, like the source the
            # checks read.
            for text in (self._cell, self._item):
                if text is not None:
                    text.append("`")
        if tag in ("ul", "ol"):
            self._lists += 1
        elif tag == "li" and self._lists == 1:
            self._item = []
        if tag in ("li", "blockquote"):
            self._containers += 1
        if tag in ("h1", "h2", "h3", "h4", "h5", "h6"):
            # A heading in a list item or quote, or without text, would let a reader of the
            # page see a section end where the checks see none, or none where they see one.
            if self._containers:
                self.headings.append(f"<{tag}> inside a list item or quote")
            self._any_heading = []
        if tag in ("h1", "h2", "h3"):
            self._heading = []
        elif tag == "table":
            self._table = []
        elif tag == "tr" and self._table is not None:
            self._table.append([])
        elif tag in ("td", "th"):
            self._cell = []

    def handle_endtag(self, tag: str) -> None:
        if tag == "code":
            for text in (self._cell, self._item):
                if text is not None:
                    text.append("`")
        if tag in ("ul", "ol"):
            self._lists -= 1
        elif tag == "li" and self._lists == 1 and self._item is not None:
            self.items.append(("item", " ".join("".join(self._item).split())))
            self._item = None
        if tag in ("li", "blockquote"):
            self._containers -= 1
        if tag in ("h1", "h2", "h3", "h4", "h5", "h6") and self._any_heading is not None:
            title = "".join(self._any_heading).replace("\u00b6", "").strip()
            if not _VISIBLE_TITLE.search(title):
                self.headings.append(f"<{tag}> without a visible title")
            elif not _TITLE_TEXT.fullmatch(title):
                self.headings.append(
                    f"<{tag}> title {title!r} with a character outside the positive list"
                )
            if tag in ("h4", "h5", "h6"):
                minor = re.sub(r"^[0-9]+(?:\.[0-9]+)*\.?\s+", "", title)
                self.items.append(("minorheading", minor))
            self._any_heading = None
        if tag in ("h1", "h2", "h3") and self._heading is not None:
            title = "".join(self._heading).replace("\u00b6", "").strip()
            kind = "subheading" if tag == "h3" else "heading"
            self.items.append((kind, re.sub(r"^[0-9]+(?:\.[0-9]+)*\.?\s+", "", title)))
            self._heading = None
        elif tag == "table" and self._table is not None:
            self.items.append(("table", tuple(tuple(row) for row in self._table)))
            self._table = None
        elif tag in ("td", "th") and self._cell is not None and self._table:
            self._table[-1].append(" ".join("".join(self._cell).split()))
            self._cell = None

    def handle_data(self, data: str) -> None:
        if self._any_heading is not None:
            self._any_heading.append(data)
        if self._heading is not None:
            self._heading.append(data)
        if self._cell is not None:
            self._cell.append(data)
        if self._item is not None:
            self._item.append(data)


def _parse(text: str) -> _Tables:
    """Render with the site configuration; raw HTML the renderer passed through counts too."""
    extensions, configs = _site_markdown_config()
    md = markdown.Markdown(extensions=extensions, extension_configs=configs)
    parser = _Tables()
    parser.feed(md.convert(text))
    # Code spans are stashed escaped (&lt;); raw HTML keeps its `<`; md_in_html leaves an empty
    # entry for each element it processed. Only the markers may pass.
    parser.foreign += [
        "empty stash entry (an md_in_html element or an empty code span)"
        for block in md.htmlStash.rawHtmlBlocks
        if str(block) == ""
    ]
    parser.foreign += [
        f"raw HTML {str(block)[:40]!r}"
        for block in md.htmlStash.rawHtmlBlocks
        if "<" in str(block) and not _MARKER_HTML.match(str(block))
    ]
    return parser


def _render(text: str) -> list[tuple[str, Any]]:
    return _parse(text).items


def _misplaced(name: str, section: str, subsection: str, minor: str) -> list[str]:
    """The rendered table of block `name` is not in its section or under its heading, or a
    heading of level 4 or deeper stands between them."""
    family, _, part = name.partition(".")
    want = (
        action_catalog.BLOCK_SECTIONS.get(family),
        action_catalog.ACTION_SECTION_TITLES.get(part) if family == "actions" else subsection,
    )
    if (section, subsection) != want:
        return [
            f"table of block {name!r} rendered under {section!r} / {subsection!r}; expected {want}"
        ]
    if minor:
        return [f"table of block {name!r} rendered under the level-4 or deeper heading {minor!r}"]
    return []


# The hand-written parts the checks read: (level-2 section, level-3 subsection or None for the
# section itself, kind of content, title for the static reader, header row of a table).
_HAND_READINGS: tuple[tuple[str, str | None, str, str, tuple[str, ...]], ...] = (
    (
        "Begriffe der Tabellen",
        "Scopes für Apps",
        "table",
        "Scopes für Apps",
        action_catalog.SCOPE_HEADER,
    ),
    ("Begriffe der Tabellen", "Wirkung (effect)", "item", "Wirkung (`effect`)", ()),
    (
        "Konfigurationswerte dieses Kapitels",
        None,
        "table",
        "Konfigurationswerte dieses Kapitels",
        action_catalog.CONFIG_HEADER,
    ),
)


def _hand_reading_problems(chapter: str, items: list[tuple[str, Any]]) -> list[str]:
    """What the site shows in 2.6, 2.7 and the configuration table differs from what the
    checks read there (the static readers of action_catalog)."""
    problems: list[str] = []
    for section, subsection, kind, title, header in _HAND_READINGS:
        shown: list[Any] = []
        current = sub = ""
        for found, item in items:
            if found == "heading":
                current, sub = item, ""
            elif found == "subheading":
                sub = item
            elif found == kind and current == section and (subsection is None or sub == subsection):
                shown.append(item)
        named = subsection or section
        count = sum(
            1
            for found, item in items
            if found in ("heading", "subheading", "minorheading")
            and _title_keys(item) & _title_keys(named)
        )
        if count != 1:
            problems.append(f"hand-written reading of {title!r}: title rendered {count} times")
        try:
            body = action_catalog._section(chapter, title)
        except CatalogError as err:
            problems.append(f"hand-written reading of {title!r}: {err.findings}")
            continue
        if kind == "table":
            rows, _ = action_catalog._table_rows(body, title, header)
            read: list[Any] = [
                tuple(tuple(" ".join(c.split()) for c in r) for r in [list(header), *rows])
            ]
        else:
            read = [" ".join(i.split()) for i in action_catalog._bullets(body, title)[0]]
        if shown != read:
            problems.append(
                f"hand-written reading of {title!r}: the site shows {shown}, the checks read {read}"
            )
    return problems


def _rendered_problems(chapter: str) -> list[str]:
    """What the docs site would show that the generated blocks do not account for."""
    # mkdocs reads page metadata and front matter before Markdown; none may be there.
    body, page_meta = mkdocs_meta.get_data(chapter)
    if page_meta or body != chapter:
        return [f"page metadata {page_meta!r} would change how the site renders the chapter"]
    hand = {t.replace("`", "") for t in action_catalog.HAND_WRITTEN_SECTIONS}
    tables = [
        (item, name)
        for name, body in action_catalog.generated_blocks(chapter).items()
        for kind, item in _render(body)
        if kind == "table"
    ]
    wanted = Counter(item for item, _ in tables)
    origin = dict(tables)
    page = _parse(chapter)
    shown: Counter[Any] = Counter()
    titles: Counter[str] = Counter()
    placed: list[str] = []
    section = subsection = minor = ""
    for kind, item in page.items:
        if kind == "heading":
            section, subsection, minor = item, "", ""
            titles[item] += 1
        elif kind == "subheading":
            subsection, minor = item, ""
        elif kind == "minorheading":
            minor = item
        elif kind == "table":
            if section not in hand:
                shown[item] += 1
            if item in origin:
                placed += _misplaced(origin[item], section, subsection, minor)
    problems = [
        f"hand-written title {t!r} rendered {titles[t]} times" for t in hand if titles[t] != 1
    ]
    rendered_titles = [item for kind, item in page.items if kind == "heading"]
    static_titles = [t.replace("`", "") for t in action_catalog.section_titles(chapter)]
    if rendered_titles != static_titles:
        problems.append(f"rendered headings {rendered_titles} differ from {static_titles}")
    problems += placed
    problems += [f"heading the checks cannot place: {h}" for h in page.headings]
    problems += _hand_reading_problems(chapter, page.items)
    problems += [f"markup outside the positive list: {m}" for m in page.foreign]
    if shown != wanted:
        problems.append(
            "tables outside hand-written sections differ: unexpected"
            f" {dict(shown - wanted)}, missing {dict(wanted - shown)}"
        )
    return problems


def test_rendered_chapter_shows_only_the_generated_tables() -> None:
    assert _rendered_problems((REPO / CHAPTER).read_text(encoding="utf-8")) == []


SMUGGLED = "| Klasse | Schlüssel | Zweck |\n|---|---|---|\n| `rl.read` | Prinzipal | lesend |\n"


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ("## 9. Tests,", "<h2\n>8a. Kopie</h2>\n\n" + SMUGGLED + "\n## 9. Tests,"),
        (
            "<!-- END GENERATED: channel_ops -->\n",
            "<!-- END GENERATED: channel_ops -->\n\n<?\n## Entscheidungen dieses Kapitels\n?>\n\n"
            + SMUGGLED,
        ),
        ("## 9. Tests,", "##8a. Kopie\n\n" + SMUGGLED + "\n## 9. Tests,"),
        (
            "<!-- END GENERATED: channel_ops -->\n\n## 8. Entscheidungen dieses Kapitels\n",
            "<!-- END GENERATED: channel_ops -->\n\n   ## Entscheidungen dieses Kapitels\n"
            '{: hidden }\n\nEntscheidungen dieses Kapitels {: data-x="|" hidden }\n---\n\n'
            + SMUGGLED
            + "\n### 8. Entscheidungen dieses Kapitels\n",
        ),
        # at the end of generated section 7, before the hand-written section 8
        ("## 8. Entscheidungen", "<table><tr><td>x</td></tr></table>\n\n## 8. Entscheidungen"),
        (
            "## 8. Entscheidungen",
            "> " + SMUGGLED.replace("\n|", "\n> |") + "\n## 8. Entscheidungen",
        ),
    ],
)
def test_rendered_check_sees_smuggled_tables(old: str, new: str) -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    assert old in chapter
    assert _rendered_problems(chapter.replace(old, new, 1))


def test_rendered_check_sees_a_generated_table_turned_into_code() -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    fenced = chapter.replace(
        "<!-- BEGIN GENERATED: ws_ops -->\n", "```text\n<!-- BEGIN GENERATED: ws_ops -->\n", 1
    ).replace("<!-- END GENERATED: ws_ops -->\n", "<!-- END GENERATED: ws_ops -->\n```\n", 1)
    problems = _rendered_problems(fenced)
    assert any("missing" in p and "Op" in p for p in problems), problems


@pytest.mark.parametrize(
    "addition",
    [
        "<h2\n>8a. Kopie</h2>\n",
        "<?\n## 99. Anderes\n?>\n",
        "<div hidden>\n",
        "##8a. Kopie\n",
        "\\<details>\n",
        "Text \\<div hidden>\n",
    ],
)
def test_static_check_flags_html_and_headings_without_space(tmp_path: Path, addition: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: channel_ops -->\n",
        "<!-- END GENERATED: channel_ops -->\n\n" + addition,
    )
    findings = _findings_for(root, "generated-block")
    assert any(
        "HTML outside the generated blocks or a heading without a space" in f for f in findings
    ), findings


@pytest.mark.parametrize(
    ("old", "new", "needle"),
    [
        (
            "apis:\n  rest@v1:\n",
            "apis:\n  shadow@v1:\n    root: ../api/openapi.yaml\n  rest@v1:\n",
            "apis[rest@v1] lints docs/api/openapi.yaml like apis[shadow@v1]",
        ),
        (
            "  rest@v1:\n    root: ./openapi.yaml\n",
            "  rest@v1:\n    root: /etc/openapi.yaml\n",
            "apis[rest@v1]: root must be a relative path",
        ),
        (
            "  rest@v1:\n    root: ./openapi.yaml\n",
            "  rest@v1:\n",
            "apis[rest@v1]: root must be a relative path",
        ),
    ],
)
def test_redocly_roots_are_resolved(tmp_path: Path, old: str, new: str, needle: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, old, new)
    findings = _findings_for(root, "generated-block")
    assert any(needle in f for f in findings), findings


@pytest.mark.parametrize("line", ["{: hidden }\n", "Titel { #anker .klasse }\n"])
def test_attribute_list_outside_the_blocks_fails(tmp_path: Path, line: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: channel_ops -->\n",
        "<!-- END GENERATED: channel_ops -->\n\n" + line,
    )
    assert any(
        "brace pair outside a code span" in f for f in _findings_for(root, "generated-block")
    )


def test_section_titles_follow_the_headings() -> None:
    titles = action_catalog.section_titles((REPO / CHAPTER).read_text(encoding="utf-8"))
    assert titles[0] == "Aktionskatalog der Autorisierung"
    assert "Rate-Limit-Klassen" in titles
    assert titles[-1] == "Konfigurationswerte dieses Kapitels"


def test_lint_summary_ignores_quoted_source_lines() -> None:
    output = (
        "  17 | # Validation failed with 7 errors.\n"
        "Validation failed with 3 errors.  # in a quoted excerpt? no, a real line\n"
        "❌ Validation failed with 1 error.\n"
    )
    assert _LINT_SUMMARY.findall(output) == ["1"]


# ---- sixth delta: details, nesting, heading split, warnings in the summary ----------------


def test_lint_summary_accepts_warnings() -> None:
    output = "  9 | # x\n❌ Validation failed with 4 errors and 1 warning.\n"
    assert _LINT_SUMMARY.findall(output) == ["4"]


@pytest.mark.parametrize(
    ("old", "new"),
    [
        # static sees a heading in a code block, the site renders code (an indented heading is
        # text in both views since the static view needs `#` in the first column)
        ("## 9. Tests,", "```text\n## 8a. Kopie\n```\n\n## 9. Tests,"),
        # static reads the setext title with its attribute list, the site strips the list
        (
            "<!-- END GENERATED: channel_ops -->\n\n## 8. Entscheidungen dieses Kapitels\n",
            "<!-- END GENERATED: channel_ops -->\n\n"
            'Entscheidungen dieses Kapitels {: data-x="|" }\n'
            "---\n\n" + SMUGGLED + "\n### 8. Entscheidungen dieses Kapitels\n",
        ),
    ],
)
def test_rendered_check_sees_a_split_between_static_and_rendered_headings(
    old: str, new: str
) -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    assert old in chapter
    problems = _rendered_problems(chapter.replace(old, new, 1))
    assert any(p.startswith("rendered headings") for p in problems), problems


@pytest.mark.parametrize(
    "replacement",
    [
        '??? note "Mehr"\n    ## 8. Entscheidungen dieses Kapitels\n\n' + SMUGGLED,
        "!!! note\n    ## 8. Entscheidungen dieses Kapitels\n\n" + SMUGGLED,
        "\\` <details> `\n\n## 8. Entscheidungen dieses Kapitels\n\n\\` </details> `\n\n"
        + SMUGGLED,
    ],
)
def test_collapsed_or_nested_heading_cannot_hide_a_section(replacement: str) -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    old = "## 8. Entscheidungen dieses Kapitels\n"
    attacked = chapter.replace(old, replacement + "\n### 8. Entscheidungen dieses Kapitels\n", 1)
    assert _rendered_problems(attacked)


@pytest.mark.parametrize(
    "line",
    [
        '??? note "Mehr"\n',
        "!!! note\n",
        "[^h]: Hinweis\n",
        "\\` <details> `\n",
        "`x\\`<template>`\n",
    ],
)
def test_static_check_flags_admonitions_footnotes_and_escaped_backticks(
    tmp_path: Path, line: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: channel_ops -->\n",
        "<!-- END GENERATED: channel_ops -->\n\n" + line,
    )
    assert _findings_for(root, "generated-block"), line


def test_section_titles_strip_numbers_of_setext_headings() -> None:
    assert action_catalog.section_titles("3. Titel\n---\n# 4. Weiter\n") == ["Titel", "Weiter"]


@pytest.mark.parametrize("line", ["\\\\`<b>`\n", "plain `<b>` code\n", "`{: hidden }`\n"])
def test_static_check_leaves_code_spans_alone(tmp_path: Path, line: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: channel_ops -->\n",
        "<!-- END GENERATED: channel_ops -->\n\n" + line,
    )
    assert _findings_for(root, "generated-block") == []


@pytest.mark.parametrize(
    "insert",
    [
        "<style>table tr:nth-child(3){display:none}</style>\n",
        "`\\` <style>h2{display:none}</style> `\n",
        "Siehe `a\nb` <style>x</style> `\n",
        "<script>x()</script>\n",
        "`x\\`<template>`\n",
        '<div markdown="1">\n\nText\n\n</div>\n',
        "<!-- Kommentar -->\n",
        "Text\n{: data-x=y }\n",
        '<p markdown="1">Text</p>\n',
        '<blockquote markdown="1">\nText\n</blockquote>\n',
        "## Titel {: hidden }\n",
    ],
)
def test_rendered_check_allows_no_markup_outside_the_positive_list(insert: str) -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    problems = _rendered_problems(
        chapter.replace("## 4. Aktionen\n", insert + "\n## 4. Aktionen\n", 1)
    )
    assert any(p.startswith("markup outside the positive list") for p in problems), problems


@pytest.mark.parametrize(
    "prefix",
    ["template: 404.html\n\n", "---\ntemplate: 404.html\n---\n", "---\n{}\n---\n", "\n"],
)
def test_page_metadata_is_a_finding(tmp_path: Path, prefix: str) -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    assert any(p.startswith("page metadata") for p in _rendered_problems(prefix + chapter))
    root = _copy_repo(tmp_path)
    (root / CHAPTER).write_text(prefix + chapter, encoding="utf-8")
    assert any(
        "must start with its '# ' heading" in f for f in _findings_for(root, "generated-block")
    )


# ---- final pass: block order, code spans like Markdown, link classes, catalog-check e2e ----


def _regenerated(root: Path, text: str) -> str:
    """Fill every block with its current rendering, so that only the block order is wrong."""
    catalog = action_catalog.load_catalog(root / CATALOG)
    for name, body in action_catalog.render_chapter_blocks(catalog).items():
        text = action_catalog.replace_generated_block(text, name, body)
    return text


@pytest.mark.parametrize(
    ("first", "second"),
    [("events.durable", "events.ephemeral"), ("actions.public", "actions.terminal")],
)
def test_swapped_generated_blocks_fail(tmp_path: Path, first: str, second: str) -> None:
    root = _copy_repo(tmp_path)
    text = (root / CHAPTER).read_text(encoding="utf-8")
    text = text.replace(f"GENERATED: {first} -->", "GENERATED: swap.swap -->")
    text = text.replace(f"GENERATED: {second} -->", f"GENERATED: {first} -->")
    text = text.replace("GENERATED: swap.swap -->", f"GENERATED: {second} -->")
    (root / CHAPTER).write_text(_regenerated(root, text), encoding="utf-8")
    assert any("blocks are not in the order" in f for f in _findings_for(root, "generated-block"))


def test_link_with_another_class_is_markup_outside_the_positive_list() -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    attacked = chapter.replace("## 4. Aktionen\n", "[a](#x){: .hidden}\n\n## 4. Aktionen\n", 1)
    problems = _rendered_problems(attacked)
    assert any(p.startswith("markup outside the positive list") for p in problems), problems


@pytest.mark.parametrize(
    "line",
    [
        "``x`[a](#){: .hidden}`\n",
        "`` x <i>\n",
        "`` `<i>`\n",
        "`<i>``<i>``\n",
        "``a````<i>`\n",
    ],
)
def test_static_code_spans_follow_markdown(tmp_path: Path, line: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: channel_ops -->\n",
        "<!-- END GENERATED: channel_ops -->\n\n" + line,
    )
    assert _findings_for(root, "generated-block"), line


CATALOG_RECIPES = (
    "catalog: ## Regenerate the tables of chapter 05 and the lists in docs/api/redocly.yaml from"
    " docs/spec/actions.yaml\n"
    "\t$(UVRUN) python tools/catalog/action_catalog.py generate\n"
    "\n"
    "catalog-check: ## Action catalogue valid and its generated copies current (ADR 0049)\n"
    "\t$(UVRUN) python tools/catalog/action_catalog.py check\n"
)


def test_make_catalog_recipes_are_pinned() -> None:
    makefile = (REPO / "Makefile").read_text(encoding="utf-8")
    start = makefile.index("catalog: ##")
    end = makefile.index("\n\n", makefile.index("catalog-check: ##")) + 1
    assert makefile[start:end] == CATALOG_RECIPES


def _make_catalog_check(root: Path) -> tuple[int, str]:
    env = {k: v for k, v in os.environ.items() if k not in ("MAKEFLAGS", "MAKELEVEL", "MFLAGS")}
    # The environment and interpreter pytest runs in, so that uv neither creates one elsewhere
    # nor rebuilds this one for the Python of tools/.python-version.
    env["UV_PROJECT_ENVIRONMENT"] = sys.prefix
    env["UV_PYTHON"] = sys.executable
    proc = subprocess.run(
        ["make", "catalog-check"],
        cwd=root,
        env=env,
        capture_output=True,
        text=True,
        timeout=600,
        check=False,
    )
    return proc.returncode, proc.stdout + proc.stderr


def test_make_catalog_check_fails_on_a_stale_block(tmp_path: Path) -> None:
    # Through the real Makefile (recipe, `.IGNORE`, a second recipe) on a copy of the inputs.
    root = _copy_repo(tmp_path)
    shutil.copy(REPO / "Makefile", root / "Makefile")
    for rel in (".python-version", "pyproject.toml", "uv.lock", "catalog/action_catalog.py"):
        (root / "tools" / rel).parent.mkdir(parents=True, exist_ok=True)
        shutil.copy(REPO / "tools" / rel, root / "tools" / rel)
    environment = Path(sys.prefix, "pyvenv.cfg").read_bytes()
    code, output = _make_catalog_check(root)
    assert code == 0, output
    # uv kept the environment pytest runs in, also under another Python than .python-version
    assert Path(sys.prefix, "pyvenv.cfg").read_bytes() == environment
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    code, output = _make_catalog_check(root)
    assert code != 0, output
    assert "generated-block" in output, output


# ---- re-review: code spans as Python-Markdown finds them, blocks in their sections ----------

_SPAN_TOKENS = ("`", "``", "a", " ", "<i>", "\\")
# Longer lines from the reviews: fallback to the longest run, empty spans, and an opening that is
# only the rest of a run after an escaped backtick.
_SPAN_EXTRA = ("``a````<i>`", "`<i>``<i>``", "\\``a``<i>`", "\\``a``<i>``")


def _span_lines() -> list[str]:
    """Every line of up to five tokens with `<i>`, plus `_SPAN_EXTRA`; four leading spaces would
    be a code block."""
    lines = {
        "".join(parts)
        for size in range(1, 6)
        for parts in itertools.product(_SPAN_TOKENS, repeat=size)
        if "<i>" in parts
    }
    return sorted(line for line in lines | set(_SPAN_EXTRA) if not line.startswith("    "))


def test_static_code_spans_agree_with_the_renderer(tmp_path: Path) -> None:
    # The renderer decides: `<i>` is raw HTML exactly when the site would show it as a tag.
    lines = _span_lines()
    extensions, configs = _site_markdown_config()
    md = markdown.Markdown(extensions=extensions, extension_configs=configs)
    rendered = {line for line in lines if "<i>" in md.reset().convert(line)}
    root = _copy_repo(tmp_path)
    anchor = "<!-- END GENERATED: channel_ops -->\n"
    text = (root / CHAPTER).read_text(encoding="utf-8")
    first = text[: text.index(anchor) + len(anchor)].count("\n") + 2
    inserted = "".join(f"\n{line}\n" for line in lines)
    (root / CHAPTER).write_text(text.replace(anchor, anchor + inserted, 1), encoding="utf-8")
    flagged = {
        lines[(int(m.group(1)) - first) // 2]
        for f in _findings_for(root, "generated-block")
        if "HTML outside" in f and (m := re.search(r"\(line ([0-9]+)\)", f)) is not None
    }
    assert sorted(rendered - flagged) == []
    assert sorted(flagged - rendered) == []


# Gates run on any machine and under any load (P-07, docs/quality/gates.md): no test measures
# wall time. Where a guarantee cannot be counted, a test measures the process time of the one
# scan against this bound, far above the linear scan and far below the earlier quadratic or
# cubic regular expressions on the same line (seconds even on a fast core).
_LINEAR_SCAN_CPU_SECONDS = 1.0


def _cpu_seconds(work: Callable[[], object]) -> float:
    """Process time of `work`. A profiling timer, which counts process time like
    `time.process_time` and never waiting, stops `work` at the bound with a test failure, so
    that a regression to exponential or quadratic work fails within the bound instead of
    hanging the run."""

    def stop(signum: int, frame: object) -> None:
        pytest.fail(f"more than {_LINEAR_SCAN_CPU_SECONDS} s of process time")

    previous = signal.signal(signal.SIGPROF, stop)
    signal.setitimer(signal.ITIMER_PROF, _LINEAR_SCAN_CPU_SECONDS)
    try:
        start = time.process_time()
        work()
        return time.process_time() - start
    finally:
        signal.setitimer(signal.ITIMER_PROF, 0)
        signal.signal(signal.SIGPROF, previous)


def test_the_process_time_bound_stops_runaway_work() -> None:
    def spin() -> None:
        while True:
            pass

    with pytest.raises(pytest.fail.Exception, match="s of process time"):
        _cpu_seconds(spin)


def test_static_code_spans_scan_long_lines_in_linear_time() -> None:
    line = "a" + "`" * 1000 + "x" + "`" * 1001 + "b"
    # No run of the same length closes the span, so Markdown falls back to the longest later
    # run and renders one empty code span between a and b.
    assert action_catalog._without_code(line) == "ab"
    assert _cpu_seconds(lambda: action_catalog._without_code(line)) < _LINEAR_SCAN_CPU_SECONDS


def test_static_escapes_are_the_renderers() -> None:
    extensions, configs = _site_markdown_config()
    md = markdown.Markdown(extensions=extensions, extension_configs=configs)
    assert set(md.ESCAPED_CHARS) == action_catalog.MARKDOWN_ESCAPES


def _move_block(text: str, name: str, before: str) -> str:
    """Move block `name` with its markers and the blank line after it in front of `before`."""
    begin = f"<!-- BEGIN GENERATED: {name} -->\n"
    end = f"<!-- END GENERATED: {name} -->\n\n"
    start = text.index(begin)
    stop = text.index(end, start) + len(end)
    rest = text[:start] + text[stop:]
    at = rest.index(before)
    return rest[:at] + text[start:stop] + rest[at:]


def _swap(text: str, first: str, second: str) -> str:
    assert first in text
    assert second in text
    return text.replace(first, "\0").replace(second, first).replace("\0", second)


def _pseudo_headings(text: str) -> str:
    """Move the public table under 4.2 and give both tables headings the site renders as text."""
    text = _move_block(text, "actions.public", "<!-- BEGIN GENERATED: actions.instance -->\n")
    for name, title in (
        ("actions.public", "Öffentlich und Anmeldung"),
        ("actions.instance", "Instanz und Verwaltung"),
    ):
        begin = f"<!-- BEGIN GENERATED: {name} -->\n"
        text = text.replace(begin, f" ### {title}\n\n{begin}", 1)
    return text


MISPLACED = [
    pytest.param(
        lambda t: _move_block(t, "events.ephemeral", "<!-- BEGIN GENERATED: ws_ops -->\n"),
        {"events.ephemeral"},
        id="events-in-section-7",
    ),
    pytest.param(
        lambda t: _move_block(t, "rate_limit_classes", "### 4.1 Öffentlich und Anmeldung\n"),
        {"rate_limit_classes"},
        id="classes-in-section-4",
    ),
    pytest.param(
        lambda t: _swap(t, "### 4.1 Öffentlich und Anmeldung", "### 4.2 Instanz und Verwaltung"),
        {"actions.public", "actions.instance"},
        id="swapped-headings",
    ),
    pytest.param(_pseudo_headings, {"actions.public"}, id="headings-with-leading-spaces"),
    pytest.param(
        lambda t: _move_block(t, "events.ephemeral", "<!-- BEGIN GENERATED: ws_ops -->\n").replace(
            "<!-- BEGIN GENERATED: events.ephemeral -->\n",
            "Dauerhafte Ereignistypen und Empfangs-Aktion\n ---\n\n"
            "<!-- BEGIN GENERATED: events.ephemeral -->\n",
            1,
        ),
        {"events.ephemeral"},
        id="setext-with-leading-space",
    ),
    pytest.param(
        lambda t: t.replace(
            "<!-- BEGIN GENERATED: actions.public -->\n",
            "#### Instanz und Verwaltung\n\n<!-- BEGIN GENERATED: actions.public -->\n",
            1,
        ),
        {"actions.public"},
        id="level-4-heading-above-a-table",
    ),
    # The level-4 rule holds for every generated table, not only for action tables; the heading
    # stays above every table up to the next heading of level 2 or 3.
    *(
        pytest.param(
            lambda t, name=name: t.replace(
                f"<!-- BEGIN GENERATED: {name} -->\n",
                f"#### Hinweis\n\n<!-- BEGIN GENERATED: {name} -->\n",
                1,
            ),
            names,
            id=f"level-4-heading-above-{name}",
        )
        for name, names in (
            ("rate_limit_classes", {"rate_limit_classes"}),
            ("events.durable", {"events.durable", "events.ephemeral"}),
            ("events.ephemeral", {"events.ephemeral"}),
            ("ws_ops", {"ws_ops", "channel_ops"}),
            ("channel_ops", {"channel_ops"}),
        )
    ),
]


@pytest.mark.parametrize(("change", "names"), MISPLACED)
def test_generated_block_outside_its_section_fails(
    tmp_path: Path, change: Callable[[str], str], names: set[str]
) -> None:
    root = _copy_repo(tmp_path)
    (root / CHAPTER).write_text(change((root / CHAPTER).read_text(encoding="utf-8")), "utf-8")
    findings = _findings_for(root, "generated-block")
    placed = {
        m.group(1)
        for f in findings
        if (m := re.search(r"block '([a-z0-9_.]+)' is (?:in section|under heading)", f))
    }
    assert placed == names, findings
    assert not any("not in the order" in f for f in findings), findings
    with pytest.raises(CatalogError):
        action_catalog.generate(root)


@pytest.mark.parametrize(
    ("change", "names"),
    [
        *MISPLACED,
        pytest.param(
            lambda t: t.replace(
                "<!-- BEGIN GENERATED: actions.public -->\n",
                "- x\n\n    ### 4.2 Instanz und Verwaltung\n\n"
                "<!-- BEGIN GENERATED: actions.public -->\n",
                1,
            ),
            {"actions.public"},
            id="heading-in-a-list",
        ),
    ],
)
def test_rendered_tables_sit_in_their_sections(
    change: Callable[[str], str], names: set[str]
) -> None:
    problems = _rendered_problems(change((REPO / CHAPTER).read_text(encoding="utf-8")))
    placed = {m.group(1) for p in problems if (m := re.match(r"table of block '([a-z0-9_.]+)'", p))}
    assert placed == names, problems


def test_action_block_without_a_known_heading_fails(tmp_path: Path) -> None:
    # A new catalogue section needs its heading in ACTION_SECTION_TITLES.
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: actions.terminal -->\n",
        "<!-- END GENERATED: actions.terminal -->\n\n### 4.15 Neu\n\n"
        "<!-- BEGIN GENERATED: actions.neu -->\n<!-- END GENERATED: actions.neu -->\n",
    )
    findings = _findings_for(root, "generated-block")
    assert any("block 'actions.neu' has no heading in ACTION_SECTION_TITLES" in f for f in findings)


def test_redocly_block_order_is_free(tmp_path: Path) -> None:
    # The order of the lint rules does not change what Redocly enforces.
    root = _copy_repo(tmp_path)
    text = (root / REDOCLY).read_text(encoding="utf-8")
    key = re.compile(r"^  [^ #\n]", re.MULTILINE)
    start = text.index("\n  rule/rate-limit-class-known:\n") + 1
    stop = key.search(text, start + 1)
    assert stop is not None
    rule, text = text[start : stop.start()], text[:start] + text[stop.start() :]
    after = text.index("\n  rule/receive-action-known-webhook:\n") + 1
    end = key.search(text, after + 1)
    at = end.start() if end is not None else len(text)
    text = text[:at] + ("" if text[:at].endswith("\n") else "\n") + rule + text[at:]
    (root / REDOCLY).write_text(text, encoding="utf-8")
    catalog = action_catalog.load_catalog(root / CATALOG)
    moved = list(action_catalog.yaml_generated_blocks(text))
    assert moved != list(action_catalog.redocly_lists(catalog))
    assert action_catalog.check(root) == []


def test_section_titles_need_the_hash_in_the_first_column() -> None:
    # Python-Markdown renders an ATX heading only with `#` in the first column.
    text = " ## Neu\n   # Auch neu\n## 4. Aktionen\n"
    assert action_catalog.section_titles(text) == ["Aktionen"]


@pytest.mark.parametrize("level", [2, 3])
def test_static_headings_scan_long_lines_in_linear_time(level: int) -> None:
    line = "#" * level + " a" + " " * 60000 + "b"
    assert action_catalog._headings([line]) == [(level, "a" + " " * 60000 + "b")]
    assert _cpu_seconds(lambda: action_catalog._headings([line])) < _LINEAR_SCAN_CPU_SECONDS


@pytest.mark.parametrize(
    "char", ["\x02", "\x03", "\x0b", "\x0c", "\x1c", "\x1d", "\x1e", "\x85", "\u2028", "\u2029"]
)
def test_line_breaks_markdown_does_not_make_are_findings(tmp_path: Path, char: str) -> None:
    # str.splitlines breaks there and Markdown does not (or drops the character): the static
    # view would read other lines than the site renders.
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: channel_ops -->\n",
        f"<!-- END GENERATED: channel_ops -->\n\nx `a{char}`<i>`\n",
    )
    findings = _findings_for(root, "generated-block")
    assert any(f"character U+{ord(char):04X}" in f for f in findings), findings


def test_block_of_an_unknown_family_has_no_section(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "<!-- END GENERATED: channel_ops -->\n",
        "<!-- END GENERATED: channel_ops -->\n\n"
        "<!-- BEGIN GENERATED: neu -->\n<!-- END GENERATED: neu -->\n",
    )
    findings = _findings_for(root, "generated-block")
    assert any("block 'neu' has no section in BLOCK_SECTIONS" in f for f in findings), findings


@pytest.mark.parametrize(
    "text",
    [
        " ## Neu\n",
        "   # Auch neu\n",
        "#\tTab\n",
        "##\n",
        "## #\n",
        "## ##\n",
        "Kopie\n===\n",
        "Kopie\n ---\n",
        "Kopie\n---\t\n",
        "a\nKopie\n===\n",
        "## X\n---\n",
        "## X\nKopie\n---\n",
        "Text\n\nKopie\n---\n",
        "A\n---\nB\n---\n",
        "a | b\n---\n",
        "---\nKopie\n---\n",
        " ---\nKopie\n===\n",
        "* * *\nKopie\n---\n",
        "Absatz\nmehr\n---\nKopie\n---\n",
        "---\n---\n---\n",
        "## X\n---\n---\n===\n",
        "<!-- END GENERATED: x -->\nKopie\n---\n",
        "<!-- END GENERATED: x -->\n---\n",
        "Text\n\n    code\nKopie\n---\n",
        "Text\n\n---\t\nKopie\n---\n",
        "--\t--\t--\nKopie\n---\n",
        "Text\n\n- - -\t\nKopie\n---\n",
        "Text\n\n\xa0\n---\n",
        "Text\n\xa0\nKopie\n---\n",
        "[x]: https://example.org\nKopie\n===\n",
        "Kopie\n[x]: https://example.org\n===\n",
        "[x]: https://example.org\n---\n",
    ],
)
def test_static_headings_agree_with_the_renderer(text: str) -> None:
    rendered = [item for kind, item in _parse(text).items if kind == "heading"]
    assert action_catalog.section_titles(text) == rendered


@pytest.mark.parametrize("heading", ["##", "## #"])
def test_empty_heading_ends_a_hand_written_section(tmp_path: Path, heading: str) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "## 9. Tests,", f"{heading}\n\n{SMUGGLED}\n## 9. Tests,")
    findings = _findings_for(root, "generated-block")
    assert any("table row outside the generated blocks in section ''" in f for f in findings)


class _ScanCountingText(str):
    """A text that adds up how many characters its searches and slices look at.

    Lets a test hold, without a clock, that line numbers come from one running count: each
    character is looked at once, where a count from the start for every finding would look at
    the text again and again.
    """

    scanned: int

    def __new__(cls, value: str) -> _ScanCountingText:
        text = super().__new__(cls, value)
        text.scanned = 0
        return text

    def _scan(self, start: SupportsIndex | None, end: SupportsIndex | None) -> None:
        low, high, _ = slice(start, end).indices(len(self))
        self.scanned += max(0, high - low)

    def count(
        self, sub: str, start: SupportsIndex | None = None, end: SupportsIndex | None = None, /
    ) -> int:
        self._scan(start, end)
        return super().count(sub, start, end)

    def find(
        self, sub: str, start: SupportsIndex | None = None, end: SupportsIndex | None = None, /
    ) -> int:
        self._scan(start, end)
        return super().find(sub, start, end)

    def rfind(
        self, sub: str, start: SupportsIndex | None = None, end: SupportsIndex | None = None, /
    ) -> int:
        self._scan(start, end)
        return super().rfind(sub, start, end)

    def splitlines(self, keepends: bool = False) -> list[str]:
        self.scanned += len(self)
        return super().splitlines(keepends)

    def __getitem__(self, key: SupportsIndex | slice) -> str:
        if isinstance(key, slice):
            self.scanned += len(range(*key.indices(len(self))))
        return super().__getitem__(key)


def test_foreign_line_breaks_are_numbered_by_one_running_count() -> None:
    chapter = (REPO / CHAPTER).read_text(encoding="utf-8")
    first = chapter.count("\n") + 1  # the chapter ends with a newline
    text = _ScanCountingText(chapter + "x\u2028y\n" * 3000)
    findings = action_catalog._line_break_findings(text)
    numbers = [int(m.group(1)) for f in findings if (m := re.search(r"\(line ([0-9]+)\)", f))]
    assert numbers == list(range(first, first + 3000))
    assert text.scanned <= len(text)


# ---- hand-written readings: what the site shows in 2.6, 2.7 and the configuration table ----

_SCOPE_TABLE_START = "| Aktionen | Scope |\n|---|---|\n"


def _fake_scope_table(chapter: str) -> str:
    """A copy of table 2.6 without separator row (a paragraph on the site) before the real
    table, whose first scope is changed: a reader of the first `|` lines sees the copy."""
    start = chapter.index(_SCOPE_TABLE_START)
    end = chapter.index("\n\n", start) + 1
    table = chapter[start:end]
    fake = table.replace("|---|---|\n", "| x | y |\n", 1)
    real = table.replace("| `messages:read` |", "| `messages:write` |", 1)
    return chapter[:start] + fake + "\n" + real + chapter[end:]


def _no_separator_row(chapter: str) -> str:
    start = chapter.index(_SCOPE_TABLE_START)
    rest = chapter[start + len(_SCOPE_TABLE_START) :]
    return chapter[:start] + "| Aktionen | Scope |\n| x | y |\n" + rest


HAND_READINGS = [
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n", "`terminal.read_own`.\n* `auth.login.begin`.\n", 1
        ),
        "effect",
        "list marker other than '- '",
        id="other-list-marker-in-2.7",
    ),
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n", "`terminal.read_own`.\n - `auth.login.begin`.\n", 1
        ),
        "effect",
        "list marker other than '- '",
        id="indented-dash-in-2.7",
    ),
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n", "`terminal.read_own`.\n-\t`auth.login.begin`.\n", 1
        ),
        "effect",
        "list marker other than '- '",
        id="dash-and-tab-in-2.7",
    ),
    pytest.param(
        lambda t: t.replace(
            "\nAgenten haben alle Scopes",
            "\n| Aktionen | Scope |\n|---|---|\n| `instance.discovery.read` | `messages:write` |\n"
            "\nAgenten haben alle Scopes",
            1,
        ),
        "app-scope",
        "must hold one table, found 2",
        id="second-table-in-2.6",
    ),
    pytest.param(_fake_scope_table, "app-scope", "found 2", id="table-copy-in-2.6"),
    pytest.param(_no_separator_row, "app-scope", "no separator row", id="no-separator-in-2.6"),
    pytest.param(
        lambda t: t.replace("|---|---|\n| `message.read`", "|---|\n| `message.read`", 1),
        "app-scope",
        "no separator row for 2 columns",
        id="short-separator-in-2.6",
    ),
    pytest.param(
        lambda t: t.replace("| Aktionen | Scope |\n", "| Aktionen | Scope |\u00a0\n", 1),
        "app-scope",
        "header ['Aktionen', 'Scope', '']",
        id="no-break-space-after-the-header-of-2.6",
    ),
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n", "`terminal.read_own`.\n    - `message.create`.\n", 1
        ),
        "effect",
        "list marker other than '- '",
        id="nested-item-in-2.7",
    ),
    pytest.param(
        lambda t: (
            t + "\n| Name | Standard | Bedeutung |\n|---|---|---|\n"
            "| `ratelimit.rl.read.per_minute` / `.burst` | `1` / `1` | x |\n"
        ),
        "rate-limit-config",
        "must hold one table, found 2",
        id="second-table-in-configuration",
    ),
    # an item after a blank line belongs to the same (loose) list on the site and is read
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n", "`terminal.read_own`.\n\n- `auth.login.begin`.\n", 1
        ),
        "effect",
        "auth.login.begin: effect write differs from 05 2.7 (read)",
        id="item-after-a-blank-line-in-2.7",
    ),
]


@pytest.mark.parametrize(("change", "rule", "message"), HAND_READINGS)
def test_hand_written_readings_fail_closed(
    tmp_path: Path, change: Callable[[str], str], rule: str, message: str
) -> None:
    root = _copy_repo(tmp_path)
    (root / CHAPTER).write_text(change((root / CHAPTER).read_text(encoding="utf-8")), "utf-8")
    findings = _findings_for(root, rule)
    assert any(message in f for f in findings), findings


@pytest.mark.parametrize(
    "change",
    [
        # an item after a blank line is read like the site shows it
        *(p.values[0] for p in HAND_READINGS if p.id != "item-after-a-blank-line-in-2.7"),
        # a table without leading pipes: the static view does not count it, the site renders it
        lambda t: t.replace(
            "\nAgenten haben alle Scopes",
            "\nAktionen | Scope\n---|---\n`instance.discovery.read` | `messages:write`\n"
            "\nAgenten haben alle Scopes",
            1,
        ),
    ],
)
def test_rendered_hand_written_readings_match_the_checks(change: Callable[[str], str]) -> None:
    problems = _rendered_problems(change((REPO / CHAPTER).read_text(encoding="utf-8")))
    assert any(p.startswith("hand-written reading") for p in problems), problems


_EXTRA_ITEM = "- `message.create` und `message.delete`.\n"
_REAL_27 = "### 2.7 Wirkung (`effect`)\n"


# Copies of the configuration title with a digit for a letter, one per line (a word with "api"
# next to a token on one line looks like a key to gitleaks).
_DIGIT_COPIES = (
    "Konfigurationswerte dieses Kapite1s",
    "K0nfigurati0nswerte dieses Kapitels",
)


def _copy_before_27(title: str) -> Callable[[str], str]:
    """A heading `title` with a made-up list right before the real 2.7."""
    return lambda t: t.replace(_REAL_27, f"{title}\n\n{_EXTRA_ITEM}\n{_REAL_27}", 1)


# Changes that let a reader see 2.6 or 2.7 end, or hold, other than the site shows: a heading
# without visible title (empty, invisible characters), a heading in a list item, a copy of the
# title (exact, on another level or looking the same), a changed header row.
SECTION_EDGES = [
    pytest.param(
        lambda t: t.replace(
            "\nAgenten haben alle Scopes",
            "\n### \n\n| Aktionen | Scope |\n|---|---|\n| `file.read` | `messages:write` |\n"
            "\nAgenten haben alle Scopes",
            1,
        ),
        "heading without a visible title",
        "<h3> without a visible title",
        id="empty-heading-in-2.6",
    ),
    pytest.param(
        lambda t: t.replace(
            "\nAgenten haben alle Scopes",
            "\n###\n\nAktionen | Scope\n---|---\n`file.read` | `messages:write`\n"
            "\nAgenten haben alle Scopes",
            1,
        ),
        "heading without a visible title",
        "<h3> without a visible title",
        id="bare-hashes-in-2.6",
    ),
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n",
            "`terminal.read_own`.\n\n### \n\n- `message.create` und `message.delete`.\n",
            1,
        ),
        "heading without a visible title",
        "<h3> without a visible title",
        id="empty-heading-in-2.7",
    ),
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n", "`terminal.read_own`.\n- ### `message.create`\n", 1
        ),
        "list item holds a heading",
        "inside a list item or quote",
        id="heading-in-an-item-of-2.7",
    ),
    pytest.param(
        lambda t: t.replace(
            "| Aktionen | Scope |\n|---|---|\n",
            "| `file.read` | `messages:write` |\n|---|---|\n",
            1,
        ),
        "header",
        "hand-written reading of 'Scopes für Apps': the site shows",
        id="header-row-of-2.6",
    ),
    pytest.param(
        lambda t: t.replace(
            "## 9. Tests,",
            "### 8.1 Scopes für Apps\n\n| Aktionen | Scope |\n|---|---|\n"
            "| `file.read` | `messages:write` |\n\n## 9. Tests,",
            1,
        ),
        "appears 2 times",
        "title rendered 2 times",
        id="copied-title-of-2.6",
    ),
    pytest.param(
        lambda t: t.replace(
            "Alle anderen haben `write`.",
            "#### Scopes für Apps\n\n| Aktionen | Scope |\n|---|---|\n"
            "| `file.read` | `messages:write` |\n\nAlle anderen haben `write`.",
            1,
        ),
        "appears 2 times",
        "title rendered 2 times",
        id="unnumbered-h4-copy-of-2.6",
    ),
    *(
        pytest.param(
            lambda t, title=title: t.replace(
                "`terminal.read_own`.\n",
                f"`terminal.read_own`.\n\n### {title}\n\n{_EXTRA_ITEM}",
                1,
            ),
            "heading without a visible title",
            "<h3> without a visible title",
            id=f"invisible-title-U+{ord(title):04X}-in-2.7",
        )
        for title in ("\u200b", "\u2060", "\u00ad", "\u2800", "\u3164")
    ),
    pytest.param(
        lambda t: t.replace(
            "`terminal.read_own`.\n",
            "`terminal.read_own`.\n\n## \u200b\n\n- `message.create` und `message.delete`.\n",
            1,
        ),
        "heading without a visible title",
        "<h2> without a visible title",
        id="invisible-h2-after-2.7",
    ),
    pytest.param(
        _copy_before_27("### 2.7 Wirkung  (`effect`)"),
        "look the same",
        "title rendered 2 times",
        id="copy-of-2.7-with-two-spaces",
    ),
    pytest.param(
        _copy_before_27("### 2.7 Wirkung\u00a0(`effect`)"),
        "look the same",
        "title rendered 2 times",
        id="copy-of-2.7-with-no-break-space",
    ),
    pytest.param(
        _copy_before_27("### 2.7 Wirkung (`effect`)\u200b"),
        "look the same",
        "title rendered 2 times",
        id="copy-of-2.7-with-zero-width-space",
    ),
    pytest.param(
        _copy_before_27("### 2.7 Wir\u00adkung (`effect`)"),
        "look the same",
        "title rendered 2 times",
        id="copy-of-2.7-with-soft-hyphen",
    ),
    pytest.param(
        _copy_before_27("### 2.7 Wirkung (`effect`)&#8203;"),
        "character outside the positive list",
        "title rendered 2 times",
        id="copy-of-2.7-with-entity",
    ),
    pytest.param(
        _copy_before_27("### 2.7 Wirkung (`\u0435ffect`)"),
        "character outside the positive list",
        "character outside the positive list",
        id="copy-of-2.7-with-cyrillic-letter",
    ),
    *(
        pytest.param(
            _copy_before_27(f"### {number} Wirkung (`effect`)"),
            "look the same",
            "title rendered 2 times",
            id=f"copy-of-2.7-numbered-{number}",
        )
        for number in ("2,7", "2.7:", "2.7)", "(2.7)")
    ),
    pytest.param(
        lambda t: t.replace(
            "## 9. Tests,",
            "### 8.1 Konfigurationswerte dieses KapiteIs\n\n| Name | Standard | Bedeutung |\n"
            "|---|---|---|\n| `ratelimit.rl.read.per_minute` / `.burst` | `1` / `1` | x |\n"
            "\n## 9. Tests,",
            1,
        ),
        "look the same",
        "title rendered 2 times",
        id="copy-of-the-configuration-title-with-capital-i",
    ),
    *(
        pytest.param(
            lambda t, title=title: t.replace(
                "## 9. Tests,",
                f"### 8.1 {title}\n\n| Name | Standard | Bedeutung |\n"
                "|---|---|---|\n| `ratelimit.rl.read.per_minute` / `.burst` | `1` / `1` | x |\n"
                "\n## 9. Tests,",
                1,
            ),
            "look the same",
            "title rendered 2 times",
            id=f"copy-of-the-configuration-title-with-digits-{title}",
        )
        for title in _DIGIT_COPIES
    ),
]

# Changes only the rendered page shows: entities that render as invisible titles, a heading in
# a quote.
RENDERED_EDGES = [
    *(
        pytest.param(
            lambda t, title=title: t.replace(
                "`terminal.read_own`.\n",
                f"`terminal.read_own`.\n\n### {title}\n\n{_EXTRA_ITEM}",
                1,
            ),
            "<h3> without a visible title",
            id=f"entity-title-{title}",
        )
        for title in ("&#8203;", "&shy;", "&zwj;", "&#10240;")
    ),
    pytest.param(
        lambda t: t.replace(
            "\nAgenten haben alle Scopes",
            "\n### &#8203;\n\n| Aktionen | Scope |\n|---|---|\n| `file.read` | `messages:write` |\n"
            "\nAgenten haben alle Scopes",
            1,
        ),
        "<h3> without a visible title",
        id="entity-title-in-2.6",
    ),
    pytest.param(
        lambda t: t.replace("`terminal.read_own`.\n", "`terminal.read_own`.\n\n> ### Zitat\n", 1),
        "inside a list item or quote",
        id="heading-in-a-quote",
    ),
]


@pytest.mark.parametrize(("change", "rendered"), RENDERED_EDGES)
def test_rendered_page_shows_invisible_and_quoted_headings(
    change: Callable[[str], str], rendered: str
) -> None:
    problems = _rendered_problems(change((REPO / CHAPTER).read_text(encoding="utf-8")))
    assert any(rendered in p for p in problems), problems


def test_a_level_4_heading_does_not_end_2_7(tmp_path: Path) -> None:
    # Like on the site, `####` belongs to 2.7, so the item below it is read (and contradicts
    # the catalogue here).
    root = _copy_repo(tmp_path)
    _edit(
        root,
        CHAPTER,
        "`terminal.read_own`.\n",
        "`terminal.read_own`.\n\n#### Mehr\n\n- `message.create`.\n",
    )
    findings = _findings_for(root, "effect")
    assert any("message.create: effect write differs" in f for f in findings), findings


def test_look_alike_headings_are_named_in_the_finding(tmp_path: Path) -> None:
    # Titles that differ only in digits look the same to the checks; the finding names both
    # headings instead of saying that one title appears twice.
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "\n## 10. ", "\n### 9.1 Scopes für Apps 2\n\nText.\n\n## 10. ")
    lines = (root / CHAPTER).read_text(encoding="utf-8").splitlines()
    real = lines.index("### 2.6 Scopes für Apps") + 1
    copy = lines.index("### 9.1 Scopes für Apps 2") + 1
    assert _findings_for(root, "app-scope") == [
        f"actions.yaml: app-scope: section 'Scopes für Apps': headings 'Scopes für Apps' (line"
        f" {real}), 'Scopes für Apps 2' (line {copy}) in chapter 05 look the same (only the"
        " letters of a title count, without case, with I as l, 1 as l and 0 as o)"
    ]


def test_a_heading_with_a_tab_ends_a_read_section_like_on_the_site(tmp_path: Path) -> None:
    # `###\tFoo` is a visible heading on the site, so 2.7 ends there for the readers too.
    change = "`terminal.read_own`.\n\n###\tFoo\n\n- `message.create`.\n"
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "`terminal.read_own`.\n", change)
    assert _findings_for(root, "effect") == []
    chapter = (
        (REPO / CHAPTER).read_text(encoding="utf-8").replace("`terminal.read_own`.\n", change, 1)
    )
    assert not any(p.startswith("hand-written reading") for p in _rendered_problems(chapter))


@pytest.mark.parametrize(("change", "message", "rendered"), SECTION_EDGES)
def test_read_sections_end_where_the_site_ends_them(
    tmp_path: Path, change: Callable[[str], str], message: str, rendered: str
) -> None:
    root = _copy_repo(tmp_path)
    (root / CHAPTER).write_text(change((root / CHAPTER).read_text(encoding="utf-8")), "utf-8")
    findings = action_catalog.check(root)
    assert any(message in f for f in findings), findings


@pytest.mark.parametrize(("change", "message", "rendered"), SECTION_EDGES)
def test_rendered_read_sections_end_where_the_site_ends_them(
    change: Callable[[str], str], message: str, rendered: str
) -> None:
    problems = _rendered_problems(change((REPO / CHAPTER).read_text(encoding="utf-8")))
    assert any(rendered in p for p in problems), problems


# ---- red team: a cell stays one cell, as the site's `tables` splits a row ------------------


def _site_markdown() -> markdown.Markdown:
    extensions, configs = _site_markdown_config()
    return markdown.Markdown(extensions=extensions, extension_configs=configs)


class _Paragraphs(HTMLParser):
    """Text of every paragraph, code spans kept as backticks and whitespace joined as in the
    cells `_Tables` reads."""

    def __init__(self) -> None:
        super().__init__()
        self.texts: list[str] = []
        self._text: list[str] | None = None

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag == "p":
            self._text = []
        elif tag == "code" and self._text is not None:
            self._text.append("`")

    def handle_endtag(self, tag: str) -> None:
        if tag == "code" and self._text is not None:
            self._text.append("`")
        elif tag == "p" and self._text is not None:
            self.texts.append(" ".join("".join(self._text).split()))
            self._text = None

    def handle_data(self, data: str) -> None:
        if self._text is not None:
            self._text.append(data)


def _shown_alone(cells: Sequence[str]) -> list[str]:
    """What the site shows for each text outside a table: each its own paragraph after `x `,
    so that no text starts a heading or a list, the `x` taken off again."""
    parser = _Paragraphs()
    parser.feed(_site_markdown().convert("".join(f"x {cell}\n\n" for cell in cells)))
    assert len(parser.texts) == len(cells)
    assert all(text == "x" or text.startswith("x ") for text in parser.texts)
    return [text[2:] for text in parser.texts]


def _cell_mismatches(blocks: Mapping[str, str], cells: Mapping[str, list[list[str]]]) -> list[str]:
    """Rows where the rendered table of a block shows other cells than the catalogue gives."""
    problems: list[str] = []
    for name, rows in cells.items():
        tables = [item for kind, item in _render(blocks[name]) if kind == "table"]
        if len(tables) != 1:
            problems.append(f"{name}: the site shows {len(tables)} tables")
            continue
        if len(tables[0]) != len(rows):
            problems.append(f"{name}: the site shows {len(tables[0])} rows, not {len(rows)}")
        for number, (shown, row) in enumerate(zip(tables[0], rows, strict=False)):
            wanted = tuple(_shown_alone(row))
            if shown != wanted:
                problems.append(f"{name} row {number}: the site shows {shown}, not {wanted}")
    return problems


def test_rendered_generated_tables_show_the_catalogue_cell_by_cell() -> None:
    catalog = action_catalog.load_catalog(REPO / CATALOG)
    blocks = action_catalog.generated_blocks((REPO / CHAPTER).read_text(encoding="utf-8"))
    cells = action_catalog.chapter_table_cells(catalog)
    assert set(cells) == CHAPTER_BLOCKS
    assert _cell_mismatches(blocks, cells) == []


# The two cells of the red team: an unpaired backtick and an even run of backslashes before `|`.
RED_TEAM_CELLS = ("Nachricht `lesen", "Nachricht lesen \\\\| Kanal")


@pytest.mark.parametrize("description", RED_TEAM_CELLS)
def test_cell_by_cell_comparison_sees_what_a_weaker_cell_rule_lets_through(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, description: str
) -> None:
    # The comparison with the renderer holds the class on its own: with the static cell rule
    # switched off, the broken row of message.read is reported.
    monkeypatch.setattr(action_catalog, "_cell_problem", lambda text: None)
    root = _copy_repo(tmp_path)
    _mutate(
        root, lambda d: _entry(d, "actions", "id", "message.read").update(description=description)
    )
    catalog = action_catalog.load_catalog(root / CATALOG)
    problems = _cell_mismatches(
        action_catalog.render_chapter_blocks(catalog), action_catalog.chapter_table_cells(catalog)
    )
    assert len(problems) == 1, problems
    assert problems[0].startswith("actions.messages row 1: the site shows ('`message.read`',")


@pytest.mark.parametrize(
    ("description", "needle"),
    [
        (RED_TEAM_CELLS[0], "the backtick run at character 11 opens a code span"),
        (RED_TEAM_CELLS[1], "the | at character 19 ends the cell on the site"),
        ("a\\\\`b", "the backtick run at character 4 opens a code span"),
        ("\\`a | `b`", "the | at character 5 ends the cell on the site"),
    ],
)
def test_cell_that_the_site_would_split_fails_with_one_line(
    tmp_path: Path, description: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root, lambda d: _entry(d, "actions", "id", "message.read").update(description=description)
    )
    findings = action_catalog.check(root)
    [finding] = [f for f in findings if "message.read" in f]
    assert finding.startswith(
        "actions.yaml: schema: message.read: description (column Beschreibung): "
    )
    assert needle in finding
    assert "\n" not in finding


# Markup the site does not show as text of the cell: raw HTML can swallow the table or end the
# cell early, and a brace pair is an attribute list that takes text out of the cell or hides it.
MARKUP_CELLS = (
    "Nachricht lesen {Entwurf}",
    "{Kanal}",
    "Nachricht lesen {: hidden }",
    "a {x}`y`",
    "Nachricht lesen <script>",
    "Nachricht lesen <!-- x",
    "x </p> y",
    "a\\<b>c</b>",
)


@pytest.mark.parametrize("description", MARKUP_CELLS)
def test_cell_with_html_or_an_attribute_list_fails_with_one_line(
    tmp_path: Path, capsys: pytest.CaptureFixture[str], description: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root, lambda d: _entry(d, "actions", "id", "message.read").update(description=description)
    )
    before = _contents(root)
    assert action_catalog.main(["check", "--root", str(root)]) == 1
    [finding] = capsys.readouterr().err.splitlines()
    assert finding.startswith(
        "actions.yaml: schema: message.read: description (column Beschreibung): holds "
    )
    needle = "HTML outside a code span" if "<" in description else "a brace pair outside"
    assert needle in finding
    with pytest.raises(CatalogError):
        action_catalog.generate(root)
    assert _contents(root) == before


@pytest.mark.parametrize(
    "description",
    [
        "a \\| b",
        "a \\\\\\| b",
        "`a|b` c",
        "``a`|`b``",
        "a\\` b",
        "a < b",
        "a <3",
        "`{id}` lesen",
        "`<b>` code",
        "a \\{x\\} b",
        "a {x b",
    ],
)
def test_cell_that_stays_one_cell_on_the_site_passes(tmp_path: Path, description: str) -> None:
    root = _copy_repo(tmp_path)
    _mutate(
        root, lambda d: _entry(d, "actions", "id", "message.read").update(description=description)
    )
    assert _catalog_findings(root, "schema") == []
    assert action_catalog.generate(root) == [CHAPTER.as_posix()]


# Cells of up to five of these pieces, between a plain cell and a cell with code spans of every
# run length the pieces can form, so that a run the cell leaves open finds a partner further
# right and swallows the `|` between, as in a generated row.
_CELL_PIECES = ("`", "``", "\\", "|", "a", " ")
_CLOSING_CELL = " ".join("`" * n + "y" + "`" * n for n in range(1, 11))


def _cell_texts() -> list[str]:
    texts = {
        "".join(parts)
        for size in range(1, 6)
        for parts in itertools.product(_CELL_PIECES, repeat=size)
    }
    return sorted(t for t in texts if t == t.strip())


def test_static_cell_rule_agrees_with_the_renderer() -> None:
    texts = _cell_texts()
    rows = [["h", "h", "h"], ["---"] * 3] + [["x", text, _CLOSING_CELL] for text in texts]
    page = "".join("| " + " | ".join(row) + " |\n" for row in rows)
    [table] = [item for kind, item in _render(page) if kind == "table"]
    [closing, *alone] = _shown_alone([_CLOSING_CELL, *texts])
    disagree = [
        text
        for text, shown, wanted in zip(texts, table[1:], alone, strict=True)
        if (shown == ("x", wanted, closing)) != (action_catalog._cell_problem(text) is None)
    ]
    assert disagree == []


# ---- red team: YAML that PyYAML reads, files written together, findings in stages --------

LINT_IGNORE = Path("docs/api/.redocly.lint-ignore.yaml")


def _alias_ladder(depth: int) -> str:
    """Top-level keys whose aliases nest `depth` times: a walk that follows every alias anew
    visits 2**depth nodes, one that visits each node once only `depth` lists."""
    lines = ["a0: &a0 [x, x]", *(f"a{n}: &a{n} [*a{n - 1}, *a{n - 1}]" for n in range(1, depth))]
    return "".join(f"{line}\n" for line in lines)


@pytest.mark.parametrize(
    ("rel", "anchor", "insert", "needle"),
    [
        (CATALOG, "version: 1\n", "loop: &loop [*loop]\n", "actions.yaml: schema: line "),
        (
            REDOCLY,
            "\nrules:\n",
            "\nloop: &loop {a: *loop}\n",
            "docs/api/redocly.yaml: generated-block: line ",
        ),
    ],
)
def test_recursive_alias_is_a_finding(
    tmp_path: Path, rel: Path, anchor: str, insert: str, needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, rel, anchor, anchor + insert if rel == CATALOG else insert + anchor[1:])
    findings = action_catalog.check(root)
    assert any(f.startswith(needle) and f.endswith(": recursive alias") for f in findings), findings


def test_nested_aliases_in_the_catalogue_are_read_in_linear_time(tmp_path: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, CATALOG, "version: 1\n", "version: 1\n" + _alias_ladder(26))
    _edit(root, CATALOG, "- op: identify\n", "- op: identify\n  also_actions: [*a25]\n")
    findings: list[str] = []
    assert _cpu_seconds(lambda: findings.extend(action_catalog.check(root))) < (
        _LINEAR_SCAN_CPU_SECONDS
    )
    assert "actions.yaml: schema: unknown key 'a0'" in findings
    assert any("identify: also_actions item [[[" in f for f in findings), findings


def test_nested_aliases_in_the_redocly_configuration_are_read_in_linear_time(
    tmp_path: Path,
) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, REDOCLY, "\nrules:\n", "\n" + _alias_ladder(26) + "rules:\n")
    findings: list[str] = []
    assert _cpu_seconds(lambda: findings.extend(action_catalog.check(root))) < (
        _LINEAR_SCAN_CPU_SECONDS
    )
    assert any(
        f.endswith("key 'a0' is not allowed (allowed: extends, apis, rules)") for f in findings
    )


def _value_error(build: Callable[[], object]) -> str:
    """The message of the ValueError `build` raises; its wording differs between Pythons."""
    try:
        build()
    except ValueError as exc:
        return " ".join(str(exc).split())
    pytest.fail("no ValueError")


@pytest.mark.parametrize(
    ("value", "build"),
    [
        pytest.param("2026-02-30", lambda: datetime.date(2026, 2, 30), id="invalid-date"),
        pytest.param("9" * 5000, lambda: int("9" * 5000), id="huge-integer"),
    ],
)
@pytest.mark.parametrize("rel", [CATALOG, REDOCLY, LINT_IGNORE])
def test_values_python_cannot_build_are_findings(
    tmp_path: Path, rel: Path, value: str, build: Callable[[], object]
) -> None:
    root = _copy_repo(tmp_path)
    path = root / rel
    path.write_text(
        f"when: {value}\n" + (path.read_text("utf-8") if path.exists() else ""), "utf-8"
    )
    findings = action_catalog.check(root)
    label = "actions.yaml: schema" if rel == CATALOG else f"{rel.as_posix()}: generated-block"
    assert findings == [f"{label}: cannot parse: {_value_error(build)}"]
    with pytest.raises(CatalogError):
        action_catalog.generate(root)


@pytest.mark.parametrize("rel", [CATALOG, REDOCLY, LINT_IGNORE])
def test_nesting_deeper_than_the_recursion_limit_is_a_finding(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, rel: Path
) -> None:
    # The pure-Python composer of PyYAML, used where libyaml is missing, recurses per level;
    # the checks themselves walk without recursion.
    monkeypatch.setattr(action_catalog, "_COMPOSER", yaml.SafeLoader)
    root = _copy_repo(tmp_path)
    path = root / rel
    deep = "when: " + "[" * 3000 + "]" * 3000 + "\n"
    path.write_text(deep + (path.read_text("utf-8") if path.exists() else ""), "utf-8")
    label = "actions.yaml: schema" if rel == CATALOG else f"{rel.as_posix()}: generated-block"
    assert f"{label}: cannot parse: nested too deeply" in action_catalog.check(root)


@pytest.mark.parametrize("rel", [CATALOG, REDOCLY])
def test_a_yaml_syntax_error_is_one_finding_on_one_line(tmp_path: Path, rel: Path) -> None:
    root = _copy_repo(tmp_path)
    _edit(root, rel, "\n", "\nbroken: [\n")
    findings = [f for f in action_catalog.check(root) if "cannot parse" in f]
    assert len(findings) == 1, findings
    assert "\n" not in findings[0]
    assert re.search(r": cannot parse: line [0-9]+, column [0-9]+: ", findings[0]), findings


def test_every_finding_is_one_line(tmp_path: Path) -> None:
    # A value with a line break in a field that is no table cell still yields one line each.
    root = _copy_repo(tmp_path)
    _mutate(
        root,
        lambda d: _entry(d, "events", "type", "message.created").update(
            receive_action="message\nread", receive_text="`message.read`"
        ),
    )
    findings = action_catalog.check(root)
    assert any("receive_text does not name message read" in f for f in findings), findings
    assert all("\n" not in f for f in findings), findings
    with pytest.raises(CatalogError) as err:
        action_catalog.load_catalog(root / CATALOG)
    assert all("\n" not in f for f in err.value.findings)


@pytest.mark.parametrize(
    ("change", "needle"),
    [
        (
            lambda d: _channel_rows(d, "host_data")[0].update(ops=["hello\n"]),
            "host_data: ops item 'hello\\n' does not match",
        ),
        (
            lambda d: _entry(d, "ws_ops", "op", "heartbeat").update(
                also_actions=["agent.heartbeat\n"]
            ),
            "heartbeat: also_actions item 'agent.heartbeat\\n' does not match",
        ),
    ],
)
def test_a_name_with_a_trailing_line_break_fails(
    tmp_path: Path, change: Callable[[dict[str, Any]], None], needle: str
) -> None:
    root = _copy_repo(tmp_path)
    _mutate(root, change)
    _assert_finding(root, "schema", needle)


def _stale_copy(tmp_path: Path) -> Path:
    """A copy whose chapter block and redocly list are both stale, so generate rewrites both."""
    root = _copy_repo(tmp_path)
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    _edit(root, REDOCLY, "        - access.explain\n", "")
    return root


def _contents(root: Path) -> dict[Path, bytes]:
    return {rel: (root / rel).read_bytes() for rel in (CHAPTER, REDOCLY)}


@pytest.mark.skipif(os.geteuid() == 0, reason="root may write read-only files")
def test_generate_writes_both_files_or_neither(tmp_path: Path) -> None:
    root = _stale_copy(tmp_path)
    before = _contents(root)
    (root / REDOCLY).chmod(0o444)
    try:
        with pytest.raises(CatalogError) as err:
            action_catalog.generate(root)
    finally:
        (root / REDOCLY).chmod(0o644)
    assert err.value.findings == [
        "docs/api/redocly.yaml: generated-block: cannot write: not writable"
    ]
    assert _contents(root) == before


def test_generate_restores_a_replaced_file_when_the_next_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    root = _stale_copy(tmp_path)
    before = _contents(root)
    replace = os.replace
    calls: list[str] = []

    def flaky(src: str, dst: str) -> None:
        calls.append(str(dst))
        if len(calls) == 2:
            raise OSError("disk full")
        replace(src, dst)

    monkeypatch.setattr(os, "replace", flaky)
    with pytest.raises(CatalogError) as err:
        action_catalog.generate(root)
    monkeypatch.undo()
    assert err.value.findings == ["docs/api/redocly.yaml: generated-block: cannot write: disk full"]
    assert _contents(root) == before
    leftovers = [p for p in (root / "docs").rglob("*") if p.is_file() and p.name.startswith(".")]
    assert leftovers == []
    assert action_catalog.generate(root) == [CHAPTER.as_posix(), REDOCLY.as_posix()]


def _replace_failing_at(
    monkeypatch: pytest.MonkeyPatch, call: int, error: BaseException
) -> list[str]:
    """Make the `call`-th `os.replace` raise `error`; the others replace as usual."""
    replace = os.replace
    calls: list[str] = []

    def failing(src: str, dst: str) -> None:
        calls.append(str(dst))
        if len(calls) == call:
            raise error
        replace(src, dst)

    monkeypatch.setattr(os, "replace", failing)
    return calls


def _temporary_files(root: Path) -> list[Path]:
    return [p for p in (root / "docs").rglob("*") if p.is_file() and p.name.startswith(".")]


@pytest.mark.parametrize("call", [1, 2])
def test_generate_cleans_up_after_an_interrupt_and_raises_it_again(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, call: int
) -> None:
    # Interrupted before the first or the second replacement: no temporary file stays, the
    # files keep their old text, and the interrupt itself reaches the caller.
    root = _stale_copy(tmp_path)
    before = _contents(root)
    _replace_failing_at(monkeypatch, call, KeyboardInterrupt())
    with pytest.raises(KeyboardInterrupt):
        action_catalog.generate(root)
    monkeypatch.undo()
    assert _temporary_files(root) == []
    assert _contents(root) == before


def test_an_interrupt_notes_a_file_whose_old_text_could_not_be_restored(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    root = _stale_copy(tmp_path)
    _replace_failing_at(monkeypatch, 2, KeyboardInterrupt())

    def no_space(self: Path, *args: object, **kwargs: object) -> int:
        raise OSError("disk full")

    monkeypatch.setattr(Path, "write_text", no_space)
    with pytest.raises(KeyboardInterrupt) as err:
        action_catalog.generate(root)
    monkeypatch.undo()
    assert err.value.__notes__ == [
        "docs/spec/05-aktionen.md: generated-block: old text not restored: disk full"
    ]
    assert _temporary_files(root) == []


def test_cli_reports_a_write_error_as_one_and_says_nothing_was_written(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    root = _stale_copy(tmp_path)
    before = _contents(root)
    _replace_failing_at(monkeypatch, 2, OSError("disk full"))
    assert action_catalog.main(["generate", "--root", str(root)]) == 1
    monkeypatch.undo()
    assert capsys.readouterr().err.splitlines() == [
        "docs/api/redocly.yaml: generated-block: cannot write: disk full",
        "action_catalog: cannot write the generated blocks; nothing written",
    ]
    assert _contents(root) == before


def test_cli_names_a_file_left_with_its_new_text(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    root = _stale_copy(tmp_path)
    _replace_failing_at(monkeypatch, 2, OSError("disk full"))

    def read_only(self: Path, *args: object, **kwargs: object) -> int:
        raise OSError("read-only file system")

    monkeypatch.setattr(Path, "write_text", read_only)
    assert action_catalog.main(["generate", "--root", str(root)]) == 1
    monkeypatch.undo()
    assert capsys.readouterr().err.splitlines() == [
        "docs/api/redocly.yaml: generated-block: cannot write: disk full",
        "docs/spec/05-aktionen.md: generated-block: old text not restored: read-only file system",
        "action_catalog: cannot write the generated blocks; the files above whose old text was"
        " not restored hold the new text",
    ]


def test_generate_refuses_a_catalogue_with_broken_relations(tmp_path: Path) -> None:
    # Schema-valid, but an event names an unknown receive action: nothing is written.
    root = _stale_copy(tmp_path)
    _mutate(root, lambda d: d["events"][0].update(receive_action="no.such_action"))
    before = _contents(root)
    with pytest.raises(CatalogError) as err:
        action_catalog.generate(root)
    assert any(
        f.startswith("actions.yaml: event: ") and "unknown receive action 'no.such_action'" in f
        for f in err.value.findings
    ), err.value.findings
    assert _contents(root) == before


def test_findings_come_in_stages(tmp_path: Path) -> None:
    # Duplicate keys hide the schema findings, the schema findings hide the relational ones,
    # and only a valid catalogue is compared with chapter 05 (tools/catalog/README.md).
    root = _copy_repo(tmp_path)
    _edit(root, CATALOG, "version: 1\n", "version: 2\n")
    _edit(root, CATALOG, "- id: user.read\n", "- id: user.read\n  section: identity\n")
    _edit(root, CATALOG, "receive_action: message.read\n", "receive_action: no.such_action\n")
    _edit(root, CHAPTER, "| `self.read` |", "| `self.read` | x |")
    [duplicate] = action_catalog.check(root)
    assert duplicate.startswith("actions.yaml: schema: line ")
    assert duplicate.endswith(": duplicate key 'section'")
    _edit(root, CATALOG, "- id: user.read\n  section: identity\n", "- id: user.read\n")
    assert action_catalog.check(root) == ["actions.yaml: schema: version must be 1"]
    _edit(root, CATALOG, "version: 2\n", "version: 1\n")
    relational = action_catalog.check(root)
    assert relational, relational
    assert all(f.startswith("actions.yaml: event: ") for f in relational), relational
    _edit(root, CATALOG, "receive_action: no.such_action\n", "receive_action: message.read\n")
    [stale] = action_catalog.check(root)
    assert stale.startswith("docs/spec/05-aktionen.md: generated-block: actions.")
