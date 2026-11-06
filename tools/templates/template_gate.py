# SPDX-License-Identifier: Apache-2.0
"""Template gate: every agent template is generic and closed (docs/spec/26-builtin-agents.md 7.3).

`make templates-check` runs this over `templates/agents/`. Per template folder it checks, in this
order (the brief A1-06, design note 2):

1. ``layout``: exactly ``template.yaml``, ``CLAUDE.md`` and ``skills/<name>/SKILL.md`` per listed
   skill (26 7.1); every other file, folder or symbolic link is a finding, among them
   ``.claude/``, ``.mcp.json``, ``.lsp.json``, a plugin manifest, hooks, commands, agents and
   output styles (26 5.5).
2. ``schema``: ``template.yaml`` follows ``agent_template.v1``.
3. ``frontmatter``: each ``SKILL.md`` starts with a frontmatter of ``name`` and ``description``
   only.
4. ``definition``: the template as a definition draft passes the rules of 26 4.3
   (tools/templates/definition_rules.py, the port of the server's validator).
5. ``host``, ``path``, ``ip``, ``person``, ``secret``: hosts, URLs and e-mail addresses outside
   ``example.com``, ``example.org`` and ``example.net``; absolute and ``~`` paths; IP addresses;
   names of people outside Alex, Sam and Robin (a heuristic, see `person_names`); values in a
   token format of ``.gitleaks.toml``.
6. ``denylist``: the generic deny rules of tools/denylist, and the private deny list when one is
   reachable as for ``denylist_scan.py --allow-missing``. The gate never needs the private list.

Each finding prints as ``templates: <path>: <rule>: <detail>``; a detail never holds a matched
private term (every printed text is masked with the deny rules), and a secret-looking value is
never printed.

Usage:
  template_gate.py [--root templates/agents] [--repo .] [--list <private deny list>]

Exit codes: 0 clean, 1 findings, 2 configuration error.
"""

from __future__ import annotations

import argparse
import ipaddress
import json
import re
import sys
import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import definition_rules as rules
import yaml

# The Makefile runs this file as a script, so Python puts only tools/templates on sys.path; the
# scanner lives in tools/denylist (pytest finds it through its pythonpath setting instead).
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "denylist"))

from denylist_scan import (
    DEFAULT_GENERIC,
    ConfigError,
    Report,
    RuleSet,
    load_private_list_text,
    match_text,
    parse_rules,
    resolve_list_path,
)

REPO = Path(__file__).resolve().parents[2]
RULES = (
    "layout",
    "schema",
    "frontmatter",
    "definition",
    "host",
    "path",
    "ip",
    "person",
    "secret",
    "denylist",
)
"""The rule names of the gate in the order it checks them."""

TEMPLATE_FILE = "template.yaml"
INSTRUCTIONS_FILE = "CLAUDE.md"
SKILL_FILE = "SKILL.md"
ROOT_README = "README.md"

TEMPLATE_KEYS = (
    "name",
    "description",
    "version",
    "tool_class_wish",
    "model_hint",
    "required_secrets",
    "mcp_servers",
    "skills",
)
REQUIRED_TEMPLATE_KEYS = ("name", "description", "version", "tool_class_wish", "skills")
FRONTMATTER_KEYS = ("name", "description")

EXAMPLE_PERSONS = frozenset({"Alex", "Sam", "Robin"})
EXAMPLE_DOMAINS = ("example.com", "example.org", "example.net")

# Capitalized words that stand where a name could stand but name no person. The list only keeps
# the shipped texts free of false findings; it is no proof (docs/decisions.d/A1-06.md).
NON_PERSON_WORDS = frozenset(
    {
        "Claude",
        "Markdown",
        "English",
        "German",
        "French",
        "Spanish",
        "Monday",
        "Tuesday",
        "Wednesday",
        "Thursday",
        "Friday",
        "Saturday",
        "Sunday",
        "Today",
        "Tomorrow",
        "Yesterday",
        "Notes",
        "Note",
        "Owner",
        "Example",
        "Read",
        "Grep",
        "Glob",
        "Bash",
        "Edit",
        "Write",
        "Skill",
    }
)

