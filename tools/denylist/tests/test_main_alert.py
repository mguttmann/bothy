# SPDX-License-Identifier: Apache-2.0
"""Tests for the issue that a red run of denylist-main.yml opens or updates."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import denylist_scan as ds
import main_alert as ma
import pytest
import yaml
from git_fixtures import commit, init_repo

TESTDATA = Path(__file__).resolve().parents[1] / "testdata"
EXAMPLE_LIST = TESTDATA / "example-denylist.txt"
GENERIC = Path(__file__).resolve().parents[1] / "generic.rules"
WORKFLOW = Path(__file__).resolve().parents[3] / ".github" / "workflows" / "denylist-main.yml"
RUN_URL = "https://github.com/owner/repo/actions/runs/123"
SHA = "c" * 40
SUMMARY = {
    "exit": 1,
    "findings": 3,
    "kinds": {"commit-message": 1, "content": 2},
    "locations": ["a.md:2", "commit abcdef123456 message:1"],
    "truncated": 0,
}


@pytest.fixture(autouse=True)
def _isolated_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in (
        "BOTHY_DENYLIST_FILE",
        "BOTHY_DENYLIST_ALLOW_AUTHORS",
        "BOTHY_DENYLIST_NAMES",
        "BOTHY_DENYLIST_SUMMARY",
        "PR_TITLE",
        "PR_BODY",
        "PR_BODY_FILE",
        "SUMMARY",
        "RUN_URL",
        "REPO",
        "EVENT_NAME",
        "SHA",
    ):
        monkeypatch.delenv(name, raising=False)


class FakeGh:
    """Records gh calls; `issue list` answers with `open_issue`."""

    def __init__(self, open_issue: str = "") -> None:
        self.open_issue = open_issue
        self.calls: list[tuple[list[str], str | None]] = []

    def __call__(self, args: list[str], stdin: str | None = None) -> str:
        self.calls.append((args, stdin))
        if args[:2] == ["issue", "list"]:
            return self.open_issue + "\n"
        return ""

    def verbs(self) -> list[str]:
        return [" ".join(a[:2]) for a, _ in self.calls]


# --- body -------------------------------------------------------------------------


def test_body_names_run_counts_and_locations() -> None:
    body = ma.issue_body(json.dumps(SUMMARY), RUN_URL, "push", SHA)
    assert RUN_URL in body
    assert SHA in body
    assert "Findings: 3 (commit-message 1, content 2)" in body
    assert "a.md:2\ncommit abcdef123456 message:1\n" in body
    assert "rule line" not in body.lower()
    assert "push-discipline.md" in body


def test_body_of_a_truncated_list() -> None:
    summary = {**SUMMARY, "truncated": 7}
    assert "and 7 more locations" in ma.issue_body(json.dumps(summary), RUN_URL, "push", SHA)


@pytest.mark.parametrize("raw", ["", "not json", "[]", '{"exit": 2, "error": "configuration"}'])
def test_body_without_a_usable_summary_points_to_the_run(raw: str) -> None:
    body = ma.issue_body(raw, RUN_URL, "workflow_dispatch", SHA)
    assert RUN_URL in body
    assert "no finding summary" in body
    assert "Findings:" not in body


def test_body_drops_unexpected_fields_and_characters() -> None:
    summary = {
        **SUMMARY,
        "kinds": {"content": 1, "a`b": 2, "x": "nine"},
        "locations": ["a.md:2", "```\n@team", "x" * 500, 7],
        "text": "acme-widgets",
    }
    body = ma.issue_body(json.dumps(summary), RUN_URL, "push", SHA)
    assert "acme" not in body
    assert "\n@team\n" in body.split("```text", 1)[1]  # inside the block, so no mention
    assert "x" * 201 not in body
    assert body.count("```") == 2
    assert "nine" not in body


def test_body_from_a_real_scan_never_shows_a_term(tmp_path: Path) -> None:
    """End to end: scanner summary to issue body, with terms in content, path and message."""
    repo = init_repo(tmp_path / "repo")
    (repo / "a.md").write_text("acme-widgets at globex.test\n")
    (repo / "project-falcon.md").write_text("ok\n")
    commit(repo, "docs: notes about project falcon")
    out = tmp_path / "summary.json"
    rc = ds.main(
        [
            "--repo",
            str(repo),
            "--list",
            str(EXAMPLE_LIST),
            "--generic",
            str(GENERIC),
            "--summary",
            str(out),
        ]
    )
    assert rc == 1
    body = ma.issue_body(out.read_text(encoding="utf-8"), RUN_URL, "push", SHA)
    for term in ("acme", "globex", "falcon", "rule line"):
        assert term not in body.lower()
    assert "a.md:1" in body


# --- open or update -------------------------------------------------------------


def test_opens_an_issue_with_the_label_when_none_is_open() -> None:
    gh = FakeGh()
    ma.open_or_update(gh, "owner/repo", "BODY")
    assert gh.verbs() == ["label create", "issue list", "issue create"]
    create, stdin = gh.calls[-1]
    assert create[create.index("--label") + 1] == ma.LABEL
    assert create[create.index("--title") + 1] == ma.TITLE
    assert create[create.index("--repo") + 1] == "owner/repo"
    assert stdin == "BODY"
    listing, _ = gh.calls[1]
    assert listing[listing.index("--label") + 1] == ma.LABEL
    assert listing[listing.index("--state") + 1] == "open"


def test_comments_on_the_open_issue() -> None:
    gh = FakeGh(open_issue="42")
    ma.open_or_update(gh, "owner/repo", "BODY")
    assert gh.verbs() == ["label create", "issue list", "issue comment"]
    comment, stdin = gh.calls[-1]
    assert comment[2] == "42"
    assert stdin == "BODY"


def test_dry_run_prints_and_calls_nothing(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setenv("SUMMARY", json.dumps(SUMMARY))
    monkeypatch.setenv("RUN_URL", RUN_URL)
    monkeypatch.setenv("REPO", "owner/repo")
    monkeypatch.setenv("EVENT_NAME", "push")
    monkeypatch.setenv("SHA", SHA)

    def forbidden(*_args: Any, **_kwargs: Any) -> str:
        raise AssertionError("gh called in a dry run")

    monkeypatch.setattr(ma, "run_gh", forbidden)
    assert ma.main(["issue", "--dry-run"]) == 0
    out = capsys.readouterr().out
    assert "gh issue create" in out
    assert "a.md:2" in out


def test_issue_command_requires_the_run_context(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("RUN_URL", RUN_URL)
    assert ma.main(["issue", "--dry-run"]) == 2


# --- job output -----------------------------------------------------------------


def test_output_writes_one_compact_line(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    path = tmp_path / "summary.json"
    path.write_text(json.dumps(SUMMARY, indent=2), encoding="utf-8")
    assert ma.main(["output", str(path)]) == 0
    out = capsys.readouterr().out
    assert out.count("\n") == 1
    key, value = out.strip().split("=", 1)
    assert key == "summary"
    assert json.loads(value) == SUMMARY


def test_output_without_a_summary_file(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    assert ma.main(["output", str(tmp_path / "missing.json")]) == 0
    assert capsys.readouterr().out == "summary=\n"


# --- workflow -----------------------------------------------------------------


def load_workflow() -> dict[str, Any]:
    doc = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(doc, dict)
    return doc


def test_only_the_alert_job_may_write_issues() -> None:
    doc = load_workflow()
    assert doc["permissions"] == {"contents": "read"}
    jobs = doc["jobs"]
    alert = jobs["alert"]
    assert alert["permissions"] == {"contents": "read", "issues": "write"}
    assert "environment" not in alert
    for name, job in jobs.items():
        if name != "alert":
            assert "issues" not in (job.get("permissions") or {}), name


def test_alert_runs_only_after_a_red_scan_on_main() -> None:
    alert = load_workflow()["jobs"]["alert"]
    assert alert["needs"] == "denylist"
    condition = " ".join(str(alert["if"]).split())
    assert "failure()" in condition
    assert "github.ref == 'refs/heads/main'" in condition
    text = WORKFLOW.read_text(encoding="utf-8")
    assert "secrets." not in text.split("\n  alert:", 1)[1]
