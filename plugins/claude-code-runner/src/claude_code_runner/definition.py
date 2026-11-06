# SPDX-License-Identifier: Apache-2.0
"""The runner's own copy of the definition rules (docs/spec/26-builtin-agents.md 4.2, 4.3).

The closed schema ``agent_definition.v1`` and its refusals, ported from the server's validator
(``services/server/internal/agents/domain/definition``) with the rulings of
``docs/decisions.d/A1-02.md``. It is the one place of these rules in the runner: the builder checks
its draft with it before it sends one (26 6.2 item 3), and the hosted turns of A1-05 check the
current definition with it before every turn (26 5.6). The shared corpus ``tests/agent-definitions``
holds it equal to the server: every fixture gives exactly its expected findings.

A finding names a JSON Pointer and a field code, never the submitted value.

Examples:
    >>> lim = Limits(64, 4, 300, 64, 2, 2, 8, 50.0, ("npx", "uvx"))
    >>> doc = ('{"instructions": "Run !`id`", "skills": [], "mcp_servers": [],'
    ...        ' "settings": {"tool_class_wish": "chat"}, "origin": {"kind": "manual"}}')
    >>> [(f.pointer, f.code) for f in validate(doc, lim, "chat")]
    [('/instructions', 'inline_shell')]
"""

from __future__ import annotations

import json
import math
import posixpath
import re
import unicodedata
from collections.abc import Callable, Mapping
from dataclasses import dataclass, field
from typing import Any
from urllib.parse import urlsplit

__all__ = [
    "CODE_INLINE_SHELL",
    "CODE_SKILL_FRONTMATTER",
    "CODE_UNKNOWN_MEMBER",
    "FieldError",
    "Limits",
    "has_frontmatter",
    "has_import_outside_folder",
    "has_inline_shell",
    "text_rule",
    "validate",
    "validate_value",
]

CODE_UNKNOWN_MEMBER = "unknown_member"
CODE_SKILL_FRONTMATTER = "skill_frontmatter"
CODE_INLINE_SHELL = "inline_shell"
CODE_IMPORT_OUTSIDE_FOLDER = "import_outside_folder"
_SECRET_VALUE = "secret_value_not_allowed"  # noqa: S105 - a field code  # nosec B105
_ENV_RESERVED = "env_name_reserved"
_MCP_COMMAND = "mcp_command_not_allowed"
_MCP_PACKAGE = "mcp_package_unpinned"
_CLASS_TOO_LOW = "tool_class_too_low"
_TOO_LONG = "too_long"
_TOO_MANY = "too_many"
_INVALID_NAME = "invalid_name"
_DUPLICATE_NAME = "duplicate_name"
_REQUIRED = "required"
_INVALID_TYPE = "invalid_type"
_INVALID_ENUM = "invalid_enum"
_INVALID = "invalid"
_TOO_LARGE = "too_large"
_TOO_SMALL = "too_small"

_CLASSES = ("chat", "scoped", "host")
_ORIGIN_KINDS = ("template", "interview", "manual")

_SKILL_NAME = re.compile(r"^[a-z0-9][a-z0-9-]{0,63}$")
_FILE_NAME = re.compile(r"^[a-z0-9][a-z0-9._-]{0,63}\.md$")
_ENV_NAME = re.compile(r"^[A-Z_][A-Z0-9_]*$")
_HEADER_NAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$")
_MODEL = re.compile(r"^[A-Za-z0-9._:/@-]+$")
_BUILTIN_TOOL = re.compile(r"^[A-Za-z][A-Za-z0-9_]*(\([^()]*\))?$")
_MCP_TOOL = re.compile(r"^[A-Za-z0-9_-]+$")
_PACKAGE = re.compile(
    r"^(@[a-z0-9][a-z0-9._-]*/)?[A-Za-z0-9][A-Za-z0-9._-]*@"
    r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$"
)
_TEMPLATE_VERSION = re.compile(r"^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$")
_DRIVE = re.compile(r"^[A-Za-z]:")

