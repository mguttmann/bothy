# SPDX-License-Identifier: Apache-2.0
"""The pin follows upstream main (D-HERMES-PIN-1) and the daily proposal (D-HERMES-PIN-2)."""

from __future__ import annotations

import datetime
import json
import os
import re
import subprocess
from pathlib import Path

import follow_main
import pytest
import yaml
from follow_main import (
    BOT_EMAIL,
    FOLLOW_MAIN,
    FollowMainError,
    bump_chapter_text,
    bump_pin_text,
    current_pin_sentence,
    decide,
    main_head,
)
from hermes_harness.pin import PIN_FILE, load_pin

REPO = Path(__file__).resolve().parents[4]
WORKFLOW = REPO / ".github" / "workflows" / "hermes-pin.yml"
OLD = "a8b0bc15053c1d71fb2bfea39007847c9e1c5673"
NEW = "0123456789abcdef0123456789abcdef01234567"
DAY = datetime.date(2026, 11, 8)

PIN = f"""\
# SPDX-License-Identifier: Apache-2.0
# Follows upstream main ({FOLLOW_MAIN}).
repository = "https://github.com/NousResearch/hermes-agent"
commit = "{OLD}"
python = "3.14"
install = "uv-lock"
extras = ["anthropic"]
recorded_on = 2026-11-07
"""

CHAPTER = f"""\
### 17.10 To check before building

Measured in M2-32 against Hermes Agent `v2026.9.24`.

{current_pin_sentence(OLD, datetime.date(2026, 11, 7))}

| Item | Result | Evidence | Consequence |
|---|---|---|---|
| `x` | confirmed: `{OLD[:12]}` | `a.json` | none |

## 18. Next
"""


def test_current_pin_sentence_names_the_short_commit_the_day_and_the_ruling() -> None:
    assert current_pin_sentence(NEW, DAY) == (
        f"Current pin: upstream main commit `{NEW[:12]}`, recorded on 2026-11-08 ({FOLLOW_MAIN})."
    )


def test_bump_pin_sets_commit_and_day_and_drops_a_tag() -> None:
    tagged = PIN.replace("commit =", 'tag = "v0.23.0"\ncommit =')
    for text in (PIN, tagged):
        bumped = bump_pin_text(text, NEW, DAY)
        assert f'commit = "{NEW}"' in bumped
        assert "recorded_on = 2026-11-08" in bumped
        assert "tag =" not in bumped
        assert OLD not in bumped
        assert bumped.startswith("# SPDX-License-Identifier: Apache-2.0\n")


def test_bumped_pin_file_still_loads(tmp_path: Path) -> None:
    path = tmp_path / "pin.toml"
    path.write_text(bump_pin_text(PIN_FILE.read_text(encoding="utf-8"), NEW, DAY), "utf-8")
    pin = load_pin(path)
    assert (pin.tag, pin.commit, pin.recorded_on) == (None, NEW, DAY)


@pytest.mark.parametrize(
    "text",
    [
        pytest.param(PIN.replace(f'commit = "{OLD}"\n', ""), id="no-commit"),
        pytest.param(PIN.replace("recorded_on = 2026-11-07\n", ""), id="no-day"),
        pytest.param(PIN + f'commit = "{OLD}"\n', id="two-commits"),
    ],
)
def test_bump_pin_refuses_a_file_it_cannot_rewrite_exactly(text: str) -> None:
    with pytest.raises(FollowMainError):
        bump_pin_text(text, NEW, DAY)


def test_bump_pin_refuses_a_short_commit() -> None:
    with pytest.raises(FollowMainError):
        bump_pin_text(PIN, NEW[:12], DAY)


def test_bump_chapter_rewrites_only_the_current_pin_sentence() -> None:
    bumped = bump_chapter_text(CHAPTER, NEW, DAY)
    assert current_pin_sentence(NEW, DAY) in bumped
    # History and table rows keep the commit they name.
    assert f"confirmed: `{OLD[:12]}`" in bumped
    assert bumped.count("Current pin:") == 1


@pytest.mark.parametrize(
    "text",
    [
        pytest.param(CHAPTER.replace("Current pin:", "Pin:"), id="missing"),
        pytest.param(
            CHAPTER.replace("| Item", current_pin_sentence(OLD, DAY) + "\n\n| Item"), id="twice"
        ),
        pytest.param(CHAPTER.replace("### 17.10", "### 17.9"), id="no-section"),
    ],
)
def test_bump_chapter_refuses_a_chapter_without_exactly_one_sentence(text: str) -> None:
    with pytest.raises(FollowMainError):
        bump_chapter_text(text, NEW, DAY)


