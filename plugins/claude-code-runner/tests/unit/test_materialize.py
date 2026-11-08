# SPDX-License-Identifier: Apache-2.0
"""The agent and control folders of a hosted agent (26 5.1, 5.2, 5.4, 5.5, 5.6; BA-T11, BA-T12).

The runner writes every file the CLI reads from the definition and never copies one; the
frontmatter of ``SKILL.md`` comes from a serializer that quotes every scalar; names that would
leave their folder are refused before anything is written; the settings file holds exactly 26
5.4; every surface of 26 5.5 that the runner did not write is removed before the turn, a
symbolic link anywhere too, and the agent's other files stay; the per-turn MCP file holds the
resolved secrets with mode ``0600`` and is gone after the turn.
"""

from __future__ import annotations

import json
import logging
import os
import stat
from pathlib import Path
from typing import Any
from uuid import UUID

import pytest

from claude_code_runner.definition import Definition, Limits, Skill, SkillFile, parse
from claude_code_runner.materialize import (
    SETTINGS,
    AgentFolders,
    MaterializeError,
    definition_resolver,
    frontmatter,
    loaded_servers,
    materialize,
    remove_turn_mcp,
    sweep_turn_files,
    write_turn_mcp,
)

AGENT = UUID("0192f4a0-7b1c-7d07-84b0-358556a08a99")
RUN = UUID("0192f4a0-7b1c-7d07-84b0-000000000001")
OTHER_RUN = UUID("0192f4a0-7b1c-7d07-84b0-000000000002")
CORPUS = Path(__file__).resolve().parents[4] / "tests" / "agent-definitions"


def _full() -> Definition:
    raw = json.loads((CORPUS / "valid" / "full-host.json").read_text())["definition"]
    definition, found = parse(json.dumps(raw).encode(), Limits.runner(("npx", "uvx")), "host")
    assert found == []
    assert definition is not None
    return definition


def _small(**changes: Any) -> Definition:
    base: dict[str, Any] = {
        "instructions": "You help Sam.\n",
        "skills": [],
        "mcp_servers": [],
        "settings": {"tool_class_wish": "chat"},
        "origin": {"kind": "manual"},
    }
    base.update(changes)
    definition, found = parse(json.dumps(base).encode(), Limits.runner(("npx",)), "host")
    assert found == []
    assert definition is not None
    return definition


@pytest.fixture
def folders(tmp_path: Path) -> AgentFolders:
    root = tmp_path / "agents"
    root.mkdir(mode=0o700)
    return AgentFolders(root, AGENT)


def _tree(base: Path) -> list[str]:
    return sorted(str(p.relative_to(base)) for p in base.rglob("*"))


def test_folders_are_next_to_each_other(folders: AgentFolders) -> None:
    assert folders.agent == folders.root / str(AGENT)
    assert folders.control == folders.root / f"{AGENT}.control"
    assert folders.settings == folders.control / "settings.json"
    assert folders.plugin == folders.control / "plugin"
    assert folders.turn_mcp(RUN) == folders.control / "turns" / f"{RUN}.mcp.json"


def test_files_of_a_full_definition_under_host(folders: AgentFolders) -> None:
    done = materialize(folders, _full(), 7, "host")
    agent, control = folders.agent, folders.control
    for folder in (agent, control):
        assert stat.S_IMODE(folder.stat().st_mode) == 0o700
    assert (agent / "CLAUDE.md").read_text() == _full().instructions
    assert json.loads((agent / ".claude" / "settings.local.json").read_text()) == SETTINGS
    assert json.loads((control / "settings.json").read_text()) == SETTINGS
    manifest = json.loads((control / "plugin" / ".claude-plugin" / "plugin.json").read_text())
    assert manifest == {"name": f"bothy-agent-{AGENT}", "version": "7"}
    skill = (control / "plugin" / "skills" / "notes" / "SKILL.md").read_text()
    assert skill == (
        '---\nname: "notes"\ndescription: "Writes short notes"\n---\nWrite short notes.\n'
    )
    files = control / "plugin" / "skills" / "notes"
    assert (files / "checklist.md").read_text() == "- [ ] Changelog\n- [ ] Tag\n"
    assert (files / "style-guide.v2.md").read_text() == "Short sentences.\n"
    assert _tree(control / "plugin") == [
        ".claude-plugin",
        ".claude-plugin/plugin.json",
        "skills",
        "skills/notes",
        "skills/notes/SKILL.md",
        "skills/notes/checklist.md",
        "skills/notes/style-guide.v2.md",
        "skills/summarize-2",
        "skills/summarize-2/SKILL.md",
    ]
    mcp = json.loads((control / "mcp.json").read_text())
    assert mcp == {
        "mcpServers": {
            "docs": {
                "type": "http",
                "url": "https://mcp.example.com/v1",
                "headers": {
                    "Authorization": {"secret_ref": "env:DOCS_MCP_TOKEN"},
                    "X-Team": "release",
                },
            },
            "files": {
                "type": "stdio",
                "command": "npx",
                "args": ["@example/mcp-files@1.4.2", "--root", "/srv/shared", "--read-only"],
                "env": {
                    "FILES_API_KEY": {"secret_ref": "file:/run/secrets/files-key"},
                    "LOG_LEVEL": "info",
                },
            },
            "search-py": {
                "type": "stdio",
                "command": "uvx",
                "args": ["mcp-search@0.3.0-beta.1+build.7"],
                "env": {},
            },
        }
    }
    assert done.has_skills is True
    assert [s.name for s in done.servers] == ["docs", "files", "search-py"]


