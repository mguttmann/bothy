# SPDX-License-Identifier: Apache-2.0
"""The installer's plan with a profile in memory, cards and the store (notes 12, 17, 18)."""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest
from bothy_sdk.ids import uuid7
from bothy_sdk.settings import SdkSettings

from bothy_hermes_adapter import cards
from bothy_hermes_adapter.__main__ import build_parser
from bothy_hermes_adapter.installer import (
    CONTROL_SENDER,
    InstallError,
    InstallRequest,
    check,
    install,
    plan_install,
    plugin_entry,
    switch_mode,
)
from bothy_hermes_adapter.runtime import CardKind, CardRequest, OpenCard, ResolvedCredential
from bothy_hermes_adapter.store import CallbackStore

pytestmark = pytest.mark.anyio

INSTANCE = UUID("0192f4a0-7b1c-7000-8000-000000000001")
CANARY = "sk-ant-api03-canary-" + "z" * 16


@pytest.fixture
def anyio_backend() -> str:
    return "asyncio"


@dataclass
class MemoryBridge:
    home: Path
    config: dict[str, Any] = field(default_factory=dict)
    env: dict[str, str] = field(default_factory=dict)
    credential: ResolvedCredential = field(
        default_factory=lambda: ResolvedCredential("anthropic", "api_key")
    )
    login_file: bool = False

    def read_config(self) -> dict[str, Any]:
        return dict(self.config)

    def write_config(self, config: Any) -> None:
        self.config = dict(config)

    def env_defines(self, name: str) -> bool:
        return bool(self.env.get(name))

    def set_env(self, name: str, value: str) -> None:
        self.env[name] = value

    def remove_env(self, name: str) -> None:
        self.env.pop(name, None)

    def predict(self) -> ResolvedCredential:
        return self.credential

    def account_home(self) -> Path:
        home = self.home / "account"
        if self.login_file:
            (home / ".claude").mkdir(parents=True, exist_ok=True)
            (home / ".claude" / ".credentials.json").write_text("{}", encoding="utf-8")
        return home

    def api_key_variable(self) -> str:
        return "ANTHROPIC_API_KEY"

    def oauth_variables(self) -> tuple[str, ...]:
        return ("ANTHROPIC_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN")


def _request(**over: Any) -> InstallRequest:
    values: dict[str, Any] = {
        "server_url": "https://example.com/api/v1",
        "instance_id": INSTANCE,
        "token_file": Path("/srv/hermes/token"),
        "busy_input_mode": "steer",
    }
    values.update(over)
    return InstallRequest(**values)


def test_plan_install_writes_every_part() -> None:
    planned = plan_install({"plugins": {"enabled": ["other"]}}, _request())
    section = planned["gateway"]["platforms"]["bothy"]
    assert section["enabled"] is True
    assert section["extra"]["hermes_adapter.instance_id"] == str(INSTANCE)
    assert section["extra"]["allow_admin_from"] == [CONTROL_SENDER]
    assert section["home_channel"]["chat_id"] == "unassigned"
    assert planned["plugins"]["enabled"] == ["other", "bothy"]
    assert planned["display"]["busy_input_mode"] == "steer"
    assert planned["display"]["platforms"]["bothy"]["tool_progress"] == "off"
    assert planned["display"]["platforms"]["bothy"]["busy_steer_ack_enabled"] is False
    again = plan_install(planned, _request(busy_input_mode="queue"))
    assert again["plugins"]["enabled"] == ["other", "bothy"]
    assert again["display"]["busy_input_mode"] == "queue"


@pytest.mark.parametrize("mode", ["interrupt", "nonsense"])
def test_plan_install_refuses_interrupt(mode: str) -> None:
    with pytest.raises(InstallError, match="steer or queue"):
        plan_install({}, _request(busy_input_mode=mode))


def test_plan_install_keeps_the_platform_key() -> None:
    planned = plan_install({}, _request())
    with pytest.raises(InstallError, match="fixed"):
        plan_install(planned, _request(platform_key="other"))


def test_install_switch_and_check_without_values(tmp_path: Path) -> None:
    bridge = MemoryBridge(tmp_path, env={"ANTHROPIC_API_KEY": CANARY})
    placed: dict[Path, str] = {}
    lines = install(bridge, _request(), placed.__setitem__)
    assert lines[0] == "installed"
    assert set(placed) == {
        tmp_path / "plugins" / "bothy" / "plugin.yaml",
        tmp_path / "plugins" / "bothy" / "__init__.py",
    }
    assert "predicted mode: organization" in "\n".join(lines)
    with pytest.raises(InstallError, match="credential-holder"):
        switch_mode(bridge, "personal", None)
    holder = uuid7()
    bridge.credential = ResolvedCredential("anthropic", "oauth")
    personal = switch_mode(bridge, "personal", holder)
    assert bridge.env == {"BOTHY_ALLOWED_USERS": str(holder)}
    assert "predicted mode: personal" in "\n".join(personal)
    with pytest.raises(InstallError, match="ANTHROPIC_API_KEY"):
        switch_mode(bridge, "organization", None)
    bridge.env.update({"ANTHROPIC_API_KEY": CANARY, "ANTHROPIC_TOKEN": CANARY})
    bridge.credential = ResolvedCredential("anthropic", "api_key")
    organization = switch_mode(bridge, "organization", None)
    assert set(bridge.env) == {"ANTHROPIC_API_KEY"}
    assert bridge.config["auth"]["adopt_external_logins"] is False
    assert "restart" in "\n".join(organization)
    with pytest.raises(InstallError, match="mode must be"):
        switch_mode(bridge, "shared", None)
    bridge.credential = ResolvedCredential(None, "unknown")
    checked = check(bridge)
    assert "not determinable" in "\n".join(checked)
    for output in (lines, personal, organization, checked):
        assert CANARY not in "\n".join(output)
        assert "https://example.com" not in "\n".join(output)


