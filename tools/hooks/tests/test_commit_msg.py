# SPDX-License-Identifier: Apache-2.0
"""Tests for the Conventional Commits and DCO sign-off check."""

from __future__ import annotations

import os
import subprocess
from pathlib import Path

import commit_msg
import pytest
from git_fixtures import commit, git, init_repo

ALEX = "Alex Example <alex@example.com>"
REPO = Path(__file__).resolve().parents[3]
# The test may run under `make test`; its make flags must not reach the make run below.
_MAKE_VARIABLES = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS"}


def _check(
    message: str, *, signoff: bool = False, identity: str = ALEX, allow_fixup: bool = True
) -> list[str]:
    return commit_msg.check_message(
        message, require_signoff=signoff, author=identity, allow_fixup=allow_fixup
    )


@pytest.mark.parametrize(
    "header",
    [
        "feat: add thread close button",
        "fix(sync): resume after reconnect",
        "docs(adr)!: replace MkDocs",
        "chore(deps): update uv.lock",
        "revert: feat: add thread close button",
    ],
)
def test_valid_headers(header: str) -> None:
    assert _check(header + "\n") == []


@pytest.mark.parametrize(
    ("header", "reason"),
    [
        ("Add thread close button", "type"),
        ("feature: add x", "type"),
        ("feat:add x", "type"),
        ("feat(): add x", "type"),
        ("feat: ", "description"),
        ("fix CI", "type"),
        ("typo", "type"),
    ],
)
def test_invalid_headers(header: str, reason: str) -> None:
    errors = _check(header + "\n")
    assert errors
    assert reason in errors[0]


def test_header_length_limit() -> None:
    header = "feat: " + "x" * commit_msg.HEADER_MAX_LENGTH
    assert any("longer" in e for e in _check(header))


def test_body_needs_blank_line() -> None:
    assert any("blank line" in e for e in _check("feat: x\nbody without gap\n"))


def test_comments_and_scissors_are_ignored() -> None:
    scissors = "# " + "-" * 24 + " >8 " + "-" * 24
    msg = f"feat: x\n\n# Please enter the commit message\n{scissors}\ndiff a b\n"
    assert _check(msg) == []


def test_signoff_required_and_matching() -> None:
    msg = f"feat: x\n\nbody\n\nSigned-off-by: {ALEX}\n"
    assert _check(msg, signoff=True) == []


def test_signoff_missing() -> None:
    assert any("Signed-off-by" in e for e in _check("feat: x\n", signoff=True))


def test_signoff_of_other_person_is_not_enough() -> None:
    msg = "feat: x\n\nSigned-off-by: Sam Example <sam@example.com>\n"
    assert any("Signed-off-by" in e for e in _check(msg, signoff=True))


def test_signoff_email_comparison_ignores_case() -> None:
    msg = "feat: x\n\nSigned-off-by: Alex Example <ALEX@example.com>\n"
    assert _check(msg, signoff=True) == []


def test_fixup_allowed_locally_rejected_in_ranges() -> None:
    assert _check("fixup! feat: x\n") == []
    assert any("fixup" in e for e in _check("fixup! feat: x\n", allow_fixup=False))


def test_merge_commit_message_is_accepted() -> None:
    assert (
        commit_msg.check_message(
            "Merge branch 'main' into feat/x\n", require_signoff=True, author=ALEX, merge=True
        )
        == []
    )


def test_merge_header_of_an_ordinary_commit_is_checked() -> None:
    """Only a real merge skips the rules, never a header that merely starts with `Merge `."""
    errors = _check("Merge whatever I like\n", signoff=True)
    assert any("header must be" in e for e in errors)
    assert any("Signed-off-by" in e for e in errors)


