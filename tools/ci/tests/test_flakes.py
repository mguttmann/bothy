# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/ci/flakes.py: reruns of failed tests in CI and the flake report.

The end-to-end tests run the pinned gotestsum and the real go on a throwaway module: a test that
fails once passes with a flake report, a test that always fails, a data race and a TestMain that
always fails still fail, and the merged profile of a run with a rerun has the coverage of a run
without one.
"""

from __future__ import annotations

import json
import subprocess
import sys
from collections.abc import Sequence
from pathlib import Path
from typing import Any

import fetch_tools
import flakes
import go_shards
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
PKG = "example.com/mod/a"


def _event(action: str, test: str = "", package: str = PKG) -> str:
    return json.dumps({"Action": action, "Package": package, "Test": test})


# --- the attempts ------------------------------------------------------------------------------


def test_the_first_attempt_is_go_test_json_with_the_flags_and_packages() -> None:
    args = flakes.attempt_args(["./..."], ["-race", "-run", "^(A|B)$"], [])
    assert args == ["test", "-json", "-race", "-run", "^(A|B)$", "./..."]


@pytest.mark.parametrize(
    "flags",
    [["-race", "-run", "^(A|B)$"], ["-race", "-run=^(A|B)$"], ["-race", "-test.run=^(A|B)$"]],
)
def test_a_rerun_tests_one_package_with_its_own_run_filter(flags: list[str]) -> None:
    args = flakes.attempt_args(["./...", "x"], flags, ["-test.run=^TestA$", PKG])
    assert args == ["test", "-json", "-race", "-run=^TestA$", PKG]


def test_a_rerun_keeps_the_skip_filter_of_a_part() -> None:
    args = flakes.attempt_args([PKG], ["-skip", "^(A)$"], ["-test.run=^TestB$", PKG])
    assert args == ["test", "-json", "-skip", "^(A)$", "-run=^TestB$", PKG]


def test_a_package_that_failed_without_a_test_reruns_whole() -> None:
    # gotestsum names such a failure with an empty test name: -test.run=^$ would run no test.
    flags = ["-race", "-run", "^(A|B)$"]
    args = flakes.attempt_args(["./..."], flags, ["-test.run=^$", PKG])
    assert args == ["test", "-json", *flags, PKG]


def test_each_attempt_writes_its_own_profile(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    monkeypatch.chdir(tmp_path)
    calls: list[list[str]] = []
    monkeypatch.setattr(flakes.os, "execvp", lambda _f, a: calls.append(a))
    base = ["exec", "--packages", "./...", "--", "-race", "-coverprofile=c.out"]
    flakes.main(base)
    flakes.main([*base, "-test.run=^TestA$", PKG])
    assert calls == [
        ["go", "test", "-json", "-race", "-coverprofile=c.out.attempt1", "./..."],
        ["go", "test", "-json", "-race", "-coverprofile=c.out.attempt2", "-run=^TestA$", PKG],
    ]


def test_the_profile_is_the_merge_of_the_attempts(tmp_path: Path) -> None:
    profile = tmp_path / "c.out"
    for number, text in (
        (2, "mode: atomic\nb.go:1.1,2.2 1 1\n"),
        (10, ""),
        (1, "mode: atomic\na.go:1.1,2.2 1 0\n"),
    ):
        (tmp_path / f"c.out.attempt{number}").write_text(text, encoding="utf-8")
    flakes.merge_attempts(profile)
    # In the order of the attempts; an attempt that wrote nothing (a build error) is left out.
    assert (
        profile.read_text(encoding="utf-8") == "mode: atomic\na.go:1.1,2.2 1 0\nb.go:1.1,2.2 1 1\n"
    )
    assert sorted(p.name for p in tmp_path.iterdir()) == ["c.out"]


def test_gotestsum_reruns_twice_and_never_after_a_data_race() -> None:
    command = flakes.go_command("gts", Path("ev.json"), ["./..."], ["-race", "-v"])
    assert command[:9] == [
        "gts",
        "--format=standard-verbose",
        "--rerun-fails=2",
        "--rerun-fails-max-failures=10",
        "--rerun-fails-abort-on-data-race",
        "--rerun-fails-run-root-test",
        "--jsonfile-timing-events=ev.json",
        "--raw-command",
        "--",
    ]
    assert command[-6:] == ["exec", "--packages", "./...", "--", "-race", "-v"]
    assert "--format=standard-quiet" in flakes.go_command("gts", Path("e"), ["p"], ["-race"])
    # docs/quality/gates.md, section Flakes, names the same rerun flags.
    text = " ".join((REPO / "docs/quality/gates.md").read_text(encoding="utf-8").split())
    assert " ".join(command[2:6]) in text


# --- the flakes --------------------------------------------------------------------------------


def test_a_test_that_failed_and_then_passed_is_a_flake() -> None:
    lines = [
        _event("pass", "TestOk"),
        _event("fail", "TestFlaky"),
        _event("fail", "TestAlways"),
        _event("fail"),
        _event("pass", "TestFlaky"),
        _event("fail", "TestAlways"),
        _event("fail"),
        _event("skip", "TestSkipped"),
    ]
    assert flakes.flakes_from_events(lines) == [
        {"tool": "go", "package": PKG, "test": "TestFlaky", "runs": 2, "failures": 1}
    ]


def test_a_flaky_subtest_names_the_subtest_and_a_package_only_alone() -> None:
    other = "example.com/mod/b"
    lines = [
        _event("fail", "TestA/one"),
        _event("fail", "TestA"),
        _event("fail"),
        _event("fail", "", other),
        _event("pass", "TestA/one"),
        _event("pass", "TestA"),
        _event("pass"),
        _event("pass", "", other),
        "",
    ]
    assert [(f["package"], f["test"]) for f in flakes.flakes_from_events(lines)] == [
        (PKG, "TestA/one"),
        (other, ""),
    ]


def test_the_flakes_go_to_a_file_and_the_job_summary(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    summary = tmp_path / "summary.md"
    monkeypatch.setenv("GITHUB_STEP_SUMMARY", str(summary))
    flakes.record(tmp_path / "out", "go", [])
    assert not (tmp_path / "out").exists()
    flake = {"tool": "go", "package": PKG, "test": "TestA|`x`", "runs": 2, "failures": 1}
    flakes.record(tmp_path / "out", "go", [flake])
    [written] = (tmp_path / "out").iterdir()
    assert flakes.load_flakes([written]) == [flake]
    assert "| go | `example.com/mod/a` | `TestA\\|'x'` | 2 | 1 |" in summary.read_text(
        encoding="utf-8"
    )
    assert f"FLAKE (go): {PKG} TestA|`x`: passed after 1 failed run(s)" in capsys.readouterr().out


def test_load_flakes_skips_other_files(tmp_path: Path) -> None:
    (tmp_path / "a.json").write_text("not json", encoding="utf-8")
    (tmp_path / "b.json").write_text('{"schema": 2, "flakes": []}', encoding="utf-8")
    (tmp_path / "c.json").write_text('{"schema": 1, "flakes": [{"tool": "go"}]}', encoding="utf-8")
    assert flakes.load_flakes(tmp_path.iterdir()) == []


def test_dart_flakes_are_the_tests_that_reported_an_error_and_passed(tmp_path: Path) -> None:
    suite = REPO / "packages/domain/test/a_test.dart"
    events = [
        {"type": "suite", "suite": {"id": 0, "path": str(suite)}},
        {"type": "testStart", "test": {"id": 1, "name": "loading", "suiteID": 0}},
        {"type": "testDone", "testID": 1, "result": "success", "hidden": True},
        {"type": "testStart", "test": {"id": 3, "name": "flaky once", "suiteID": 0}},
        {"type": "error", "testID": 3, "isFailure": True},
        {"type": "print", "testID": 3, "message": "Retry: flaky once"},
        {"type": "testDone", "testID": 3, "result": "success", "hidden": False},
        {"type": "testStart", "test": {"id": 4, "name": "always fails", "suiteID": 0}},
        {"type": "error", "testID": 4, "isFailure": True},
        {"type": "error", "testID": 4, "isFailure": True},
        {"type": "testDone", "testID": 4, "result": "failure", "hidden": False},
    ]
    report = tmp_path / "r.json"
    report.write_text("\n".join(json.dumps(e) for e in events) + "\nnot json\n", encoding="utf-8")
    assert (
        flakes.main(
            ["dart", "--dir", str(tmp_path / "out"), "--package", "packages/domain", str(report)]
        )
        == 0
    )
    [written] = (tmp_path / "out").iterdir()
    [flake] = json.loads(written.read_text(encoding="utf-8"))["flakes"]
    assert flake == {
        "tool": "dart",
        "package": "packages/domain",
        "file": "packages/domain/test/a_test.dart",
        "test": "flaky once",
        "runs": 2,
        "failures": 1,
    }
    # No report (the run did not start): nothing to record.
    assert (
        flakes.main(
            ["dart", "--dir", str(tmp_path / "none"), "--package", "p", str(tmp_path / "x")]
        )
        == 0
    )
    assert not (tmp_path / "none").exists()


def test_every_dart_member_has_the_ci_preset_with_one_retry() -> None:
    # `add_presets: [ci]` of CI fails for a member without it ("Undefined preset").
    members = subprocess.run(
        ["git", "ls-files", "--", "apps/*pubspec.yaml", "packages/*pubspec.yaml"],
        cwd=REPO,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.split()
    assert members
    for pubspec in members:
        config = yaml.safe_load(
            (REPO / pubspec).with_name("dart_test.yaml").read_text(encoding="utf-8")
        )
        assert config["presets"]["ci"] == {"retry": 1}, pubspec


# --- the end-to-end runs with gotestsum and go ------------------------------------------------

_GO_CODE = """package a

