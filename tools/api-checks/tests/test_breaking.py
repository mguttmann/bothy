# SPDX-License-Identifier: Apache-2.0
"""Tests for the breaking-change check G6 (docs/spec/06-api.md sections 13 and 15, ticket M7-11).

Every file in fixtures/g6/cases/ patches a copy of fixtures/g6/base.yaml and lists the findings
the check must report, exactly (an empty list for an additive change). The base selection runs
against fixture repositories in `tmp_path`; the repository itself is never tagged.
"""

from __future__ import annotations

import copy
import os
import subprocess
from pathlib import Path
from typing import Any

import breaking
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
FIXTURES = Path(__file__).resolve().parent / "fixtures" / "g6"
CASES = sorted((FIXTURES / "cases").glob("*.yaml"))
_LOADER = getattr(yaml, "CSafeLoader", yaml.SafeLoader)


def _load(path: Path) -> Any:
    return yaml.load(path.read_text(encoding="utf-8"), Loader=_LOADER)  # noqa: S506 - safe loader


def _pointer(path: str) -> list[str]:
    return [p.replace("~1", "/").replace("~0", "~") for p in path.split("/")[1:]]


def _apply(doc: Any, patch: list[dict[str, Any]]) -> Any:
    """Apply the fixture patch operations `set`, `delete` and `append` to a copy of doc."""
    out = copy.deepcopy(doc)
    for step in patch:
        *parents, last = _pointer(step["path"])
        node = out
        for key in parents:
            node = node[int(key)] if isinstance(node, list) else node[key]
        if step["op"] == "set":
            node[last] = step["value"]
        elif step["op"] == "append":
            (node[int(last)] if isinstance(node, list) else node.setdefault(last, [])).append(
                step["value"]
            )
        elif step["op"] == "delete":
            del node[int(last) if isinstance(node, list) else last]
        else:
            raise AssertionError(f"unknown patch op {step['op']}")
    return out


def _compare(base: Any, revision: Any) -> list[tuple[str, str]]:
    asyncapi = _load(FIXTURES / "asyncapi.yaml")
    findings = breaking.compare(base, revision, asyncapi_docs=[asyncapi])
    return sorted((f.row, f.location) for f in findings)


@pytest.mark.parametrize("case", CASES, ids=[c.stem for c in CASES])
def test_mutation_fixture(case: Path) -> None:
    spec = _load(case)
    base = _load(FIXTURES / "base.yaml")
    revision = _apply(base, spec["patch"])
    expected = sorted((row, location) for row, location in spec["expect"])
    assert _compare(base, revision) == expected


def test_every_breaking_row_has_a_fixture() -> None:
    rows = {row for case in CASES for row, _ in _load(case)["expect"]}
    assert rows == set(breaking.ROWS)
    assert any(_load(c)["expect"] == [] for c in CASES)


def test_base_against_itself_has_no_finding() -> None:
    base = _load(FIXTURES / "base.yaml")
    assert _compare(base, copy.deepcopy(base)) == []


def test_real_document_against_itself_has_no_finding() -> None:
    doc = _load(REPO / "docs/api/openapi.yaml")
    asyncapi = _load(REPO / "docs/api/asyncapi.yaml")
    assert breaking.compare(doc, copy.deepcopy(doc), asyncapi_docs=[asyncapi]) == []


def test_real_document_detects_a_removed_event_field() -> None:
    doc = _load(REPO / "docs/api/openapi.yaml")
    revision = copy.deepcopy(doc)
    del revision["components"]["schemas"]["TypingStartedPayload"]["properties"]["expires_at"]
    findings = breaking.compare(doc, revision, asyncapi_docs=[])
    assert [(f.row, f.location) for f in findings] == [
        ("removed", "components/schemas/TypingStartedPayload/properties/expires_at")
    ]


def test_finding_line_names_path_and_rule() -> None:
    finding = breaking.Finding("authz", "GET /things", "x-authz-action thing.read -> thing.list")
    line = finding.line()
    assert line.startswith("api-breaking: 06 13 row 7 ")
    assert "GET /things" in line
    assert "x-authz-action" in line


def test_unreached_schema_counts_in_both_directions() -> None:
    """Without asyncapi.yaml an event schema could be a client frame, so new required fails."""
    base = _load(FIXTURES / "base.yaml")
    revision = _apply(
        base,
        [
            {
                "op": "set",
                "path": "/components/schemas/ThingRenamedEvent/properties/new_name",
                "value": {"type": "string"},
            },
            {
                "op": "append",
                "path": "/components/schemas/ThingRenamedEvent/required",
                "value": "new_name",
            },
        ],
    )
    findings = breaking.compare(base, revision, asyncapi_docs=[])
    assert [(f.row, f.location) for f in findings] == [
        ("required", "components/schemas/ThingRenamedEvent/properties/new_name")
    ]