_PERSON_CUE = re.compile(
    r"(?i:\b(?:for|by|from|to|with|ask|tell|thanks|thank you|dear|hi|hello|hey|cc|ping|contact"
    r"|assigned to|assign to)[ \t]+)([A-Z][a-z]+)\b"
)
_PERSON_MORE = re.compile(r"[ \t]*(?:,|&|\band\b|\bor\b)[ \t]*([A-Z][a-z]+)\b")
_PERSON_FIELD = re.compile(
    r"(?im)^[ \t>*-]*(?:authors?|owners?|maintainers?|contacts?|reviewers?|assignees?|attendees?"
    r"|participants?|from|to|cc)[ \t]*:[ \t]*(.+)$"
)
_PERSON_MENTION = re.compile(r"(?<![\w@])@([A-Z][a-z]+)(?![\w./-])")
_LEADING_NAME = re.compile(r"[ \t]*@?([A-Z][a-z]+)\b")

_URL_HOST = re.compile(
    r"(?i)\b[a-z][a-z0-9+.-]*://(?:[^/\s?#@]*@)?(\[[^\]\s]*\]|[^/\s?#:'\"<>()\[\]]+)"
)
_EMAIL_HOST = re.compile(r"(?i)(?<![\w.%+-])[\w.%+-]+@((?:[a-z0-9-]+\.)+[a-z]{2,})\b")
_BARE_TLDS = (
    "com|net|org|io|dev|app|ai|co|de|uk|eu|info|biz|me|us|cloud|tech|xyz|ch|at|fr|nl|se|no|es"
    "|it|pl|local|internal|lan|home|corp|intra|test|invalid|localhost|example"
)
_BARE_HOST = re.compile(
    r"(?i)(?<![\w@./:-])((?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+(?:" + _BARE_TLDS + r"))"
    r"(?![\w-]|\.[a-z0-9])"
)

_PATH_START = r"(?:^|(?<=[\s\"'(=`:,\[]))"
_ABSOLUTE_PATH = re.compile(_PATH_START + r"(/[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]*)+)", re.M)
_HOME_PATH = re.compile(_PATH_START + r"(~[A-Za-z0-9._-]*/)", re.M)
_WINDOWS_PATH = re.compile(r"\b([A-Za-z]:\\\S*)")

_IPV4 = re.compile(r"(?<![\w.])(\d{1,3}(?:\.\d{1,3}){3})(?![\w]|\.\d)")
_IPV6 = re.compile(r"(?<![\w:.])([0-9A-Fa-f]{0,4}(?::[0-9A-Fa-f]{0,4}){2,7})(?![\w:])")

# Token formats beyond the custom rules of .gitleaks.toml that a template must never hold; the
# default rules of gitleaks are not readable as a file, so the common ones are named here.
BUILTIN_SECRETS = (
    ("aws-access-key", r"\b((?:AKIA|ASIA)[0-9A-Z]{16})\b"),
    ("private-key", r"-----BEGIN[ A-Z0-9_-]{0,100}PRIVATE KEY"),
    ("github-token", r"\bgh[pousr]_[0-9A-Za-z]{36}\b"),
    ("slack-token", r"\bxox[baprs]-[0-9A-Za-z-]{10,}"),
)


@dataclass(frozen=True)
class Finding:
    """One finding of the gate: the masked path (with line), the rule and a masked detail."""

    path: str
    rule: str
    detail: str

    def __str__(self) -> str:
        """Return the line the gate prints."""
        return f"templates: {self.path}: {self.rule}: {self.detail}"


@dataclass(frozen=True)
class SecretPattern:
    """A token format: the rule id (of .gitleaks.toml or BUILTIN_SECRETS) and its pattern."""

    rule_id: str
    pattern: re.Pattern[str]