def test_settings_hold_exactly_the_runner_settings() -> None:
    # 26 5.4: the three switches and edit and write deny rules for the control files of the
    # working folder; nothing else (no allow rule, env, helper, MCP, plugin or directories key).
    patterns = ["CLAUDE.md", "CLAUDE.local.md", ".claude/**", ".mcp.json", ".lsp.json"]
    patterns.append(".claude-plugin/**")
    assert {
        "disableSkillShellExecution": True,
        "disableAllHooks": True,
        "disableWorkflows": True,
        "permissions": {
            "deny": [f"{tool}({p})" for p in patterns for tool in ("Edit", "Write")],
        },
    } == SETTINGS


@pytest.mark.parametrize(
    ("tool_class", "servers"),
    [("chat", []), ("scoped", ["docs"]), ("host", ["docs", "files", "search-py"])],
)
def test_servers_per_class(folders: AgentFolders, tool_class: str, servers: list[str]) -> None:
    # 26 4.4 items 3 and 4: none under chat, http only under scoped, all under host.
    assert [s.name for s in loaded_servers(_full(), tool_class)] == servers
    materialize(folders, _full(), 1, tool_class)
    written = json.loads((folders.control / "mcp.json").read_text())["mcpServers"]
    assert list(written) == servers


@pytest.mark.parametrize("tool_class", ["chat", "scoped"])
def test_no_local_settings_below_host(folders: AgentFolders, tool_class: str) -> None:
    materialize(folders, _full(), 1, "host")
    materialize(folders, _full(), 1, tool_class)
    assert not (folders.agent / ".claude").exists()
    assert (folders.control / "settings.json").exists()


@pytest.mark.parametrize(
    "description", ["Two\nlines", "a: b\nhooks: {x: 1}", 'quote " and \\ back', chr(0x2028) + "x"]
)
def test_frontmatter_is_two_quoted_scalars(description: str) -> None:
    # BA-T11: a line break or a colon in a value never starts a key, by the serializer alone.
    text = frontmatter("notes", description)
    lines = text.split("\n")
    assert lines[0] == "---"
    assert lines[3] == "---"
    assert lines[4] == ""
    assert len(lines) == 5
    assert lines[1] == 'name: "notes"'
    key, value = lines[2].split(": ", 1)
    assert key == "description"
    assert value.startswith('"')
    assert value.endswith('"')
    assert json.loads(value) == description


@pytest.mark.parametrize("name", ["../x.md", "a/b.md", "SKILL.md", "skill.md", ".hidden.md", ""])
def test_bad_file_names_are_refused_before_writing(folders: AgentFolders, name: str) -> None:
    files = (SkillFile(name, "x"),)
    definition = Definition("i", (Skill("notes", "d", "b", files),), (), _full().settings, "manual")
    with pytest.raises(MaterializeError) as err:
        materialize(folders, definition, 1, "chat")
    assert err.value.code == "invalid_name"
    assert not folders.control.exists()
    assert not (folders.root / "x.md").exists()


@pytest.mark.parametrize("name", ["../up", "a/b", "", "UP"])
def test_bad_skill_names_are_refused_before_writing(folders: AgentFolders, name: str) -> None:
    definition = Definition("i", (Skill(name, "d", "b"),), (), _full().settings, "manual")
    with pytest.raises(MaterializeError):
        materialize(folders, definition, 1, "chat")
    assert not folders.control.exists()