def test_the_real_chapter_carries_the_sentence_of_the_real_pin() -> None:
    pin = load_pin(PIN_FILE)
    chapter = (REPO / "docs" / "spec" / "09-agents.md").read_text(encoding="utf-8")
    assert bump_chapter_text(chapter, pin.commit, pin.recorded_on) == chapter


def _fake_ls_remote(monkeypatch: pytest.MonkeyPatch, out: str, code: int = 0) -> list[list[str]]:
    calls: list[list[str]] = []

    def run(args: list[str], **_kw: object) -> subprocess.CompletedProcess[str]:
        calls.append(args)
        return subprocess.CompletedProcess(args, code, out, "boom")

    monkeypatch.setattr(follow_main.subprocess, "run", run)
    return calls


def test_main_head_reads_refs_heads_main(monkeypatch: pytest.MonkeyPatch) -> None:
    calls = _fake_ls_remote(monkeypatch, f"{NEW}\trefs/heads/main\n")
    assert main_head("https://example.invalid/hermes") == NEW
    assert calls[0][-2:] == ["https://example.invalid/hermes", "refs/heads/main"]


@pytest.mark.parametrize(
    ("out", "code"),
    [
        pytest.param("", 0, id="no-branch"),
        pytest.param(f"{NEW[:12]}\trefs/heads/main\n", 0, id="short"),
        pytest.param(f"{NEW}\trefs/heads/main-old\n", 0, id="other-ref"),
        pytest.param(f"{NEW}\trefs/heads/main\n{OLD}\trefs/heads/main\n", 0, id="two"),
        pytest.param(f"{NEW}\trefs/heads/main\n", 2, id="git-failed"),
    ],
)
def test_main_head_fails_closed(monkeypatch: pytest.MonkeyPatch, out: str, code: int) -> None:
    _fake_ls_remote(monkeypatch, out, code)
    with pytest.raises(FollowMainError):
        main_head("https://example.invalid/hermes")


@pytest.mark.parametrize(
    ("head", "proposal", "authors", "propose", "reason"),
    [
        pytest.param(OLD, None, [], False, "already", id="main-is-newest"),
        pytest.param(NEW, None, [], True, "", id="no-proposal"),
        pytest.param(NEW, OLD, [BOT_EMAIL], True, "", id="older-bot-proposal"),
        pytest.param(NEW, NEW, [BOT_EMAIL], False, "proposal", id="proposal-is-newest"),
        pytest.param(NEW, OLD, [BOT_EMAIL, "a@example.org"], False, "maintainer", id="hands-on"),
    ],
)
def test_decide_proposes_only_a_newer_commit_and_keeps_maintainer_work(
    head: str, proposal: str | None, authors: list[str], propose: bool, reason: str
) -> None:
    decision = decide(head=head, pinned=OLD, proposal=proposal, proposal_authors=authors)
    assert decision.propose is propose
    assert reason in decision.reason


def _git(cwd: Path, *args: str) -> str:
    env = {
        "PATH": os.environ["PATH"],
        "HOME": str(cwd),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": os.devnull,
    }
    return subprocess.run(
        ["git", "-c", "commit.gpgsign=false", *args],
        cwd=cwd,
        check=True,
        capture_output=True,
        text=True,
        env=env,
    ).stdout.strip()


def _repo(tmp_path: Path, upstream: Path) -> Path:
    root = tmp_path / "repo"
    pin = root / follow_main.PIN_PATH
    chapter = root / follow_main.CHAPTER_PATH
    pin.parent.mkdir(parents=True)
    chapter.parent.mkdir(parents=True)
    pin.write_text(PIN.replace("https://github.com/NousResearch/hermes-agent", upstream.as_uri()))
    chapter.write_text(CHAPTER)
    _git(root, "init", "-q", "-b", "main")
    _git(root, "add", ".")
    ident = ["-c", "user.name=Dev", "-c", "user.email=dev@example.org"]
    _git(root, *ident, "commit", "-q", "-m", "init")
    return root


def _upstream(tmp_path: Path) -> tuple[Path, str]:
    up = tmp_path / "up"
    up.mkdir()
    _git(up, "init", "-q", "-b", "main")
    ident = ["-c", "user.name=Up", "-c", "user.email=up@example.org"]
    _git(up, *ident, "commit", "-q", "--allow-empty", "-m", "one")
    return up, _git(up, "rev-parse", "HEAD")