@dataclass
class Context:
    """What the gate needs besides the templates: limits, secret formats and deny rules."""

    limits: rules.Limits
    secret_patterns: list[SecretPattern]
    rulesets: list[RuleSet]
    private_loaded: bool
    _report: Report = field(init=False)

    def __post_init__(self) -> None:
        """Prepare the masking of printed text with every deny rule."""
        self._report = Report([], self.private_loaded, self.rulesets)

    @classmethod
    def load(cls, repo: Path, list_path: Path | None) -> Context:
        """Read the limits, the token formats and the deny rules of the repository at `repo`.

        Args:
            repo: the repository root (``tests/agent-definitions/limits.json``,
                ``.gitleaks.toml``).
            list_path: the private deny list; None or a missing file runs the generic rules
                only, like ``denylist_scan.py --allow-missing``.

        Raises:
            ConfigError: a broken limit file, token pattern or deny rule.
        """
        try:
            limits = rules.Limits.from_keys(
                json.loads(
                    (repo / "tests" / "agent-definitions" / "limits.json").read_text(
                        encoding="utf-8"
                    )
                )
            )
        except (OSError, ValueError) as exc:
            raise ConfigError(f"limits of tests/agent-definitions: {exc}") from None
        rulesets = [parse_rules(DEFAULT_GENERIC.read_text(encoding="utf-8"), "generic")]
        private_loaded = list_path is not None and list_path.is_file()
        if list_path is not None and private_loaded:
            rulesets.insert(0, load_private_list_text(list_path.read_text(encoding="utf-8")))
        return cls(limits, _secret_patterns(repo), rulesets, private_loaded)

    def mask(self, text: str) -> str:
        """Return `text` with every deny rule match replaced by asterisks."""
        return str(self._report.mask(text))


def _secret_patterns(repo: Path) -> list[SecretPattern]:
    try:
        data = tomllib.loads((repo / ".gitleaks.toml").read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError) as exc:
        raise ConfigError(f".gitleaks.toml: {exc}") from None
    out = []
    for rule in [*data.get("rules", []), *({"id": i, "regex": r} for i, r in BUILTIN_SECRETS)]:
        try:
            out.append(SecretPattern(str(rule["id"]), re.compile(str(rule["regex"]))))
        except (KeyError, re.error) as exc:
            raise ConfigError(f"token pattern {rule.get('id', '?')}: {exc}") from None
    return out


# ---- YAML -----------------------------------------------------------------------------------


class _StrictLoader(yaml.SafeLoader):
    """A safe loader that refuses a key twice in one mapping (Go's YAML decoder does too)."""


def _no_duplicates(loader: _StrictLoader, node: yaml.MappingNode) -> dict[Any, Any]:
    seen: set[Any] = set()
    for key_node, _ in node.value:
        key = loader.construct_object(key_node)
        if key in seen:
            raise yaml.constructor.ConstructorError(
                None, None, f"key {key!r} appears twice", key_node.start_mark
            )
        seen.add(key)
    return loader.construct_mapping(node)


_StrictLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _no_duplicates)


def _load_yaml(text: str) -> Any:
    return yaml.load(text, Loader=_StrictLoader)  # noqa: S506  # nosec B506 - a SafeLoader subclass


# ---- the parts of one template --------------------------------------------------------------


@dataclass
class _Template:
    """What the checks read of one template folder."""

    slug: str
    display: str
    ctx: Context
    findings: list[Finding] = field(default_factory=list)
    texts: dict[str, str] = field(default_factory=dict)
    meta: dict[str, Any] | None = None
    skill_names: list[str] | None = None

    def add(self, rel: str, rule: str, detail: str, line: int | None = None) -> None:
        where = f"{self.display}/{rel}" if rel else self.display
        if line is not None:
            where += f":{line}"
        self.findings.append(Finding(self.ctx.mask(where), rule, self.ctx.mask(detail)))


