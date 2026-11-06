# SPDX-License-Identifier: Apache-2.0
"""BA-T17: the template gate fails every fixture with its rule and passes the shipped templates.

tools/templates/tests/fixtures/<rule>/<case>/ holds one failing template per surface of
docs/spec/26-builtin-agents.md 5.5 and per rule of 26 7.3 (design note 3 of the brief A1-06); the
directory name <rule> is the rule the gate must report, and no other.
"""

from __future__ import annotations

import shutil
from pathlib import Path

import pytest
import template_gate as gate

REPO = Path(__file__).resolve().parents[3]
FIXTURES = Path(__file__).resolve().parent / "fixtures"
SHIPPED = REPO / "templates" / "agents"
CASES = sorted(p for p in FIXTURES.glob("*/*") if p.is_dir())

# Every surface of 26 5.5 and every rule of 26 7.3 has a fixture (design note 3).
REQUIRED = {
    # 5.5 folder content, settings files and plugin folder
    "layout/claude-settings",
    "layout/claude-skills",
    "layout/claude-local",
    "layout/mcp-json",
    "layout/lsp-json",
    "layout/plugin-manifest",
    "layout/hooks-json",
    "layout/commands",
    "layout/agents",
    "layout/output-styles",
    "layout/monitors",
    "layout/bin",
    # 5.5 SKILL.md frontmatter
    "frontmatter/hooks",
    "frontmatter/allowed-tools",
    # 5.5 inline shell in both forms, @ imports, MCP headersHelper
    "definition/inline-shell-backtick",
    "definition/inline-shell-fence",
    "definition/import-absolute",
    "definition/mcp-headers-helper",
    # 7.3 items 1 to 5
    "layout/skill-extra-file",
    "schema/unknown-key-settings",
    "definition/stdio-below-host",
    "host/url-outside",
    "host/email-outside",
    "path/absolute",
    "path/home",
    "ip/ipv4",
    "ip/ipv6",
    "person/for-name",
    "person/field",
    "secret/product-token",
    "denylist/generic-rule",
}

# The field code of 26 4.3 each definition fixture must name.
DEFINITION_CODES = {
    "inline-shell-backtick": "inline_shell",
    "inline-shell-fence": "inline_shell",
    "import-absolute": "import_outside_folder",
    "import-home": "import_outside_folder",
    "mcp-headers-helper": "unknown_member",
    "stdio-below-host": "tool_class_too_low",
    "frontmatter-twice": "skill_frontmatter",
}


@pytest.fixture(autouse=True)
def _no_private_list(monkeypatch: pytest.MonkeyPatch) -> None:
    # The gate needs no private deny list (BA-T17): none is reachable in these tests, neither
    # through the environment nor through a .denylist-path of the clone; only --list names one.
    monkeypatch.delenv("BOTHY_DENYLIST_FILE", raising=False)
    monkeypatch.setattr(gate, "resolve_list_path", lambda _repo, explicit: explicit)


def _context() -> gate.Context:
    return gate.Context.load(REPO, list_path=None)


def test_every_surface_and_rule_has_a_fixture() -> None:
    have = {f"{p.parent.name}/{p.name}" for p in CASES}
    assert have >= REQUIRED
    assert {p.parent.name for p in CASES} == set(gate.RULES)


@pytest.mark.parametrize("case", CASES, ids=lambda p: f"{p.parent.name}/{p.name}")
def test_each_fixture_fails_with_its_rule(case: Path) -> None:
    findings = gate.check_template(case, f"templates/agents/{case.name}", _context())
    assert findings, "the fixture passed the gate"
    assert {f.rule for f in findings} == {case.parent.name}
    if case.parent.name == "definition":
        assert any(f.detail.endswith(DEFINITION_CODES[case.name]) for f in findings)


def test_the_shipped_templates_pass() -> None:
    assert gate.check_root(SHIPPED, _context()) == []


