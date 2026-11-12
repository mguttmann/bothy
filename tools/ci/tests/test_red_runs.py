# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/ci/red_runs.py: the failed runs that no pull request shows get an issue.

The runs and jobs of the tests follow what the GitHub API answered for runs of this repository
on 2026-11-12, in the projection the tool asks for (`RUN_FIELDS`, `JOB_FIELDS`): the nightly
mobile E2E run 43140974784, the platform run 42556146760 of the host service, the docs run
42343870078 whose deploy job hit its time limit, and the run 43190542851 of `ci.yml` on `main`,
which GitHub lists as cancelled although a job failed. Ids, names, events and conclusions are
the recorded ones; the lists of steps are shortened and the addresses are example ones. The
watched workflows are read from `.github/workflows` itself.
"""

from __future__ import annotations

import datetime as dt
import json
from collections.abc import Sequence
from pathlib import Path
from typing import Any

import check_runners as cr
import flakes
import pytest
import red_runs
import yaml

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "red-runs.yml"
NOW = "2026-11-12T12:00:00Z"

Job = dict[str, Any]


def _job(name: str, conclusion: str, number: int, steps: Sequence[tuple[str, str]]) -> Job:
    return {
        "name": name,
        "conclusion": conclusion,
        "html_url": f"https://github.com/o/r/actions/runs/1/job/{number}",
        "steps": [{"name": n, "conclusion": c} for n, c in steps],
    }


def _run(run: int, conclusion: str, updated: str, event: str = "schedule") -> dict[str, Any]:
    return {
        "id": run,
        "html_url": f"https://github.com/o/r/actions/runs/{run}",
        "event": event,
        "head_branch": "main",
        "head_sha": "e0ad8a6a33bc9cde0060bf86b5cc6b054136c079",
        "conclusion": conclusion,
        "updated_at": updated,
    }


MOBILE = _run(43140974784, "failure", "2026-11-12T02:53:50Z")
MOBILE_JOBS = [
    _job(
        "mobile E2E (Android API 36)",
        "failure",
        130927582023,
        [
            ("Set up job", "success"),
            ("Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1", "success"),
            ("Enable hardware acceleration (kvm)", "success"),
            ("make image (host platform)", "success"),
            ("Install the emulator packages", "success"),
            ("make e2e-mobile (Android)", "failure"),
            ("Run actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9", "success"),
            ("Post Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e", "skipped"),
            ("Complete job", "success"),
        ],
    )
]
PLATFORM = _run(42556146760, "failure", "2026-11-09T04:26:22Z")
PLATFORM_JOBS = [
    _job(
        f"platform suite ({system})",
        "failure",
        number,
        [
            ("Set up job", "success"),
            ("Unit tests (Linux and macOS, runner user)", "skipped" if windows else "success"),
            ("Unit tests (Windows, runner user)", "success" if windows else "skipped"),
            ("Platform suite (Linux and macOS, as root)", "skipped" if windows else "failure"),
            ("Platform suite (Windows, elevated)", "failure" if windows else "skipped"),
            ("Keep the result file", "success"),
            ("Complete job", "success"),
        ],
    )
    for system, number, windows in [
        ("ubuntu-24.04", 128962173623, False),
        ("macos-15", 128962173721, False),
        ("windows-2025", 128962173821, True),
    ]
]
# The deploy job ran into its timeout-minutes: GitHub cancels the job and lists the run as
# cancelled, not as failed.
DOCS = _run(42343870078, "cancelled", "2026-11-07T05:59:35Z")
DOCS_JOBS = [
    _job("build site", "success", 1, [("Set up job", "success"), ("Complete job", "success")]),
    _job(
        "deploy to Pages",
        "cancelled",
        2,
        [
            ("Set up job", "success"),
            ("Run actions/deploy-pages@368f82528645a54fb793d4d04e342629a3f51346", "cancelled"),
        ],
    ),
]
# A push to main: one job hit its time limit, ci-ok failed, the run counts as cancelled.
CI = _run(43190542851, "cancelled", "2026-11-12T11:27:19Z", event="push")
CI_JOBS = [
    _job("plan", "success", 1, [("Set up job", "success"), ("Complete job", "success")]),
    _job("desktop trial (macOS)", "skipped", 2, []),
    _job(
        "apps (macOS)",
        "cancelled",
        131092489935,
        [("Set up job", "success"), ("make apps-platform", "cancelled")],
    ),
    _job(
        "ci-ok",
        "failure",
        131151729685,
        [("Set up job", "success"), ("Every job the plan needs passed", "failure")],
    ),
]


class FakeGh:
    """The gh calls of `report` against an in-memory repository."""

    def __init__(self) -> None:
        # The completed runs of each workflow file, and the jobs of each run.
        self.runs: dict[str, list[dict[str, Any]]] = {}
        self.jobs: dict[int, list[Job]] = {}
        self.issues: list[dict[str, Any]] = []
        # (issue, author, body, created_at) of every comment.
        self.comments: list[tuple[int, str, str, str]] = []
        self.labels: list[list[str]] = []
        self.closed: list[tuple[int, str]] = []
        self.calls: list[list[str]] = []

    def add(self, file: str, run: dict[str, Any], jobs: Sequence[Job] = ()) -> None:
        """A completed run of the workflow `file` with its jobs."""
        self.runs.setdefault(file, []).append(run)
        self.jobs[int(run["id"])] = list(jobs)

    def bodies(self, number: int) -> list[str]:
        """The comments of the issue `number`."""
        return [body for issue, _, body, _ in self.comments if issue == number]

    def __call__(self, args: Sequence[str]) -> str:
        args = list(args)
        self.calls.append(args)
        if args[0] == "api" and "/actions/workflows/" in args[-3]:
            path, _, query = args[-3].partition("?")
            params = dict(p.split("=", 1) for p in query.split("&"))
            assert params["status"] == "completed"
            assert args[-2:] == ["--jq", f".workflow_runs[] | {red_runs.RUN_FIELDS}"]
            # The window of the API counts from when a run was created; the runs of the tests
            # have one time for both.
            found = [
                r
                for r in self.runs.get(path.split("/")[-2], [])
                if r["event"] == params["event"]
                and r["updated_at"] >= params.get("created", ">=").removeprefix(">=")
            ]
            found.sort(key=lambda r: -int(r["id"]))
            return "".join(json.dumps(r) + "\n" for r in found[: int(params["per_page"])])
        if args[0] == "api" and args[-3].split("?")[0].endswith("/jobs"):
            assert args[-2:] == ["--jq", f".jobs[] | {red_runs.JOB_FIELDS}"]
            run = int(args[-3].split("/runs/")[1].split("/")[0])
            return "".join(json.dumps(j) + "\n" for j in self.jobs[run])
        if args[0] == "api" and args[-3].endswith("/comments"):
            number = int(args[-3].split("/issues/")[1].split("/")[0])
            return "".join(
                json.dumps({"body": body, "user": user, "created_at": at}) + "\n"
                for issue, user, body, at in self.comments
                if issue == number
            )
        if args[:2] == ["issue", "list"]:
            assert args[args.index("--label") + 1] == red_runs.LABEL
            return json.dumps(self.issues)
        if args[:2] == ["label", "create"]:
            self.labels.append(args)
            return ""
        if args[:2] == ["issue", "create"]:
            number = 7 + len(self.issues)
            self.issues.append(
                {
                    "number": number,
                    "title": args[args.index("--title") + 1],
                    "state": "OPEN",
                    "body": args[args.index("--body") + 1],
                }
            )
            return f"https://github.com/o/r/issues/{number}\n"
        if args[:2] == ["issue", "comment"]:
            self.comments.append((int(args[2]), flakes.BOT, args[args.index("--body") + 1], NOW))
            return ""
        if args[:2] == ["issue", "edit"]:
            [issue] = [i for i in self.issues if i["number"] == int(args[2])]
            issue["body"] = args[args.index("--body") + 1]
            return ""
        if args[:2] == ["issue", "close"]:
            [issue] = [i for i in self.issues if i["number"] == int(args[2])]
            issue["state"] = "CLOSED"
            self.closed.append((int(args[2]), args[args.index("--comment") + 1]))
            return ""
        raise AssertionError(args)


def _time(text: str) -> dt.datetime:
    return dt.datetime.fromisoformat(text.replace("Z", "+00:00"))


def _report(gh: FakeGh, now: str = NOW, hours: int = 48, dry_run: bool = False) -> int:
    return red_runs.report("o/r", hours, gh, _time(now), dry_run=dry_run)


def _writes(gh: FakeGh) -> list[list[str]]:
    return [c for c in gh.calls if c[0] != "api" and c[:2] != ["issue", "list"]]


# --- which runs are watched --------------------------------------------------------------------


def test_every_scheduled_workflow_and_the_push_to_main_of_ci_is_watched() -> None:
    watched = red_runs.watched()
    scheduled = set()
    for path in sorted((REPO / ".github" / "workflows").glob("*.y*ml")):
        doc = cr.load_yaml(path.read_text(encoding="utf-8"))
        if "schedule" in cr._triggers(doc):
            scheduled.add(path.name)
    assert {file for file, events in watched.items() if "schedule" in events} == scheduled
    # The workflows of the audit findings, and the watcher of the watchers itself.
    assert {"host-platform.yml", "e2e-mobile.yml", "e2e-desktop.yml", "red-runs.yml"} <= scheduled
    # The full suite of a push to main is the only run of a change after its merge.
    assert watched["ci.yml"] == ("push",)
    assert {f for f, events in watched.items() if "push" in events} == set(cr.MAIN_PUSH_WORKFLOWS)
    # denylist-main.yml opens its own issue with what its scan found (job `alert`).
    assert "denylist-main.yml" not in watched


def test_a_run_of_a_pull_request_or_started_by_hand_is_not_read() -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", {**MOBILE, "event": "workflow_dispatch"}, MOBILE_JOBS)
    gh.add("ci.yml", {**CI, "event": "pull_request"}, CI_JOBS)
    assert _report(gh) == 0
    assert gh.issues == []
    events = {
        dict(p.split("=", 1) for p in c[-3].partition("?")[2].split("&"))["event"]
        for c in gh.calls
        if c[0] == "api" and "/actions/workflows/" in c[-3]
    }
    assert events == {"schedule", "push"}


# --- a failed run opens an issue ---------------------------------------------------------------


def test_a_failed_scheduled_run_opens_an_issue_that_names_its_job_and_step(
    capsys: pytest.CaptureFixture[str],
) -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    assert _report(gh) == 0
    [issue] = gh.issues
    assert issue["title"] == "Red runs of e2e-mobile.yml"
    [comment] = gh.bodies(issue["number"])
    assert "Run [43140974784](https://github.com/o/r/actions/runs/43140974784)" in comment
    assert "(schedule, `main`, e0ad8a6a33bc): failure" in comment
    assert (
        "| [`mobile E2E (Android API 36)`](https://github.com/o/r/actions/runs/1/job/130927582023)"
        " | failure | `make e2e-mobile (Android)` |"
    ) in comment
    # Only what did not pass is named: no step that passed or was skipped.
    assert "Set up job" not in comment
    assert "Post Run" not in comment
    assert "| 1 | 43140974784 | 2026-11-12 | `mobile E2E (Android API 36)` |" in issue["body"]
    assert "red-runs: 1 failed run(s) added to issue #7 (e2e-mobile.yml)" in capsys.readouterr().out


def test_the_label_of_the_issue_comes_from_the_label_file() -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    assert _report(gh) == 0
    labels = yaml.safe_load((REPO / ".github" / "labels.yml").read_text(encoding="utf-8"))
    [entry] = [item for item in labels if item["name"] == red_runs.LABEL]
    [created] = gh.labels
    assert created[2] == red_runs.LABEL
    assert created[created.index("--color") + 1] == entry["color"]
    assert created[created.index("--description") + 1] == entry["description"]
    [issue] = [c for c in gh.calls if c[:2] == ["issue", "create"]]
    assert issue[issue.index("--label") + 1] == red_runs.LABEL


def test_every_failed_job_of_a_matrix_is_named() -> None:
    gh = FakeGh()
    gh.add("host-platform.yml", PLATFORM, PLATFORM_JOBS)
    assert _report(gh, now="2026-11-09T05:00:00Z") == 0
    [issue] = gh.issues
    [comment] = gh.bodies(issue["number"])
    for system, step in [
        ("ubuntu-24.04", "Platform suite (Linux and macOS, as root)"),
        ("macos-15", "Platform suite (Linux and macOS, as root)"),
        ("windows-2025", "Platform suite (Windows, elevated)"),
    ]:
        rows = [line for line in comment.splitlines() if line.startswith("| ")]
        [row] = [line for line in rows if f"platform suite ({system})" in line]
        assert row.endswith(f"| failure | `{step}` |")
        assert f"| 1 | 42556146760 | 2026-11-09 | `platform suite ({system})` |" in issue["body"]


def test_a_cancelled_run_is_red_when_one_of_its_jobs_ran_and_did_not_pass() -> None:
    gh = FakeGh()
    gh.add("docs.yml", DOCS, DOCS_JOBS)
    gh.add("ci.yml", CI, CI_JOBS)
    assert _report(gh, now="2026-11-07T07:00:00Z", hours=24 * 7) == 0
    by_title = {i["title"]: i for i in gh.issues}
    [docs] = gh.bodies(by_title["Red runs of docs.yml"]["number"])
    assert "): cancelled" in docs
    assert (
        "| cancelled | `Run actions/deploy-pages@368f82528645a54fb793d4d04e342629a3f51346` |"
        in docs
    )
    assert "build site" not in docs
    [ci] = gh.bodies(by_title["Red runs of ci.yml"]["number"])
    assert "(push, `main`, " in ci
    assert "`apps (macOS)`" in ci
    assert "| failure | `Every job the plan needs passed` |" in ci
    assert "`plan`" not in ci
    assert "desktop trial" not in ci


def test_a_run_cancelled_before_a_job_ran_is_not_red() -> None:
    gh = FakeGh()
    # A newer push took the place of a waiting run: no job, or jobs that never started.
    gh.add("ci.yml", _run(50, "cancelled", "2026-11-12T10:00:00Z", event="push"), [])
    gh.add(
        "ci.yml",
        _run(51, "cancelled", "2026-11-12T10:05:00Z", event="push"),
        [_job("gates", "cancelled", 1, []), _job("plan", "skipped", 2, [])],
    )
    assert _report(gh) == 0
    assert gh.issues == []
    assert _writes(gh) == []


def test_a_run_that_failed_before_any_job_started_is_red() -> None:
    gh = FakeGh()
    gh.add("advisories.yml", _run(60, "startup_failure", "2026-11-12T05:18:00Z"), [])
    assert _report(gh) == 0
    [issue] = gh.issues
    [comment] = gh.bodies(issue["number"])
    assert "): startup_failure" in comment
    assert "| 1 | 60 | 2026-11-12 | (no job ran) |" in issue["body"]


def test_a_workflow_whose_runs_passed_gets_no_issue(capsys: pytest.CaptureFixture[str]) -> None:
    gh = FakeGh()
    gh.add("e2e-desktop.yml", _run(70, "success", "2026-11-10T05:10:00Z"))
    gh.add("codeql.yml", _run(71, "skipped", "2026-11-10T05:10:00Z"))
    assert _report(gh, now="2026-11-10T06:00:00Z") == 0
    assert gh.issues == []
    assert _writes(gh) == []
    # No job list is read for a run that passed.
    assert not [c for c in gh.calls if c[0] == "api" and "/jobs" in c[-3]]
    assert "red-runs: no new failed run" in capsys.readouterr().out


# --- the issue over time -----------------------------------------------------------------------


def test_a_reported_run_is_not_reported_again_and_the_next_one_counts_up() -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    assert _report(gh) == 0
    assert _report(gh) == 0
    [issue] = gh.issues
    assert len(gh.bodies(issue["number"])) == 1
    # The next night fails too: the same issue, a second comment, the row counts two runs.
    gh.add("e2e-mobile.yml", _run(43300000000, "failure", "2026-11-13T02:55:00Z"), MOBILE_JOBS)
    assert _report(gh, now="2026-11-13T03:11:00Z") == 0
    assert len(gh.issues) == 1
    assert len(gh.bodies(issue["number"])) == 2
    assert "| 2 | 43300000000 | 2026-11-13 | `mobile E2E (Android API 36)` |" in issue["body"]
    assert len([c for c in gh.calls if c[:2] == ["issue", "create"]]) == 1


def test_the_most_frequent_job_heads_the_list() -> None:
    gh = FakeGh()
    ios = _job("apps (iOS)", "failure", 3, [("make apps-platform (iOS)", "failure")])
    gh.add("ci.yml", _run(80, "failure", "2026-11-12T08:00:00Z", event="push"), [ios])
    gh.add("ci.yml", _run(81, "failure", "2026-11-12T09:00:00Z", event="push"), [ios, CI_JOBS[3]])
    gh.add("ci.yml", CI, CI_JOBS)
    assert _report(gh) == 0
    [issue] = gh.issues
    rows = [line for line in issue["body"].splitlines() if line.startswith("| ") and "`" in line]
    assert rows == [
        "| 2 | 43190542851 | 2026-11-12 | `ci-ok` |",
        "| 2 | 81 | 2026-11-12 | `apps (iOS)` |",
        "| 1 | 43190542851 | 2026-11-12 | `apps (macOS)` |",
    ]


def test_the_issue_stays_open_while_the_newest_run_is_red_or_a_run_failed_in_the_last_week() -> (
    None
):
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    assert _report(gh) == 0
    # Ten days later no run of the window failed, but the newest run is still the red one.
    assert _report(gh, now="2026-11-22T12:00:00Z") == 0
    assert gh.issues[0]["state"] == "OPEN"
    # The next night passes: the last failed run is one day old.
    gh.add("e2e-mobile.yml", _run(43400000000, "success", "2026-11-13T02:50:00Z"))
    assert _report(gh, now="2026-11-13T03:11:00Z") == 0
    assert gh.issues[0]["state"] == "OPEN"
    assert gh.closed == []


def test_the_issue_closes_when_the_newest_run_passed_and_none_failed_for_seven_days() -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    assert _report(gh) == 0
    gh.add("e2e-mobile.yml", _run(43900000000, "success", "2026-11-19T02:40:00Z"))
    assert _report(gh, now="2026-11-19T02:50:00Z") == 0
    assert gh.issues[0]["state"] == "OPEN", "a few minutes short of seven days"
    assert _report(gh, now="2026-11-19T03:11:00Z") == 0
    assert gh.issues[0]["state"] == "CLOSED"
    [(number, note)] = gh.closed
    assert number == gh.issues[0]["number"]
    assert "43900000000" in note
    assert "7 days" in note


def test_a_closed_issue_keeps_its_runs_reported_and_the_next_failed_run_opens_a_new_one() -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    assert _report(gh) == 0
    # A maintainer closes the issue by hand while its run is still in the window.
    gh.issues[0]["state"] = "CLOSED"
    assert _report(gh) == 0
    assert len(gh.issues) == 1
    assert len(gh.comments) == 1
    gh.add("e2e-mobile.yml", _run(43300000000, "failure", "2026-11-13T02:55:00Z"), MOBILE_JOBS)
    assert _report(gh, now="2026-11-13T03:11:00Z") == 0
    assert [i["state"] for i in gh.issues] == ["CLOSED", "OPEN"]
    # The new issue lists its own run only.
    assert "| 1 | 43300000000 | 2026-11-13 |" in gh.issues[1]["body"]
    assert "43140974784" not in gh.issues[1]["body"]


def test_each_workflow_has_its_own_issue() -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    gh.add("host-platform.yml", PLATFORM, PLATFORM_JOBS)
    assert _report(gh, hours=24 * 7) == 0
    assert sorted(i["title"] for i in gh.issues) == [
        "Red runs of e2e-mobile.yml",
        "Red runs of host-platform.yml",
    ]
    for issue in gh.issues:
        assert len(gh.bodies(issue["number"])) == 1


def test_a_run_counts_from_its_end_or_from_its_comment() -> None:
    old = red_runs.marker(1, ["gates"], "2026-11-01T10:00:00Z")
    undated = '<!-- red-runs run=2 {"jobs": ["gates"]} -->'
    found = red_runs.parse_comments([(old, NOW), (undated, NOW), (undated.replace("=2", "=3"), "")])
    assert found[1].seen == _time("2026-11-01T10:00:00Z")
    assert found[2].seen == _time(NOW)
    assert 3 not in found
    # A run without an end time counts from the moment of the report.
    text = red_runs.comment({**MOBILE, "updated_at": None}, [], _time(NOW))
    assert red_runs.parse_comments([(text, "")])[43140974784].seen == _time(NOW)


def test_an_issue_of_a_workflow_without_any_run_stays_open() -> None:
    gh = FakeGh()
    gh.issues.append(
        {"number": 7, "title": "Red runs of e2e-mobile.yml", "state": "OPEN", "body": ""}
    )
    gh.comments.append((7, flakes.BOT, red_runs.marker(1, ["gates"], "2026-10-01T10:00:00Z"), NOW))
    assert _report(gh) == 0
    assert gh.issues[0]["state"] == "OPEN"


# --- what the issue may contain ----------------------------------------------------------------


def test_only_the_comments_of_the_workflow_count() -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    gh.issues.append(
        {"number": 7, "title": "Red runs of e2e-mobile.yml", "state": "OPEN", "body": ""}
    )
    # Anyone can comment on a public issue: a forged hidden line hides no run and adds no row.
    forged = red_runs.marker(43140974784, ["forged job"], "2026-11-12T02:53:50Z")
    gh.comments.append((7, "someone", forged, NOW))
    # A hidden line of the workflow that cannot be read reports nothing either.
    gh.comments.append((7, flakes.BOT, "<!-- red-runs run=5 {broken} -->", NOW))
    gh.comments.append((7, flakes.BOT, '<!-- red-runs run=6 {"jobs": "gates"} -->', NOW))
    assert _report(gh) == 0
    assert len(gh.bodies(7)) == 4
    assert "forged job" not in gh.issues[0]["body"]
    assert "| 5 |" not in gh.issues[0]["body"]
    assert "| 6 |" not in gh.issues[0]["body"]
    assert (
        "| 1 | 43140974784 | 2026-11-12 | `mobile E2E (Android API 36)` |" in gh.issues[0]["body"]
    )


def test_a_name_cannot_leave_its_cell_or_the_hidden_line() -> None:
    gh = FakeGh()
    odd = _job("a | b `c` <!-- x --> @team", "failure", 9, [("s1 | <b>", "failure")])
    gh.add("e2e-mobile.yml", MOBILE, [odd])
    assert _report(gh) == 0
    [comment] = gh.bodies(7)
    assert "`a \\| b 'c' \u2039!-- x --\u203a @team`" in comment
    assert "`s1 \\| \u2039b\u203a`" in comment
    assert comment.count("-->") == 1
    assert red_runs.parse_comments([(comment, NOW)])[43140974784].jobs == (odd["name"],)


def test_a_job_link_outside_github_is_not_linked() -> None:
    gh = FakeGh()
    job = {**MOBILE_JOBS[0], "html_url": "https://example.com/x) [y](https://example.com"}
    gh.add("e2e-mobile.yml", MOBILE, [job])
    assert _report(gh) == 0
    [comment] = gh.bodies(7)
    assert "example.com" not in comment
    assert "| `mobile E2E (Android API 36)` | failure |" in comment


# --- a dry run ---------------------------------------------------------------------------------


def test_a_dry_run_reads_the_runs_and_writes_nothing(capsys: pytest.CaptureFixture[str]) -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", MOBILE, MOBILE_JOBS)
    gh.add("host-platform.yml", PLATFORM, PLATFORM_JOBS)
    assert _report(gh, hours=24 * 7, dry_run=True) == 0
    assert _writes(gh) == []
    out = capsys.readouterr().out
    assert "dry run: gh issue create -R o/r --title 'Red runs of e2e-mobile.yml'" in out
    assert "dry run: gh issue create -R o/r --title 'Red runs of host-platform.yml'" in out
    assert "platform suite (windows-2025)" in out


# --- the command -------------------------------------------------------------------------------


def test_the_command_reports_a_failed_gh_call(
    monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    def broken(args: Sequence[str]) -> str:
        raise ValueError("gh api repos/o/r failed: HTTP 403")

    monkeypatch.setattr(flakes, "_gh", broken)
    assert red_runs.main(["report", "--repo", "o/r"]) == 1
    assert "red-runs: gh api repos/o/r failed: HTTP 403" in capsys.readouterr().err


def test_the_command_runs_the_report_with_its_window(monkeypatch: pytest.MonkeyPatch) -> None:
    gh = FakeGh()
    gh.add("e2e-mobile.yml", {**MOBILE, "updated_at": "2999-01-01T00:00:00Z"}, MOBILE_JOBS)
    monkeypatch.setattr(flakes, "_gh", gh)
    assert red_runs.main(["report", "--repo", "o/r", "--hours", "1", "--dry-run"]) == 0
    assert _writes(gh) == []
    assert any("created=>=" in c[-3] for c in gh.calls if c[0] == "api")


# --- the workflow ------------------------------------------------------------------------------


def _workflow() -> dict[str, Any]:
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    assert isinstance(doc, dict)
    return doc


def test_the_workflow_runs_every_hour_and_by_hand() -> None:
    triggers = cr._triggers(_workflow())
    assert set(triggers) == {"schedule", "workflow_dispatch"}
    [entry] = triggers["schedule"]
    minute, hour, dom, month, dow = entry["cron"].split()
    assert minute.isdigit()
    assert (hour, dom, month, dow) == ("*", "*", "*", "*")


def test_the_workflow_may_read_runs_and_write_issues_only() -> None:
    doc = _workflow()
    assert doc["permissions"] == {"contents": "read"}
    (job,) = doc["jobs"].values()
    assert job["permissions"] == {"contents": "read", "actions": "read", "issues": "write"}
    text = WORKFLOW.read_text(encoding="utf-8")
    assert "secrets." not in text
    assert cr.check_workflow(WORKFLOW.name, text) == []


def test_the_workflow_runs_the_report_for_its_repository() -> None:
    (job,) = _workflow()["jobs"].values()
    [step] = [s for s in job["steps"] if "red_runs.py" in str(s.get("run", ""))]
    assert "red_runs.py report --repo" in step["run"]
    assert "--dry-run" not in step["run"]
    assert step["env"]["GH_TOKEN"] == "${{ github.token }}"