def _plant(base: Path, rel: str, text: str = "planted") -> Path:
    path = base / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    return path


SURFACES = [
    ".claude/settings.json",
    ".claude/settings.local.json",
    ".claude/skills/evil/SKILL.md",
    ".claude/commands/x.md",
    ".claude/agents/x.md",
    ".claude/workflows/x.md",
    ".claude/rules/x.md",
    ".claude/hooks/x.sh",
    ".mcp.json",
    ".lsp.json",
    "CLAUDE.local.md",
    ".claude-plugin/plugin.json",
    "sub/deeper/.claude/settings.json",
    "sub/.mcp.json",
    "sub/deeper/CLAUDE.local.md",
    "sub/.claude-plugin/plugin.json",
    "sub/.lsp.json",
]
PLUGIN_PARTS = [
    "plugin/hooks/hooks.json",
    "plugin/.mcp.json",
    "plugin/.lsp.json",
    "plugin/monitors/monitors.json",
    "plugin/commands/x.md",
    "plugin/agents/x.md",
    "plugin/output-styles/x.md",
    "plugin/bin/tool",
    "plugin/skills/evil/SKILL.md",
    "plugin/skills/notes/extra.md",
    "plugin/.claude-plugin/marketplace.json",
    "turns/0192f4a0-7b1c-7d07-84b0-0000000000ff.mcp.json",
    "stray.txt",
]


def test_rebuild_removes_every_foreign_surface_and_keeps_other_files(
    folders: AgentFolders, tmp_path: Path, caplog: pytest.LogCaptureFixture
) -> None:
    # BA-T12: planted surfaces of 26 5.5 in the agent and control folders.
    materialize(folders, _full(), 1, "scoped")
    for rel in SURFACES:
        _plant(folders.agent, rel)
    for rel in PLUGIN_PARTS:
        _plant(folders.control, rel)
    kept = [
        _plant(folders.agent, "notes/today.md", "keep me"),
        _plant(folders.agent, "docs/release.md", "keep me"),
        _plant(folders.agent, ".git/config", "keep me"),
    ]
    outside = _plant(tmp_path, "outside/secret.txt", "outside")
    (folders.agent / "link").symlink_to(outside)
    (folders.agent / "notes" / "dirlink").symlink_to(outside.parent)
    (folders.control / "plugin" / "skills" / "notes" / "link.md").symlink_to(outside)
    (folders.agent / "CLAUDE.md").write_text("changed by the turn")
    with caplog.at_level(logging.INFO, logger="claude_code_runner"):
        materialize(folders, _full(), 2, "scoped")
    for rel in SURFACES:
        assert not (folders.agent / rel).exists(), rel
    assert not (folders.agent / ".claude").exists()
    assert not (folders.agent / "sub" / "deeper" / ".claude").exists()
    for path in kept:
        assert path.read_text() == "keep me"
    assert (folders.agent / "CLAUDE.md").read_text() == _full().instructions
    assert not os.path.lexists(folders.agent / "link")
    assert not os.path.lexists(folders.agent / "notes" / "dirlink")
    assert outside.read_text() == "outside"
    assert _tree(folders.control) == [
        "mcp.json",
        "plugin",
        "plugin/.claude-plugin",
        "plugin/.claude-plugin/plugin.json",
        "plugin/skills",
        "plugin/skills/notes",
        "plugin/skills/notes/SKILL.md",
        "plugin/skills/notes/checklist.md",
        "plugin/skills/notes/style-guide.v2.md",
        "plugin/skills/summarize-2",
        "plugin/skills/summarize-2/SKILL.md",
        "settings.json",
    ]
    events = [json.loads(r.getMessage())["event"] for r in caplog.records]
    assert events.count("hosted_link_removed") == 3
    assert "hosted_surface_removed" in events
    assert "planted" not in caplog.text
    assert str(outside) not in caplog.text


def test_a_link_in_place_of_a_folder_is_never_followed(
    folders: AgentFolders, tmp_path: Path
) -> None:
    outside = tmp_path / "outside"
    outside.mkdir()
    folders.control.mkdir(mode=0o700)
    (folders.control / "plugin").symlink_to(outside)
    folders.agent.symlink_to(outside)
    materialize(folders, _full(), 1, "host")
    assert list(outside.iterdir()) == []
    assert not folders.agent.is_symlink()
    assert (folders.control / "plugin" / "skills" / "notes" / "SKILL.md").exists()