def test_range_mode_checks_a_one_parent_commit_named_merge(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    (repo / "b").write_text("b\n")
    commit(repo, "Merge whatever I like, no type, no sign-off")
    rc = commit_msg.main(["--repo", str(repo), "--range", f"{base}..HEAD", "--require-signoff"])
    assert rc == 1
    assert "Merge whatever" in capsys.readouterr().out


def test_file_mode_accepts_the_message_of_a_merge_in_progress(repo: Path, tmp_path: Path) -> None:
    """`git merge` runs the commit-msg hook with MERGE_HEAD in the git directory."""
    msg = tmp_path / "MSG"
    msg.write_text("Merge branch 'main' into feat/x\n")
    assert commit_msg.main(["--repo", str(repo), str(msg)]) == 1
    head = git(repo, "rev-parse", "HEAD").strip()
    (repo / ".git" / "MERGE_HEAD").write_text(head + "\n")
    assert commit_msg.main(["--repo", str(repo), str(msg)]) == 0


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    init_repo(tmp_path)
    (tmp_path / "a").write_text("a\n")
    commit(tmp_path, "chore: start")
    return tmp_path


def test_range_mode_checks_every_commit(repo: Path, capsys: pytest.CaptureFixture[str]) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    (repo / "b").write_text("b\n")
    commit(repo, "feat: add b", signoff=True)
    (repo / "c").write_text("c\n")
    commit(repo, "fix CI")
    rc = commit_msg.main(["--repo", str(repo), "--range", f"{base}..HEAD", "--require-signoff"])
    out = capsys.readouterr().out
    assert rc == 1
    assert "fix CI" in out
    assert "add b" not in out


def test_range_mode_passes_for_clean_range(repo: Path) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    (repo / "b").write_text("b\n")
    commit(repo, "feat: add b", signoff=True)
    assert (
        commit_msg.main(["--repo", str(repo), "--range", f"{base}..HEAD", "--require-signoff"]) == 0
    )


def test_file_mode_reads_message_file(repo: Path, tmp_path: Path) -> None:
    msg = tmp_path / "MSG"
    msg.write_text("feat: x\n\nSigned-off-by: Alex Example <alex@example.com>\n")
    assert commit_msg.main(["--repo", str(repo), "--require-signoff", str(msg)]) == 0


def test_file_mode_without_signoff_needs_no_identity(tmp_path: Path) -> None:
    """Without --require-signoff the message alone counts; tests have no git identity."""
    repo = tmp_path / "plain"
    subprocess.run(["git", "init", "-q", str(repo)], check=True, timeout=60)
    msg = tmp_path / "MSG"
    msg.write_text("fix: keep the gates green\n")
    assert commit_msg.main(["--repo", str(repo), str(msg)]) == 0


@pytest.mark.parametrize(
    ("title", "passes"),
    [
        ("fix: keep the gates green", True),
        ("Keep the gates green", False),
        ("Merge anything into main", False),
    ],
    ids=["conventional title", "plain title", "title starting with Merge"],
)
def test_commitlint_recipe_checks_the_pull_request_title(title: str, *, passes: bool) -> None:
    """The CI job gates sets PR_TITLE on every pull request, on a runner without git identity."""
    env = {k: v for k, v in os.environ.items() if k not in _MAKE_VARIABLES}
    env.update(
        COMMIT_RANGE="HEAD..HEAD",
        PR_TITLE=title,
        PR_BODY="Body.\n\nSigned-off-by: Alex Example <alex@example.com>\n",
    )
    run = subprocess.run(
        ["make", "-s", "-C", str(REPO), "commitlint"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=300,
    )
    output = run.stdout + run.stderr
    assert "CalledProcessError" not in output, output
    assert (run.returncode == 0) is passes, output
    assert ("commit message: " in run.stdout) is not passes, output


# ---- dependency bots (D-DEPS-3) --------------------------------------------------------
DEPENDABOT = "dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>"
RENOVATE = "renovate[bot] <29139614+renovate[bot]@users.noreply.github.com>"
BUMP = "build(deps): bump example from 1.0.0 to 1.1.0\n\nBumps example.\n"


def _range(repo: Path, base: str, *extra: str) -> int:
    return commit_msg.main(
        ["--repo", str(repo), "--range", f"{base}..HEAD", "--require-signoff", *extra]
    )


@pytest.mark.parametrize(
    ("login", "identity"), [("dependabot[bot]", DEPENDABOT), ("renovate[bot]", RENOVATE)]
)
def test_bot_commit_in_its_own_pull_request_needs_no_signoff(
    repo: Path, capsys: pytest.CaptureFixture[str], login: str, identity: str
) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, BUMP, author=identity)
    assert _range(repo, base, "--pr-author", login) == 0
    out = capsys.readouterr().out
    assert f"sign-off not required: commit by {login} in its own pull request" in out


def test_bot_commit_without_the_pull_request_author_still_needs_signoff(repo: Path) -> None:
    """Locally and on every other pull request the author identity alone exempts nothing."""
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, BUMP, author=DEPENDABOT)
    assert _range(repo, base) == 1


def test_forged_bot_identity_in_a_human_pull_request_needs_signoff(repo: Path) -> None:
    """Anyone can write a bot e-mail into a commit; only GitHub sets the pull request author."""
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, BUMP, author=DEPENDABOT)
    assert _range(repo, base, "--pr-author", "mallory") == 1


def test_other_bot_identity_in_a_bot_pull_request_needs_signoff(repo: Path) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, BUMP, author=DEPENDABOT)
    assert _range(repo, base, "--pr-author", "renovate[bot]") == 1


