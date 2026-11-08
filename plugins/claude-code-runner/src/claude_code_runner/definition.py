# SPDX-License-Identifier: Apache-2.0
"""The runner's own check of an agent definition (docs/spec/26-builtin-agents.md 4.2 to 4.4).

The one place of the definition rules in the runner: a port of the server's validator
(``services/server/internal/agents/domain/definition``, rulings of ``decisions.d/A1-02.md``)
held equal to it by the shared corpus ``tests/agent-definitions`` at the repository root. The
runner repeats the check before every turn of a hosted agent (26 5.6 item 1): a definition that
fails ends the run with ``tool_class_mismatch`` before ``run.start``. The builder checks its draft
with `validate_value` before it sends one (26 6.2 item 3).

`check` returns every finding as a JSON Pointer and a field code, never the submitted value;
`parse` also returns the typed `Definition` when there is none. `required_class` is the class a
definition needs (a ``stdio`` server needs ``host``, 26 4.4 item 4) and `intersect` the tool list
of a ``scoped`` turn (26 4.4 item 2).

The server's size bounds (``agents.definition.*``) are server keys the runner cannot read; the
runner checks with `Limits.runner`, which leaves them out and keeps every other rule
(decisions.d/A1-05.md).

Examples:
    >>> raw = b'{"instructions": "", "skills": [], "mcp_servers": [], '
    >>> raw += b'"settings": {"tool_class_wish": "chat"}, "origin": {"kind": "manual"}}'
    >>> check(raw, Limits.runner(("npx",)), "chat")
    []
"""

from __future__ import annotations

import json
import math
import re
import unicodedata
from collections.abc import Callable, Mapping, Sequence
from dataclasses import dataclass, field
from typing import Any
from urllib.parse import urlsplit

from claude_code_runner.commandline import scoped_tools

__all__ = [
    "Definition",
    "Entry",
    "Finding",
    "Limits",
    "McpServer",
    "Settings",
    "Skill",
    "SkillFile",
    "check",
    "has_frontmatter",
    "has_import_outside_folder",
    "has_inline_shell",
    "intersect",
    "parse",
    "required_class",
    "text_rule",
    "validate",
    "validate_value",
]

# ---- patterns (26 4.2, decisions.d/A1-02.md D-A1-02-3 to D-A1-02-7) ------------------------

_SKILL_NAME = re.compile(r"[a-z0-9][a-z0-9-]{0,63}")
_FILE_NAME = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}\.md")
_ENV_NAME = re.compile(r"[A-Z_][A-Z0-9_]*")
_HEADER_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_-]{0,127}")
_MODEL = re.compile(r"[A-Za-z0-9._:/@-]+")
_BUILTIN_TOOL = re.compile(r"[A-Za-z][A-Za-z0-9_]*(\([^()]*\))?")
_MCP_TOOL = re.compile(r"[A-Za-z0-9_-]+")
_PACKAGE = re.compile(
    r"(@[a-z0-9][a-z0-9._-]*/)?[A-Za-z0-9][A-Za-z0-9._-]*@"
    r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?"
)
_TEMPLATE_VERSION = re.compile(r"[0-9A-Za-z][0-9A-Za-z.+-]{0,63}")
_DRIVE = re.compile(r"[A-Za-z]:")

_JS_SPACE = "\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"
"""White space of a JavaScript regular expression (``\\s``), which the pinned CLI uses."""
_INLINE_SHELL = re.compile(f"(?:^|[{_JS_SPACE}])!`[^`]+`")
"""The CLI's ``(?<=^|\\s)!`[^`]+``` with the look-behind written as a group (26 4.3 item 3)."""
_IMPORT = re.compile(f"(?:^|[{_JS_SPACE}])@((?:\\\\ |[^{_JS_SPACE}])+)")
"""An ``@<path>`` token; a backslash before a space continues the path (26 4.3 item 4)."""
_FENCE_SHELL = "```!"