# The white space of a JavaScript regular expression (\s), which the pinned CLI uses in its inline
# shell pattern; Python's \s holds a different set, so the class is written out like the server's.
_JS_SPACE = "\t\n\v\f\r \u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"
# (?<=^|\s)!`[^`]+` of the pinned CLI with the look-behind written as a group.
_INLINE_SHELL = re.compile("(?:^|[" + _JS_SPACE + "])!`[^`]+`")
# An @ token at the start or after white space; a backslash followed by a space continues it.
_IMPORT = re.compile("(?:^|[" + _JS_SPACE + "])@((?:\\\\ |[^" + _JS_SPACE + "])+)")
_FENCE_SHELL = "```!"

_RESERVED_ENV = (
    "PATH",
    "HOME",
    "USER",
    "LOGNAME",
    "SHELL",
    "TMPDIR",
    "CLAUDE_CONFIG_DIR",
    "CLAUDE_PRODUCT_RUN",
    "NODE_OPTIONS",
)
_RESERVED_ENV_PREFIXES = ("ANTHROPIC_", "CLAUDE_CODE_", "LD_", "DYLD_")
_SECRET_WORDS = ("KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "AUTH", "COOKIE")

_HTTP_MEMBERS = ("name", "transport", "url", "headers")
_STDIO_MEMBERS = ("name", "transport", "command", "package", "args", "env")
_ALL_MEMBERS = ("name", "transport", "url", "headers", "command", "package", "args", "env")
_PARTS = ("instructions", "skills", "mcp_servers", "settings", "origin")
_MCP_PREFIX = "mcp__"


@dataclass(frozen=True)
class FieldError:
    """One finding: the JSON Pointer into the definition and the field code."""

    pointer: str
    code: str


@dataclass(frozen=True)
class Limits:
    """The configured bounds of a definition (26 14), the values of the server's keys.

    Attributes:
        instructions_max_bytes: ``agents.definition.instructions_max_bytes``.
        max_skills: ``agents.definition.max_skills``.
        skill_description_max_chars: ``agents.definition.skill_description_max_chars``.
        skill_body_max_bytes: ``agents.definition.skill_body_max_bytes``.
        max_skill_files: ``agents.definition.max_skill_files``.
        max_mcp_servers: ``agents.definition.max_mcp_servers``.
        max_allowed_tools: ``agents.definition.max_allowed_tools``.
        max_cost_usd_per_run: ``agents.definition.max_cost_usd_per_run``.
        stdio_runners: ``agents.mcp_stdio_runners``.
    """

    instructions_max_bytes: int
    max_skills: int
    skill_description_max_chars: int
    skill_body_max_bytes: int
    max_skill_files: int
    max_mcp_servers: int
    max_allowed_tools: int
    max_cost_usd_per_run: float
    stdio_runners: tuple[str, ...]

    @classmethod
    def from_keys(cls, values: Mapping[str, Any]) -> Limits:
        """Return the limits from a mapping of the server's configuration keys.

        Args:
            values: ``agents.definition.*`` and ``agents.mcp_stdio_runners`` by key, as in
                ``tests/agent-definitions/limits.json``.

        Returns:
            The limits.

        Raises:
            KeyError: A key is missing.
        """
        p = "agents.definition."
        return cls(
            instructions_max_bytes=int(values[p + "instructions_max_bytes"]),
            max_skills=int(values[p + "max_skills"]),
            skill_description_max_chars=int(values[p + "skill_description_max_chars"]),
            skill_body_max_bytes=int(values[p + "skill_body_max_bytes"]),
            max_skill_files=int(values[p + "max_skill_files"]),
            max_mcp_servers=int(values[p + "max_mcp_servers"]),
            max_allowed_tools=int(values[p + "max_allowed_tools"]),
            max_cost_usd_per_run=float(values[p + "max_cost_usd_per_run"]),
            stdio_runners=tuple(str(v) for v in values["agents.mcp_stdio_runners"]),
        )


def has_frontmatter(text: str) -> bool:
    """Tell whether the first line of ``text`` is ``---`` (26 4.3 item 2).

    An optional byte order mark and trailing blanks of the line are ignored.
    """
    first = text.removeprefix("\ufeff").split("\n", 1)[0]
    return first.rstrip(" \t\r") == "---"


def has_inline_shell(text: str) -> bool:
    """Tell whether ``text`` holds one of the two inline shell forms of 26 4.3 item 3."""
    return _FENCE_SHELL in text or _INLINE_SHELL.search(text) is not None


def has_import_outside_folder(text: str) -> bool:
    r"""Tell whether ``text`` holds an ``@`` import that leaves the agent folder (26 4.3 item 4).

    The path is read as the CLI reads it: an escaped space (``\ ``) becomes a space and everything
    from ``#`` on is dropped; a leading quote is ignored. It leaves the folder when it is absolute
    (``/``, ``\`` or a drive), starts with ``~`` or has a ``..`` segment with ``/`` or ``\``.
    """
    for match in _IMPORT.finditer(text):
        path = match.group(1).replace("\\ ", " ").split("#", 1)[0]
        if _outside_folder(path.lstrip("\"' ")):
            return True
    return False


def _outside_folder(path: str) -> bool:
    if path.startswith(("/", "\\", "~")) or _DRIVE.match(path):
        return True
    return any(seg.strip() == ".." for seg in re.split(r"[/\\]", path) if seg)


def _secret_name(name: str) -> bool:
    upper = name.upper()
    return any(word in upper for word in _SECRET_WORDS)


def _reserved_env(name: str) -> bool:
    return name in _RESERVED_ENV or name.startswith(_RESERVED_ENV_PREFIXES)


def _valid_secret_ref(ref: str) -> bool:
    scheme, sep, rest = ref.partition(":")
    if not sep:
        return False
    if scheme == "env":
        return _ENV_NAME.match(rest) is not None
    if scheme == "file":
        return rest != "/" and rest.startswith("/") and posixpath.normpath(rest) == rest
    return False


def _valid_url(url: str) -> bool:
    try:
        parts = urlsplit(url)
        host = parts.hostname
    except ValueError:
        return False
    return (
        parts.scheme == "https"
        and parts.netloc != ""
        and bool(host)
        and parts.username is None
        and "#" not in url
    )


def _valid_model(model: str) -> bool:
    return _MODEL.match(model) is not None and not model.startswith("-")


def _valid_tool(tool: str, listed: list[str]) -> bool:
    if not tool.startswith(_MCP_PREFIX):
        return _BUILTIN_TOOL.match(tool) is not None
    server, sep, rest = tool[len(_MCP_PREFIX) :].partition("__")
    if server not in listed:
        return False
    return not sep or _MCP_TOOL.match(rest) is not None


def text_rule(text: str) -> str | None:
    """Return the SEC-IN-4 rule a display text breaks, or ``None``.

    The rules are invalid UTF-8 (a lone surrogate), a control character (category Cc, line breaks
    included) and a bidi control.
    """
    for ch in text:
        cp = ord(ch)
        if 0xD800 <= cp <= 0xDFFF:
            return "invalid_utf8"
        if unicodedata.category(ch) == "Cc":
            return "control_character"
        if cp in (0x061C, 0x200E, 0x200F) or 0x202A <= cp <= 0x202E or 0x2066 <= cp <= 0x2069:
            return "bidi_control"
    return None


def _escape(name: str) -> str:
    return name.replace("~", "~0").replace("/", "~1")


def _at(pointer: str, name: str) -> str:
    return pointer + "/" + _escape(name)


@dataclass
class _Names:
    fold: bool = False
    seen: set[str] = field(default_factory=set)

    def first(self, name: str) -> bool:
        key = name.lower() if self.fold else name
        if key in self.seen:
            return False
        self.seen.add(key)
        return True


@dataclass
class _Checker:
    lim: Limits
    tool_class: str
    found: list[FieldError] = field(default_factory=list)

    def add(self, pointer: str, code: str) -> None:
        self.found.append(FieldError(pointer, code))

    def below_host(self) -> bool:
        rank = _CLASSES.index(self.tool_class) if self.tool_class in _CLASSES else -1
        return rank < _CLASSES.index("host")

    def object(self, pointer: str, value: Any, allowed: tuple[str, ...]) -> dict[str, Any] | None:
        if not isinstance(value, dict):
            self.add(pointer, _INVALID_TYPE)
            return None
        for name in sorted(k for k in value if k not in allowed):
            self.add(_at(pointer, name), CODE_UNKNOWN_MEMBER)
        return value

    def array(self, pointer: str, m: dict[str, Any], name: str, required: bool) -> list[Any] | None:
        if name not in m:
            if required:
                self.add(_at(pointer, name), _REQUIRED)
            return None
        value = m[name]
        if not isinstance(value, list):
            self.add(_at(pointer, name), _INVALID_TYPE)
            return None
        return value

    def string(self, pointer: str, m: dict[str, Any], name: str, required: bool) -> str | None:
        if name not in m:
            if required:
                self.add(_at(pointer, name), _REQUIRED)
            return None
        return self.string_value(_at(pointer, name), m[name])

    def string_value(self, pointer: str, value: Any) -> str | None:
        if not isinstance(value, str):
            self.add(pointer, _INVALID_TYPE)
            return None
        return value

    def display(self, pointer: str, text: str) -> bool:
        rule = text_rule(text)
        if rule is not None:
            self.add(pointer, rule)
            return False
        return True

    def markdown(self, pointer: str, text: str) -> None:
        self.display(pointer, text.replace("\n", " ").replace("\r", " "))

    def max_bytes(self, pointer: str, text: str, limit: int) -> None:
        if len(text.encode("utf-8", errors="surrogatepass")) > limit:
            self.add(pointer, _TOO_LONG)

    def max_items(self, pointer: str, n: int, limit: int) -> None:
        if n > limit:
            self.add(pointer, _TOO_MANY)

    def text_rules(self, pointer: str, text: str, front: bool) -> None:
        if front and has_frontmatter(text):
            self.add(pointer, CODE_SKILL_FRONTMATTER)
        if has_inline_shell(text):
            self.add(pointer, CODE_INLINE_SHELL)
        if has_import_outside_folder(text):
            self.add(pointer, CODE_IMPORT_OUTSIDE_FOLDER)

    def name(
        self, pointer: str, m: dict[str, Any], valid: Callable[[str], bool], names: _Names
    ) -> str | None:
        text = self.string(pointer, m, "name", True)
        if text is None:
            return None
        np = _at(pointer, "name")
        if not self.display(np, text):
            pass
        elif not valid(text):
            self.add(np, _INVALID_NAME)
        elif not names.first(text):
            self.add(np, _DUPLICATE_NAME)
        return text

    def definition(self, value: Any) -> None:
        m = self.object("", value, _PARTS)
        if m is None:
            return
        text = self.string("", m, "instructions", True)
        if text is not None:
            self.markdown("/instructions", text)
            self.max_bytes("/instructions", text, self.lim.instructions_max_bytes)
            self.text_rules("/instructions", text, False)
        self.skills(m)
        servers = self.servers(m)
        self.settings(m, servers)
        self.origin(m)

    def skills(self, m: dict[str, Any]) -> None:
        items = self.array("", m, "skills", True)
        if items is None:
            return
        self.max_items("/skills", len(items), self.lim.max_skills)
        names = _Names()
        for i, item in enumerate(items):
            ip = f"/skills/{i}"
            sm = self.object(ip, item, ("name", "description", "body", "files"))
            if sm is None:
                continue
            self.name(ip, sm, lambda n: _SKILL_NAME.match(n) is not None, names)
            desc = self.string(ip, sm, "description", True)
            if desc is not None:
                dp = ip + "/description"
                if not self.display(dp, desc):
                    pass
                elif desc.strip() == "":
                    self.add(dp, _REQUIRED)
                elif len(desc) > self.lim.skill_description_max_chars:
                    self.add(dp, _TOO_LONG)
            body = self.string(ip, sm, "body", True)
            if body is not None:
                self.skill_text(ip + "/body", body)
            self.files(ip, sm)

    def skill_text(self, pointer: str, text: str) -> None:
        self.markdown(pointer, text)
        self.max_bytes(pointer, text, self.lim.skill_body_max_bytes)
        self.text_rules(pointer, text, True)

    def files(self, pointer: str, m: dict[str, Any]) -> None:
        items = self.array(pointer, m, "files", False)
        if items is None:
            return
        fp = pointer + "/files"
        self.max_items(fp, len(items), self.lim.max_skill_files)
        names = _Names()
        for i, item in enumerate(items):
            ip = f"{fp}/{i}"
            fm = self.object(ip, item, ("name", "content"))
            if fm is None:
                continue
            self.name(
                ip,
                fm,
                lambda n: _FILE_NAME.match(n) is not None and n.lower() != "skill.md",
                names,
            )
            content = self.string(ip, fm, "content", True)
            if content is not None:
                self.skill_text(ip + "/content", content)

    def servers(self, m: dict[str, Any]) -> list[str]:
        items = self.array("", m, "mcp_servers", True)
        listed: list[str] = []
        if items is None:
            return listed
        self.max_items("/mcp_servers", len(items), self.lim.max_mcp_servers)
        names = _Names()
        for i, item in enumerate(items):
            ip = f"/mcp_servers/{i}"
            members: tuple[str, ...] = _ALL_MEMBERS
            if isinstance(item, dict):
                transport = item.get("transport")
                if transport == "http":
                    members = _HTTP_MEMBERS
                elif transport == "stdio":
                    members = _STDIO_MEMBERS
            sm = self.object(ip, item, members)
            if sm is None:
                continue
            server_name = self.name(ip, sm, lambda n: _SKILL_NAME.match(n) is not None, names)
            listed.append(server_name if server_name is not None else "")
            transport = self.string(ip, sm, "transport", True)
            if transport == "http":
                self.http_server(ip, sm)
            elif transport == "stdio":
                self.stdio_server(ip, sm)
            elif transport is not None:
                self.add(ip + "/transport", _INVALID_ENUM)
        return listed

    def http_server(self, pointer: str, m: dict[str, Any]) -> None:
        url = self.string(pointer, m, "url", True)
        if url is not None and self.display(pointer + "/url", url) and not _valid_url(url):
            self.add(pointer + "/url", _INVALID)
        self.entries(
            pointer,
            m,
            "headers",
            lambda n: _HEADER_NAME.match(n) is not None,
            fold=True,
            reserved=None,
        )

    def stdio_server(self, pointer: str, m: dict[str, Any]) -> None:
        cp = pointer + "/command"
        if "command" not in m:
            self.add(cp, _MCP_COMMAND)
        else:
            cmd = self.string_value(cp, m["command"])
            if cmd is not None and self.display(cp, cmd) and cmd not in self.lim.stdio_runners:
                self.add(cp, _MCP_COMMAND)
        pp = pointer + "/package"
        if "package" not in m:
            self.add(pp, _MCP_PACKAGE)
        else:
            pkg = self.string_value(pp, m["package"])
            if pkg is not None and self.display(pp, pkg) and _PACKAGE.match(pkg) is None:
                self.add(pp, _MCP_PACKAGE)
        args = self.array(pointer, m, "args", False)
        for i, arg in enumerate(args or []):
            ap = f"{pointer}/args/{i}"
            text = self.string_value(ap, arg)
            if text is not None:
                self.display(ap, text)
        self.entries(
            pointer,
            m,
            "env",
            lambda n: _ENV_NAME.match(n) is not None,
            fold=False,
            reserved=_reserved_env,
        )
        if self.below_host():
            self.add(pointer + "/transport", _CLASS_TOO_LOW)

    def entries(  # noqa: PLR0913 - the parts of one entry list
        self,
        pointer: str,
        m: dict[str, Any],
        member: str,
        valid: Callable[[str], bool],
        *,
        fold: bool,
        reserved: Callable[[str], bool] | None,
    ) -> None:
        items = self.array(pointer, m, member, False)
        if items is None:
            return
        lp = _at(pointer, member)
        names = _Names(fold=fold)
        for i, item in enumerate(items):
            ip = f"{lp}/{i}"
            em = self.object(ip, item, ("name", "value", "secret_ref"))
            if em is None:
                continue
            entry_name = self.name(ip, em, valid, names)
            if (
                reserved is not None
                and entry_name is not None
                and valid(entry_name)
                and reserved(entry_name)
            ):
                self.add(ip + "/name", _ENV_RESERVED)
            self.entry_value(ip, em, entry_name or "")

    def entry_value(self, pointer: str, m: dict[str, Any], name: str) -> None:
        has_value, has_ref = "value" in m, "secret_ref" in m
        if has_value and has_ref:
            self.add(pointer, _INVALID)
            return
        if not has_value and not has_ref:
            self.add(pointer, _REQUIRED)
            return
        if has_value:
            vp = pointer + "/value"
            value = self.string_value(vp, m["value"])
            if value is not None:
                self.display(vp, value)
                if _secret_name(name):
                    self.add(vp, _SECRET_VALUE)
            return
        rp = pointer + "/secret_ref"
        ref = self.string_value(rp, m["secret_ref"])
        if ref is not None and self.display(rp, ref) and not _valid_secret_ref(ref):
            self.add(rp, _INVALID)

    def settings(self, m: dict[str, Any], servers: list[str]) -> None:
        if "settings" not in m:
            self.add("/settings", _REQUIRED)
            return
        sm = self.object(
            "/settings",
            m["settings"],
            ("model", "tool_class_wish", "allowed_tools", "max_cost_usd_per_run"),
        )
        if sm is None:
            return
        model = self.string("/settings", sm, "model", False)
        if model is not None and self.display("/settings/model", model) and not _valid_model(model):
            self.add("/settings/model", _INVALID)
        wish = self.string("/settings", sm, "tool_class_wish", True)
        if wish is not None and wish not in _CLASSES:
            self.add("/settings/tool_class_wish", _INVALID_ENUM)
        self.tools(sm, servers)
        self.cost(sm)

    def tools(self, m: dict[str, Any], servers: list[str]) -> None:
        items = self.array("/settings", m, "allowed_tools", False)
        if items is None:
            return
        tp = "/settings/allowed_tools"
        self.max_items(tp, len(items), self.lim.max_allowed_tools)
        names = _Names()
        for i, item in enumerate(items):
            ip = f"{tp}/{i}"
            tool = self.string_value(ip, item)
            if tool is None:
                continue
            if not self.display(ip, tool):
                continue
            if not _valid_tool(tool, servers):
                self.add(ip, _INVALID)
            elif not names.first(tool):
                self.add(ip, _DUPLICATE_NAME)

    def cost(self, m: dict[str, Any]) -> None:
        if "max_cost_usd_per_run" not in m:
            return
        cp = "/settings/max_cost_usd_per_run"
        value = m["max_cost_usd_per_run"]
        if isinstance(value, bool) or not isinstance(value, int | float):
            self.add(cp, _INVALID_TYPE)
            return
        try:
            amount = float(value)
        except OverflowError:
            # An integer beyond the range of a float (10**400) is no amount, as on the server.
            self.add(cp, _INVALID)
            return
        if math.isinf(amount) or math.isnan(amount):
            self.add(cp, _INVALID)
        elif amount <= 0:
            self.add(cp, _TOO_SMALL)
        elif amount > self.lim.max_cost_usd_per_run:
            self.add(cp, _TOO_LARGE)

    def origin(self, m: dict[str, Any]) -> None:
        if "origin" not in m:
            self.add("/origin", _REQUIRED)
            return
        om = self.object("/origin", m["origin"], ("kind", "template_slug", "template_version"))
        if om is None:
            return
        kind = self.string("/origin", om, "kind", True)
        if kind is not None and kind not in _ORIGIN_KINDS:
            self.add("/origin/kind", _INVALID_ENUM)
        slug = self.string("/origin", om, "template_slug", False)
        if (
            slug is not None
            and self.display("/origin/template_slug", slug)
            and _SKILL_NAME.match(slug) is None
        ):
            self.add("/origin/template_slug", _INVALID)
        version = self.string("/origin", om, "template_version", False)
        if (
            version is not None
            and self.display("/origin/template_version", version)
            and _TEMPLATE_VERSION.match(version) is None
        ):
            self.add("/origin/template_version", _INVALID)


def validate_value(value: Any, limits: Limits, tool_class: str) -> list[FieldError]:
    """Check a decoded definition as ``agent_definition.v1`` for an agent of ``tool_class``.

    Args:
        value: The definition as decoded from JSON.
        limits: The bounds of 26 14.
        tool_class: ``chat``, ``scoped`` or ``host``; a ``stdio`` server below ``host`` is
            ``tool_class_too_low``.

    Returns:
        Every finding in document order; none means the definition passes 26 4.2 and 4.3.
    """
    checker = _Checker(limits, tool_class)
    checker.definition(value)
    return checker.found


def validate(raw: str, limits: Limits, tool_class: str) -> list[FieldError]:
    """Check the JSON text ``raw`` like `validate_value`.

    A text that is not exactly one JSON value is one ``invalid_type`` finding on the root.

    Args:
        raw: The JSON text.
        limits: The bounds of 26 14.
        tool_class: ``chat``, ``scoped`` or ``host``.

    Returns:
        Every finding.
    """
    try:
        value = json.loads(raw, parse_constant=_no_constant)
    except ValueError:
        return [FieldError("", _INVALID_TYPE)]
    return validate_value(value, limits, tool_class)


def _no_constant(name: str) -> Any:
    raise ValueError(name)