def test_human_commit_in_a_bot_pull_request_needs_signoff(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, BUMP, author=DEPENDABOT)
    commit(repo, "fix: adapt to the new release")
    assert _range(repo, base, "--pr-author", "dependabot[bot]") == 1
    out = capsys.readouterr().out
    human = git(repo, "rev-parse", "HEAD").strip()[:12]
    assert f"{human}: missing 'Signed-off-by:'" in out
    assert out.count("missing 'Signed-off-by:'") == 1


def test_bot_commit_still_needs_a_conventional_header(
    repo: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, "Bump example from 1.0.0 to 1.1.0", author=DEPENDABOT)
    assert _range(repo, base, "--pr-author", "dependabot[bot]") == 1
    assert "header must be" in capsys.readouterr().out


def test_bot_exemption_needs_the_bot_login_exactly(repo: Path) -> None:
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, BUMP, author=DEPENDABOT)
    assert _range(repo, base, "--pr-author", "dependabot") == 1
    assert _range(repo, base, "--pr-author", "Dependabot[bot]") == 1


def _make_commitlint(repo: Path, base: str, **extra: str) -> subprocess.CompletedProcess[str]:
    env = {k: v for k, v in os.environ.items() if k not in _MAKE_VARIABLES}
    # The recipe runs in the real checkout; GIT_DIR points its git calls at the fixture.
    env.update(GIT_DIR=str(repo / ".git"), COMMIT_RANGE=f"{base}..HEAD", **extra)
    if "PR_AUTHOR" not in extra:
        env.pop("PR_AUTHOR", None)
    return subprocess.run(
        ["make", "-s", "-C", str(REPO), "commitlint"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=300,
    )


@pytest.mark.parametrize(
    ("extra", "passes"),
    [({"PR_AUTHOR": "dependabot[bot]"}, True), ({}, False), ({"PR_AUTHOR": "alex"}, False)],
    ids=["bot pull request", "no pull request author", "human pull request"],
)
def test_commitlint_recipe_passes_the_pull_request_author(
    repo: Path, extra: dict[str, str], *, passes: bool
) -> None:
    """CI sets PR_AUTHOR from the event; the recipe hands it to the range check."""
    base = git(repo, "rev-parse", "HEAD").strip()
    commit(repo, BUMP, author=DEPENDABOT)
    run = _make_commitlint(repo, base, **extra)
    output = run.stdout + run.stderr
    assert (run.returncode == 0) is passes, output
    assert ("missing 'Signed-off-by:'" in output) is not passes, output


# ---- the squash commit text: pull request body (#872) ------------------------------------
SIGNED = "Fixes the gate.\n\nSigned-off-by: Alex Example <alex@example.com>\n"


@pytest.mark.parametrize(
    ("body", "passes"),
    [
        (SIGNED, True),
        ("Fixes the gate.\n", False),
        ("", False),
        # A sign-off inside an HTML comment of the template is not part of the commit text.
        ("Text\n<!--\nSigned-off-by: Alex Example <alex@example.com>\n-->\n", False),
        ("Signed-off-by: Alex Example\n", False),
    ],
    ids=["signed", "unsigned", "empty", "in a comment", "without e-mail"],
)
def test_pull_request_body_needs_a_signoff(body: str, *, passes: bool) -> None:
    assert (commit_msg.check_pr_body(body, author="alex") == []) is passes


@pytest.mark.parametrize("login", ["dependabot[bot]", "renovate[bot]", "github-actions[bot]"])
def test_pull_request_body_of_a_bot_needs_no_signoff(login: str) -> None:
    assert commit_msg.check_pr_body("Bumps example.\n", author=login) == []


def test_pr_body_mode_reads_the_file(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    body = tmp_path / "BODY"
    body.write_text("Fixes the gate.\n", encoding="utf-8")
    assert commit_msg.main(["--pr-body", str(body), "--pr-author", "alex"]) == 1
    assert "pull request body: " in capsys.readouterr().out
    body.write_text(SIGNED, encoding="utf-8")
    assert commit_msg.main(["--pr-body", str(body), "--pr-author", "alex"]) == 0


@pytest.mark.parametrize(
    ("body", "passes"), [(SIGNED, True), ("Fixes the gate.\n", False), ("", False)]
)
def test_commitlint_recipe_checks_the_pull_request_body(body: str, *, passes: bool) -> None:
    """With PR_TITLE set (every pull request) the body, the rest of the squash text, is checked."""
    env = {k: v for k, v in os.environ.items() if k not in _MAKE_VARIABLES}
    env.update(COMMIT_RANGE="HEAD..HEAD", PR_TITLE="fix: keep the gates green", PR_BODY=body)
    run = subprocess.run(
        ["make", "-s", "-C", str(REPO), "commitlint"],
        env=env,
        capture_output=True,
        text=True,
        check=False,
        timeout=300,
    )
    output = run.stdout + run.stderr
    assert (run.returncode == 0) is passes, output
