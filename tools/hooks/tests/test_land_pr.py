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
TITLE = "fix(ci): keep the gates green"
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
    ) -> None:
        self.body = body
        self.fail = fail
        self.state = state
        self.on_main = on_main
        self.calls: list[list[str]] = []
        self.body_file_text: str | None = None

    def __call__(self, args: list[str]) -> str:
        self.calls.append(args)
        if self.fail and args[: len(self.fail.split())] == self.fail.split():
            raise subprocess.CalledProcessError(1, ["gh", *args])
        if args[:2] == ["pr", "view"] and "mergeCommit" in args:
            return json.dumps({"mergeCommit": {"oid": MERGED}})
        if args[:2] == ["pr", "view"]:
            return json.dumps(
                {"title": TITLE, "body": self.body, "headRefOid": HEAD, "state": self.state}
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
            return self.on_main or ""
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


def test_bot_pull_request_needs_the_maintainer_signoff_too() -> None:
    """CI exempts a bot's body; the squash commit on main still carries the sign-off."""
    gh = FakeGh("Bumps example from 1.0 to 1.1.\n")
    assert _land(gh) == 1
    assert gh.merge_call() is None


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
