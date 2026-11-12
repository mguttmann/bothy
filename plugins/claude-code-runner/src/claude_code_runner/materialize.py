# SPDX-License-Identifier: Apache-2.0
r"""The agent and control folders of a hosted agent (docs/spec/26-builtin-agents.md 5.1 to 5.6).

`materialize` writes every file the CLI reads from a checked definition, never copying a file
from it (26 5.2), and rebuilds both folders before every turn (26 5.6):

1. a symbolic link anywhere in the agent or the control folder is removed and logged
   (``hosted_link_removed``), because a link could point the CLI at a file outside the folder;
2. every surface of 26 5.5 in the agent folder that the runner did not write is removed at any
   depth (any ``.claude`` except its own ``.claude/settings.local.json`` under ``host``, any
   ``.mcp.json``, ``.lsp.json``, ``CLAUDE.local.md`` and ``.claude-plugin``), logged as
   ``hosted_surface_removed`` with the surface name only; the agent's other files stay;
3. the owned files are written atomically, so a concurrent turn of the same agent reads the old
   or the new content, never a part;
4. the control folder keeps only what the runner wrote: the files of this definition and the
   per-turn MCP files of the runs that are still active.

Names come from the closed schema (26 4.2); a name that would leave its folder is refused with
`MaterializeError` before anything is written. Secret values appear only in the per-turn MCP
file of `write_turn_mcp` (mode ``0600``, removed by `remove_turn_mcp` when the turn ends and by
`sweep_turn_files` at start), never in the environment of a turn (26 5.2, R-BA-08).

Examples:
    >>> frontmatter("notes", "Writes: short notes")
    '---\nname: "notes"\ndescription: "Writes: short notes"\n---\n'
"""

from __future__ import annotations

import json
import logging
import os
import re
import shutil
import stat
from collections.abc import Callable, Collection, Iterable, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from uuid import UUID

from claude_code_runner.definition import Definition, Entry, McpServer
from claude_code_runner.secrets import SecretError, parse_secret_ref, read_open_file
from claude_code_runner.statedir import write_private

__all__ = [
    "SETTINGS",
    "AgentFolders",
    "MaterializeError",
    "Materialized",
    "control_name",
    "definition_resolver",
    "frontmatter",
    "loaded_servers",
    "materialize",
    "remove_turn_mcp",
    "sweep_turn_files",
    "write_turn_mcp",
]

_LOG = logging.getLogger("claude_code_runner")

_PROTECTED = ("CLAUDE.md", "CLAUDE.local.md", ".claude/**", ".mcp.json", ".lsp.json")
_PROTECTED_PLUGIN = ".claude-plugin/**"

SETTINGS: dict[str, Any] = {
    "disableSkillShellExecution": True,
    "disableAllHooks": True,
    "disableWorkflows": True,
    "permissions": {
        "deny": [
            f"{tool}({pattern})"
            for pattern in (*_PROTECTED, _PROTECTED_PLUGIN)
            for tool in ("Edit", "Write")
        ],
    },
}
"""The runner settings of 26 5.4, exactly: three restrictive switches and the edit and write
deny rules (rule syntax of the pinned CLI, patterns relative to the working folder) for the
control files inside the working folder. No allow rule, ``env``, command helper, MCP switch,
plugin or marketplace key and no ``additionalDirectories``."""

_SURFACE_DIRS = frozenset({".claude", ".claude-plugin"})
_SURFACE_FILES = frozenset(
    {".mcp.json", ".lsp.json", "CLAUDE.local.md", ".claude", ".claude-plugin"}
)
_SKILL_NAME = re.compile(r"[a-z0-9][a-z0-9-]{0,63}")
_FILE_NAME = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}\.md")
_LINE_BREAKS = {chr(c): f"\\u{c:04x}" for c in (0x85, 0x2028, 0x2029)}
"""Line breaks a YAML reader may see inside a scalar; written as escapes (`frontmatter`)."""


def _log(event: str, **ids: object) -> None:
    _LOG.info(json.dumps({"event": event, **{k: str(v) for k, v in ids.items()}}))