def _read_files(t: _Template, folder: Path) -> list[str]:
    """Read every file below `folder`; report links, return the relative paths of all entries."""
    entries = []
    for path in sorted(folder.rglob("*")):
        rel = path.relative_to(folder).as_posix()
        entries.append(rel)
        if path.is_symlink():
            t.add(rel, "layout", "symbolic link")
            continue
        if not path.is_file():
            continue
        try:
            t.texts[rel] = path.read_bytes().decode("utf-8")
        except UnicodeDecodeError:
            t.add(rel, "layout", "not UTF-8 text")
    return entries


def _check_layout(t: _Template, folder: Path, entries: list[str]) -> None:
    skills = t.skill_names
    allowed_files = {TEMPLATE_FILE, INSTRUCTIONS_FILE}
    for rel in (TEMPLATE_FILE, INSTRUCTIONS_FILE):
        if rel not in entries:
            t.add(rel, "layout", "missing")
    if skills is not None:
        allowed_files |= {f"skills/{n}/{SKILL_FILE}" for n in skills}
        for n in skills:
            if f"skills/{n}/{SKILL_FILE}" not in entries:
                t.add(f"skills/{n}/{SKILL_FILE}", "layout", "missing")
    for rel in entries:
        path = folder / rel
        if path.is_symlink():
            continue
        parts = rel.split("/")
        if path.is_dir():
            in_layout = rel == "skills" or (
                len(parts) == 2 and parts[0] == "skills" and (skills is None or parts[1] in skills)
            )
            if not in_layout and not any(path.iterdir()):
                t.add(rel, "layout", "folder outside the layout")
            continue
        if rel in allowed_files:
            continue
        if skills is None and len(parts) == 3 and parts[0] == "skills" and parts[2] == SKILL_FILE:
            continue
        t.add(rel, "layout", "file outside the layout")


def _check_schema(t: _Template) -> None:
    text = t.texts.get(TEMPLATE_FILE)
    if text is None:
        return
    try:
        meta = _load_yaml(text)
    except yaml.YAMLError as exc:
        mark = getattr(exc, "problem_mark", None) or getattr(exc, "context_mark", None)
        line = mark.line + 1 if mark is not None else None
        t.add(TEMPLATE_FILE, "schema", "not valid YAML or a key appears twice", line)
        return
    if not isinstance(meta, dict):
        t.add(TEMPLATE_FILE, "schema", "not a mapping")
        return
    t.meta = meta
    for key in sorted(str(k) for k in meta if k not in TEMPLATE_KEYS):
        t.add(TEMPLATE_FILE, "schema", f"unknown key {key}")
    for key in REQUIRED_TEMPLATE_KEYS:
        if key not in meta:
            t.add(TEMPLATE_FILE, "schema", f"missing {key}")
    for key in ("name", "description"):
        if key in meta and not _one_line(meta[key]):
            t.add(TEMPLATE_FILE, "schema", f"{key} is not one line of text")
    if "version" in meta and not (
        isinstance(meta["version"], str) and rules.TEMPLATE_VERSION.fullmatch(meta["version"])
    ):
        t.add(TEMPLATE_FILE, "schema", "version is not a version string")
    if "tool_class_wish" in meta and meta["tool_class_wish"] not in rules.TOOL_CLASSES:
        t.add(TEMPLATE_FILE, "schema", "tool_class_wish is not chat, scoped or host")
    if "model_hint" in meta and not isinstance(meta["model_hint"], str):
        t.add(TEMPLATE_FILE, "schema", "model_hint is not a string")
    if "mcp_servers" in meta and not isinstance(meta["mcp_servers"], list):
        t.add(TEMPLATE_FILE, "schema", "mcp_servers is not a list")
    secrets = _names(t, meta, "required_secrets", rules.valid_env_name)
    t.skill_names = _names(t, meta, "skills", rules.valid_skill_name)
    _check_secret_refs(t, meta, set(secrets or []))


def _one_line(v: object) -> bool:
    return isinstance(v, str) and v.strip() != "" and rules.display_rule(v) is None


