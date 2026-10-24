# SPDX-License-Identifier: Apache-2.0
"""The TITLE line of the first reply of a new session (09 12 point 5, 18.4)."""

from __future__ import annotations

from claude_code_runner.title import split_title


def test_split_when_requested() -> None:
    assert split_title("TITLE: Fix the login form\nDone.", True) == ("Fix the login form", "Done.")
    assert split_title("TITLE:   \nDone.", True) == (None, "Done.")
    assert split_title("\n\nTITLE: A\n\nDone.", True) == ("A", "Done.")
    assert split_title("TITLE: Only", True) == ("Only", "")


def test_untouched_when_not_requested_or_absent() -> None:
    assert split_title("TITLE: A\nDone.", False) == (None, "TITLE: A\nDone.")
    assert split_title("Done.\nTITLE: A", True) == (None, "Done.\nTITLE: A")
