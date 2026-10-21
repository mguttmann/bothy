# SPDX-License-Identifier: Apache-2.0
"""Order of the tool classes (09 2.4, 07 2.11, AG-30)."""

from __future__ import annotations

import pytest

from claude_code_runner.toolclass import ORDER, exceeds, rank


def test_order() -> None:
    assert ORDER == ("chat", "scoped", "host")
    assert [rank(c) for c in ORDER] == [0, 1, 2]


@pytest.mark.parametrize("configured", ORDER)
@pytest.mark.parametrize("token", ORDER)
def test_exceeds_all_nine_pairs(configured: str, token: str) -> None:
    assert exceeds(configured, token) is (ORDER.index(configured) > ORDER.index(token))


def test_unknown_class_rejected() -> None:
    with pytest.raises(ValueError, match="tool class"):
        rank("full")