def _names(t: _Template, meta: dict[str, Any], key: str, valid: Any) -> list[str] | None:
    if key not in meta:
        return [] if key not in REQUIRED_TEMPLATE_KEYS else None
    v = meta[key]
    if not isinstance(v, list) or not all(isinstance(x, str) for x in v):
        t.add(TEMPLATE_FILE, "schema", f"{key} is not a list of names")
        return None
    out: list[str] = []
    for x in v:
        if not valid(x):
            t.add(TEMPLATE_FILE, "schema", f"{key} holds an invalid name")
        elif x in out:
            t.add(TEMPLATE_FILE, "schema", f"{key} holds a name twice")
        else:
            out.append(x)
    return out


def _check_secret_refs(t: _Template, meta: dict[str, Any], declared: set[str]) -> None:
    """A template names its secrets as env:<NAME> listed in required_secrets (26 7.1)."""
    servers = meta.get("mcp_servers")
    if not isinstance(servers, list):
        return
    for server in servers:
        if not isinstance(server, dict):
            continue
        for member in ("headers", "env"):
            entries = server.get(member)
            if not isinstance(entries, list):
                continue
            for entry in entries:
                if not isinstance(entry, dict) or "secret_ref" not in entry:
                    continue
                ref = entry["secret_ref"]
                name = ref.removeprefix("env:") if isinstance(ref, str) else ""
                if not (isinstance(ref, str) and ref.startswith("env:") and name in declared):
                    t.add(
                        TEMPLATE_FILE,
                        "schema",
                        "secret_ref is not env:<NAME> of a name in required_secrets",
                    )


def _parse_skill(t: _Template, name: str) -> tuple[str, str] | None:
    """Return description and body of skills/<name>/SKILL.md, or None after a finding."""
    rel = f"skills/{name}/{SKILL_FILE}"
    text = t.texts.get(rel)
    if text is None:
        return None
    lines = text.split("\n")
    if lines[0] != "---":
        t.add(rel, "frontmatter", "does not start with a frontmatter line ---", 1)
        return None
    try:
        end = lines.index("---", 1)
    except ValueError:
        t.add(rel, "frontmatter", "the frontmatter has no closing line ---", 1)
        return None
    try:
        fm = _load_yaml("\n".join(lines[1:end]))
    except yaml.YAMLError:
        t.add(rel, "frontmatter", "not valid YAML or a key appears twice", 1)
        return None
    if not isinstance(fm, dict):
        t.add(rel, "frontmatter", "not a mapping", 1)
        return None
    ok = True
    for key in sorted(str(k) for k in fm if k not in FRONTMATTER_KEYS):
        t.add(rel, "frontmatter", f"key {key} other than name and description", 1)
        ok = False
    if fm.get("name") != name:
        t.add(rel, "frontmatter", "name differs from the skill folder", 1)
        ok = False
    if not _one_line(fm.get("description")):
        t.add(rel, "frontmatter", "description is not one line of text", 1)
        ok = False
    if not ok:
        return None
    return str(fm["description"]), "\n".join(lines[end + 1 :])


def draft(slug: str, meta: dict[str, Any], instructions: str, skills: list[Any]) -> dict[str, Any]:
    """Return the definition draft of a template (26 7.1: one to one, origin.kind template).

    Args:
        slug: the template folder name.
        meta: template.yaml.
        instructions: CLAUDE.md.
        skills: one ``{"name", "description", "body"}`` per listed skill, in the order of
            ``skills[]``.

    Returns:
        The draft as agent_definition.v1; ``settings.model`` is ``model_hint`` when set.
    """
    settings: dict[str, Any] = {"tool_class_wish": meta.get("tool_class_wish")}
    if "model_hint" in meta:
        settings["model"] = meta["model_hint"]
    origin: dict[str, Any] = {"kind": "template", "template_slug": slug}
    if isinstance(meta.get("version"), str):
        origin["template_version"] = meta["version"]
    servers = meta.get("mcp_servers", [])
    return {
        "instructions": instructions,
        "skills": skills,
        "mcp_servers": servers if isinstance(servers, list) else [],
        "settings": settings,
        "origin": origin,
    }


