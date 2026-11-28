# SPDX-License-Identifier: Apache-2.0
"""Tests for the landing step: the squash commit gets exactly the checked pull request text."""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

import land_pr
import pytest

JANE = "Jane Example <12345+jane-example@users.noreply.github.com>"
SIGNED = f"Fixes the gate.\n\nSigned-off-by: {JANE}\n"
# GitHub's default squash text (viewerMergeBodyText) as it reached main on 2026-11-25 and
# 2026-11-26: wrapped at 72 columns, then "---------" and the trailers of the branch commits.
WRAPPED = (
    "Fixes the gate and keeps the text of the pull request as the message of\n"
    "the squash commit.\n\n"
    "Signed-off-by: Jane Example\n"
    "<12345+jane-example@users.noreply.github.com>\n\n"
    "---------\n\n"
    f"Signed-off-by: {JANE}\n"
)
# The trailer of a branch commit signed with the machine's git identity, a work address.
OTHER_ADDRESS = (
    f"Fixes the gate.\n\nSigned-off-by: Jane Example <jane@corp.example>\nSigned-off-by: {JANE}\n"
)
JANE_GH = {"is_bot": False, "login": "jane-example", "name": "Jane Example"}
# What gh pr view reports for a pull request of hermes-pin.yml (CI sees github-actions[bot]).
ACTIONS_GH = {"is_bot": True, "login": "app/github-actions"}
ACTIONS = "github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>"
TITLE = "fix(ci): keep the gates green"
REPO = Path(__file__).resolve().parents[3]
# The squash commit of #1784 on main, a pull request of hermes-pin.yml that landed with the
# maintainer's sign-off and the bot as author, so denylist-main failed (FIX-LAND-IDENTITY).
LEAKED = "0117ec48c63ac7670175f09c48e6305fbd60977a"
HEAD = "0123456789abcdef0123456789abcdef01234567"
MERGED = "fedcba9876543210fedcba9876543210fedcba98"


class FakeGh:
    """Answers the gh calls of land_pr; `on_main` is the message GitHub writes on main."""

    def __init__(
        self,
        body: str,
        *,
        state: str = "OPEN",
        on_main: str | None = None,
        fail: str = "",
        merged_anyway: bool = False,
        author: dict[str, object] | None = None,
        landed_author: str | None = None,
    ) -> None:
        self.body = body
        self.author = JANE_GH if author is None else author
        self.landed_author = landed_author
        self.fail = fail
        self.merged_anyway = merged_anyway
        self.state = state
        self.on_main = on_main
        self.calls: list[list[str]] = []
        self.body_file_text: str | None = None

    def __call__(self, args: list[str]) -> str:
        self.calls.append(args)
        if self.fail and args[: len(self.fail.split())] == self.fail.split():
            if args[:2] == ["pr", "merge"] and self.merged_anyway:
                self.state = "MERGED"
                self.on_main = f"{args[args.index('--subject') + 1]}\n\n" + Path(
                    args[args.index("--body-file") + 1]
                ).read_text(encoding="utf-8")
            raise subprocess.CalledProcessError(1, ["gh", *args])
        if args[:2] == ["pr", "view"] and args[-1] == "state":
            return json.dumps({"state": self.state})
        if args[:2] == ["pr", "view"] and "mergeCommit" in args:
            return json.dumps({"mergeCommit": {"oid": MERGED}})
        if args[:2] == ["pr", "view"]:
            return json.dumps(
                {
                    "title": TITLE,
                    "body": self.body,
                    "headRefOid": HEAD,
                    "state": self.state,
                    "author": self.author,
                }
            )
        if args[:2] == ["pr", "merge"]:
            self.body_file_text = Path(args[args.index("--body-file") + 1]).read_text(
                encoding="utf-8"
            )
            subject = args[args.index("--subject") + 1]
            if self.on_main is None:
                self.on_main = f"{subject}\n\n{self.body_file_text}"
            return ""
        if args[0] == "api":
            # GitHub writes the squash commit with the --author-email of a maintainer's own
            # pull request and otherwise as the pull request author (bots: always).
            merge = self.merge_call() or []
            author = self.landed_author or (JANE if "--author-email" in merge else ACTIONS)
            name, email = author[:-1].split(" <")
            message = self.on_main or ""
            return json.dumps({"message": message, "author": {"name": name, "email": email}})
        raise AssertionError(args)

    def merge_call(self) -> list[str] | None:
        return next((c for c in self.calls if c[:2] == ["pr", "merge"]), None)


