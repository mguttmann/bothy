# SPDX-License-Identifier: Apache-2.0
"""Shared settings of the runner tests.

The tests against the installed Claude Code CLI (``tests/real_cli``) are collected only with
``CLAUDE_CODE_RUNNER_REAL_CLI=1``: a real turn costs money and uses the operator's login, so
nothing in ``make check`` depends on them (docs/spec/09-agenten.md 16.1, design note 17).
"""

from __future__ import annotations

import os

import pytest

collect_ignore_glob = [] if os.environ.get("CLAUDE_CODE_RUNNER_REAL_CLI") == "1" else ["real_cli/*"]


@pytest.fixture
def anyio_backend() -> str:
    """Async tests run on asyncio, like the runner."""
    return "asyncio"
