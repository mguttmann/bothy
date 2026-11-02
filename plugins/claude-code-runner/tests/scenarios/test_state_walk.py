# SPDX-License-Identifier: Apache-2.0
"""The walk over a live state directory: entries that vanish after the listing are skipped."""

from __future__ import annotations

from pathlib import Path

import pytest

from .conftest import assert_private, live_entries, live_files


def _state(tmp_path: Path) -> tuple[Path, Path, Path]:
    state = tmp_path / "state"
    state.mkdir(mode=0o700)
    state.chmod(0o700)
    db = state / "inbox.sqlite3"
    db.write_bytes(b"db")
    db.chmod(0o600)
    journal = state / "inbox.sqlite3-journal"
    journal.write_bytes(b"journal")
    journal.chmod(0o600)
    return state, db, journal


def test_an_entry_gone_after_the_listing_is_skipped(tmp_path: Path) -> None:
    _state_dir, db, journal = _state(tmp_path)
    listing = [db, journal]
    journal.unlink()  # SQLite removed its rollback journal between listing and stat
    assert [p for p, _ in live_entries(listing)] == [db]
    assert live_files(listing) == [(db, b"db")]


def test_assert_private_checks_every_entry_that_exists(tmp_path: Path) -> None:
    state, _db, journal = _state(tmp_path)
    assert assert_private(state) == {"inbox.sqlite3", "inbox.sqlite3-journal"}
    journal.chmod(0o644)
    with pytest.raises(AssertionError, match=r"inbox\.sqlite3-journal"):
        assert_private(state)
    (state / "sub").mkdir(mode=0o755)
    (state / "sub").chmod(0o755)
    journal.chmod(0o600)
    with pytest.raises(AssertionError, match="sub"):
        assert_private(state)