def _run_cli(root: Path, tmp_path: Path, *extra: str) -> dict[str, str]:
    output = tmp_path / "out.txt"
    output.write_text("")
    code = follow_main.main(
        [
            "bump",
            "--root",
            str(root),
            "--today",
            "2026-11-08",
            "--github-output",
            str(output),
            *list(extra),
        ]
    )
    assert code == 0
    return dict(line.split("=", 1) for line in output.read_text().splitlines())


def test_cli_bumps_pin_and_chapter_to_the_upstream_main_head(tmp_path: Path) -> None:
    up, head = _upstream(tmp_path)
    root = _repo(tmp_path, up)
    out = _run_cli(root, tmp_path)
    assert out == {"propose": "true", "commit": head, "short": head[:12], "reason": out["reason"]}
    pin = (root / follow_main.PIN_PATH).read_text()
    assert f'commit = "{head}"' in pin
    assert "recorded_on = 2026-11-08" in pin
    chapter = (root / follow_main.CHAPTER_PATH).read_text()
    assert current_pin_sentence(head, DAY) in chapter


def test_cli_changes_nothing_when_main_is_already_pinned(tmp_path: Path) -> None:
    up, _head = _upstream(tmp_path)
    root = _repo(tmp_path, up)
    before = (root / follow_main.PIN_PATH).read_text()
    out = _run_cli(root, tmp_path, "--commit", OLD)
    assert out["propose"] == "false"
    assert (root / follow_main.PIN_PATH).read_text() == before


def test_cli_reads_pin_and_authors_of_the_open_proposal(tmp_path: Path) -> None:
    up, head = _upstream(tmp_path)
    root = _repo(tmp_path, up)
    # An open proposal of the bot that already names the head: nothing to do.
    _git(root, "checkout", "-q", "-b", "proposal")
    pin = root / follow_main.PIN_PATH
    pin.write_text(bump_pin_text(pin.read_text(), head, DAY))
    bot = ["-c", "user.name=github-actions[bot]", "-c", f"user.email={BOT_EMAIL}"]
    _git(root, *bot, "commit", "-q", "-am", "bump")
    _git(root, "checkout", "-q", "main")
    out = _run_cli(root, tmp_path, "--proposal", "proposal")
    assert (out["propose"], "proposal" in out["reason"]) == ("false", True)
    # A maintainer's commit on the proposal: never replaced.
    _git(root, "checkout", "-q", "proposal")
    (root / "note.txt").write_text("fix")
    _git(root, "add", "note.txt")
    _git(root, "-c", "user.name=Dev", "-c", "user.email=dev@example.org", "commit", "-qm", "fix")
    _git(root, "checkout", "-q", "main")
    out = _run_cli(root, tmp_path, "--proposal", "proposal", "--commit", NEW)
    assert (out["propose"], "maintainer" in out["reason"]) == ("false", True)
    assert OLD in (root / follow_main.PIN_PATH).read_text()


def test_workflow_proposes_daily_on_a_free_runner_and_runs_the_recording_and_tests() -> None:
    text = WORKFLOW.read_text(encoding="utf-8")
    flow = yaml.safe_load(text)
    triggers = flow[True]  # YAML 1.1 reads the key "on" as True
    [cron] = [entry["cron"] for entry in triggers["schedule"]]
    minute, hour, day, month, weekday = cron.split()
    assert (day, month, weekday) == ("*", "*", "*")  # once per day
    assert minute.isdigit()
    assert hour.isdigit()
    assert "workflow_dispatch" in triggers
    assert flow["permissions"] == {"contents": "read"}
    [job] = flow["jobs"].values()
    assert job["runs-on"] == "ubuntu-24.04"
    assert "actions" not in job["permissions"]
    runs = "\n".join(str(step.get("run", "")) for step in job["steps"])
    runs = re.sub(r"\\\n\s*", "", runs)  # join continued shell lines
    assert "follow_main.py bump" in runs
    assert "make -C plugins/hermes-adapter record" in runs
    assert "make -C plugins/hermes-adapter test" in runs
    assert follow_main.BRANCH in text
    assert "--force-with-lease" in runs
    assert re.search(r"gh pr (create|edit)", runs)


def test_renovate_leaves_the_pin_to_the_daily_workflow() -> None:
    renovate = (REPO / "renovate.json").read_text(encoding="utf-8")
    assert "hermes_pin" not in renovate
    assert "hermes-agent" not in json.dumps(json.loads(renovate))