_RESERVED_ENV = frozenset(
    {
        "PATH",
        "HOME",
        "USER",
        "LOGNAME",
        "SHELL",
        "TMPDIR",
        "CLAUDE_CONFIG_DIR",
        "CLAUDE_PRODUCT_RUN",
        "NODE_OPTIONS",
    }
)
_RESERVED_ENV_PREFIXES = ("ANTHROPIC_", "CLAUDE_CODE_", "LD_", "DYLD_")
_SECRET_WORDS = ("KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "AUTH", "COOKIE")
_BIDI = frozenset({0x061C, 0x200E, 0x200F, *range(0x202A, 0x202F), *range(0x2066, 0x206A)})

_CLASSES = ("chat", "scoped", "host")
_ORIGINS = ("template", "interview", "manual")
_HTTP_MEMBERS = ("name", "transport", "url", "headers")
_STDIO_MEMBERS = ("name", "transport", "command", "package", "args", "env")
_ALL_MEMBERS = ("name", "transport", "url", "headers", "command", "package", "args", "env")


# ---- results -------------------------------------------------------------------------------


@dataclass(frozen=True)
class Finding:
    """One finding: the JSON Pointer into the definition and the field code, never the value.

    Attributes:
        pointer: The JSON Pointer (RFC 6901), ``""`` for the root.
        code: The field code of 26 4.3 or of the shape and text checks.
    """

    pointer: str
    code: str


_LIMIT_KEYS = {
    "agents.definition.instructions_max_bytes": "instructions_max_bytes",
    "agents.definition.max_skills": "max_skills",
    "agents.definition.skill_description_max_chars": "skill_description_max_chars",
    "agents.definition.skill_body_max_bytes": "skill_body_max_bytes",
    "agents.definition.max_skill_files": "max_skill_files",
    "agents.definition.max_mcp_servers": "max_mcp_servers",
    "agents.definition.max_allowed_tools": "max_allowed_tools",
    "agents.definition.max_cost_usd_per_run": "max_cost_usd_per_run",
    "agents.mcp_stdio_runners": "stdio_runners",
}
"""The configuration key of each field of `Limits` (26 14)."""


@dataclass(frozen=True)
class Limits:
    """The bounds a check runs with (26 14).

    ``None`` leaves a size bound out (the runner cannot read the server's keys).

    Attributes:
        instructions_max_bytes: ``agents.definition.instructions_max_bytes``.
        max_skills: ``agents.definition.max_skills``.
        skill_description_max_chars: ``agents.definition.skill_description_max_chars``.
        skill_body_max_bytes: ``agents.definition.skill_body_max_bytes``.
        max_skill_files: ``agents.definition.max_skill_files``.
        max_mcp_servers: ``agents.definition.max_mcp_servers``.
        max_allowed_tools: ``agents.definition.max_allowed_tools``.
        max_cost_usd_per_run: ``agents.definition.max_cost_usd_per_run``.
        stdio_runners: The commands a ``stdio`` server may use (``agents.mcp_stdio_runners``
            on the server, ``runner.mcp_stdio_runners`` in the runner).
    """

    instructions_max_bytes: int | None
    max_skills: int | None
    skill_description_max_chars: int | None
    skill_body_max_bytes: int | None
    max_skill_files: int | None
    max_mcp_servers: int | None
    max_allowed_tools: int | None
    max_cost_usd_per_run: float | None
    stdio_runners: tuple[str, ...]

    @classmethod
    def from_keys(cls, values: Mapping[str, Any]) -> Limits:
        """Read the limits by configuration key (the form of the corpus' ``limits.json``).

        Args:
            values: Every key of the server's definition limits.

        Returns:
            The limits.

        Raises:
            ValueError: A key is missing or unknown.
        """
        unknown = set(values) - set(_LIMIT_KEYS)
        if unknown:
            raise ValueError("unknown limit key")
        missing = set(_LIMIT_KEYS) - set(values)
        if missing:
            raise ValueError("missing limit key")
        named = {_LIMIT_KEYS[k]: v for k, v in values.items()}
        named["stdio_runners"] = tuple(named["stdio_runners"])
        return cls(**named)

    @classmethod
    def runner(cls, stdio_runners: Sequence[str]) -> Limits:
        """The limits of the runner's own check: no size bound, its own ``stdio`` commands.

        Args:
            stdio_runners: ``runner.mcp_stdio_runners``.

        Returns:
            The limits.
        """
        return cls(None, None, None, None, None, None, None, None, tuple(stdio_runners))


@dataclass(frozen=True)
class SkillFile:
    """An extra file of a skill: a flat, validated name and Markdown content."""

    name: str
    content: str


@dataclass(frozen=True)
class Skill:
    """A skill: name, one-line description, Markdown body and extra files."""

    name: str
    description: str
    body: str
    files: tuple[SkillFile, ...] = ()


@dataclass(frozen=True)
class Entry:
    """A header or environment entry: a plain ``value`` or a ``secret_ref``, never both."""

    name: str
    value: str | None = field(default=None, repr=False)
    secret_ref: str | None = field(default=None, repr=False)


@dataclass(frozen=True)
class McpServer:
    """An MCP server: ``http`` with ``url`` and ``headers``, or ``stdio`` with a pinned package."""

    name: str
    transport: str
    url: str = ""
    headers: tuple[Entry, ...] = ()
    command: str = ""
    package: str = ""
    args: tuple[str, ...] = ()
    env: tuple[Entry, ...] = ()


@dataclass(frozen=True)
class Settings:
    """The settings: model, class wish, tool list (``None``: no narrowing) and cost cap."""

    tool_class_wish: str
    model: str | None = None
    allowed_tools: tuple[str, ...] | None = None
    max_cost_usd_per_run: float | None = None


@dataclass(frozen=True)
class Definition:
    """A definition that passed the check (schema ``agent_definition.v1``)."""

    instructions: str
    skills: tuple[Skill, ...]
    mcp_servers: tuple[McpServer, ...]
    settings: Settings
    origin_kind: str


# ---- the rules as predicates ---------------------------------------------------------------


def has_frontmatter(text: str) -> bool:
    """Tell whether the first line of ``text`` is ``---`` (26 4.3 item 2)."""
    line = text.removeprefix("\ufeff").split("\n", 1)[0]
    return line.rstrip(" \t\r") == "---"


def has_inline_shell(text: str) -> bool:
    """Tell whether ``text`` holds one of the two inline shell forms of 26 4.3 item 3."""
    return _FENCE_SHELL in text or _INLINE_SHELL.search(text) is not None


def _outside_folder(path: str) -> bool:
    if path.startswith(("/", "\\", "~")) or _DRIVE.match(path):
        return True
    segments = [s for s in re.split(r"[/\\]", path) if s]
    return any(s.strip() == ".." for s in segments)


def has_import_outside_folder(text: str) -> bool:
    """Tell whether ``text`` holds an ``@`` import that leaves the agent folder (26 4.3 item 4)."""
    for match in _IMPORT.finditer(text):
        path = match.group(1).replace("\\ ", " ").split("#", 1)[0]
        if _outside_folder(path.lstrip("\"' ")):
            return True
    return False


def _valid_file_name(name: str) -> bool:
    return _FILE_NAME.fullmatch(name) is not None and name.lower() != "skill.md"


def _reserved_env(name: str) -> bool:
    return name in _RESERVED_ENV or name.startswith(_RESERVED_ENV_PREFIXES)


def _secret_name(name: str) -> bool:
    upper = name.upper()
    return any(word in upper for word in _SECRET_WORDS)


def _valid_secret_ref(ref: str) -> bool:
    scheme, sep, rest = ref.partition(":")
    if not sep:
        return False
    if scheme == "env":
        return _ENV_NAME.fullmatch(rest) is not None
    if scheme == "file":
        if not rest.startswith("/") or rest == "/":
            return False
        return all(p not in ("", ".", "..") for p in rest[1:].split("/"))
    return False


def _expands_variable(text: str) -> bool:
    """Report whether ``text`` holds ``${``, the start of a CLI variable reference.

    The CLI substitutes it from its own environment in the url, headers, args and env of an MCP
    server (26 4.2), so a plain value never holds it.
    """
    return "${" in text


def _valid_url(url: str) -> bool:
    if "#" in url or _expands_variable(url):
        return False
    try:
        parts = urlsplit(url)
        _ = parts.port  # a port that is not a number is refused like the server's parser
    except ValueError:
        return False
    host = parts.hostname or ""
    return (
        parts.scheme == "https"
        and bool(parts.netloc)
        and bool(host)
        and parts.username is None
        and "@" not in parts.netloc
        and not any(c in host for c in ' \\<>"{}|^`')
    )


def _valid_model(model: str) -> bool:
    return _MODEL.fullmatch(model) is not None and not model.startswith("-")


def _valid_tool(tool: str, listed: Sequence[str]) -> bool:
    if not tool.startswith("mcp__"):
        return _BUILTIN_TOOL.fullmatch(tool) is not None
    server, sep, rest = tool[len("mcp__") :].partition("__")
    if server not in listed:
        return False
    return not sep or _MCP_TOOL.fullmatch(rest) is not None


def text_rule(text: str) -> str | None:
    """The first character rule ``text`` breaks (SEC-IN-4), or ``None``."""
    for char in text:
        point = ord(char)
        if 0xD800 <= point <= 0xDFFF:  # a lone surrogate is no UTF-8 at all
            return "invalid_utf8"
        if unicodedata.category(char) == "Cc":
            return "control_character"
        if point in _BIDI:
            return "bidi_control"
    return None


def _pointer(base: str, name: str | int) -> str:
    text = str(name).replace("~", "~0").replace("/", "~1")
    return f"{base}/{text}"


# ---- the walk ------------------------------------------------------------------------------


class _Checker:
    """Walks one decoded definition and collects the findings in document order."""

    def __init__(self, limits: Limits, tool_class: str) -> None:
        self.lim = limits
        self.below_host = tool_class not in ("host",)
        self.found: list[Finding] = []

    def add(self, pointer: str, code: str) -> None:
        self.found.append(Finding(pointer, code))

    def object(self, p: str, value: Any, *allowed: str) -> dict[str, Any] | None:
        if not isinstance(value, dict):
            self.add(p, "invalid_type")
            return None
        for name in sorted(k for k in value if k not in allowed):
            self.add(_pointer(p, name), "unknown_member")
        return value

    def array(self, p: str, m: Mapping[str, Any], name: str, *, required: bool) -> list[Any] | None:
        if name not in m:
            if required:
                self.add(_pointer(p, name), "required")
            return None
        value = m[name]
        if not isinstance(value, list):
            self.add(_pointer(p, name), "invalid_type")
            return None
        return value

    def string(self, p: str, m: Mapping[str, Any], name: str, *, required: bool) -> str | None:
        if name not in m:
            if required:
                self.add(_pointer(p, name), "required")
            return None
        return self.string_value(_pointer(p, name), m[name])

    def string_value(self, p: str, value: Any) -> str | None:
        if not isinstance(value, str):
            self.add(p, "invalid_type")
            return None
        return value

    def display(self, p: str, text: str) -> bool:
        rule = text_rule(text)
        if rule is not None:
            self.add(p, rule)
            return False
        return True

    def markdown(self, p: str, text: str) -> None:
        self.display(p, text.replace("\n", " ").replace("\r", " "))

    def max_bytes(self, p: str, text: str, limit: int | None) -> None:
        if limit is not None and len(text.encode("utf-8", "surrogatepass")) > limit:
            self.add(p, "too_long")

    def max_items(self, p: str, count: int, limit: int | None) -> None:
        if limit is not None and count > limit:
            self.add(p, "too_many")

    def text_rules(self, p: str, text: str, *, front: bool) -> None:
        if front and has_frontmatter(text):
            self.add(p, "skill_frontmatter")
        if has_inline_shell(text):
            self.add(p, "inline_shell")
        if has_import_outside_folder(text):
            self.add(p, "import_outside_folder")

    def name(
        self, p: str, m: Mapping[str, Any], valid: Callable[[str], bool], seen: set[str]
    ) -> str:
        text = self.string(p, m, "name", required=True)
        if text is None:
            return ""
        np = _pointer(p, "name")
        key = text.lower() if valid is _valid_header_name else text
        if not self.display(np, text):
            pass
        elif not valid(text):
            self.add(np, "invalid_name")
        elif key in seen:
            self.add(np, "duplicate_name")
        else:
            seen.add(key)
        return text

    # -- members --

    def definition(self, value: Any) -> Definition | None:
        m = self.object("", value, "instructions", "skills", "mcp_servers", "settings", "origin")
        if m is None:
            return None
        instructions = self.string("", m, "instructions", required=True)
        if instructions is not None:
            self.markdown("/instructions", instructions)
            self.max_bytes("/instructions", instructions, self.lim.instructions_max_bytes)
            self.text_rules("/instructions", instructions, front=False)
        skills = self.skills(m)
        servers = self.servers(m)
        settings = self.settings(m, servers)
        origin = self.origin(m)
        return Definition(instructions or "", tuple(skills), tuple(servers), settings, origin)

    def skill_text(self, p: str, text: str) -> None:
        self.markdown(p, text)
        self.max_bytes(p, text, self.lim.skill_body_max_bytes)
        self.text_rules(p, text, front=True)

    def skills(self, m: Mapping[str, Any]) -> list[Skill]:
        items = self.array("", m, "skills", required=True)
        out: list[Skill] = []
        if items is None:
            return out
        self.max_items("/skills", len(items), self.lim.max_skills)
        seen: set[str] = set()
        for i, value in enumerate(items):
            ip = _pointer("/skills", i)
            sm = self.object(ip, value, "name", "description", "body", "files")
            if sm is None:
                continue
            name = self.name(ip, sm, _valid_skill_name, seen)
            description = self.string(ip, sm, "description", required=True)
            if description is not None:
                dp = _pointer(ip, "description")
                limit = self.lim.skill_description_max_chars
                if not self.display(dp, description):
                    pass
                elif not description.strip():
                    self.add(dp, "required")
                elif limit is not None and len(description) > limit:
                    self.add(dp, "too_long")
            body = self.string(ip, sm, "body", required=True)
            if body is not None:
                self.skill_text(_pointer(ip, "body"), body)
            files = self.files(ip, sm)
            out.append(Skill(name, description or "", body or "", tuple(files)))
        return out

    def files(self, p: str, m: Mapping[str, Any]) -> list[SkillFile]:
        items = self.array(p, m, "files", required=False)
        out: list[SkillFile] = []
        if items is None:
            return out
        fp = _pointer(p, "files")
        self.max_items(fp, len(items), self.lim.max_skill_files)
        seen: set[str] = set()
        for i, value in enumerate(items):
            ip = _pointer(fp, i)
            fm = self.object(ip, value, "name", "content")
            if fm is None:
                continue
            name = self.name(ip, fm, _valid_file_name, seen)
            content = self.string(ip, fm, "content", required=True)
            if content is not None:
                self.skill_text(_pointer(ip, "content"), content)
            out.append(SkillFile(name, content or ""))
        return out

    def servers(self, m: Mapping[str, Any]) -> list[McpServer]:
        items = self.array("", m, "mcp_servers", required=True)
        out: list[McpServer] = []
        if items is None:
            return out
        self.max_items("/mcp_servers", len(items), self.lim.max_mcp_servers)
        seen: set[str] = set()
        for i, value in enumerate(items):
            ip = _pointer("/mcp_servers", i)
            members: tuple[str, ...] = _ALL_MEMBERS
            if isinstance(value, dict):
                transport = value.get("transport")
                if transport == "http":
                    members = _HTTP_MEMBERS
                elif transport == "stdio":
                    members = _STDIO_MEMBERS
            sm = self.object(ip, value, *members)
            if sm is None:
                continue
            name = self.name(ip, sm, _valid_skill_name, seen)
            transport = self.string(ip, sm, "transport", required=True) or ""
            if transport == "http":
                out.append(self.http_server(ip, sm, name))
            elif transport == "stdio":
                out.append(self.stdio_server(ip, sm, name))
            else:
                if isinstance(sm.get("transport"), str):
                    self.add(_pointer(ip, "transport"), "invalid_enum")
                out.append(McpServer(name, transport))
        return out

    def http_server(self, p: str, m: Mapping[str, Any], name: str) -> McpServer:
        url = self.string(p, m, "url", required=True)
        if url is not None:
            up = _pointer(p, "url")
            if self.display(up, url) and not _valid_url(url):
                self.add(up, "invalid")
        headers = self.entries(p, m, "headers", _valid_header_name, None)
        return McpServer(name, "http", url=url or "", headers=tuple(headers))

    def stdio_server(self, p: str, m: Mapping[str, Any], name: str) -> McpServer:
        command = self.stdio_member(p, m, "command", "mcp_command_not_allowed")
        if command and command not in self.lim.stdio_runners:
            self.add(_pointer(p, "command"), "mcp_command_not_allowed")
        package = self.stdio_member(p, m, "package", "mcp_package_unpinned")
        if package and _PACKAGE.fullmatch(package) is None:
            self.add(_pointer(p, "package"), "mcp_package_unpinned")
        args: list[str] = []
        items = self.array(p, m, "args", required=False)
        if items is not None:
            ap = _pointer(p, "args")
            for i, value in enumerate(items):
                arg = self.string_value(_pointer(ap, i), value)
                if arg is not None:
                    if self.display(_pointer(ap, i), arg) and _expands_variable(arg):
                        self.add(_pointer(ap, i), "invalid")
                    args.append(arg)
        env = self.entries(p, m, "env", _valid_env_name, _reserved_env)
        if self.below_host:
            self.add(_pointer(p, "transport"), "tool_class_too_low")
        return McpServer(
            name,
            "stdio",
            command=command or "",
            package=package or "",
            args=tuple(args),
            env=tuple(env),
        )

    def stdio_member(self, p: str, m: Mapping[str, Any], name: str, missing: str) -> str | None:
        """``command`` or ``package``: missing is ``missing``; a usable text, else ``None``."""
        mp = _pointer(p, name)
        if name not in m:
            self.add(mp, missing)
            return None
        text = self.string_value(mp, m[name])
        if text is None or not self.display(mp, text):
            return None
        if not text:
            self.add(mp, missing)
            return None
        return text

    def entries(
        self,
        p: str,
        m: Mapping[str, Any],
        member: str,
        valid: Callable[[str], bool],
        reserved: Callable[[str], bool] | None,
    ) -> list[Entry]:
        items = self.array(p, m, member, required=False)
        out: list[Entry] = []
        if items is None:
            return out
        lp = _pointer(p, member)
        seen: set[str] = set()
        for i, value in enumerate(items):
            ip = _pointer(lp, i)
            em = self.object(ip, value, "name", "value", "secret_ref")
            if em is None:
                continue
            name = self.name(ip, em, valid, seen)
            if reserved is not None and valid(name) and reserved(name):
                self.add(_pointer(ip, "name"), "env_name_reserved")
            out.append(self.entry_value(ip, em, name))
        return out

    def entry_value(self, p: str, m: Mapping[str, Any], name: str) -> Entry:
        has_value, has_ref = "value" in m, "secret_ref" in m
        if has_value and has_ref:
            self.add(p, "invalid")
            return Entry(name)
        if not has_value and not has_ref:
            self.add(p, "required")
            return Entry(name)
        if has_value:
            vp = _pointer(p, "value")
            value = self.string_value(vp, m["value"])
            if value is not None:
                if self.display(vp, value) and _expands_variable(value):
                    self.add(vp, "invalid")
                if _secret_name(name):
                    self.add(vp, "secret_value_not_allowed")
            return Entry(name, value=value)
        rp = _pointer(p, "secret_ref")
        ref = self.string_value(rp, m["secret_ref"])
        if ref is not None and self.display(rp, ref) and not _valid_secret_ref(ref):
            self.add(rp, "invalid")
        return Entry(name, secret_ref=ref)

    def settings(self, m: Mapping[str, Any], servers: Sequence[McpServer]) -> Settings:
        if "settings" not in m:
            self.add("/settings", "required")
            return Settings("")
        sm = self.object(
            "/settings",
            m["settings"],
            "model",
            "tool_class_wish",
            "allowed_tools",
            "max_cost_usd_per_run",
        )
        if sm is None:
            return Settings("")
        model = self.string("/settings", sm, "model", required=False)
        if model is not None and self.display("/settings/model", model) and not _valid_model(model):
            self.add("/settings/model", "invalid")
        wish = self.string("/settings", sm, "tool_class_wish", required=True)
        if wish is not None and wish not in _CLASSES:
            self.add("/settings/tool_class_wish", "invalid_enum")
        tools = self.tools(sm, [s.name for s in servers])
        cost = self.cost(sm)
        return Settings(wish or "", model, tools, cost)

    def tools(self, m: Mapping[str, Any], listed: Sequence[str]) -> tuple[str, ...] | None:
        items = self.array("/settings", m, "allowed_tools", required=False)
        if items is None:
            return None
        tp = "/settings/allowed_tools"
        self.max_items(tp, len(items), self.lim.max_allowed_tools)
        out: list[str] = []
        seen: set[str] = set()
        for i, value in enumerate(items):
            ip = _pointer(tp, i)
            tool = self.string_value(ip, value)
            if tool is None:
                continue
            if not self.display(ip, tool):
                pass
            elif not _valid_tool(tool, listed):
                self.add(ip, "invalid")
            elif tool in seen:
                self.add(ip, "duplicate_name")
            else:
                seen.add(tool)
            out.append(tool)
        return tuple(out)

    def cost(self, m: Mapping[str, Any]) -> float | None:
        if "max_cost_usd_per_run" not in m:
            return None
        cp = "/settings/max_cost_usd_per_run"
        value = m["max_cost_usd_per_run"]
        if isinstance(value, bool) or not isinstance(value, int | float):
            self.add(cp, "invalid_type")
            return None
        number = float(value)
        limit = self.lim.max_cost_usd_per_run
        if not math.isfinite(number):
            self.add(cp, "invalid")
        elif number <= 0:
            self.add(cp, "too_small")
        elif limit is not None and number > limit:
            self.add(cp, "too_large")
        return number

    def origin(self, m: Mapping[str, Any]) -> str:
        if "origin" not in m:
            self.add("/origin", "required")
            return ""
        om = self.object("/origin", m["origin"], "kind", "template_slug", "template_version")
        if om is None:
            return ""
        kind = self.string("/origin", om, "kind", required=True)
        if kind is not None and kind not in _ORIGINS:
            self.add("/origin/kind", "invalid_enum")
        slug = self.string("/origin", om, "template_slug", required=False)
        sp = "/origin/template_slug"
        if slug is not None and self.display(sp, slug) and not _valid_skill_name(slug):
            self.add(sp, "invalid")
        version = self.string("/origin", om, "template_version", required=False)
        vp = "/origin/template_version"
        if (
            version is not None
            and self.display(vp, version)
            and _TEMPLATE_VERSION.fullmatch(version) is None
        ):
            self.add(vp, "invalid")
        return kind or ""


def _valid_skill_name(name: str) -> bool:
    return _SKILL_NAME.fullmatch(name) is not None


def _valid_env_name(name: str) -> bool:
    return _ENV_NAME.fullmatch(name) is not None


def _valid_header_name(name: str) -> bool:
    return _HEADER_NAME.fullmatch(name) is not None


def _no_constant(name: str) -> Any:
    raise ValueError(name)


def _decode(raw: bytes) -> tuple[Any, bool]:
    """One JSON value from ``raw``; ``(None, False)`` for anything else."""
    try:
        text = raw.decode("utf-8")
        return json.loads(text, parse_constant=_no_constant), True
    except (UnicodeDecodeError, ValueError, RecursionError):
        return None, False


def parse(raw: bytes, limits: Limits, tool_class: str) -> tuple[Definition | None, list[Finding]]:
    """Check ``raw`` like `check` and return the typed definition when nothing was found.

    Args:
        raw: The definition as JSON text (the ``definition`` member of ``getAgentDefinition``).
        limits: The bounds (`Limits.runner` in the runner).
        tool_class: The class the definition is checked for; below ``host`` every ``stdio``
            server is ``tool_class_too_low``.

    Returns:
        The definition (``None`` with a finding) and every finding in document order.
    """
    value, ok = _decode(raw)
    if not ok:
        return None, [Finding("", "invalid_type")]
    checker = _Checker(limits, tool_class)
    definition = checker.definition(value)
    if checker.found:
        return None, checker.found
    return definition, []


def check(raw: bytes, limits: Limits, tool_class: str) -> list[Finding]:
    """Return every finding of ``raw`` against 26 4.2 and 4.3; none means it passes.

    Args:
        raw: The definition as JSON text.
        limits: The bounds.
        tool_class: The class the definition is checked for.

    Returns:
        The findings in document order.
    """
    return parse(raw, limits, tool_class)[1]


def validate_value(value: Any, limits: Limits, tool_class: str) -> list[Finding]:
    """Check an already decoded definition like `check`.

    Args:
        value: The definition as decoded from JSON.
        limits: The bounds.
        tool_class: The class the definition is checked for.

    Returns:
        The findings in document order.
    """
    checker = _Checker(limits, tool_class)
    checker.definition(value)
    return checker.found


def validate(raw: str, limits: Limits, tool_class: str) -> list[Finding]:
    """Check the JSON text ``raw`` like `check`.

    Args:
        raw: The definition as JSON text.
        limits: The bounds.
        tool_class: The class the definition is checked for.

    Returns:
        The findings in document order.
    """
    return check(raw.encode("utf-8", "surrogatepass"), limits, tool_class)


def required_class(definition: Definition) -> str:
    """The lowest class a definition may run with: ``host`` with a ``stdio`` server (26 4.4).

    Args:
        definition: A checked definition.

    Returns:
        ``host`` or ``chat``.
    """
    if any(s.transport == "stdio" for s in definition.mcp_servers):
        return "host"
    return "chat"


def intersect(configured: Sequence[str], allowed: Sequence[str] | None) -> list[str]:
    """The tools of a hosted ``scoped`` turn (26 4.4 item 2, the rule of 09 18.1).

    Args:
        configured: ``runner.scoped_allowed_tools`` in its order.
        allowed: ``settings.allowed_tools`` of the definition; ``None`` narrows nothing.

    Returns:
        The configured tools the definition also names, in the order of the runner key.
    """
    return scoped_tools(configured, None if allowed is None else list(allowed))