class MaterializeError(Exception):
    """A definition the runner cannot write, or a secret it cannot resolve.

    Attributes:
        code: ``invalid_name``, ``not_a_folder``, ``secret_unavailable`` or
            ``secret_out_of_scope``; never a value.
    """

    def __init__(self, code: str) -> None:
        """Create the error with its code only."""
        super().__init__(code)
        self.code = code


def control_name(agent_id: UUID) -> str:
    """The name of a hosted agent's control folder under ``runner.hosted_agents_root``.

    The command line of every turn of the agent names a file in this folder (``--settings``,
    26 5.3), which is how a leftover turn is told from any other process
    (`claude_code_runner.hosting.stop_leftover_turns`).

    Args:
        agent_id: The hosted agent.

    Returns:
        ``<agent_id>.control``.

    Examples:
        >>> control_name(UUID(int=7))
        '00000000-0000-0000-0000-000000000007.control'
    """
    return f"{agent_id}.control"


@dataclass(frozen=True)
class AgentFolders:
    """The folders of one hosted agent under ``runner.hosted_agents_root`` (26 5.1 item 2).

    Attributes:
        root: ``runner.hosted_agents_root``.
        agent_id: The hosted agent.
    """

    root: Path
    agent_id: UUID

    @property
    def agent(self) -> Path:
        """``<root>/<agent_id>/``: the working folder of every turn of the agent."""
        return self.root / str(self.agent_id)

    @property
    def control(self) -> Path:
        """``<root>/<agent_id>.control/``: the runner's files for the CLI, outside the agent."""
        return self.root / control_name(self.agent_id)

    @property
    def settings(self) -> Path:
        """The settings file of ``--settings``."""
        return self.control / "settings.json"

    @property
    def plugin(self) -> Path:
        """The plugin folder of ``--plugin-dir``."""
        return self.control / "plugin"

    @property
    def turns(self) -> Path:
        """The folder of the per-turn MCP files."""
        return self.control / "turns"

    def turn_mcp(self, run_id: UUID) -> Path:
        """The per-turn MCP file of a run.

        Args:
            run_id: The run.

        Returns:
            ``<control>/turns/<run_id>.mcp.json``.
        """
        return self.turns / f"{run_id}.mcp.json"


@dataclass(frozen=True)
class Materialized:
    """What a turn of the agent loads after `materialize`.

    Attributes:
        has_skills: The definition has skills: ``--plugin-dir`` and ``Skill`` (26 5.3).
        servers: The MCP servers of the class (26 4.4): none under ``chat``, ``http`` ones
            under ``scoped``, all under ``host``.
    """

    has_skills: bool
    servers: tuple[McpServer, ...]


def frontmatter(name: str, description: str) -> str:
    """The YAML frontmatter of a ``SKILL.md``: exactly ``name`` and ``description``, quoted.

    Each value is written as a double-quoted scalar by the JSON string encoder: a JSON string is
    a YAML 1.2 double-quoted scalar, every quote, backslash and line break inside is escaped, so
    a value can never end its scalar or start a new key (26 5.2, BA-T11).

    Args:
        name: The skill name.
        description: The skill description.

    Returns:
        The frontmatter with its two fences and a final line break.
    """
    return f"---\nname: {_quoted(name)}\ndescription: {_quoted(description)}\n---\n"


def _quoted(value: str) -> str:
    text = json.dumps(value, ensure_ascii=False)
    for raw, escaped in _LINE_BREAKS.items():
        text = text.replace(raw, escaped)
    return text


def loaded_servers(definition: Definition, tool_class: str) -> tuple[McpServer, ...]:
    """The MCP servers a turn of ``tool_class`` loads (26 4.4 items 3 and 4).

    Args:
        definition: The checked definition.
        tool_class: The class the turn runs with.

    Returns:
        None under ``chat``, the ``http`` servers under ``scoped``, all under ``host``.
    """
    if tool_class == "host":
        return definition.mcp_servers
    if tool_class == "scoped":
        return tuple(s for s in definition.mcp_servers if s.transport == "http")
    return ()


def _entries(
    entries: Iterable[Entry], resolve_value: Callable[[str], str] | None
) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for entry in entries:
        if entry.secret_ref is None:
            out[entry.name] = entry.value or ""
        elif resolve_value is None:
            out[entry.name] = {"secret_ref": entry.secret_ref}
        else:
            out[entry.name] = resolve_value(entry.secret_ref)
    return out


