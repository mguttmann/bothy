# SPDX-License-Identifier: Apache-2.0
"""Test isolation for every test under tools/.

git exports variables such as GIT_DIR to its hooks (an absolute path in a linked worktree),
and the pre-push hook runs `make check`, so the tests inherit them. With GIT_DIR set, a
fixture's `git init`, `git add` and `git commit` in a temporary directory act on the real
repository instead. The fixture below removes every variable that points git at another
repository, for each test.

A fresh GitHub-hosted runner also has no git identity: no global or system configuration,
and a host name without a domain, so git cannot guess one. A workstation usually has one,
which hides a fixture that commits or tags without naming its identity until CI fails. Each
test therefore runs like the runner: no global or system configuration, no identity from the
environment or from the calling git, and `user.useConfigOnly`, which stops the guess from the
host name that succeeds on many workstations. Fixtures name their identity through
testing/git_fixtures.py. tools/ci/tests/test_git_env_isolation.py holds both rules.
"""

from __future__ import annotations

import os

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

# Identities from the environment and the `-c` options of a calling git (a hook started by
# `git -c user.email=... push` inherits them in GIT_CONFIG_PARAMETERS).
GIT_IDENTITY_VARIABLES = (
    "GIT_AUTHOR_NAME",
    "GIT_AUTHOR_EMAIL",
    "GIT_COMMITTER_NAME",
    "GIT_COMMITTER_EMAIL",
    "EMAIL",
    "GIT_CONFIG_PARAMETERS",
)

# GIT_CONFIG_COUNT replaces any configuration a caller passed the same way.
RUNNER_GIT_ENVIRONMENT = {
    "GIT_CONFIG_GLOBAL": os.devnull,
    "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_CONFIG_COUNT": "1",
    "GIT_CONFIG_KEY_0": "user.useConfigOnly",
    "GIT_CONFIG_VALUE_0": "true",
}


@pytest.fixture(autouse=True)
def _isolate_git(monkeypatch: pytest.MonkeyPatch) -> None:
    """Let git find the repository from the working directory of each call, without identity."""
    for name in (*GIT_LOCATION_VARIABLES, *GIT_IDENTITY_VARIABLES):
        monkeypatch.delenv(name, raising=False)
    for name, value in RUNNER_GIT_ENVIRONMENT.items():
        monkeypatch.setenv(name, value)