def test_active_turn_files_stay_and_others_go(folders: AgentFolders) -> None:
    materialize(folders, _full(), 1, "host")
    mine = write_turn_mcp(folders, RUN, loaded_servers(_full(), "host"), lambda _ref: "v")
    other = write_turn_mcp(folders, OTHER_RUN, loaded_servers(_full(), "host"), lambda _ref: "v")
    assert mine is not None
    assert other is not None
    materialize(folders, _full(), 1, "host", active_runs={RUN})
    assert mine.exists()
    assert not other.exists()


def test_turn_mcp_file_holds_resolved_values_with_mode_0600(folders: AgentFolders) -> None:
    materialize(folders, _full(), 1, "host")
    values = {"env:DOCS_MCP_TOKEN": "tok-alex", "file:/run/secrets/files-key": "key-sam"}
    path = write_turn_mcp(folders, RUN, loaded_servers(_full(), "host"), values.__getitem__)
    assert path == folders.turn_mcp(RUN)
    assert stat.S_IMODE(path.stat().st_mode) == 0o600
    assert stat.S_IMODE(path.parent.stat().st_mode) == 0o700
    servers = json.loads(path.read_text())["mcpServers"]
    assert servers["docs"]["headers"] == {"Authorization": "tok-alex", "X-Team": "release"}
    assert servers["files"]["env"] == {"FILES_API_KEY": "key-sam", "LOG_LEVEL": "info"}
    # The unresolved file of the control folder never holds a value.
    assert "tok-alex" not in (folders.control / "mcp.json").read_text()
    remove_turn_mcp(folders, RUN)
    assert not path.exists()
    remove_turn_mcp(folders, RUN)  # a second removal is no error


def test_no_turn_mcp_file_without_servers(folders: AgentFolders) -> None:
    materialize(folders, _small(), 1, "scoped")
    assert write_turn_mcp(folders, RUN, [], lambda _ref: "v") is None
    assert not folders.turn_mcp(RUN).exists()


def test_sweep_removes_leftover_turn_files_of_every_agent(tmp_path: Path) -> None:
    root = tmp_path / "agents"
    root.mkdir(mode=0o700)
    a = AgentFolders(root, AGENT)
    b = AgentFolders(root, UUID("0192f4a0-7b1c-7d07-84b0-358556a08a98"))
    for f in (a, b):
        materialize(f, _full(), 1, "host")
        write_turn_mcp(f, RUN, loaded_servers(_full(), "host"), lambda _ref: "v")
    _plant(a.agent, "turns/x.mcp.json", "agent file")
    assert sweep_turn_files(root) == 2
    assert not a.turn_mcp(RUN).exists()
    assert not b.turn_mcp(RUN).exists()
    assert (a.agent / "turns" / "x.mcp.json").exists()


def test_a_resolve_failure_writes_nothing(folders: AgentFolders) -> None:
    materialize(folders, _full(), 1, "host")

    def fail(_ref: str) -> str:
        raise MaterializeError("secret_unavailable")

    with pytest.raises(MaterializeError):
        write_turn_mcp(folders, RUN, loaded_servers(_full(), "host"), fail)
    assert not folders.turn_mcp(RUN).exists()


# ---- Which secrets a definition may name (26 4.2, 5.1 item 3; red team of A1-05) ----


def _scope(tmp_path: Path) -> tuple[Path, Path]:
    secrets = tmp_path / "hosted-secrets"
    own = secrets / str(AGENT)
    own.mkdir(parents=True, mode=0o700)
    value = own / "docs_token"
    value.write_text("docs-value\n")
    value.chmod(0o600)
    return secrets, value


def test_a_file_in_the_agent_s_own_scope_resolves(tmp_path: Path) -> None:
    secrets, value = _scope(tmp_path)
    assert definition_resolver(secrets, AGENT, ())(f"file:{value}") == "docs-value"


def _private(path: Path, text: str = "runner-credential\n") -> Path:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_text(text)
    path.chmod(0o600)
    return path