# ---- major version exception (requirement 14) ---------------------------------------------


def _removed_field(version: str) -> tuple[Any, Any]:
    base = _load(FIXTURES / "base.yaml")
    revision = _apply(
        base,
        [
            {"op": "delete", "path": "/components/schemas/Thing/properties/size"},
            {"op": "set", "path": "/info/version", "value": version},
        ],
    )
    return base, revision


@pytest.mark.parametrize(
    ("base_version", "revision_version", "accepted"),
    [
        ("1.2.0", "2.0.0", True),
        ("1.2.0", "1.3.0", False),
        ("1.2.0", "1.2.1", False),
        ("0.2.0", "1.0.0", True),
        ("0.2.0", "0.3.0", False),
        ("1.2.0", "2.0.0-beta.1", True),
    ],
)
def test_major_version_exception(base_version: str, revision_version: str, accepted: bool) -> None:
    base, revision = _removed_field(revision_version)
    base["info"]["version"] = base_version
    result = breaking.evaluate(base, revision, asyncapi_docs=[])
    assert [f.row for f in result.findings] == ["removed"]
    assert result.accepted is accepted


def test_invalid_info_version_is_an_error() -> None:
    base, revision = _removed_field("two")
    with pytest.raises(breaking.BreakingError, match=r"info\.version"):
        breaking.evaluate(base, revision, asyncapi_docs=[])


# ---- base selection (requirement 12) ------------------------------------------------------


@pytest.mark.parametrize(
    ("tags", "expected"),
    [
        ([], None),
        (["server/v0.1.0-beta.1", "server/v0.1.0-beta.2"], "server/v0.1.0-beta.2"),
        (["server/v0.1.0-beta.10", "server/v0.1.0-beta.9"], "server/v0.1.0-beta.10"),
        (["server/v0.1.0", "server/v0.2.0-beta.1"], "server/v0.1.0"),
        (["server/v0.9.0", "server/v0.10.0", "server/v0.2.0"], "server/v0.10.0"),
        (["server/vfoo", "host/v9.0.0", "v9.0.0"], None),
    ],
)
def test_select_base_tag(tags: list[str], expected: str | None) -> None:
    assert breaking.select_base_tag(tags) == expected


def _git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout


def _fixture_repo(tmp_path: Path, tags: list[tuple[str, dict[str, Any]]]) -> Path:
    """Create a repository whose commits carry the given documents, each with its tag."""
    repo = tmp_path / "repo"
    (repo / "docs/api").mkdir(parents=True)
    _git(repo, "init", "-q", "-b", "main")
    for tag, doc in tags:
        (repo / "docs/api/openapi.yaml").write_text(yaml.safe_dump(doc), encoding="utf-8")
        _git(repo, "add", "-A")
        _git(
            repo,
            "-c",
            "user.name=Alex",
            "-c",
            "user.email=alex@example.com",
            "commit",
            "-q",
            "--no-gpg-sign",
            "-m",
            f"docs: {tag}",
        )
        _git(repo, "tag", tag)
    return repo


def _write_current(repo: Path, doc: dict[str, Any]) -> None:
    (repo / "docs/api").mkdir(parents=True, exist_ok=True)
    (repo / "docs/api/openapi.yaml").write_text(yaml.safe_dump(doc), encoding="utf-8")