def mcp_document(
    servers: Sequence[McpServer], resolve_value: Callable[[str], str] | None
) -> dict[str, Any]:
    """The MCP configuration of ``servers`` in the form of ``--mcp-config``.

    An ``http`` server has ``url`` and ``headers`` only (never a ``headersHelper``, 26 5.5); a
    ``stdio`` server starts its pinned package directly after the command, then its arguments
    (decisions.d/A1-02.md).

    Args:
        servers: The servers.
        resolve_value: Resolves a ``secret_ref`` to its value; ``None`` writes each reference
            unresolved as ``{"secret_ref": <ref>}``.

    Returns:
        ``{"mcpServers": {...}}``.
    """
    out: dict[str, Any] = {}
    for server in servers:
        if server.transport == "http":
            out[server.name] = {
                "type": "http",
                "url": server.url,
                "headers": _entries(server.headers, resolve_value),
            }
        else:
            out[server.name] = {
                "type": "stdio",
                "command": server.command,
                "args": [server.package, *server.args],
                "env": _entries(server.env, resolve_value),
            }
    return {"mcpServers": out}


def definition_resolver(
    secrets_dir: Path | None, agent_id: UUID, refused: Iterable[Path]
) -> Callable[[str], str]:
    """The resolver of a hosted agent's ``secret_ref`` values (26 4.2, 5.1 item 3).

    A definition may name only ``file:`` references whose file lies, also after following every
    link, in ``<secrets_dir>/<agent_id>/`` (that subfolder itself never a link); never ``env:``
    (the runner's own environment), never another agent's folder and never one of ``refused``
    (the runner's own credentials). Without
    ``secrets_dir`` no reference resolves. The file is opened below the subfolder without
    following a link, must have one hard link (a second name may lie outside) and is then read
    as any ``secret_ref`` (16 5.2).

    Args:
        secrets_dir: ``runner.hosted_secrets_dir``.
        agent_id: The hosted agent.
        refused: The files of the runner's own credentials.

    Returns:
        A function from a reference to its value; it raises `MaterializeError` with
        ``secret_out_of_scope`` or ``secret_unavailable``, never naming the reference or a value.
    """
    refused_paths = tuple(refused)

    def resolve_value(ref: str) -> str:
        try:
            parsed = parse_secret_ref(ref)
        except SecretError:
            raise MaterializeError("secret_unavailable") from None
        if secrets_dir is None or parsed.scheme != "file":
            raise MaterializeError("secret_out_of_scope")
        try:
            # Anchored to the configured folder: a link at the agent's subfolder moves nothing.
            scope = secrets_dir.resolve(strict=True) / str(agent_id)
            real = Path(parsed.target).resolve(strict=True)
            inside = not scope.is_symlink() and real.is_relative_to(scope)
            own = any(real == p.resolve() for p in refused_paths)
        except (OSError, RuntimeError):
            raise MaterializeError("secret_unavailable") from None
        if not inside or own:
            raise MaterializeError("secret_out_of_scope")
        fd = _open_below(scope, real.relative_to(scope).parts)
        try:
            # A hard link has no path to resolve: a second name may lie outside the scope.
            single = os.fstat(fd).st_nlink == 1
        except OSError:
            os.close(fd)
            raise MaterializeError("secret_unavailable") from None
        if not single:
            os.close(fd)
            raise MaterializeError("secret_out_of_scope")
        try:
            return read_open_file(fd).reveal()
        except SecretError:
            raise MaterializeError("secret_unavailable") from None

    return resolve_value


def _open_below(scope: Path, parts: Sequence[str]) -> int:
    """Open the file ``scope/<parts>`` for reading without following any link.

    Each step opens one name relative to the folder opened before (``O_NOFOLLOW``), so a link
    swapped in after the scope check fails the open instead of leaving the scope (issue 575).

    Raises:
        MaterializeError: ``secret_unavailable`` when a step fails or ``parts`` is empty.
    """
    if not parts:
        raise MaterializeError("secret_unavailable")
    folder = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
    try:
        fd = os.open(scope, folder)
    except OSError:
        raise MaterializeError("secret_unavailable") from None
    try:
        for part in parts[:-1]:
            step = os.open(part, folder, dir_fd=fd)
            os.close(fd)
            fd = step
        return os.open(parts[-1], os.O_RDONLY | os.O_NOFOLLOW, dir_fd=fd)
    except OSError:
        raise MaterializeError("secret_unavailable") from None
    finally:
        os.close(fd)