def _check_definition(t: _Template) -> None:
    if t.meta is None:
        return
    skills = []
    for name in t.skill_names or []:
        parsed = _parse_skill(t, name)
        if parsed is not None:
            skills.append({"name": name, "description": parsed[0], "body": parsed[1]})
    wish = t.meta.get("tool_class_wish")
    tool_class = wish if wish in rules.TOOL_CLASSES else "chat"
    d = draft(t.slug, t.meta, t.texts.get(INSTRUCTIONS_FILE, ""), skills)
    for f in rules.validate(d, t.ctx.limits, str(tool_class)):
        t.add("", "definition", f"{f.pointer or '/'}: {f.code}")


# ---- content heuristics ---------------------------------------------------------------------


def _example_domain(host: str) -> bool:
    h = host.lower().rstrip(".")
    return any(h == d or h.endswith("." + d) for d in EXAMPLE_DOMAINS)


def foreign_hosts(text: str) -> list[str]:
    """Return the hosts of URLs, e-mail addresses and bare host names outside example domains."""
    out: list[str] = []
    for pattern in (_URL_HOST, _EMAIL_HOST, _BARE_HOST):
        for m in pattern.finditer(text):
            host = m.group(1).strip("[]")
            if not _example_domain(host) and host not in out:
                out.append(host)
    return out


def absolute_paths(text: str) -> list[str]:
    """Return absolute paths, ``~`` paths and Windows drive paths in `text`."""
    return [
        m.group(1) for p in (_ABSOLUTE_PATH, _HOME_PATH, _WINDOWS_PATH) for m in p.finditer(text)
    ]


def ip_addresses(text: str) -> list[str]:
    """Return the IPv4 and IPv6 addresses in `text`."""
    out = []
    for m in _IPV4.finditer(text):
        try:
            ipaddress.IPv4Address(m.group(1))
        except ValueError:
            continue
        out.append(m.group(1))
    for m in _IPV6.finditer(text):
        try:
            ipaddress.IPv6Address(m.group(1))
        except ValueError:
            continue
        out.append(m.group(1))
    return out


def person_names(text: str) -> list[str]:
    """Return capitalized words in a person position that are not Alex, Sam or Robin.

    A person position is a word after a cue such as "for", "by", "to", "ask" or "thanks" (and
    the names joined to it by a comma, "and" or "or"), the value of a field such as "Owner:" or
    "Attendees:", and an ``@`` mention. This is a heuristic, not a proof: a name elsewhere stays
    unseen and is left to review and the private deny list (docs/decisions.d/A1-06.md).
    """
    found: list[str] = []

    def take(name: str) -> None:
        if name not in EXAMPLE_PERSONS and name not in NON_PERSON_WORDS and name not in found:
            found.append(name)

    for m in _PERSON_CUE.finditer(text):
        take(m.group(1))
        pos = m.end()
        while more := _PERSON_MORE.match(text, pos):
            take(more.group(1))
            pos = more.end()
    for m in _PERSON_FIELD.finditer(text):
        for item in re.split(r",|;|&|\band\b|\bor\b", m.group(1)):
            lead = _LEADING_NAME.match(item)
            if lead:
                take(lead.group(1))
    for m in _PERSON_MENTION.finditer(text):
        take(m.group(1))
    return found


def _line_of(text: str, needle: str) -> int:
    i = text.find(needle)
    return text.count("\n", 0, i) + 1 if i >= 0 else 1


def _check_content(t: _Template) -> None:
    for rel, text in t.texts.items():
        for host in foreign_hosts(text):
            t.add(rel, "host", f"host outside the example domains: {host}", _line_of(text, host))
        for p in absolute_paths(text):
            t.add(rel, "path", f"absolute or home path: {p}", _line_of(text, p))
        for ip in ip_addresses(text):
            t.add(rel, "ip", f"IP address: {ip}", _line_of(text, ip))
        for name in person_names(text):
            t.add(rel, "person", f"name outside Alex, Sam and Robin: {name}", _line_of(text, name))
        for sp in t.ctx.secret_patterns:
            for m in sp.pattern.finditer(text):
                line = text.count("\n", 0, m.start()) + 1
                t.add(rel, "secret", f"secret-looking value ({sp.rule_id})", line)