def test_check_without_tag_passes_with_notice(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    _git(repo, "init", "-q", "-b", "main")
    _write_current(repo, _load(FIXTURES / "base.yaml"))
    lines: list[str] = []
    assert breaking.check(repo, out=lines.append) == 0
    assert lines == ["api-breaking: no published version (no tag server/v*); nothing to compare"]


def test_check_compares_with_the_stable_tag(tmp_path: Path) -> None:
    base = _load(FIXTURES / "base.yaml")
    beta = _apply(base, [{"op": "delete", "path": "/components/schemas/Thing/properties/tags"}])
    repo = _fixture_repo(tmp_path, [("server/v1.2.0", base), ("server/v1.3.0-beta.1", beta)])
    # The working tree lacks `tags` like the beta: a break against the stable base only.
    _write_current(repo, beta)
    lines: list[str] = []
    assert breaking.check(repo, out=lines.append) == 1
    assert lines[0] == (
        "api-breaking: 06 13 row 1 remove or rename: components/schemas/Thing/properties/tags: "
        "property removed"
    )
    assert lines[-1].startswith("api-breaking: 1 breaking change against server/v1.2.0")


def test_check_compares_with_the_newest_beta_without_stable(tmp_path: Path) -> None:
    base = _load(FIXTURES / "base.yaml")
    beta2 = _apply(base, [{"op": "delete", "path": "/components/schemas/Thing/properties/tags"}])
    repo = _fixture_repo(
        tmp_path, [("server/v0.1.0-beta.1", base), ("server/v0.1.0-beta.2", beta2)]
    )
    lines: list[str] = []
    assert breaking.check(repo, out=lines.append) == 0
    assert lines == [
        "api-breaking: docs/api/openapi.yaml has no breaking change against server/v0.1.0-beta.2"
    ]


def test_check_reports_an_accepted_major_break(tmp_path: Path) -> None:
    base = _load(FIXTURES / "base.yaml")
    repo = _fixture_repo(tmp_path, [("server/v1.2.0", base)])
    _write_current(repo, _removed_field("2.0.0")[1])
    lines: list[str] = []
    assert breaking.check(repo, out=lines.append) == 0
    assert lines[0].startswith("api-breaking: accepted (info.version 1.2.0 -> 2.0.0): 06 13 row 1")


def test_check_ignores_tags_not_merged_into_head(tmp_path: Path) -> None:
    base = _load(FIXTURES / "base.yaml")
    repo = _fixture_repo(tmp_path, [("server/v1.2.0", base)])
    _git(repo, "checkout", "-q", "--orphan", "other")
    _write_current(repo, base)
    _git(repo, "add", "-A")
    _git(
        repo,
        "-c",
        "user.name=Robin",
        "-c",
        "user.email=robin@example.com",
        "commit",
        "-q",
        "--no-gpg-sign",
        "-m",
        "docs: other",
    )
    lines: list[str] = []
    assert breaking.check(repo, out=lines.append) == 0
    assert lines[0].startswith("api-breaking: no published version")


def test_check_tag_without_document_fails(tmp_path: Path) -> None:
    repo = tmp_path / "repo"
    repo.mkdir()
    _git(repo, "init", "-q", "-b", "main")
    (repo / "README.md").write_text("x\n", encoding="utf-8")
    _git(repo, "add", "-A")
    _git(
        repo,
        "-c",
        "user.name=Sam",
        "-c",
        "user.email=sam@example.com",
        "commit",
        "-q",
        "--no-gpg-sign",
        "-m",
        "docs: x",
    )
    _git(repo, "tag", "server/v0.1.0")
    _write_current(repo, _load(FIXTURES / "base.yaml"))
    lines: list[str] = []
    assert breaking.check(repo, out=lines.append) == 1
    assert "server/v0.1.0" in lines[0]
    assert "docs/api/openapi.yaml" in lines[0]


def test_main_runs_against_the_repository() -> None:
    assert breaking.main(["check", "--root", str(REPO)]) == 0


# ---- wiring into make check (requirement 15) ----------------------------------------------


def _make_prerequisites(target: str) -> list[str]:
    joined = (REPO / "Makefile").read_text(encoding="utf-8").replace("\\\n", " ")
    found: list[str] = []
    for line in joined.splitlines():
        if line.startswith(f"{target}:"):
            found += line.split(":", 1)[1].split("##", 1)[0].split()
    return found


def test_make_gates_run_api_breaking_after_api_checks() -> None:
    gates = _make_prerequisites("gates")
    assert gates.index("api-checks") + 1 == gates.index("api-breaking")
    assert "api-breaking" in _make_prerequisites(".PHONY")


def _make_api_breaking(components: str) -> str:
    env = {k: v for k, v in os.environ.items() if k not in ("MAKEFLAGS", "MAKELEVEL", "MFLAGS")}
    done = subprocess.run(
        ["make", "-C", str(REPO), "api-breaking", f"COMPONENTS={components}"],
        capture_output=True,
        text=True,
        env=env,
        check=True,
    )
    return done.stdout


def test_make_api_breaking_runs_for_the_server_component() -> None:
    assert "api-breaking: " in _make_api_breaking("server")
    assert "SKIP" not in _make_api_breaking("server")


def test_make_api_breaking_skips_without_the_server_component() -> None:
    out = _make_api_breaking("apps")
    assert "SKIP  api-breaking" in out
    assert "component server not selected" in out