def test_switch_refuses_without_install_and_with_a_login_file(tmp_path: Path) -> None:
    with pytest.raises(InstallError, match="not installed"):
        switch_mode(MemoryBridge(tmp_path), "personal", uuid7())
    bridge = MemoryBridge(tmp_path, env={"ANTHROPIC_API_KEY": CANARY}, login_file=True)
    install(bridge, _request(), lambda path, text: None)
    with pytest.raises(InstallError, match="login file"):
        switch_mode(bridge, "organization", None)


def test_install_refuses_a_section_that_does_not_read_back(tmp_path: Path) -> None:
    class Forgetful(MemoryBridge):
        def write_config(self, config: Any) -> None:
            self.config = {}

    with pytest.raises(InstallError, match="read back"):
        install(Forgetful(tmp_path), _request(), lambda path, text: None)


def test_plugin_entry_and_parser() -> None:
    entry = plugin_entry("bothy")
    assert "kind: platform" in entry["plugin.yaml"]
    assert 'make_register("bothy")' in entry["__init__.py"]
    args = build_parser().parse_args(
        [
            "switch-mode",
            "personal",
            "--profile-home",
            "/srv/h",
            "--credential-holder",
            str(INSTANCE),
        ]
    )
    assert (args.command, args.mode, args.credential_holder) == (
        "switch-mode",
        "personal",
        INSTANCE,
    )


# ---- cards -----------------------------------------------------------------------------------


def test_card_builders() -> None:
    settings = SdkSettings()
    many = CardRequest(CardKind.CLARIFY, "c1", "Pick one\nof these.", tuple("ABCDEFG"))
    [select] = cards.build_components(many, settings)
    assert select.custom_id == "cl:c1:select"
    assert cards.status_line(many) == "Pick one"
    open_question = CardRequest(CardKind.CLARIFY, "c2", "What else?")
    assert cards.build_components(open_question, settings) == []
    partial = CardRequest(CardKind.APPROVAL, "a1", "Run it?", ("once", "deny"), "k")
    assert [c.custom_id for c in cards.build_components(partial, settings)] == [
        "appr:a1:once",
        "appr:a1:deny",
    ]
    confirm = CardRequest(CardKind.SLASH_CONFIRM, "f1", "", (), "k", "/reload")
    assert cards.card_text(confirm).startswith("**/reload**")
    assert cards.status_line(confirm) == "/reload"
    too_long = CardRequest(CardKind.CLARIFY, "x" * 200, "Q?", ("A",))
    with pytest.raises(ValueError, match="custom_id"):
        cards.build_components(too_long, settings)
    long_label = CardRequest(CardKind.CLARIFY, "c3", "Q?", ("word " * 40,))
    [button] = cards.build_components(long_label, settings)
    assert len(button.to_dict()["definition"]["label"]) <= 80


def test_parse_and_chosen_label() -> None:
    card = OpenCard(uuid7(), CardRequest(CardKind.CLARIFY, "c1", "Q?", ("Red", "Blue")))
    assert cards.parse_custom_id("cl:c1:1") == ("cl", "c1", "1")
    assert cards.parse_custom_id("other") is None
    assert cards.parse_custom_id("xx:a:b") is None
    assert cards.chosen_label(card, "cl:c1:1", {}) == "Blue"
    assert cards.chosen_label(card, "cl:c1:select", {"values": ["0"]}) == "Red"
    assert cards.chosen_label(card, "cl:c1:9", {}) is None
    assert cards.chosen_label(card, "cl:other:0", {}) is None
    approval = OpenCard(uuid7(), CardRequest(CardKind.APPROVAL, "a1", "Q?", (), "k"))
    assert cards.chosen_label(approval, "appr:a1:session", {}) == "session"


async def test_store_claims_once_and_keeps_cards(tmp_path: Path) -> None:
    path = tmp_path / "hermes.sqlite3"
    store = CallbackStore(path)
    run = uuid7()
    assert await store.claim("cl:c1")
    assert not await store.claim("cl:c1")
    assert await store.resolved("cl:c1")
    assert not await store.resolved("cl:c2")
    await store.put_card(run, {"kind": "clarify", "hermes_id": "c1"})
    assert (await store.card(run)) == {"kind": "clarify", "hermes_id": "c1"}
    await store.drop_card(run)
    assert await store.card(run) is None
    await store.close()
    assert path.stat().st_mode & 0o077 == 0
    reopened = CallbackStore(path)
    assert not await reopened.claim("cl:c1")
    await reopened.close()