def _json(document: Any) -> str:
    return json.dumps(document, indent=2, ensure_ascii=False) + "\n"


def _owned(
    folders: AgentFolders, definition: Definition, version: int, tool_class: str
) -> dict[Path, str]:
    """Every file the runner writes for the definition, by path; names checked first."""
    files: dict[Path, str] = {folders.agent / "CLAUDE.md": definition.instructions}
    if tool_class == "host":
        # Only a turn with --setting-sources reads local settings (26 5.2, 5.3).
        files[folders.agent / ".claude" / "settings.local.json"] = _json(SETTINGS)
    files[folders.settings] = _json(SETTINGS)
    manifest = {"name": f"bothy-agent-{folders.agent_id}", "version": str(version)}
    files[folders.plugin / ".claude-plugin" / "plugin.json"] = _json(manifest)
    for skill in definition.skills:
        if _SKILL_NAME.fullmatch(skill.name) is None:
            raise MaterializeError("invalid_name")
        folder = folders.plugin / "skills" / skill.name
        files[folder / "SKILL.md"] = frontmatter(skill.name, skill.description) + skill.body
        for extra in skill.files:
            if _FILE_NAME.fullmatch(extra.name) is None or extra.name.lower() == "skill.md":
                raise MaterializeError("invalid_name")
            files[folder / extra.name] = extra.content
    servers = loaded_servers(definition, tool_class)
    files[folders.control / "mcp.json"] = _json(mcp_document(servers, None))
    return files


def materialize(
    folders: AgentFolders,
    definition: Definition,
    version: int,
    tool_class: str,
    *,
    active_runs: Collection[UUID] = (),
) -> Materialized:
    """Rebuild the agent and control folders from the definition (see the module docstring).

    Args:
        folders: The agent's folders.
        definition: The checked definition (`claude_code_runner.definition.parse`).
        version: The definition version (the ``version`` of ``plugin.json``).
        tool_class: The class the next turn runs with.
        active_runs: Runs of the agent whose per-turn MCP files stay.

    Returns:
        What a turn loads.

    Raises:
        MaterializeError: ``invalid_name`` before anything is written, or ``not_a_folder``
            when the root is not a folder.
        OSError: A file could not be written.
    """
    files = _owned(folders, definition, version, tool_class)
    if not folders.root.is_dir() or folders.root.is_symlink():
        raise MaterializeError("not_a_folder")
    ids = {"agent_id": folders.agent_id}
    for base in (folders.agent, folders.control):
        _private_folder(base, ids)
        _remove_links(base, ids)
    _remove_surfaces(folders.agent, keep=set(files), ids=ids)
    for path, text in files.items():
        _write(path, text, ids)
    keep = set(files) | {folders.turn_mcp(r) for r in active_runs}
    _remove_foreign(folders.control, keep, ids)
    return Materialized(bool(definition.skills), loaded_servers(definition, tool_class))


def _private_folder(path: Path, ids: dict[str, Any]) -> None:
    """``path`` as a folder of mode ``0700``; a link or a file in its place is removed first."""
    if path.is_symlink():
        path.unlink()
        _log("hosted_link_removed", **ids)
    elif path.exists() and not path.is_dir():
        path.unlink()
    path.mkdir(mode=0o700, exist_ok=True)
    path.chmod(0o700)


def _remove_links(base: Path, ids: dict[str, Any]) -> None:
    for top, dirs, names in os.walk(base, followlinks=False):
        for name in [*dirs, *names]:
            path = Path(top) / name
            if path.is_symlink():
                path.unlink()
                _log("hosted_link_removed", **ids)
        dirs[:] = [d for d in dirs if not (Path(top) / d).is_symlink()]


def _remove(path: Path) -> None:
    if path.is_dir() and not path.is_symlink():
        shutil.rmtree(path)
    else:
        path.unlink(missing_ok=True)


