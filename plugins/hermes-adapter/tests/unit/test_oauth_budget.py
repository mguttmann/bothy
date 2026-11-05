# SPDX-License-Identifier: Apache-2.0
"""The reproduction of the host's OAuth system text budget stays faithful (D-FHUSC-6)."""

from __future__ import annotations

from oauth_budget import DEFAULT_BUDGET, trim


def _text(text: str) -> dict[str, str]:
    return {"type": "text", "text": text}


def test_default_budget_is_3000() -> None:
    # The host patch's default; the live profiles do not set oauth_system_budget_chars.
    assert DEFAULT_BUDGET == 3000


def test_identity_block_kept_and_the_rest_capped() -> None:
    identity = _text("I" * 5000)
    first = _text("a" * 2000)
    second = _text("b" * 500 + "\n" + "c" * 1500)
    third = _text("dropped")
    tool = {"type": "image", "source": "x"}
    out = trim([identity, first, second, tool, third])
    assert out[0] == identity
    assert out[1] == first
    # 1000 characters left: cut at the line break after 500 (more than half of the room).
    assert out[2] == _text("b" * 500)
    assert out[3] == tool
    assert len(out) == 4
    assert sum(len(b["text"]) for b in out[1:] if b.get("type") == "text") <= DEFAULT_BUDGET


def test_hard_cut_without_a_late_line_break_and_off_switch() -> None:
    blocks = [_text("id"), _text("x\n" + "y" * 50)]
    assert trim(blocks, 10) == [_text("id"), _text(("x\n" + "y" * 50)[:10])]
    assert trim(blocks, 0) is blocks
    assert trim("plain", 10) == "plain"
