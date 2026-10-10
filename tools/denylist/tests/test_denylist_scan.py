# SPDX-License-Identifier: Apache-2.0
"""Tests for the white-label deny-list scanner.

Every test builds a throw-away git repository; the terms come from
tools/denylist/testdata/example-denylist.txt (invented, neutral words).
"""

from __future__ import annotations

import subprocess
from pathlib import Path

import denylist_scan as ds
import pytest

TESTDATA = Path(__file__).resolve().parents[1] / "testdata"
EXAMPLE_LIST = TESTDATA / "example-denylist.txt"
EXAMPLE_ALLOW = TESTDATA / "example-allow-authors.txt"
GENERIC = Path(__file__).resolve().parents[1] / "generic.rules"


@pytest.fixture(autouse=True)
def _isolated_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    """A maintainer's configured list must not leak into the tests."""
    for name in ("BOTHY_DENYLIST_FILE", "BOTHY_DENYLIST_ALLOW_AUTHORS", "BOTHY_DENYLIST_NAMES"):
        monkeypatch.delenv(name, raising=False)


def git(repo: Path, *args: str, env: dict[str, str] | None = None) -> str:
    import os

    full_env = {**os.environ, **(env or {})}
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True, env=full_env
    ).stdout


def commit(repo: Path, message: str, *, author: str = "Alex Example <alex@example.com>") -> None:
    name, email = author[:-1].split(" <")
    env = {
        "GIT_AUTHOR_NAME": name,
        "GIT_AUTHOR_EMAIL": email,
        "GIT_COMMITTER_NAME": name,
        "GIT_COMMITTER_EMAIL": email,
    }
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "--allow-empty", "-m", message, env=env)


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    r = tmp_path / "repo"
    r.mkdir()
    git(r, "init", "-q", "-b", "main")
    git(r, "config", "commit.gpgsign", "false")
    git(r, "config", "tag.gpgsign", "false")
    git(r, "config", "core.hooksPath", "/dev/null")
    (r / "README.md").write_text("# Demo\n\nNothing to see.\n")
    commit(r, "docs: start")
    return r


def run(
    repo: Path, *extra: str, list_path: Path | None = EXAMPLE_LIST, allow: Path | None = None
) -> ds.Report:
    args = ["--repo", str(repo), "--generic", str(GENERIC)]
    if list_path is not None:
        args += ["--list", str(list_path)]
    if allow is not None:
        args += ["--allow-authors", str(allow)]
    return ds.scan(ds.parse_args([*args, *extra]))


# --- rule parsing -------------------------------------------------------------


def test_parse_literal_regex_comments_and_blank_lines() -> None:
    rules = ds.parse_rules("# c\n\nAcme\nre:\\bfoo\\d\n  \n", "private")
    assert [(r.lineno, r.kind) for r in rules.rules] == [(3, "literal"), (4, "regex")]


def test_parse_exclude_directive() -> None:
    rules = ds.parse_rules("@exclude-path: docs/*.md\nfoo\n", "generic")
    assert rules.exclude == ["docs/*.md"]


def test_parse_invalid_regex_names_line_not_content() -> None:
    with pytest.raises(ds.ConfigError) as err:
        ds.parse_rules("ok\nre:(secret-term\n", "private")
    assert "line 2" in str(err.value)
    assert "secret-term" not in str(err.value)


def test_parse_rejects_empty_private_list() -> None:
    with pytest.raises(ds.ConfigError, match="no rules"):
        ds.load_private_list_text("# only comments\n")


def test_matching_ignores_case() -> None:
    rules = ds.parse_rules("acme-widgets\n", "private")
    hits = ds.match_text("We sell ACME-Widgets.", rules)
    assert [(h.offset, h.text) for h in hits] == [(8, "ACME-Widgets")]


# --- list discovery -----------------------------------------------------------


