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


def _newest(monkeypatch: pytest.MonkeyPatch, refs: list[str]) -> str | None:
    lines = [f"{index:040x}\trefs/tags/{ref}" for index, ref in enumerate(refs, start=1)]

    def fake_run(*_args: Any, **_kwargs: Any) -> subprocess.CompletedProcess[str]:
        return subprocess.CompletedProcess(args=[], returncode=0, stdout="\n".join(lines))

    monkeypatch.setattr(fake_hermes_kit.subprocess, "run", fake_run)
    return fake_hermes_kit.newest_release(load_pin(PIN_FILE)).tag


def test_version_tags_compare_by_number_not_by_text(monkeypatch: pytest.MonkeyPatch) -> None:
    assert _newest(monkeypatch, ["v0.22.9", "v0.22.10", "v0.21.7", "v2026.9.24"]) == "v0.22.10"


def test_calendar_tags_alone_still_resolve(monkeypatch: pytest.MonkeyPatch) -> None:
    assert _newest(monkeypatch, ["v2026.9.21", "v2026.9.24", "v2026.5.29.2"]) == "v2026.9.24"


def test_candidates_canaries_and_other_refs_are_no_release(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    refs = [
        "v0.22.4",
        "rc.1-v0.22.5",
        "abandoned-rc.2-v0.22.5",
        "v0.22.5+canary.20261029T064840Z",
        "backup/precopystrip-20260616-2058",
        "inputs-0",
    ]
    assert _newest(monkeypatch, refs) == "v0.22.4"


def test_the_peeled_commit_of_an_annotated_tag_wins(monkeypatch: pytest.MonkeyPatch) -> None:
    out = "\n".join(
        [
            "56b6a31fededdf7364e1a0a79548d144e7fd81a3\trefs/tags/v0.22.5",
            "bf0203a794a07aac3ff764972ab9087baa4e4f4e\trefs/tags/v0.22.5^{}",
        ]
    )

    def fake_run(*_args: Any, **_kwargs: Any) -> subprocess.CompletedProcess[str]:
        return subprocess.CompletedProcess(args=[], returncode=0, stdout=out)

    monkeypatch.setattr(fake_hermes_kit.subprocess, "run", fake_run)
    newest = fake_hermes_kit.newest_release(load_pin(PIN_FILE))
    assert (newest.tag, newest.commit) == ("v0.22.5", "bf0203a794a07aac3ff764972ab9087baa4e4f4e")


def test_renovate_never_proposes_a_date_tag_after_the_scheme_change() -> None:
    # Renovate compares the tags of the pin as versions too: v2026.9.24 would outrank v0.22.5.
    import json

    renovate = json.loads((fake_hermes_kit.REPO / "renovate.json").read_text(encoding="utf-8"))
    rules = [
        rule
        for rule in renovate["packageRules"]
        if rule.get("matchFileNames") == ["plugins/hermes-adapter/tests/hermes_pin.toml"]
    ]
    assert len(rules) == 1
    assert rules[0]["allowedVersions"] == f"<{fake_hermes_kit._FIRST_CALENDAR_YEAR}"
