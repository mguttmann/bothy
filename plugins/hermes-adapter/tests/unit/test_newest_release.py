# SPDX-License-Identifier: Apache-2.0
"""``--hermes-version=latest`` must pick the newest Hermes release (09 17.1).

Hermes Agent changed its tag scheme in October 2026: the releases after ``v2026.9.24`` are
``v0.22.x`` (``v0.22.5`` on 2026-10-22, a descendant of ``v2026.9.24``), next to canary tags
``v0.22.5+canary.<stamp>``. ``newest_release`` compares the numbers of the tag names, so
``v2026.9.24`` outranks ``v0.22.5`` and "latest" silently resolves to the pin itself.
"""

from __future__ import annotations

import subprocess
from typing import Any

import fake_hermes_kit
import pytest
from hermes_harness.pin import PIN_FILE, load_pin

# The shape of `git ls-remote --tags https://github.com/NousResearch/hermes-agent` on
# 2026-10-29 (commits shortened to placeholders, tag names as listed there).
LS_REMOTE = "\n".join(
    [
        "1111111111111111111111111111111111111111\trefs/tags/v2026.9.21",
        "f97608f178d1ffeca59860195ab7da295f7c8e5f\trefs/tags/v2026.9.24",
        "2222222222222222222222222222222222222222\trefs/tags/v0.22.2",
        "3333333333333333333333333333333333333333\trefs/tags/v0.22.4",
        "4444444444444444444444444444444444444444\trefs/tags/v0.22.5",
        "5555555555555555555555555555555555555555\trefs/tags/v0.22.5+canary.20261029T064840Z",
    ]
)


def test_latest_is_the_newest_release_after_the_tag_scheme_change(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    def fake_run(*_args: Any, **_kwargs: Any) -> subprocess.CompletedProcess[str]:
        return subprocess.CompletedProcess(args=[], returncode=0, stdout=LS_REMOTE + "\n")

    monkeypatch.setattr(fake_hermes_kit.subprocess, "run", fake_run)
    newest = fake_hermes_kit.newest_release(load_pin(PIN_FILE))
    assert newest.tag == "v0.22.5"