def test_missing_list_fails_closed(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("BOTHY_DENYLIST_FILE", raising=False)
    with pytest.raises(ds.ConfigError, match="deny list not found"):
        run(repo, list_path=None)


def test_allow_missing_runs_generic_rules_only(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("BOTHY_DENYLIST_FILE", raising=False)
    (repo / "notes.md").write_text("acme-widgets everywhere\n")
    commit(repo, "docs: notes")
    report = run(repo, "--allow-missing", list_path=None)
    assert report.private_loaded is False
    assert report.findings == []


def test_list_from_environment(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("BOTHY_DENYLIST_FILE", str(EXAMPLE_LIST))
    (repo / "a.md").write_text("acme-widgets\n")
    commit(repo, "docs: a")
    report = run(repo, list_path=None)
    assert [f.kind for f in report.findings] == ["content"]


def test_list_from_dotfile_relative_to_repo(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("BOTHY_DENYLIST_FILE", raising=False)
    target = repo.parent / "ops" / "denylist.txt"
    target.parent.mkdir()
    target.write_text(EXAMPLE_LIST.read_text())
    (repo / ".denylist-path").write_text("../ops/denylist.txt\n")
    (repo / "a.md").write_text("globex.test\n")
    git(repo, "add", "a.md")
    commit(repo, "docs: a")
    report = run(repo, list_path=None)
    assert [f.location for f in report.findings] == ["a.md:1:1"]


# --- what is scanned ----------------------------------------------------------


def test_content_finding_has_location_and_rule(repo: Path) -> None:
    (repo / "docs").mkdir()
    (repo / "docs" / "x.md").write_text("line one\nabout Project Falcon here\n")
    commit(repo, "docs: x")
    report = run(repo)
    [f] = report.findings
    assert (f.kind, f.location, f.rule.source, f.rule.lineno) == (
        "content",
        "docs/x.md:2:7",
        "private",
        7,
    )


def test_untracked_files_are_not_scanned(repo: Path) -> None:
    (repo / "scratch.md").write_text("acme-widgets\n")
    assert run(repo).findings == []


def test_file_name_is_scanned(repo: Path) -> None:
    (repo / "acme-widgets-notes.md").write_text("clean\n")
    commit(repo, "docs: notes")
    assert [f.kind for f in run(repo).findings] == ["path"]


def test_binary_content_is_scanned(repo: Path) -> None:
    (repo / "image.png").write_bytes(b"\x89PNG\x00\x00tEXtAuthor\x00globex.test\x00")
    commit(repo, "docs: image")
    [f] = run(repo).findings
    assert f.kind == "content"
    assert f.location.startswith("image.png@")


def test_commit_message_is_scanned(repo: Path) -> None:
    commit(repo, "feat: wire up the acme-widgets importer")
    [f] = run(repo).findings
    assert f.kind == "commit-message"


def test_branch_and_tag_names_are_scanned(repo: Path) -> None:
    git(repo, "branch", "feature/project-falcon")
    git(repo, "tag", "-a", "v0.0.1", "-m", "release for globex.test")
    kinds = sorted(f.kind for f in run(repo).findings)
    assert kinds == ["ref", "tag-message"]


def test_extra_name_is_scanned(repo: Path) -> None:
    report = run(repo, "--name", "feat/acme-widgets-sync")
    assert [f.kind for f in report.findings] == ["name"]


def test_extra_names_from_environment(repo: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("BOTHY_DENYLIST_NAMES", "feat/x;$(touch pwned)\nfeat/acme-widgets\n")
    report = run(repo)
    assert [f.kind for f in report.findings] == ["name"]
    assert not (repo / "pwned").exists()


def test_author_identity_is_scanned(repo: Path) -> None:
    commit(repo, "docs: y", author="Sam Example <sam@initech.test>")
    findings = run(repo).findings
    assert [(f.kind, f.location.split()[-1]) for f in findings] == [
        ("identity", "author"),
        ("identity", "committer"),
    ]


def test_allowed_identity_is_not_reported(repo: Path) -> None:
    commit(
        repo,
        "docs: y\n\nSigned-off-by: Robin Example <robin@initech.test>",
        author="Robin Example <robin@initech.test>",
    )
    assert run(repo, allow=EXAMPLE_ALLOW).findings == []


def test_allowed_identity_does_not_hide_other_text_in_message(repo: Path) -> None:
    commit(
        repo,
        "docs: about globex.test\n\nSigned-off-by: Robin Example <robin@initech.test>",
        author="Robin Example <robin@initech.test>",
    )
    [f] = run(repo, allow=EXAMPLE_ALLOW).findings
    assert f.kind == "commit-message"


def test_range_limits_commit_checks(repo: Path) -> None:
    commit(repo, "feat: acme-widgets in old history")
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, "docs: clean change")
    assert run(repo, "--range", f"{base}..HEAD").findings == []
    assert len(run(repo).findings) == 1


def test_history_finds_deleted_content_and_names(repo: Path) -> None:
    (repo / "old-acme-widgets.txt").write_text("project falcon\n")
    commit(repo, "docs: temp")
    (repo / "old-acme-widgets.txt").unlink()
    commit(repo, "docs: remove temp")
    assert run(repo).findings == []
    kinds = sorted(f.kind for f in run(repo, "--history").findings)
    assert kinds == ["history-content", "history-path"]


def test_history_scans_unmerged_branches(repo: Path) -> None:
    git(repo, "checkout", "-q", "-b", "side")
    commit(repo, "feat: globex.test integration")
    git(repo, "checkout", "-q", "main")
    report = run(repo, "--history")
    assert "commit-message" in [f.kind for f in report.findings]


# --- generic rules ------------------------------------------------------------


@pytest.mark.parametrize(
    "text",
    [
        "see " + "/" + "Users/alex/Projekte/demo",
        "ssh build-box.tail1234" + ".ts" + ".net",
        "mail sam@corp-intern.de",
        "connect to 192.168.10.4",
        "connect to 100.101.102.103",
        "channel " + "1234567890" + "12345678",
    ],
)
def test_generic_rules_catch_leaks(repo: Path, text: str) -> None:
    (repo / "a.md").write_text(text + "\n")
    commit(repo, "docs: a")
    report = run(repo, "--allow-missing", list_path=None)
    assert [f.rule.source for f in report.findings] == ["generic"]


@pytest.mark.parametrize(
    "text",
    [
        "mail security@example.com or alex@docs.example.org",
        "bind 127.0.0.1:9000, doc ranges 192.0.2.10 198.51.100.7 203.0.113.9",
        "image sonarqube:26.9.0.129388-community",
        "sha256:" + "0123456789" * 6 + "abcd",
        "clone git@github.com:OWNER/bothy.git",
        "reserved test@initech.test",
        "path " + "/" + "Users/<user>/project",
        'svg d="M-312.40625 42.75 C0.1515133509874999' + '6 42.75"',
    ],
)
def test_generic_rules_allow_documentation_values(repo: Path, text: str) -> None:
    (repo / "a.md").write_text(text + "\n")
    commit(repo, "docs: a")
    assert run(repo, "--allow-missing", list_path=None).findings == []


def test_generic_rules_skip_identity_trailers(repo: Path) -> None:
    commit(repo, "feat: x\n\nSigned-off-by: Sam Doe <sam@personal-mail.de>")
    assert run(repo, "--allow-missing", list_path=None).findings == []


# --- output -------------------------------------------------------------------


def test_output_is_redacted_by_default(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / "a.md").write_text("acme-widgets\n")
    commit(repo, "docs: a")
    rc = ds.main(["--repo", str(repo), "--list", str(EXAMPLE_LIST), "--generic", str(GENERIC)])
    out = capsys.readouterr().out
    assert rc == 1
    assert "a.md:1:1" in out
    assert "private rule line 5" in out
    assert "acme" not in out.lower()


def test_output_masks_terms_in_paths(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / "acme-widgets-notes.md").write_text("clean\n")
    commit(repo, "docs: notes")
    ds.main(["--repo", str(repo), "--list", str(EXAMPLE_LIST), "--generic", str(GENERIC)])
    out = capsys.readouterr().out
    assert "path ************-notes.md" in out
    assert "acme" not in out.lower()


def test_reveal_prints_match(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (repo / "a.md").write_text("acme-widgets\n")
    commit(repo, "docs: a")
    ds.main(
        ["--repo", str(repo), "--list", str(EXAMPLE_LIST), "--generic", str(GENERIC), "--reveal"]
    )
    assert "acme-widgets" in capsys.readouterr().out


def test_config_error_exit_code(
    repo: Path, capsys: pytest.CaptureFixture[str], monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.delenv("BOTHY_DENYLIST_FILE", raising=False)
    rc = ds.main(["--repo", str(repo), "--generic", str(GENERIC)])
    assert rc == 2
    assert "deny list not found" in capsys.readouterr().err


def test_clean_repo_exit_zero(repo: Path) -> None:
    rc = ds.main(["--repo", str(repo), "--list", str(EXAMPLE_LIST), "--generic", str(GENERIC)])
    assert rc == 0


def test_this_repository_passes_generic_rules() -> None:
    """The in-repo generic rules hold for the repository itself."""
    root = Path(__file__).resolve().parents[3]
    report = ds.scan(
        ds.parse_args(["--repo", str(root), "--allow-missing", "--range", "HEAD..HEAD"])
    )
    assert [f.location for f in report.findings] == []
