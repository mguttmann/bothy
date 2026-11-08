# SPDX-License-Identifier: Apache-2.0
"""The rules of agent_definition.v1 (docs/spec/26-builtin-agents.md 4.2 and 4.3) in Python.

The template gate (tools/templates/template_gate.py) turns every template into a definition draft
and checks it with these rules. They are a port of the Go validator of the server
(services/server/internal/agents/domain/definition, A1-02): the same field codes, JSON Pointers
and patterns, read as Go reads them (RE2 patterns matched in full, Go's url.Parse, path.Clean and
strings.TrimSpace). tools/templates/tests/test_definition_rules.py runs the whole shared corpus
tests/agent-definitions through this module, so a difference to the Go validator fails there; a
rule changes in the corpus first (tests/agent-definitions/README.md).

Example:
    >>> lim = Limits.from_keys(json.loads(Path("tests/agent-definitions/limits.json").read_text()))
    >>> [(f.pointer, f.code) for f in validate({"skills": []}, lim, "chat")][:1]
    [('/instructions', 'required')]
"""

from __future__ import annotations

import json
import math
import posixpath
import re
import unicodedata
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

# The field codes of 26 4.3 (docs/decisions.d/A1-02.md D-A1-02-1).
UNKNOWN_MEMBER = "unknown_member"
SKILL_FRONTMATTER = "skill_frontmatter"
INLINE_SHELL = "inline_shell"
IMPORT_OUTSIDE_FOLDER = "import_outside_folder"
SECRET_VALUE_NOT_ALLOWED = "secret_value_not_allowed"  # noqa: S105  # nosec B105 - a field code
ENV_NAME_RESERVED = "env_name_reserved"
MCP_COMMAND_NOT_ALLOWED = "mcp_command_not_allowed"
MCP_PACKAGE_UNPINNED = "mcp_package_unpinned"
TOOL_CLASS_TOO_LOW = "tool_class_too_low"
TOO_LONG = "too_long"
TOO_MANY = "too_many"
INVALID_NAME = "invalid_name"
DUPLICATE_NAME = "duplicate_name"
# The shape codes of the request validation of the API and the codes of the text check.
REQUIRED = "required"
INVALID_TYPE = "invalid_type"
INVALID_ENUM = "invalid_enum"
INVALID = "invalid"
TOO_LARGE = "too_large"
TOO_SMALL = "too_small"
INVALID_UTF8 = "invalid_utf8"
CONTROL_CHARACTER = "control_character"
BIDI_CONTROL = "bidi_control"

# The patterns of 26 4.2 and of docs/decisions.d/A1-02.md, matched in full (Go's ^...$ in RE2
# never matches before a trailing line feed, Python's $ does, so fullmatch is used).
SKILL_NAME = re.compile(r"[a-z0-9][a-z0-9-]{0,63}")
FILE_NAME = re.compile(r"[a-z0-9][a-z0-9._-]{0,63}\.md")
ENV_NAME = re.compile(r"[A-Z_][A-Z0-9_]*")
HEADER_NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_-]{0,127}")
MODEL = re.compile(r"[A-Za-z0-9._:/@-]+")
BUILTIN_TOOL = re.compile(r"[A-Za-z][A-Za-z0-9_]*(\([^()]*\))?")
MCP_TOOL = re.compile(r"[A-Za-z0-9_-]+")
PACKAGE = re.compile(
    r"(@[a-z0-9][a-z0-9._-]*/)?[A-Za-z0-9][A-Za-z0-9._-]*@"
    r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?"
)
TEMPLATE_VERSION = re.compile(r"[0-9A-Za-z][0-9A-Za-z.+-]{0,63}")