def _check_denylist(t: _Template, entries: list[str]) -> None:
    for rel in entries:
        repo_path = f"templates/agents/{t.slug}/{rel}"
        texts = [("path", rel)]
        if rel in t.texts:
            texts.append(("content", t.texts[rel]))
        for kind, text in texts:
            for ruleset in t.ctx.rulesets:
                if not ruleset.applies_to(repo_path):
                    continue
                for hit in match_text(text, ruleset):
                    line = text.count("\n", 0, hit.offset) + 1 if kind == "content" else None
                    detail = f"{kind} matches {hit.rule.source} rule line {hit.rule.lineno}"
                    t.add(rel, "denylist", detail, line)


def check_template(folder: Path, display: str, ctx: Context) -> list[Finding]:
    """Check one template folder with every rule of the gate.

    Args:
        folder: the template folder; its name is the slug.
        display: the path printed for the folder (``templates/agents/<slug>``).
        ctx: limits, token formats and deny rules.

    Returns:
        Every finding, in the order of the rules; empty for a template that passes.
    """
    t = _Template(folder.name, display, ctx)
    entries = _read_files(t, folder)
    _check_schema(t)
    _check_layout(t, folder, entries)
    _check_definition(t)
    _check_content(t)
    _check_denylist(t, entries)
    order = {r: i for i, r in enumerate(RULES)}
    return sorted(t.findings, key=lambda f: order[f.rule])


def _display(root: Path, repo: Path) -> str:
    try:
        return root.resolve().relative_to(repo.resolve()).as_posix()
    except ValueError:
        return root.as_posix()


def check_root(root: Path, ctx: Context, repo: Path = REPO) -> list[Finding]:
    """Check every template folder under `root`; only template folders and README.md may be there.

    Returns:
        Every finding of every template, folder by folder in name order.
    """
    display = _display(root, repo)
    findings: list[Finding] = []
    for entry in sorted(root.iterdir()):
        where = ctx.mask(f"{display}/{entry.name}")
        if entry.is_symlink():
            findings.append(Finding(where, "layout", "symbolic link"))
        elif entry.is_dir() and not rules.valid_skill_name(entry.name):
            findings.append(Finding(where, "layout", "slug outside ^[a-z0-9][a-z0-9-]{0,63}$"))
        elif entry.is_dir():
            findings += check_template(entry, f"{display}/{entry.name}", ctx)
        elif entry.name != ROOT_README:
            findings.append(Finding(where, "layout", "file outside the layout"))
    return findings


def main(argv: list[str] | None = None) -> int:
    """Command line entry point; exit code 0 clean, 1 findings, 2 configuration error."""
    parser = argparse.ArgumentParser(description="Template gate (26 7.3).")
    parser.add_argument("--repo", type=Path, default=REPO)
    parser.add_argument("--root", type=Path, default=None, help="default: templates/agents")
    parser.add_argument("--list", type=Path, default=None, help="private deny list (optional)")
    args = parser.parse_args(sys.argv[1:] if argv is None else argv)
    repo: Path = args.repo
    root: Path = args.root or repo / "templates" / "agents"
    try:
        ctx = Context.load(repo, resolve_list_path(repo, args.list))
    except ConfigError as exc:
        print(f"templates: {exc}", file=sys.stderr)
        return 2
    if not root.is_dir():
        print(f"templates: no template folder at {_display(root, repo)}", file=sys.stderr)
        return 2
    findings = check_root(root, ctx, repo)
    for f in findings:
        print(f)
    count = sum(1 for p in root.iterdir() if p.is_dir() and not p.is_symlink())
    private = "loaded" if ctx.private_loaded else "absent"
    print(
        f"templates: {count} templates checked, {len(findings)} finding(s); "
        f"private deny list {private}"
    )
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