func Add(x, y int) int {
	if x > 100 {
		return 0
	}
	return x + y
}
"""
_GO_TEST = """package a

import (
	"os"
	"testing"
)

var ran bool

func TestAdd(t *testing.T) {
	ran = true
	if Add(1, 2) != 3 {
		t.Fatal("add")
	}
}

// Fails in the first attempt only when FLAKY names a file that does not exist yet; it runs the
// same code in every attempt.
func TestFlaky(t *testing.T) {
	Add(1000, 1)
	if m := os.Getenv("FLAKY"); m != "" {
		if _, err := os.Stat(m); err != nil {
			_ = os.WriteFile(m, nil, 0o600)
			t.Fatal("first attempt")
		}
	}
}

func TestAlways(t *testing.T) {
	if os.Getenv("ALWAYS") != "" {
		t.Fatal("always")
	}
}

// Races in the first attempt only, like FLAKY.
func TestRace(t *testing.T) {
	m := os.Getenv("RACE")
	if m == "" {
		return
	}
	if _, err := os.Stat(m); err == nil {
		return
	}
	_ = os.WriteFile(m, nil, 0o600)
	x := 0
	done := make(chan bool)
	go func() { x++; done <- true }()
	x++
	<-done
}

func TestMain(m *testing.M) {
	code := m.Run()
	// A check after the tests, such as a leak check: it fails whenever TestAdd ran.
	if os.Getenv("MAIN_FAILS") != "" && ran {
		os.Exit(1)
	}
	os.Exit(code)
}
"""
_FLAGS = ["-count=1", "-race", "-covermode=atomic", "-coverpkg=./...", "-coverprofile=coverage.out"]


@pytest.fixture(scope="module")
def gotestsum() -> str:
    """The pinned gotestsum (fetched into the tool cache on first use)."""
    lock = fetch_tools.load_lock(fetch_tools.DEFAULT_LOCK)
    plat = fetch_tools.platform_key()
    return str(fetch_tools.ensure(lock, "gotestsum", plat, fetch_tools.DEFAULT_CACHE))


def _module(tmp_path: Path) -> Path:
    module = tmp_path / "mod"
    (module / "a").mkdir(parents=True)
    (module / "go.mod").write_text("module example.com/mod\n\ngo 1.26\n", encoding="utf-8")
    (module / "a" / "a.go").write_text(_GO_CODE, encoding="utf-8")
    (module / "a" / "a_test.go").write_text(_GO_TEST, encoding="utf-8")
    return module


def _run(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, gts: str, env: dict[str, str]
) -> tuple[int, list[dict[str, Any]], Path]:
    module = tmp_path / "mod"
    monkeypatch.chdir(module)
    for name in ("FLAKY", "ALWAYS", "RACE", "MAIN_FAILS"):
        monkeypatch.delenv(name, raising=False)
    for name, value in env.items():
        monkeypatch.setenv(name, value)
    out = tmp_path / "flakes"
    code = flakes.main(
        ["go", "--dir", str(out), "--gotestsum", gts, "--packages", "./...", "--", *_FLAGS]
    )
    found = flakes.load_flakes(out.glob("*.json")) if out.exists() else []
    return code, found, module / "coverage.out"


def test_a_test_that_fails_once_passes_with_a_flake_and_the_same_coverage(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, gotestsum: str
) -> None:
    _module(tmp_path)
    code, found, profile = _run(tmp_path, monkeypatch, gotestsum, {})
    assert (code, found) == (0, [])
    clean = go_shards.total(profile)
    code, found, profile = _run(tmp_path, monkeypatch, gotestsum, {"FLAKY": str(tmp_path / "m")})
    assert code == 0
    assert found == [{"tool": "go", "package": PKG, "test": "TestFlaky", "runs": 2, "failures": 1}]
    assert go_shards.total(profile) == pytest.approx(clean)
    assert sorted(p.name for p in profile.parent.iterdir()) == ["a", "coverage.out", "go.mod"]


@pytest.mark.parametrize("env", ["ALWAYS", "RACE", "MAIN_FAILS"])
def test_a_failure_in_every_attempt_or_a_race_still_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, gotestsum: str, env: str
) -> None:
    # ALWAYS fails every attempt; RACE races in the first attempt only (a race never reruns);
    # MAIN_FAILS fails after the tests, so its package must rerun whole, not with `-run ^$`.
    _module(tmp_path)
    code, found, _ = _run(tmp_path, monkeypatch, gotestsum, {env: str(tmp_path / "marker")})
    assert code != 0
    assert found == []


def test_go_shards_runs_each_go_test_through_flakes(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, gotestsum: str
) -> None:
    calls: list[Sequence[str]] = []

    class Popen:
        def __init__(self, args: Sequence[str], **_: Any) -> None:
            calls.append(args)

        def wait(self) -> int:
            return 0

    monkeypatch.setattr(go_shards, "list_packages", lambda *_: [PKG])
    monkeypatch.setattr(go_shards.subprocess, "Popen", Popen)
    monkeypatch.setattr(go_shards, "merge_profiles", lambda *_: None)
    monkeypatch.setattr(Path, "unlink", lambda *_: None)
    monkeypatch.setattr(Path, "replace", lambda *_: None)
    argv = [
        "test",
        "--shard",
        "1/1",
        "--module",
        "m",
        "--flakes",
        str(tmp_path),
        "--gotestsum",
        gotestsum,
    ]
    assert go_shards.main([*argv, "m", "--", "-race", "-coverprofile=c.out"]) == 0
    assert calls == [
        [
            sys.executable,
            str(go_shards.FLAKES),
            "go",
            "--dir",
            str(tmp_path),
            "--gotestsum",
            gotestsum,
            "--packages",
            PKG,
            "--",
            "-race",
            "-coverprofile=c.out",
        ]
    ]


# --- the tracking issue ------------------------------------------------------------------------


class FakeGh:
    """The gh calls of `issue` against an in-memory repository."""

    def __init__(self, artifacts: dict[int, dict[str, list[dict[str, Any]]]]) -> None:
        self.artifacts = artifacts
        self.issues: list[dict[str, Any]] = []
        self.comments: list[str] = []
        self.calls: list[list[str]] = []

    def __call__(self, args: Sequence[str]) -> str:
        args = list(args)
        self.calls.append(args)
        if args[:2] == ["api", "--paginate"] and "/runs?" in args[2]:
            runs = [
                {
                    "id": i,
                    "html_url": f"https://x/runs/{i}",
                    "event": "push",
                    "head_branch": "main",
                    "head_sha": "abc",
                }
                for i in sorted(self.artifacts)
            ]
            return "".join(json.dumps(r) + "\n" for r in runs)
        if args[:2] == ["issue", "list"]:
            return json.dumps([{"number": i["number"], "title": i["title"]} for i in self.issues])
        if args[:2] == ["api", "--paginate"] and args[2].endswith("/comments"):
            return "".join(json.dumps({"body": b}) + "\n" for b in self.comments)
        if args[0] == "api" and "/artifacts" in args[1]:
            run = int(args[1].split("/runs/")[1].split("/")[0])
            names = [*self.artifacts[run], "go-shard-1"]
            return "".join(json.dumps({"name": n}) + "\n" for n in names)
        if args[:2] == ["run", "download"]:
            run, name, target = (
                int(args[2]),
                args[args.index("-n") + 1],
                Path(args[args.index("-D") + 1]),
            )
            target.mkdir(parents=True)
            payload = {"schema": 1, "flakes": self.artifacts[run][name]}
            (target / "go-1.json").write_text(json.dumps(payload), encoding="utf-8")
            return ""
        if args[:2] == ["label", "create"]:
            return ""
        if args[:2] == ["issue", "create"]:
            self.issues.append({"number": 7, "title": args[args.index("--title") + 1]})
            return "https://github.com/o/r/issues/7\n"
        if args[:2] == ["issue", "comment"]:
            self.comments.append(args[args.index("--body") + 1])
            return ""
        if args[:2] == ["issue", "edit"]:
            self.issues[0]["body"] = args[args.index("--body") + 1]
            return ""
        raise AssertionError(args)


def _flake(test: str) -> dict[str, Any]:
    return {"tool": "go", "package": PKG, "test": test, "runs": 2, "failures": 1}


def _issue(gh: FakeGh) -> int:
    parsed = type("A", (), {"repo": "o/r", "workflow": "ci.yml", "hours": 48})()
    return flakes._issue(parsed, gh)  # type: ignore[arg-type]


def test_the_issue_gets_one_comment_per_run_with_flakes_and_the_list(
    capsys: pytest.CaptureFixture[str],
) -> None:
    gh = FakeGh({11: {"flakes-test-server-1": [_flake("TestA")]}, 12: {}})
    assert _issue(gh) == 0
    assert gh.issues[0]["title"] == flakes.TITLE
    [label] = [c for c in gh.calls if c[:2] == ["label", "create"]]
    assert label[label.index("--color") + 1] == "d93f0b"
    [first] = gh.comments
    assert "| new | go | `example.com/mod/a` | `TestA` | 2 | 1 |" in first
    assert "| 1 | 11 | go | `example.com/mod/a` | `TestA` |" in gh.issues[0]["body"]
    # The next run sees run 11 in the comments; a new run with TestA and TestB marks only TestB new.
    gh.artifacts[13] = {"flakes-test-apps": [_flake("TestA"), _flake("Test<B>-->")]}
    assert _issue(gh) == 0
    assert len(gh.comments) == 2
    second = gh.comments[1]
    assert "|  | go | `example.com/mod/a` | `TestA` |" in second
    # Angle brackets of a name could open or close an HTML comment such as the hidden line.
    assert "| new | go | `example.com/mod/a` | `Test\u2039B\u203a--\u203a` |" in second
    assert second.count("-->") == 1
    assert flakes.parse_comments(gh.comments)[13][1]["test"] == "Test<B>-->"
    assert "| 2 | 13 | go | `example.com/mod/a` | `TestA` |" in gh.issues[0]["body"]
    assert len([c for c in gh.calls if c[:2] == ["issue", "create"]]) == 1
    assert _issue(gh) == 0
    assert len(gh.comments) == 2
    assert "no new flakes" in capsys.readouterr().out