# The white space of a JavaScript regular expression (\s), which the pinned CLI uses.
JS_SPACE = "\t\n\v\f\r " + "".join(
    chr(c)
    for c in (
        0x00A0,
        0x1680,
        *range(0x2000, 0x200B),
        0x2028,
        0x2029,
        0x202F,
        0x205F,
        0x3000,
        0xFEFF,
    )
)
_JS_SPACE_CLASS = re.escape(JS_SPACE)
INLINE_SHELL_PATTERN = re.compile("(?:^|[" + _JS_SPACE_CLASS + "])!`[^`]+`")
IMPORT_PATTERN = re.compile(
    "(?:^|[" + _JS_SPACE_CLASS + r"])@((?:\\ |[^" + _JS_SPACE_CLASS + "])+)"
)
DRIVE = re.compile(r"[A-Za-z]:")
FENCE_SHELL = "```!"

# Go's unicode.IsSpace: the Latin-1 spaces and the White_Space property above it.
_GO_SPACE = "\t\n\v\f\r " + "".join(
    chr(c)
    for c in (
        0x0085,
        0x00A0,
        0x1680,
        *range(0x2000, 0x200B),
        0x2028,
        0x2029,
        0x202F,
        0x205F,
        0x3000,
    )
)

RESERVED_ENV_NAMES = frozenset(
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
RESERVED_ENV_PREFIXES = ("ANTHROPIC_", "CLAUDE_CODE_", "LD_", "DYLD_")
SECRET_WORDS = ("KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "AUTH", "COOKIE")

TOOL_CLASSES = ("chat", "scoped", "host")
ORIGIN_KINDS = ("template", "interview", "manual")

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


@dataclass(frozen=True)
class Limits:
    """The configured bounds of a definition (26 14), by the names of their configuration keys."""

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
    def from_keys(cls, values: dict[str, Any]) -> Limits:
        """Build the limits from configuration keys (tests/agent-definitions/limits.json).

        Raises:
            ValueError: a key is missing or unknown.
        """
        unknown = sorted(set(values) - set(_LIMIT_KEYS))
        if unknown:
            raise ValueError(f"unknown limit keys: {', '.join(unknown)}")
        missing = sorted(set(_LIMIT_KEYS) - set(values))
        if missing:
            raise ValueError(f"missing limit keys: {', '.join(missing)}")
        kw: dict[str, Any] = {attr: values[key] for key, attr in _LIMIT_KEYS.items()}
        kw["stdio_runners"] = tuple(kw["stdio_runners"])
        return cls(**kw)


@dataclass(frozen=True)
class FieldError:
    """One finding: the JSON Pointer into the definition and the field code, never the value."""

    pointer: str
    code: str


# ---- text rules ------------------------------------------------------------------------------


def _go_trim_space(s: str) -> str:
    return s.strip(_GO_SPACE)


def is_bidi_control(ch: str) -> bool:
    """Report whether `ch` is a bidi control character (kernel/textcheck)."""
    c = ord(ch)
    return c in (0x061C, 0x200E, 0x200F) or 0x202A <= c <= 0x202E or 0x2066 <= c <= 0x2069


def display_rule(s: str) -> str | None:
    """Return the text rule a one-line text breaks (kernel/textcheck.DisplayText), or None.

    A lone surrogate counts as invalid UTF-8; a decoder of JSON text replaces it before (see
    `validate_text`).
    """
    for ch in s:
        if 0xD800 <= ord(ch) <= 0xDFFF:
            return INVALID_UTF8
        if unicodedata.category(ch) == "Cc":
            return CONTROL_CHARACTER
        if is_bidi_control(ch):
            return BIDI_CONTROL
    return None


def has_frontmatter(s: str) -> bool:
    """Report whether the first line of `s`, after a byte order mark, is `---` (26 4.3 item 2)."""
    s = s.removeprefix("﻿")
    line = s.split("\n", 1)[0]
    return line.rstrip(" \t\r") == "---"


def has_inline_shell(s: str) -> bool:
    """Report whether `s` holds one of the two inline shell forms of 26 4.3 item 3."""
    return FENCE_SHELL in s or INLINE_SHELL_PATTERN.search(s) is not None


def _outside_folder(p: str) -> bool:
    if p.startswith(("/", "\\", "~")) or DRIVE.match(p):
        return True
    return any(_go_trim_space(seg) == ".." for seg in re.split(r"[/\\]", p) if seg)


def has_import_outside_folder(s: str) -> bool:
    """Report whether `s` holds an `@` import that leaves the agent folder (26 4.3 item 4).

    The path is read as the pinned CLI reads it: an escaped space becomes a space, everything
    from `#` on is dropped, leading quotes and spaces are ignored.
    """
    for m in IMPORT_PATTERN.finditer(s):
        p = m.group(1).replace("\\ ", " ").split("#", 1)[0]
        if _outside_folder(p.lstrip("\"' ")):
            return True
    return False


# ---- value rules -----------------------------------------------------------------------------


def valid_skill_name(n: str) -> bool:
    """Report whether `n` is a skill name, an MCP server name or a template slug."""
    return SKILL_NAME.fullmatch(n) is not None


def valid_file_name(n: str) -> bool:
    """Report whether `n` is a flat Markdown file name of a skill other than SKILL.md."""
    return FILE_NAME.fullmatch(n) is not None and n.lower() != "skill.md"


def valid_env_name(n: str) -> bool:
    """Report whether `n` is the name of an environment entry."""
    return ENV_NAME.fullmatch(n) is not None


def valid_header_name(n: str) -> bool:
    """Report whether `n` is the name of a header."""
    return HEADER_NAME.fullmatch(n) is not None


def reserved_env(n: str) -> bool:
    """Report whether `n` is an environment name the runner sets itself (26 4.3 item 5)."""
    return n in RESERVED_ENV_NAMES or n.startswith(RESERVED_ENV_PREFIXES)


def secret_name(n: str) -> bool:
    """Report whether an entry named `n` must carry secret_ref instead of a plain value."""
    u = n.upper()
    return any(w in u for w in SECRET_WORDS)


def _go_clean_abs(p: str) -> bool:
    # Go's path.Clean turns a leading "//" into "/", posixpath.normpath keeps it.
    return not p.startswith("//") and posixpath.normpath(p) == p


def valid_secret_ref(r: str) -> bool:
    """Report whether `r` is `env:<NAME>` or `file:<clean absolute path>` other than `/`."""
    scheme, sep, rest = r.partition(":")
    if not sep:
        return False
    if scheme == "env":
        return valid_env_name(rest)
    if scheme == "file":
        return rest != "/" and rest.startswith("/") and _go_clean_abs(rest)
    return False


_SCHEME = re.compile(r"([A-Za-z][A-Za-z0-9+.-]*):(.*)", re.DOTALL)
_HOST_EXTRA = set("-_.~!$&'()*+,;=:[]<>\"")
_PCT = re.compile(r"%(?![0-9A-Fa-f]{2})")


def _valid_host(host: str) -> bool:
    if host.startswith("["):
        if _PCT.search(host):
            return False
        end = host.find("]")
        if end < 0:
            return False
        port = host[end + 1 :]
        return not port or re.fullmatch(r":[0-9]*", port) is not None
    i = host.rfind(":")
    if i >= 0 and not re.fullmatch(r"[0-9]*", host[i + 1 :]):
        return False
    # Go accepts a percent escape in a host name only for bytes above ASCII; none is needed here.
    return "%" not in host and all(
        ord(c) >= 0x80 or (c.isascii() and c.isalnum()) or c in _HOST_EXTRA for c in host
    )


def _hostname(host: str) -> str:
    if host.startswith("["):
        return host[1 : host.find("]")]
    return host.rsplit(":", 1)[0] if ":" in host else host


def expands_variable(s: str) -> bool:
    """Report whether `s` holds ``${``, the start of a CLI variable reference.

    The Claude Code CLI substitutes it in the url, headers, args and env of an MCP server from
    its own environment (26 4.2).
    """
    return "${" in s


def valid_url(u: str) -> bool:
    """Report whether `u` is an https URL with a host, no user information, fragment or ``${``.

    Read as Go's url.Parse reads it: a control character, an invalid percent escape, a bad
    port or a character outside the host set fails; the scheme compares without case.
    """
    if "#" in u or expands_variable(u) or any(ord(c) < 0x20 or ord(c) == 0x7F for c in u):
        return False
    m = _SCHEME.fullmatch(u)
    if m is None or m.group(1).lower() != "https":
        return False
    rest = m.group(2)
    if not rest.startswith("//"):
        return False
    rest = rest[2:]
    cut = min((i for i in (rest.find("/"), rest.find("?")) if i >= 0), default=len(rest))
    authority, tail = rest[:cut], rest[cut:]
    path = tail.split("?", 1)[0]
    if "@" in authority or not _valid_host(authority) or _PCT.search(path):
        return False
    return _hostname(authority) != ""


def pinned_package(p: str) -> bool:
    """Report whether `p` is `<name>@<exact version>` (26 4.3 item 6)."""
    return PACKAGE.fullmatch(p) is not None


def valid_model(m: str) -> bool:
    """Report whether `m` follows the pattern of default_model and does not start with a dash."""
    return MODEL.fullmatch(m) is not None and not m.startswith("-")


def valid_tool(t: str, listed: list[str]) -> bool:
    """Report whether `t` is a CLI tool name or `mcp__<server>[__<tool>]` of a listed server."""
    if not t.startswith("mcp__"):
        return BUILTIN_TOOL.fullmatch(t) is not None
    server, sep, tool = t.removeprefix("mcp__").partition("__")
    if server not in listed:
        return False
    return not sep or MCP_TOOL.fullmatch(tool) is not None


# ---- the walk --------------------------------------------------------------------------------


def _at(p: str, name: str) -> str:
    return p + "/" + name.replace("~", "~0").replace("/", "~1")


def _index(p: str, i: int) -> str:
    return f"{p}/{i}"


@dataclass
class _Names:
    fold: bool = False
    seen: set[str] = field(default_factory=set)

    def first(self, n: str) -> bool:
        key = n.lower() if self.fold else n
        if key in self.seen:
            return False
        self.seen.add(key)
        return True


_HTTP_MEMBERS = ("name", "transport", "url", "headers")
_STDIO_MEMBERS = ("name", "transport", "command", "package", "args", "env")
_ALL_MEMBERS = ("name", "transport", "url", "headers", "command", "package", "args", "env")


def _is_number(v: object) -> bool:
    return isinstance(v, int | float) and not isinstance(v, bool)


class _Checker:
    def __init__(self, lim: Limits, tool_class: str) -> None:
        self.lim = lim
        self.rank = TOOL_CLASSES.index(tool_class) + 1 if tool_class in TOOL_CLASSES else 0
        self.found: list[FieldError] = []

    def add(self, pointer: str, code: str) -> None:
        self.found.append(FieldError(pointer, code))

    def obj(self, p: str, v: object, *allowed: str) -> dict[str, Any] | None:
        if not isinstance(v, dict):
            self.add(p, INVALID_TYPE)
            return None
        for k in sorted(k for k in v if k not in allowed):
            self.add(_at(p, k), UNKNOWN_MEMBER)
        return v

    def array(self, p: str, m: dict[str, Any], name: str, required: bool) -> list[Any] | None:
        if name not in m:
            if required:
                self.add(_at(p, name), REQUIRED)
            return None
        v = m[name]
        if not isinstance(v, list):
            self.add(_at(p, name), INVALID_TYPE)
            return None
        return v

    def str_member(self, p: str, m: dict[str, Any], name: str, required: bool) -> str | None:
        if name not in m:
            if required:
                self.add(_at(p, name), REQUIRED)
            return None
        return self.str_value(_at(p, name), m[name])

    def str_value(self, p: str, v: object) -> str | None:
        if not isinstance(v, str):
            self.add(p, INVALID_TYPE)
            return None
        return v

    def display(self, p: str, s: str) -> bool:
        rule = display_rule(s)
        if rule is not None:
            self.add(p, rule)
            return False
        return True

    def markdown(self, p: str, s: str) -> None:
        self.display(p, s.replace("\n", " ").replace("\r", " "))

    def max_bytes(self, p: str, s: str, limit: int) -> None:
        if len(s.encode("utf-8", "surrogatepass")) > limit:
            self.add(p, TOO_LONG)

    def max_items(self, p: str, n: int, limit: int) -> None:
        if n > limit:
            self.add(p, TOO_MANY)

    def text_rules(self, p: str, s: str, front: bool) -> None:
        if front and has_frontmatter(s):
            self.add(p, SKILL_FRONTMATTER)
        if has_inline_shell(s):
            self.add(p, INLINE_SHELL)
        if has_import_outside_folder(s):
            self.add(p, IMPORT_OUTSIDE_FOLDER)

    def name(self, p: str, m: dict[str, Any], valid: Callable[[str], bool], names: _Names) -> str:
        s = self.str_member(p, m, "name", True)
        if s is None:
            return ""
        np = _at(p, "name")
        if not self.display(np, s):
            pass
        elif not valid(s):
            self.add(np, INVALID_NAME)
        elif not names.first(s):
            self.add(np, DUPLICATE_NAME)
        return s

    def definition(self, v: object) -> None:
        m = self.obj("", v, "instructions", "skills", "mcp_servers", "settings", "origin")
        if m is None:
            return
        s = self.str_member("", m, "instructions", True)
        if s is not None:
            self.markdown("/instructions", s)
            self.max_bytes("/instructions", s, self.lim.instructions_max_bytes)
            self.text_rules("/instructions", s, front=False)
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
        for i, v in enumerate(items):
            ip = _index("/skills", i)
            sm = self.obj(ip, v, "name", "description", "body", "files")
            if sm is None:
                continue
            self.name(ip, sm, valid_skill_name, names)
            d = self.str_member(ip, sm, "description", True)
            if d is not None:
                dp = _at(ip, "description")
                if not self.display(dp, d):
                    pass
                elif _go_trim_space(d) == "":
                    self.add(dp, REQUIRED)
                elif len(d) > self.lim.skill_description_max_chars:
                    self.add(dp, TOO_LONG)
            b = self.str_member(ip, sm, "body", True)
            if b is not None:
                self.skill_text(_at(ip, "body"), b)
            self.files(ip, sm)

    def skill_text(self, p: str, s: str) -> None:
        self.markdown(p, s)
        self.max_bytes(p, s, self.lim.skill_body_max_bytes)
        self.text_rules(p, s, front=True)

    def files(self, p: str, m: dict[str, Any]) -> None:
        items = self.array(p, m, "files", False)
        if items is None:
            return
        fp = _at(p, "files")
        self.max_items(fp, len(items), self.lim.max_skill_files)
        names = _Names()
        for i, v in enumerate(items):
            ip = _index(fp, i)
            fm = self.obj(ip, v, "name", "content")
            if fm is None:
                continue
            self.name(ip, fm, valid_file_name, names)
            s = self.str_member(ip, fm, "content", True)
            if s is not None:
                self.skill_text(_at(ip, "content"), s)

    def servers(self, m: dict[str, Any]) -> list[str]:
        items = self.array("", m, "mcp_servers", True)
        listed: list[str] = []
        if items is None:
            return listed
        self.max_items("/mcp_servers", len(items), self.lim.max_mcp_servers)
        names = _Names()
        for i, v in enumerate(items):
            ip = _index("/mcp_servers", i)
            members: tuple[str, ...] = _ALL_MEMBERS
            if isinstance(v, dict):
                transport = v.get("transport")
                if transport == "http":
                    members = _HTTP_MEMBERS
                elif transport == "stdio":
                    members = _STDIO_MEMBERS
            sm = self.obj(ip, v, *members)
            if sm is None:
                continue
            listed.append(self.name(ip, sm, valid_skill_name, names))
            transport_value = self.str_member(ip, sm, "transport", True)
            if transport_value == "http":
                self.http_server(ip, sm)
            elif transport_value == "stdio":
                self.stdio_server(ip, sm)
            elif transport_value is not None:
                self.add(_at(ip, "transport"), INVALID_ENUM)
        return listed

    def http_server(self, p: str, m: dict[str, Any]) -> None:
        u = self.str_member(p, m, "url", True)
        if u is not None:
            up = _at(p, "url")
            if self.display(up, u) and not valid_url(u):
                self.add(up, INVALID)
        self.entries(p, m, "headers", valid_header_name, fold=True, reserved=None)

    def stdio_server(self, p: str, m: dict[str, Any]) -> None:
        cp = _at(p, "command")
        if "command" not in m:
            self.add(cp, MCP_COMMAND_NOT_ALLOWED)
        else:
            cmd = self.str_value(cp, m["command"])
            if cmd is not None and self.display(cp, cmd) and cmd not in self.lim.stdio_runners:
                self.add(cp, MCP_COMMAND_NOT_ALLOWED)
        pp = _at(p, "package")
        if "package" not in m:
            self.add(pp, MCP_PACKAGE_UNPINNED)
        else:
            pkg = self.str_value(pp, m["package"])
            if pkg is not None and self.display(pp, pkg) and not pinned_package(pkg):
                self.add(pp, MCP_PACKAGE_UNPINNED)
        args = self.array(p, m, "args", False)
        if args is not None:
            ap = _at(p, "args")
            for i, v in enumerate(args):
                a = self.str_value(_index(ap, i), v)
                if a is not None and self.display(_index(ap, i), a) and expands_variable(a):
                    self.add(_index(ap, i), INVALID)
        self.entries(p, m, "env", valid_env_name, fold=False, reserved=reserved_env)
        if self.rank < TOOL_CLASSES.index("host") + 1:
            self.add(_at(p, "transport"), TOOL_CLASS_TOO_LOW)

    def entries(
        self,
        p: str,
        m: dict[str, Any],
        member: str,
        valid: Callable[[str], bool],
        fold: bool,
        reserved: Callable[[str], bool] | None,
    ) -> None:
        items = self.array(p, m, member, False)
        if items is None:
            return
        lp = _at(p, member)
        names = _Names(fold=fold)
        for i, v in enumerate(items):
            ip = _index(lp, i)
            em = self.obj(ip, v, "name", "value", "secret_ref")
            if em is None:
                continue
            name = self.name(ip, em, valid, names)
            if reserved is not None and valid(name) and reserved(name):
                self.add(_at(ip, "name"), ENV_NAME_RESERVED)
            self.entry_value(ip, em, name)

    def entry_value(self, p: str, m: dict[str, Any], name: str) -> None:
        has_value, has_ref = "value" in m, "secret_ref" in m
        if has_value and has_ref:
            self.add(p, INVALID)
            return
        if not has_value and not has_ref:
            self.add(p, REQUIRED)
            return
        if has_value:
            vp = _at(p, "value")
            v = self.str_value(vp, m["value"])
            if v is not None:
                if self.display(vp, v) and expands_variable(v):
                    self.add(vp, INVALID)
                if secret_name(name):
                    self.add(vp, SECRET_VALUE_NOT_ALLOWED)
            return
        rp = _at(p, "secret_ref")
        r = self.str_value(rp, m["secret_ref"])
        if r is not None and self.display(rp, r) and not valid_secret_ref(r):
            self.add(rp, INVALID)

    def settings(self, m: dict[str, Any], servers: list[str]) -> None:
        p = "/settings"
        if "settings" not in m:
            self.add(p, REQUIRED)
            return
        sm = self.obj(p, m["settings"], *_SETTINGS_MEMBERS)
        if sm is None:
            return
        model = self.str_member(p, sm, "model", False)
        if model is not None:
            mp = _at(p, "model")
            if self.display(mp, model) and not valid_model(model):
                self.add(mp, INVALID)
        wish = self.str_member(p, sm, "tool_class_wish", True)
        if wish is not None and wish not in TOOL_CLASSES:
            self.add(_at(p, "tool_class_wish"), INVALID_ENUM)
        self.tools(p, sm, servers)
        self.cost(p, sm)

    def tools(self, p: str, m: dict[str, Any], servers: list[str]) -> None:
        items = self.array(p, m, "allowed_tools", False)
        if items is None:
            return
        tp = _at(p, "allowed_tools")
        self.max_items(tp, len(items), self.lim.max_allowed_tools)
        names = _Names()
        for i, v in enumerate(items):
            ip = _index(tp, i)
            t = self.str_value(ip, v)
            if t is None:
                continue
            if not self.display(ip, t):
                pass
            elif not valid_tool(t, servers):
                self.add(ip, INVALID)
            elif not names.first(t):
                self.add(ip, DUPLICATE_NAME)

    def cost(self, p: str, m: dict[str, Any]) -> None:
        if "max_cost_usd_per_run" not in m:
            return
        cp = _at(p, "max_cost_usd_per_run")
        v = m["max_cost_usd_per_run"]
        if not _is_number(v):
            self.add(cp, INVALID_TYPE)
            return
        try:
            f = float(v)
        except OverflowError:
            f = math.inf
        if math.isinf(f):
            self.add(cp, INVALID)
        elif f <= 0:
            self.add(cp, TOO_SMALL)
        elif f > self.lim.max_cost_usd_per_run:
            self.add(cp, TOO_LARGE)

    def origin(self, m: dict[str, Any]) -> None:
        p = "/origin"
        if "origin" not in m:
            self.add(p, REQUIRED)
            return
        om = self.obj(p, m["origin"], "kind", "template_slug", "template_version")
        if om is None:
            return
        kind = self.str_member(p, om, "kind", True)
        if kind is not None and kind not in ORIGIN_KINDS:
            self.add(_at(p, "kind"), INVALID_ENUM)
        slug = self.str_member(p, om, "template_slug", False)
        if slug is not None:
            sp = _at(p, "template_slug")
            if self.display(sp, slug) and not valid_skill_name(slug):
                self.add(sp, INVALID)
        version = self.str_member(p, om, "template_version", False)
        if version is not None:
            vp = _at(p, "template_version")
            if self.display(vp, version) and TEMPLATE_VERSION.fullmatch(version) is None:
                self.add(vp, INVALID)


_SETTINGS_MEMBERS = ("model", "tool_class_wish", "allowed_tools", "max_cost_usd_per_run")


def validate(definition: object, lim: Limits, tool_class: str) -> list[FieldError]:
    """Check a decoded definition as agent_definition.v1 for an agent of class `tool_class`.

    Args:
        definition: the document as `json.loads` returns it (lone surrogates already replaced,
            see `validate_text`).
        lim: the configured limits.
        tool_class: the agent's tool class (`chat`, `scoped` or `host`); it decides
            `tool_class_too_low`.

    Returns:
        Every finding in document order; none means the definition passes 26 4.2 and 4.3.
    """
    c = _Checker(lim, tool_class)
    c.definition(definition)
    return c.found


def _replace_surrogates(v: Any) -> Any:
    # Go's JSON decoder replaces a lone surrogate escape with U+FFFD; Python keeps it.
    if isinstance(v, str):
        return re.sub("[\ud800-\udfff]", "�", v)
    if isinstance(v, list):
        return [_replace_surrogates(x) for x in v]
    if isinstance(v, dict):
        return {_replace_surrogates(k): _replace_surrogates(x) for k, x in v.items()}
    return v


def _no_constant(name: str) -> Any:
    raise ValueError(f"not JSON: {name}")


def validate_text(raw: str, lim: Limits, tool_class: str) -> list[FieldError]:
    """Check the JSON text `raw` like `validate`.

    A text that is not one JSON value is one `invalid_type` finding on the root, as in the
    server; a lone surrogate escape reads as U+FFFD, as Go's decoder reads it.
    """
    try:
        value = json.loads(raw, parse_constant=_no_constant)
    except ValueError:
        return [FieldError("", INVALID_TYPE)]
    return validate(_replace_surrogates(value), lim, tool_class)