def _land(gh: FakeGh, **kwargs: object) -> int:
    return land_pr.land("42", identity=JANE, gh=gh, **kwargs)  # type: ignore[arg-type]


def test_lands_with_the_checked_body_subject_author_and_head() -> None:
    gh = FakeGh(SIGNED)
    assert _land(gh) == 0
    call = gh.merge_call()
    assert call is not None
    assert call[call.index("--subject") + 1] == f"{TITLE} (#42)"
    assert call[call.index("--author-email") + 1] == "12345+jane-example@users.noreply.github.com"
    assert call[call.index("--match-head-commit") + 1] == HEAD
    assert "--squash" in call
    # Never GitHub's own default text: the body always goes in as a file, unchanged.
    assert "--body" not in call
    assert gh.body_file_text == SIGNED


@pytest.mark.parametrize(
    "body",
    [WRAPPED, OTHER_ADDRESS, "Fixes the gate.\n"],
    ids=["wrapped sign-off", "work address", "no sign-off"],
)
def test_refuses_a_body_that_breaks_the_rule(body: str, capsys: pytest.CaptureFixture[str]) -> None:
    gh = FakeGh(body)
    assert _land(gh) == 1
    assert gh.merge_call() is None
    assert "not merged" in capsys.readouterr().out


def test_refuses_a_signoff_of_another_identity() -> None:
    gh = FakeGh(SIGNED.replace("12345+jane-example", "777+jane"))
    assert _land(gh) == 1
    assert gh.merge_call() is None


def _leaked_commit() -> tuple[str, str]:
    """Message and author of the real squash commit of #1784, read from the repository."""
    fmt = "--format=%an <%ae>%x00%B"
    run = subprocess.run(
        ["git", "-C", str(REPO), "show", "-s", fmt, LEAKED],
        capture_output=True,
        text=True,
        check=False,
    )
    if run.returncode != 0:
        pytest.skip(f"commit {LEAKED[:9]} is not in this clone (shallow or no history)")
    author, message = run.stdout.split("\0", 1)
    return message, author


def test_refuses_the_real_text_of_the_bot_pull_request_1784(
    capsys: pytest.CaptureFixture[str],
) -> None:
    """Counter-check against the real artifact: the body that reached main as 0117ec48c."""
    message, _ = _leaked_commit()
    body = message.split("\n\n", 1)[1]
    gh = FakeGh(body, author=ACTIONS_GH)
    assert _land(gh) == 1
    assert gh.merge_call() is None
    out = capsys.readouterr().out
    assert "github-actions[bot]" in out
    assert "noreply" not in out


def test_the_real_commit_of_1784_has_a_signoff_that_does_not_repeat_its_author() -> None:
    """The readback rule is the one of denylist-main, checked on the commit that failed it."""
    message, author = _leaked_commit()
    assert land_pr.foreign_signoffs(message, author) == 1
    assert land_pr.foreign_signoffs(f"{TITLE}\n\n{SIGNED}", JANE) == 0


def test_bot_pull_request_lands_without_signoff_and_without_author_email() -> None:
    """GitHub keeps the bot as author, so no sign-off of the maintainer may reach main."""
    body = "Bumps example from 1.0 to 1.1.\n"
    gh = FakeGh(body, author=ACTIONS_GH)
    assert _land(gh) == 0
    call = gh.merge_call()
    assert call is not None
    assert "--author-email" not in call
    assert gh.body_file_text == body