def _remove_surfaces(agent: Path, *, keep: set[Path], ids: dict[str, Any]) -> None:
    """Remove the surfaces of 26 5.5 at any depth of the agent folder, except owned files."""
    local = agent / ".claude" / "settings.local.json"
    for top, dirs, names in os.walk(agent, followlinks=False):
        here = Path(top)
        for name in list(dirs):
            if name not in _SURFACE_DIRS:
                continue
            path = here / name
            dirs.remove(name)
            if local in keep and path == local.parent:
                for child in path.iterdir():
                    if child != local:
                        _remove(child)
                        _log("hosted_surface_removed", surface=name, **ids)
                continue
            _remove(path)
            _log("hosted_surface_removed", surface=name, **ids)
        for name in names:
            path = here / name
            if name in _SURFACE_FILES and path not in keep:
                path.unlink(missing_ok=True)
                _log("hosted_surface_removed", surface=name, **ids)


def _remove_foreign(control: Path, keep: set[Path], ids: dict[str, Any]) -> None:
    """Remove everything in the control folder the runner did not write (26 5.6 item 2)."""
    folders = {p for path in keep for p in path.parents if p.is_relative_to(control)}
    for top, dirs, names in os.walk(control, topdown=False, followlinks=False):
        here = Path(top)
        for name in names:
            path = here / name
            if path not in keep:
                path.unlink(missing_ok=True)
                _log("hosted_surface_removed", surface="control", **ids)
        for name in dirs:
            path = here / name
            if path not in folders:
                _remove(path)
                _log("hosted_surface_removed", surface="control", **ids)


def _write(path: Path, text: str, ids: dict[str, Any]) -> None:
    """Write an owned file atomically; a folder or a file in the way of its parents goes."""
    missing: list[Path] = []
    parent = path.parent
    while not parent.is_dir() or parent.is_symlink():
        missing.append(parent)
        parent = parent.parent
    for folder in reversed(missing):
        if folder.is_symlink():
            folder.unlink()
            _log("hosted_link_removed", **ids)
        elif folder.exists():
            folder.unlink()
        folder.mkdir(mode=0o700)
    if path.is_dir() and not path.is_symlink():
        shutil.rmtree(path)
    write_private(path, text, atomic=True)


def write_turn_mcp(
    folders: AgentFolders,
    run_id: UUID,
    servers: Sequence[McpServer],
    resolve_value: Callable[[str], str],
) -> Path | None:
    """Write the per-turn MCP file with every ``secret_ref`` resolved (26 5.2, R-BA-08).

    Every value is resolved before the file is created, so a failure leaves no file.

    Args:
        folders: The agent's folders.
        run_id: The run.
        servers: The servers of the turn's class (`loaded_servers`).
        resolve_value: Resolves a ``secret_ref`` (`definition_resolver`).

    Returns:
        The file (mode ``0600``), or ``None`` when there is no server to load.

    Raises:
        MaterializeError: A secret could not be resolved.
        OSError: The file could not be written.
    """
    if not servers:
        return None
    text = _json(mcp_document(servers, resolve_value))
    turns = folders.turns
    if turns.is_symlink():
        turns.unlink()
    turns.mkdir(mode=0o700, exist_ok=True)
    turns.chmod(0o700)
    path = folders.turn_mcp(run_id)
    path.unlink(missing_ok=True)
    write_private(path, text)
    return path


def remove_turn_mcp(folders: AgentFolders, run_id: UUID) -> None:
    """Delete the per-turn MCP file of a run (when its turn ends, also on a stop).

    Args:
        folders: The agent's folders.
        run_id: The run.
    """
    folders.turn_mcp(run_id).unlink(missing_ok=True)


def sweep_turn_files(root: Path) -> int:
    """Delete the per-turn MCP files a crashed runner left under ``root`` (26 5.2).

    Args:
        root: ``runner.hosted_agents_root``.

    Returns:
        How many files were removed.
    """
    removed = 0
    if not root.is_dir():
        return removed
    for control in root.iterdir():
        turns = control / "turns"
        if not control.name.endswith(".control") or control.is_symlink() or turns.is_symlink():
            continue
        if not turns.is_dir():
            continue
        for leftover in turns.iterdir():
            info = leftover.lstat()
            if leftover.name.endswith(".mcp.json") and not stat.S_ISDIR(info.st_mode):
                leftover.unlink(missing_ok=True)
                removed += 1
    return removed