@pytest.mark.parametrize("place", ["outside", "other_agent", "link_out", "env", "dotdot"])
def test_a_reference_outside_the_scope_is_refused(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, place: str
) -> None:
    secrets, _value = _scope(tmp_path)
    monkeypatch.setenv("RUNNER_ONLY", "runner-credential")
    token = _private(tmp_path / "agent_token")
    other = _private(secrets / "0192f4a0-7b1c-7d07-84b0-358556a08a98" / "docs_token")
    link = secrets / str(AGENT) / "link"
    link.symlink_to(token)
    ref = {
        "outside": f"file:{token}",
        "other_agent": f"file:{other}",
        "link_out": f"file:{link}",
        "env": "env:RUNNER_ONLY",
        "dotdot": f"file:{secrets / str(AGENT) / '..' / 'x'}",
    }[place]
    with pytest.raises(MaterializeError) as err:
        definition_resolver(secrets, AGENT, ())(ref)
    assert err.value.code == ("secret_unavailable" if place == "dotdot" else "secret_out_of_scope")
    assert "runner-credential" not in str(err.value)


def test_without_the_key_no_reference_resolves(tmp_path: Path) -> None:
    _secrets, value = _scope(tmp_path)
    with pytest.raises(MaterializeError) as err:
        definition_resolver(None, AGENT, ())(f"file:{value}")
    assert err.value.code == "secret_out_of_scope"


def test_a_runner_credential_in_the_scope_is_refused(tmp_path: Path) -> None:
    secrets, value = _scope(tmp_path)
    link = secrets / str(AGENT) / "alias"
    link.symlink_to(value)
    with pytest.raises(MaterializeError) as err:
        definition_resolver(secrets, AGENT, (value,))(f"file:{link}")
    assert err.value.code == "secret_out_of_scope"


def test_an_agent_s_scope_that_is_a_link_is_refused(tmp_path: Path) -> None:
    """A link at the agent's subfolder must not move the scope to another folder."""
    secrets = tmp_path / "hosted-secrets"
    secrets.mkdir(mode=0o700)
    elsewhere = _private(tmp_path / "elsewhere" / "docs_token", "elsewhere-value\n")
    (secrets / str(AGENT)).symlink_to(elsewhere.parent)
    with pytest.raises(MaterializeError) as err:
        definition_resolver(secrets, AGENT, ())(f"file:{secrets / str(AGENT) / 'docs_token'}")
    assert err.value.code == "secret_out_of_scope"
    with pytest.raises(MaterializeError) as err:
        definition_resolver(secrets, AGENT, ())(f"file:{elsewhere}")
    assert err.value.code == "secret_out_of_scope"


def test_a_hard_link_in_the_scope_is_refused(tmp_path: Path) -> None:
    """A hard link in the agent's folder to a file outside resolves inside by path (issue 575)."""
    secrets, _value = _scope(tmp_path)
    token = _private(tmp_path / "agent_token")
    alias = secrets / str(AGENT) / "alias"
    os.link(token, alias)
    with pytest.raises(MaterializeError) as err:
        definition_resolver(secrets, AGENT, ())(f"file:{alias}")
    assert err.value.code == "secret_out_of_scope"
    assert "runner-credential" not in str(err.value)


def test_a_link_swapped_in_after_the_check_is_not_followed(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """The open after the scope check follows no link (issue 575): a file swapped for a link
    between the check and the open is refused, simulated by a check that sees no link."""
    secrets, value = _scope(tmp_path)
    token = _private(tmp_path / "agent_token")
    sub = secrets / str(AGENT) / "sub"
    sub.symlink_to(token.parent, target_is_directory=True)
    value.unlink()
    value.symlink_to(token)
    real_resolve = Path.resolve

    def blind(self: Path, strict: bool = False) -> Path:
        if str(self).startswith(str(secrets)):
            real_resolve(self, strict=strict)
            return Path(os.path.normpath(self))  # the lexical path, as if no link were there
        return real_resolve(self, strict=strict)

    monkeypatch.setattr(Path, "resolve", blind)
    for ref in (f"file:{value}", f"file:{sub / 'agent_token'}"):
        with pytest.raises(MaterializeError) as err:
            definition_resolver(secrets, AGENT, ())(ref)
        assert err.value.code in ("secret_unavailable", "secret_out_of_scope")


def test_a_file_below_a_subfolder_of_the_scope_resolves(tmp_path: Path) -> None:
    secrets, _value = _scope(tmp_path)
    nested = _private(secrets / str(AGENT) / "docs" / "token", "nested-value\n")
    assert definition_resolver(secrets, AGENT, ())(f"file:{nested}") == "nested-value"


def test_skills_absent_leave_the_plugin_without_skills(folders: AgentFolders) -> None:
    done = materialize(folders, _small(), 1, "chat")
    assert done.has_skills is False
    assert not (folders.control / "plugin" / "skills").exists()
    assert (folders.agent / "CLAUDE.md").read_text() == "You help Sam.\n"