def test_the_shipped_topics_are_the_four_of_26_7_5() -> None:
    assert sorted(p.name for p in SHIPPED.iterdir() if p.is_dir()) == [
        "code-reviewer",
        "meeting-notes",
        "research-summarizer",
        "writing-helper",
    ]


def test_main_passes_the_shipped_templates_without_a_private_list(
    capsys: pytest.CaptureFixture[str],
) -> None:
    assert gate.main(["--root", str(SHIPPED), "--repo", str(REPO)]) == 0
    out = capsys.readouterr().out
    assert "4 templates" in out
    assert "private deny list absent" in out


def test_main_reports_each_finding_in_the_gate_format(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = tmp_path / "agents"
    shutil.copytree(FIXTURES / "person" / "for-name", root / "for-name")
    assert gate.main(["--root", str(root), "--repo", str(REPO)]) == 1
    lines = capsys.readouterr().out.splitlines()
    assert any(line.startswith("templates: ") and ": person: " in line for line in lines)


def test_the_root_holds_only_template_folders_and_the_readme(tmp_path: Path) -> None:
    root = tmp_path / "agents"
    shutil.copytree(SHIPPED / "writing-helper", root / "writing-helper")
    (root / "README.md").write_text("# Templates\n", encoding="utf-8")
    (root / "notes.txt").write_text("x\n", encoding="utf-8")
    (root / "Bad_Slug").mkdir()
    findings = gate.check_root(root, _context())
    assert [(f.rule, f.path.rsplit("/", 1)[-1]) for f in findings] == [
        ("layout", "Bad_Slug"),
        ("layout", "notes.txt"),
    ]


def test_a_symbolic_link_is_a_layout_finding(tmp_path: Path) -> None:
    case = tmp_path / "linked"
    shutil.copytree(SHIPPED / "writing-helper", case)
    (case / "CLAUDE.md").unlink()
    (case / "CLAUDE.md").symlink_to(SHIPPED / "writing-helper" / "CLAUDE.md")
    findings = gate.check_template(case, "templates/agents/linked", _context())
    assert [f.rule for f in findings] == ["layout"]


def test_text_that_is_not_utf8_is_a_layout_finding(tmp_path: Path) -> None:
    case = tmp_path / "latin"
    shutil.copytree(SHIPPED / "writing-helper", case)
    (case / "CLAUDE.md").write_bytes(b"caf\xe9\n")
    findings = gate.check_template(case, "templates/agents/latin", _context())
    assert [(f.rule, f.detail) for f in findings] == [("layout", "not UTF-8 text")]


def test_the_private_list_runs_when_present_and_never_prints_its_term(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    term = "quillfeather"
    private = tmp_path / "list.txt"
    private.write_text(f"{term}\n", encoding="utf-8")
    root = tmp_path / "agents"
    shutil.copytree(SHIPPED / "writing-helper", root / "writing-helper")
    claude = root / "writing-helper" / "CLAUDE.md"
    claude.write_text(claude.read_text(encoding="utf-8") + f"\nAlso {term}.\n", encoding="utf-8")
    assert gate.main(["--root", str(root), "--repo", str(REPO), "--list", str(private)]) == 1
    out = capsys.readouterr().out
    assert ": denylist: content matches private rule line 1" in out
    assert term not in out
    assert "private deny list loaded" in out


@pytest.mark.parametrize(
    ("text", "names"),
    [
        ("Notes for Alex and Sam.", []),
        ("Send it to Robin.", []),
        ("Write it in English for Markdown readers.", []),
        ("Notes for Morgan.", ["Morgan"]),
        ("Attendees: Alex, Morgan and Jordan", ["Morgan", "Jordan"]),
        ("ask @Taylor first", ["Taylor"]),
        ("Read @Notes.md first", []),
        ("Thanks Casey!", ["Casey"]),
    ],
)
def test_person_heuristic(text: str, names: list[str]) -> None:
    assert gate.person_names(text) == names


@pytest.mark.parametrize(
    ("text", "hosts"),
    [
        ("see https://docs.example.com/a and mail sam@example.org", []),
        ("https://example.net", []),
        ("see https://files.test/a", ["files.test"]),
        # Assembled, so the repository deny scan never sees an address outside the example domains.
        ("mail robin" + "@corp.example.io", ["corp.example.io"]),
        (
            "the host ci.internal and api.example.com.evil.io",
            ["ci.internal", "api.example.com.evil.io"],
        ),
        ("files template.yaml, SKILL.md and CLAUDE.md", []),
        ("http://localhost:8080/x", ["localhost"]),
    ],
)
def test_hosts_outside_the_example_domains(text: str, hosts: list[str]) -> None:
    assert gate.foreign_hosts(text) == hosts


@pytest.mark.parametrize(
    ("text", "found"),
    [
        ("keep /srv/notes/a.md", True),
        ("keep ~/notes", True),
        ("keep C:\\notes\\a.md", True),
        ("and/or, https://example.com/a/b, ./notes/a.md, @/x/y", False),
        ("use the /review command", False),
    ],
)
def test_paths(text: str, found: bool) -> None:
    assert bool(gate.absolute_paths(text)) is found


@pytest.mark.parametrize(
    ("text", "found"),
    [
        ("198.51.100.4", True),
        ("::1", True),
        ("fe80::1", True),
        ("version 1.2.3, at 10:30:00, 999.1.1.1", False),
    ],
)
def test_ip_addresses(text: str, found: bool) -> None:
    assert bool(gate.ip_addresses(text)) is found


def test_secret_patterns_come_from_the_gitleaks_rules() -> None:
    ctx = _context()
    ids = {p.rule_id for p in ctx.secret_patterns}
    assert {"bothy-api-token", "anthropic-credential-any", "tailscale-key"} <= ids
    key = "AKIA" + "Q" * 16
    assert [p.rule_id for p in ctx.secret_patterns if p.pattern.search(key)] == ["aws-access-key"]
    head = "-----BEGIN " + "OPENSSH PRIVATE KEY-----"
    assert any(p.pattern.search(head) for p in ctx.secret_patterns)


def _variant(tmp_path: Path, files: dict[str, str]) -> Path:
    case = tmp_path / "variant"
    shutil.copytree(SHIPPED / "writing-helper", case)
    for name, text in files.items():
        path = case / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
    return case


SKILL_PATH = "skills/tone-check/SKILL.md"


@pytest.mark.parametrize(
    ("files", "rule", "detail"),
    [
        ({"template.yaml": "- a list\n"}, "schema", "not a mapping"),
        ({"template.yaml": "name: [\n"}, "schema", "not valid YAML or a key appears twice"),
        (
            {
                "template.yaml": "name: x\ndescription: y\nversion: 1.0.0\ntool_class_wish: root\n"
                "model_hint: 7\nmcp_servers: {}\nskills: tone-check\n"
            },
            "schema",
            "tool_class_wish is not chat, scoped or host",
        ),
        (
            {
                "template.yaml": "name: x\ndescription: y\nversion: 1.0.0\ntool_class_wish: chat\n"
                "required_secrets: [lower]\nskills: [tone-check, shorten, shorten]\n"
            },
            "schema",
            "skills holds a name twice",
        ),
        (
            {SKILL_PATH: "---\nname: tone-check\n"},
            "frontmatter",
            "the frontmatter has no closing line ---",
        ),
        (
            {SKILL_PATH: "---\nname: [\n---\nx\n"},
            "frontmatter",
            "not valid YAML or a key appears twice",
        ),
        ({SKILL_PATH: "---\n- x\n---\nx\n"}, "frontmatter", "not a mapping"),
        (
            {SKILL_PATH: '---\nname: tone-check\ndescription: "a\\nb"\n---\nx\n'},
            "frontmatter",
            "description is not one line of text",
        ),
        ({".claude/agents/.keep": ""}, "layout", "file outside the layout"),
    ],
)
def test_schema_and_frontmatter_details(
    tmp_path: Path, files: dict[str, str], rule: str, detail: str
) -> None:
    findings = gate.check_template(
        _variant(tmp_path, files), "templates/agents/variant", _context()
    )
    assert (rule, detail) in [(f.rule, f.detail) for f in findings]


def test_an_empty_folder_outside_the_layout_is_a_finding(tmp_path: Path) -> None:
    case = _variant(tmp_path, {})
    (case / "commands").mkdir()
    findings = gate.check_template(case, "templates/agents/variant", _context())
    assert [(f.rule, f.detail) for f in findings] == [("layout", "folder outside the layout")]


def test_secret_refs_of_a_template_name_declared_secrets(tmp_path: Path) -> None:
    yaml_text = (SHIPPED / "research-summarizer" / "template.yaml").read_text(encoding="utf-8")
    case = _variant(
        tmp_path,
        {
            "template.yaml": yaml_text.replace("secret_ref: env:SEARCH_MCP_TOKEN", "secret_ref: 7")
            .replace(
                "skills:\n  - summarize-sources\n  - compare-findings",
                "skills:\n  - tone-check\n  - shorten",
            )
            .replace("    headers:", "    env: x\n    headers:")
        },
    )
    findings = gate.check_template(case, "templates/agents/variant", _context())
    assert ("schema", "secret_ref is not env:<NAME> of a name in required_secrets") in [
        (f.rule, f.detail) for f in findings
    ]


def test_main_refuses_a_missing_root_and_a_broken_configuration(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    assert gate.main(["--root", str(tmp_path / "none"), "--repo", str(REPO)]) == 2
    assert gate.main(["--repo", str(tmp_path)]) == 2
    repo = tmp_path / "repo"
    shutil.copytree(REPO / "tests" / "agent-definitions", repo / "tests" / "agent-definitions")
    (repo / ".gitleaks.toml").write_text('[[rules]]\nid = "x"\nregex = "("\n', encoding="utf-8")
    assert gate.main(["--repo", str(repo)]) == 2
    (repo / ".gitleaks.toml").write_text("not toml [", encoding="utf-8")
    assert gate.main(["--repo", str(repo)]) == 2
    err = capsys.readouterr().err
    assert "token pattern x" in err
    assert ".gitleaks.toml" in err


def test_a_linked_template_folder_is_a_layout_finding(tmp_path: Path) -> None:
    root = tmp_path / "agents"
    root.mkdir()
    (root / "linked").symlink_to(SHIPPED / "writing-helper")
    assert [(f.rule, f.detail) for f in gate.check_root(root, _context())] == [
        ("layout", "symbolic link")
    ]


def test_make_check_and_the_pull_request_ci_run_the_gate() -> None:
    makefile = (REPO / "Makefile").read_text(encoding="utf-8").replace("\\\n", " ")
    [gates] = [ln for ln in makefile.splitlines() if ln.startswith("gates: ") and "##" not in ln]
    assert "templates-check" in gates.split()
    bandit = next(line for line in makefile.splitlines() if "bandit -q -c" in line)
    assert " tools/templates;" in bandit or " tools/templates " in bandit
    ci = (REPO / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8")
    assert "tofu-check templates-check COMPONENTS=" in ci


def test_the_repository_scan_skips_only_the_deny_rule_fixture() -> None:
    generic = gate.parse_rules(gate.DEFAULT_GENERIC.read_text(encoding="utf-8"), "generic")
    assert not generic.applies_to("tools/templates/tests/fixtures/denylist/generic-rule/CLAUDE.md")
    assert generic.applies_to("tools/templates/tests/fixtures/host/url-outside/CLAUDE.md")
    assert generic.applies_to("templates/agents/generic-rule/CLAUDE.md")
