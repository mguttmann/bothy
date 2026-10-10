# SPDX-License-Identifier: Apache-2.0
"""Test isolation for every test under tools/.

git exports variables such as GIT_DIR to its hooks (an absolute path in a linked worktree),
and the pre-push hook runs `make check`, so the tests inherit them. With GIT_DIR set, a
fixture's `git init`, `git add` and `git commit` in a temporary directory act on the real
repository instead. The fixture below removes every variable that points git at another
repository, for each test; tools/ci/tests/test_git_env_isolation.py holds this.
"""

from __future__ import annotations

import pytest

GIT_LOCATION_VARIABLES = (
    "GIT_DIR",
    "GIT_WORK_TREE",
    "GIT_INDEX_FILE",
    "GIT_COMMON_DIR",
    "GIT_OBJECT_DIRECTORY",
    "GIT_ALTERNATE_OBJECT_DIRECTORIES",
    "GIT_NAMESPACE",
    "GIT_PREFIX",
)


@pytest.fixture(autouse=True)
def _isolate_git(monkeypatch: pytest.MonkeyPatch) -> None:
    """Let git find the repository from the working directory of each call."""
    for name in GIT_LOCATION_VARIABLES:
        monkeypatch.delenv(name, raising=False)