@pytest.mark.parametrize(
    "body",
    [
        SIGNED,
        f"Bumps example.\n\nCo-authored-by: {JANE}\n",
        "Bumps.\n\nsigned-off-by: x <y@z.example>\n",
    ],
    ids=["maintainer sign-off", "co-author", "lower case sign-off"],
)
def test_bot_pull_request_with_an_identity_trailer_is_refused(body: str) -> None:
    gh = FakeGh(body, author=ACTIONS_GH)
    assert _land(gh) == 1
    assert gh.merge_call() is None


@pytest.mark.parametrize(
    "author",
    [{"is_bot": False, "login": "alex"}, {"is_bot": True, "login": "app/unknown"}, {}],
    ids=["another person", "a bot not on the list", "no author"],
)
def test_pull_request_of_another_account_is_refused(author: dict[str, object]) -> None:
    """GitHub would make that account the author, which the maintainer's sign-off misses."""
    gh = FakeGh(SIGNED, author=author)
    assert _land(gh) == 1
    assert gh.merge_call() is None


def test_reports_a_landed_commit_whose_signoff_misses_its_author(
    capsys: pytest.CaptureFixture[str],
) -> None:
    gh = FakeGh(SIGNED, landed_author=ACTIONS)
    assert _land(gh) == 2
    assert "does not repeat its author" in capsys.readouterr().out


def test_refuses_a_pull_request_that_is_not_open() -> None:
    gh = FakeGh(SIGNED, state="MERGED")
    assert _land(gh) == 1
    assert gh.merge_call() is None


def test_crlf_body_lands_with_unix_line_ends() -> None:
    gh = FakeGh(SIGNED.replace("\n", "\r\n"))
    assert _land(gh) == 0
    assert gh.body_file_text == SIGNED


def test_template_comments_stay_out_of_the_commit() -> None:
    """A comment after the sign-off would end the message, so git would read no trailer."""
    body = SIGNED.replace("\n\nSigned", "\n\n<!-- checklist -->\n\nSigned")
    gh = FakeGh(f"<!-- What and why -->\n{body}<!-- end -->\n")
    assert _land(gh) == 0
    assert gh.body_file_text == SIGNED


def test_refuses_a_comment_that_would_join_text(capsys: pytest.CaptureFixture[str]) -> None:
    """CI scans the raw body; removing an inline comment would join text it saw apart."""
    gh = FakeGh(SIGNED.replace("Fixes the gate.", "Fixes node<!-- x -->.example the gate."))
    assert _land(gh) == 1
    assert gh.merge_call() is None
    assert "HTML comment" in capsys.readouterr().out


def test_failed_merge_is_not_merged(capsys: pytest.CaptureFixture[str]) -> None:
    gh = FakeGh(SIGNED, fail="pr merge")
    assert _land(gh) == 1
    assert "not merged" in capsys.readouterr().out


def test_merge_that_failed_after_merging_is_read_back() -> None:
    """gh can merge and then fail to delete a branch a worktree holds."""
    gh = FakeGh(SIGNED, fail="pr merge", merged_anyway=True)
    assert _land(gh, delete_branch=True) == 0


def test_failed_readback_after_the_merge_exits_2(capsys: pytest.CaptureFixture[str]) -> None:
    gh = FakeGh(SIGNED, fail="api")
    assert _land(gh) == 2
    assert "merged, but" in capsys.readouterr().out


def test_reports_a_commit_on_main_that_differs_from_the_checked_text(
    capsys: pytest.CaptureFixture[str],
) -> None:
    """The real artifact is read back: a changed message fails loudly after the merge."""
    gh = FakeGh(SIGNED, on_main=f"{TITLE} (#42)\n\n{WRAPPED}")
    assert _land(gh) == 2
    assert "differs" in capsys.readouterr().out


def test_dry_run_merges_nothing() -> None:
    gh = FakeGh(SIGNED)
    assert _land(gh, dry_run=True) == 0
    assert gh.merge_call() is None


@pytest.mark.parametrize(
    "identity",
    ["Jane Example <jane@corp.example>", "Jane Example", ""],
    ids=["not a no-reply address", "no address", "empty"],
)
def test_identity_must_be_a_noreply_identity(identity: str) -> None:
    with pytest.raises(land_pr.LandError):
        land_pr.parse_identity(identity)
