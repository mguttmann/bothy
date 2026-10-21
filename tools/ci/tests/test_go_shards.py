# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/ci/go_shards.py: the Go test shards of the job test (server) and their merge."""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any

import go_shards
import pytest
import yaml

RECORD_NAME = go_shards.RECORD
MODULES = ("services/host", "services/server")
PACKAGES = {
    "services/host": ["example.com/host/cmd", "example.com/host/internal/a"],
    "services/server": [
        "example.com/server/internal/wire",
        "example.com/server/internal/chat",
        "example.com/server/test/integration",
        "example.com/server/api",
        "example.com/server/internal/brand-new",
    ],
}
TIMINGS = {
    "example.com/host/cmd": 1.0,
    "example.com/host/internal/a": 2.0,
    "example.com/server/internal/wire": 600.0,
    "example.com/server/internal/chat": 200.0,
    "example.com/server/test/integration": 430.0,
    "example.com/server/api": 5.0,
    "example.com/server/removed": 900.0,
}


def _units() -> list[go_shards.Unit]:
    return [go_shards.Unit(m, p) for m in MODULES for p in PACKAGES[m]]


@pytest.mark.parametrize("total", range(1, 9))
def test_every_package_lands_in_exactly_one_shard(total: int) -> None:
    shards = go_shards.assign(_units(), TIMINGS, total)
    assert len(shards) == total
    flat = [u for shard in shards for u in shard]
    assert sorted(flat) == sorted(_units())
    assert len(flat) == len(set(flat))


def test_a_package_without_a_timing_still_runs() -> None:
    # A new package has no measured duration yet; it gets the median weight and a shard.
    shards = go_shards.assign(_units(), TIMINGS, 4)
    new = go_shards.Unit("services/server", "example.com/server/internal/brand-new")
    assert sum(shard.count(new) for shard in shards) == 1


def test_the_split_is_deterministic_and_balanced() -> None:
    first = go_shards.assign(_units(), TIMINGS, 3)
    assert first == go_shards.assign(list(reversed(_units())), TIMINGS, 3)
    # Longest first: wire, integration and chat each open a shard of their own.
    heads = [shard[0].package.rsplit("/", 1)[1] for shard in first]
    assert heads == ["wire", "integration", "chat"]


@pytest.mark.parametrize("spec", ["", "4", "0/4", "5/4", "a/4", "1/0", "2/-1"])
def test_a_bad_shard_spec_fails(spec: str) -> None:
    with pytest.raises(ValueError, match="shard"):
        go_shards.parse_shard(spec)


def test_parse_shard() -> None:
    assert go_shards.parse_shard("2/4") == (2, 4)


def test_timings_come_from_go_test_logs(tmp_path: Path) -> None:
    log = tmp_path / "a.log"
    log.write_text(
        "2026-10-21T01:48:29.0263226Z ok  \texample.com/a\t1.000s\tcoverage: 36.0% of ...\n"
        "ok  \texample.com/b\t(cached)\n"
        "?   \texample.com/c\t[no test files]\n"
        "--- FAIL: TestX (0.00s)\n"
        "FAIL\texample.com/d\t3.000s\n",
        encoding="utf-8",
    )
    other = tmp_path / "b.log"
    other.write_text("ok  \texample.com/a\t3.000s\n", encoding="utf-8")
    assert go_shards.parse_timings([log, other]) == {"example.com/a": 2.0, "example.com/d": 3.0}


def _fake_list(monkeypatch: pytest.MonkeyPatch, packages: dict[str, list[str]]) -> None:
    def fake(module: str, tags: str = "") -> list[str]:
        assert tags == "contract_validate"
        return list(packages[module])

    monkeypatch.setattr(go_shards, "list_packages", fake)


def _timings_file(tmp_path: Path) -> Path:
    path = tmp_path / "timings.json"
    path.write_text(json.dumps(TIMINGS), encoding="utf-8")
    return path


WIRE_TESTS = ["TestA", "TestB", "TestC", "TestD", "TestE"]


def _fake_tests(monkeypatch: pytest.MonkeyPatch, splits: dict[str, int]) -> None:
    monkeypatch.setattr(go_shards, "SPLITS", splits)
    monkeypatch.setattr(go_shards, "list_tests", lambda module, package: list(WIRE_TESTS))


def _run_shards(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    total: int,
    capsys: pytest.CaptureFixture[str],
    splits: dict[str, int] | None = None,
) -> Path:
    """Run `packages` for every shard and module like the Makefile, into one folder per shard."""
    _fake_list(monkeypatch, PACKAGES)
    _fake_tests(monkeypatch, splits or {})
    timings = _timings_file(tmp_path)
    shards = tmp_path / "shards"
    for index in range(1, total + 1):
        folder = shards / f"go-shard-{index}"
        for module in MODULES:
            record = folder / go_shards.RECORD
            args = ["packages", "--shard", f"{index}/{total}", "--timings", str(timings)]
            args += ["--tags", "contract_validate"]
            args += ["--record", str(record), "--module", module, *MODULES]
            assert go_shards.main(args) == 0
            printed = sorted({ln.split()[0] for ln in capsys.readouterr().out.splitlines()})
            if printed:
                profile = folder / module / "coverage.out"
                profile.parent.mkdir(parents=True, exist_ok=True)
                lines = "".join(f"{p}/x.go:1.1,2.2 1 {index}\n" for p in printed)
                profile.write_text("mode: atomic\n" + lines, encoding="utf-8")
    return shards


def test_packages_prints_the_shard_of_a_module(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fake_list(monkeypatch, PACKAGES)
    timings = _timings_file(tmp_path)
    seen: list[str] = []
    for index in (1, 2):
        for module in MODULES:
            args = ["packages", "--shard", f"{index}/2", "--timings", str(timings)]
            args += ["--tags", "contract_validate"]
            assert go_shards.main([*args, "--module", module, *MODULES]) == 0
            seen += capsys.readouterr().out.split()
    assert sorted(seen) == sorted(p for m in MODULES for p in PACKAGES[m])


def test_packages_needs_a_module_of_the_list(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fake_list(monkeypatch, PACKAGES)
    args = ["packages", "--shard", "1/2", "--timings", str(_timings_file(tmp_path))]
    args += ["--tags", "contract_validate"]
    assert go_shards.main([*args, "--module", "services/other", *MODULES]) == 1
    assert "services/other" in capsys.readouterr().err


@pytest.mark.parametrize("total", [1, 3, 4])
def test_merge_joins_the_profiles_of_every_shard(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
    total: int,
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, total, capsys)
    out = tmp_path / "repo"
    assert (
        go_shards.main(
            ["merge", "--tags", "contract_validate", "--from", str(shards), "--to", str(out)]
        )
        == 0
    ), capsys.readouterr().err
    for module in MODULES:
        lines = (out / module / "coverage.out").read_text(encoding="utf-8").splitlines()
        # One mode line, then the lines of every shard, as `go test ./...` concatenates them.
        assert lines[0] == "mode: atomic"
        assert sorted(line.split(":")[0] for line in lines[1:]) == sorted(
            f"{p}/x.go" for p in PACKAGES[module]
        )


def test_merged_coverage_equals_the_single_run(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # The same blocks with the same counts, so every total of `go tool cover` is the same.
    single = "mode: atomic\na.go:1.1,2.2 3 0\na.go:1.1,2.2 3 2\nb.go:1.1,2.2 5 0\n"
    parts = [
        "mode: atomic\na.go:1.1,2.2 3 2\n",
        "mode: atomic\na.go:1.1,2.2 3 0\nb.go:1.1,2.2 5 0\n",
    ]
    merged = tmp_path / "merged.out"
    go_shards.merge_profiles(
        [_write(tmp_path / f"{i}.out", p) for i, p in enumerate(parts)], merged
    )
    assert go_shards.total(merged) == go_shards.total(_write(tmp_path / "s.out", single))
    assert go_shards.total(merged) == pytest.approx(37.5)


def _write(path: Path, text: str) -> Path:
    path.write_text(text, encoding="utf-8")
    return path


def test_merge_fails_on_different_modes(tmp_path: Path) -> None:
    a = _write(tmp_path / "a.out", "mode: atomic\na.go:1.1,2.2 1 1\n")
    b = _write(tmp_path / "b.out", "mode: set\na.go:1.1,2.2 1 1\n")
    with pytest.raises(ValueError, match="mode"):
        go_shards.merge_profiles([a, b], tmp_path / "m.out")


def test_merge_fails_when_a_shard_is_missing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, 3, capsys)
    (shards / "go-shard-2").rename(tmp_path / "away")
    assert (
        go_shards.main(
            [
                "merge",
                "--tags",
                "contract_validate",
                "--from",
                str(shards),
                "--to",
                str(tmp_path / "r"),
            ]
        )
        == 1
    )
    assert "shard 2/3 is missing" in capsys.readouterr().err


def test_merge_fails_when_a_package_did_not_run(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, 2, capsys)
    # The module gained a package after the shards ran: it ran in no shard.
    grown = {**PACKAGES, "services/host": [*PACKAGES["services/host"], "example.com/host/late"]}
    _fake_list(monkeypatch, grown)
    assert (
        go_shards.main(
            [
                "merge",
                "--tags",
                "contract_validate",
                "--from",
                str(shards),
                "--to",
                str(tmp_path / "r"),
            ]
        )
        == 1
    )
    assert "example.com/host/late ran in no shard" in capsys.readouterr().err


def test_merge_fails_when_a_package_ran_twice(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, 2, capsys)
    one = shards / "go-shard-1" / go_shards.RECORD
    two = shards / "go-shard-2" / go_shards.RECORD
    pkg = next(line for line in one.read_text().splitlines() if line.count("\t") == 2)
    two.write_text(two.read_text() + pkg.replace("1/2", "2/2", 1) + "\n", encoding="utf-8")
    assert (
        go_shards.main(
            [
                "merge",
                "--tags",
                "contract_validate",
                "--from",
                str(shards),
                "--to",
                str(tmp_path / "r"),
            ]
        )
        == 1
    )
    assert "ran in 2 shards" in capsys.readouterr().err


def test_merge_fails_when_a_profile_is_missing(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, 2, capsys)
    next(shards.glob("go-shard-1/services/*/coverage.out")).unlink()
    assert (
        go_shards.main(
            [
                "merge",
                "--tags",
                "contract_validate",
                "--from",
                str(shards),
                "--to",
                str(tmp_path / "r"),
            ]
        )
        == 1
    )
    assert "coverage.out is missing" in capsys.readouterr().err


def test_merge_fails_when_the_shards_disagree_on_the_count(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, 2, capsys)
    record = shards / "go-shard-2" / go_shards.RECORD
    record.write_text(record.read_text().replace("2/2", "2/3"), encoding="utf-8")
    assert (
        go_shards.main(
            [
                "merge",
                "--tags",
                "contract_validate",
                "--from",
                str(shards),
                "--to",
                str(tmp_path / "r"),
            ]
        )
        == 1
    )
    assert "shard counts differ" in capsys.readouterr().err


def test_the_checked_in_timings_are_package_and_test_durations() -> None:
    data = json.loads(go_shards.TIMINGS.read_text(encoding="utf-8"))
    assert data
    assert all(
        isinstance(k, str) and isinstance(v, int | float) and v >= 0 for k, v in data.items()
    )
    # A test of a split package is `<package>#<test>`.
    assert all(k.split("#")[0] in go_shards.SPLITS for k in data if "#" in k)


# --- the workflows -------------------------------------------------------------------------

REPO = Path(__file__).resolve().parents[3]
WORKFLOWS = [REPO / ".github" / "workflows" / n for n in ("ci.yml", "build-verify.yml")]


def _jobs(path: Path) -> dict[str, Any]:
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    jobs = data["jobs"]
    assert isinstance(jobs, dict)
    return jobs


def _shard_count(job: dict[str, Any]) -> int:
    [step] = [s for s in job["steps"] if "GO_TEST_SHARD" in s.get("env", {})]
    spec = str(step["env"]["GO_TEST_SHARD"])
    assert spec.startswith("${{ matrix.shard }}/")
    return int(spec.rsplit("/", 1)[1])


@pytest.mark.parametrize("path", WORKFLOWS, ids=lambda p: p.name)
def test_the_matrix_runs_every_shard_once(path: Path) -> None:
    job = _jobs(path)["test-server"]
    total = _shard_count(job)
    assert total > 1
    assert job["strategy"]["matrix"] == {"shard": list(range(1, total + 1))}
    # Every shard reports, also after another failed: the merge then names every gap.
    assert job["strategy"]["fail-fast"] is False
    assert job["name"] == f"test (server, shard ${{{{ matrix.shard }}}} of {total})"
    [upload] = [s for s in job["steps"] if str(s.get("uses", "")).startswith("actions/upload-")]
    assert upload["with"]["name"] == "go-shard-${{ matrix.shard }}"
    assert upload["with"]["if-no-files-found"] == "error"


def test_both_workflows_split_alike() -> None:
    first, second = (_jobs(p) for p in WORKFLOWS)
    assert _shard_count(first["test-server"]) == _shard_count(second["test-server"])


@pytest.mark.parametrize("path", WORKFLOWS, ids=lambda p: p.name)
def test_the_coverage_job_merges_every_shard_before_the_threshold(path: Path) -> None:
    jobs = _jobs(path)
    job = jobs["test-server-coverage"]
    assert job["needs"] == ["plan", "test-server"]
    steps = job["steps"]
    [download] = [i for i, s in enumerate(steps) if "download-artifact" in str(s.get("uses"))]
    assert steps[download]["with"]["pattern"] == "go-shard-*"
    [merge] = [i for i, s in enumerate(steps) if "go_shards.py merge" in str(s.get("run"))]
    [gate] = [i for i, s in enumerate(steps) if "make -o test coverage" in str(s.get("run"))]
    assert download < merge < gate
    ok = "ci-ok" if path.name == "ci.yml" else "build-verify-ok"
    assert {"test-server", "test-server-coverage"} <= set(jobs[ok]["needs"])


_FAKE_UV = """#!/bin/sh
printf '%s\\n' "$@" > "$RUNNER_TEMP/uv-args"
ls "$RUNNER_TEMP/go-shards" > "$RUNNER_TEMP/folders"
for d in "$RUNNER_TEMP"/go-shards/*; do cat "$d/go-shard.txt"; done > "$RUNNER_TEMP/records"
"""


@pytest.mark.parametrize("single", [True, False])
def test_the_merge_step_unpacks_one_or_several_artifacts(tmp_path: Path, single: bool) -> None:
    """download-artifact puts a single artifact straight into its path and several into one
    folder each; every archive must reach the merge in a folder of its own."""
    [step] = [
        s
        for s in _jobs(WORKFLOWS[0])["test-server-coverage"]["steps"]
        if "go_shards.py merge" in str(s.get("run"))
    ]
    temp = tmp_path / "runner"
    artifacts = temp / "go-shard-artifacts"
    names = ["go-shard-1"] if single else ["go-shard-1", "go-shard-2", "go-shard-3"]
    for name in names:
        source = tmp_path / "src" / name
        source.mkdir(parents=True)
        (source / RECORD_NAME).write_text(f"{name}\n", encoding="utf-8")
        folder = artifacts if single else artifacts / name
        folder.mkdir(parents=True, exist_ok=True)
        subprocess.run(  # nosec B603 B607 - a fixed tar call on test files
            ["tar", "-cf", str(folder / "go-shard.tar"), RECORD_NAME], cwd=source, check=True
        )
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    uv = bin_dir / "uv"
    uv.write_text(_FAKE_UV, encoding="utf-8")
    uv.chmod(0o755)
    done = subprocess.run(  # nosec B603 B607 - the workflow's own step with a fixed environment
        ["bash", "-e", "-c", str(step["run"])],
        cwd=tmp_path,
        env={"PATH": f"{bin_dir}:/usr/bin:/bin", "RUNNER_TEMP": str(temp)},
        capture_output=True,
        text=True,
        check=False,
    )
    assert done.returncode == 0, done.stderr
    assert (temp / "folders").read_text().split() == [str(i) for i in range(1, len(names) + 1)]
    assert sorted((temp / "records").read_text().split()) == names
    assert "merge" in (temp / "uv-args").read_text().split()


def _fake_go(tmp_path: Path, monkeypatch: pytest.MonkeyPatch, script: str) -> None:
    bin_dir = tmp_path / "gobin"
    bin_dir.mkdir()
    go = bin_dir / "go"
    go.write_text("#!/bin/sh\n" + script, encoding="utf-8")
    go.chmod(0o755)
    monkeypatch.setenv("PATH", f"{bin_dir}:/usr/bin:/bin")


def test_list_packages_runs_go_list_in_the_module(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _fake_go(tmp_path, monkeypatch, 'echo "$@" > args; printf "a\\nb\\n"\n')
    module = tmp_path / "mod"
    module.mkdir()
    # The build tags of `go test`: a package of tagged files only is listed only with them.
    assert go_shards.list_packages(str(module), "contract_validate") == ["a", "b"]
    assert (module / "args").read_text().split() == ["list", "-tags", "contract_validate", "./..."]
    assert go_shards.list_packages(str(module)) == ["a", "b"]
    assert (module / "args").read_text().split() == ["list", "./..."]


def test_list_packages_fails_when_go_list_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    _fake_go(tmp_path, monkeypatch, "echo broken >&2; exit 1\n")
    with pytest.raises(ValueError, match="broken"):
        go_shards.list_packages(str(tmp_path))


def test_timings_cli(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    log = _write(tmp_path / "a.log", "ok  \texample.com/a\t4.000s\n")
    assert go_shards.main(["timings", str(log)]) == 0
    assert json.loads(capsys.readouterr().out) == {"example.com/a": 4.0}


def test_merge_fails_on_a_broken_record(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    (tmp_path / "s" / "1").mkdir(parents=True)
    _write(tmp_path / "s" / "1" / go_shards.RECORD, "garbage\n")
    assert (
        go_shards.main(
            [
                "merge",
                "--tags",
                "contract_validate",
                "--from",
                str(tmp_path / "s"),
                "--to",
                str(tmp_path),
            ]
        )
        == 1
    )
    assert "not a record line" in capsys.readouterr().err
    assert (
        go_shards.main(
            [
                "merge",
                "--tags",
                "contract_validate",
                "--from",
                str(tmp_path / "none"),
                "--to",
                str(tmp_path),
            ]
        )
        == 1
    )
    assert "no shard record" in capsys.readouterr().err


def test_packages_lists_the_longest_package_first_then_go_list_order(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # `go test` starts packages in the order given; the longest one must not start last.
    _fake_list(monkeypatch, PACKAGES)
    args = ["packages", "--shard", "1/1", "--timings", str(_timings_file(tmp_path))]
    args += ["--tags", "contract_validate"]
    assert go_shards.main([*args, "--module", "services/server", *MODULES]) == 0
    printed = capsys.readouterr().out.split()
    # Then the others in `go list` order, so long and short packages mix.
    others = [p for p in PACKAGES["services/server"] if p != "example.com/server/internal/wire"]
    assert printed == ["example.com/server/internal/wire", *others]


def test_each_module_lists_its_own_longest_package_first(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # The longest package of the shard may belong to another module; each module still starts
    # with its own longest package, then follows `go list` order.
    _fake_list(monkeypatch, PACKAGES)
    args = ["packages", "--shard", "1/1", "--timings", str(_timings_file(tmp_path))]
    args += ["--tags", "contract_validate"]
    assert go_shards.main([*args, "--module", "services/host", *MODULES]) == 0
    assert capsys.readouterr().out.split() == [
        "example.com/host/internal/a",
        "example.com/host/cmd",
    ]


@pytest.mark.parametrize("path", WORKFLOWS, ids=lambda p: p.name)
def test_the_merge_lists_packages_with_the_tags_of_make_test(path: Path) -> None:
    # A package of tagged files only is a package of `go test -tags` but not of a plain
    # `go list`; the merge must list with the tags the shards tested with.
    [line] = [
        ln
        for ln in (REPO / "Makefile").read_text(encoding="utf-8").splitlines()
        if ln.startswith("GO_TEST_TAGS :=")
    ]
    tags = line.split(":=", 1)[1].strip()
    [step] = [
        s
        for s in _jobs(path)["test-server-coverage"]["steps"]
        if "go_shards.py merge" in str(s.get("run"))
    ]
    assert f"merge --tags {tags} " in str(step["run"])


# --- a package split across shards ----------------------------------------------------------

WIRE = "example.com/server/internal/wire"
SPLITS = {WIRE: 2}


def test_a_split_package_becomes_one_unit_per_part() -> None:
    units = go_shards.expand(_units(), SPLITS)
    wire = [u for u in units if u.package == WIRE]
    assert wire == [
        go_shards.Unit("services/server", WIRE, "1/2"),
        go_shards.Unit("services/server", WIRE, "2/2"),
    ]
    assert len(units) == len(_units()) + 1


def test_the_parts_of_a_split_package_go_to_different_shards() -> None:
    # Each part weighs its share of the package, so the two halves of wire open two shards.
    shards = go_shards.assign(go_shards.expand(_units(), SPLITS), TIMINGS, 3)
    holders = [i for i, s in enumerate(shards) for u in s if u.package == WIRE]
    assert len(holders) == 2
    assert len(set(holders)) == 2
    heads = [(s[0].package.rsplit("/", 1)[1], s[0].part) for s in shards]
    assert heads == [("integration", ""), ("wire", "1/2"), ("wire", "2/2")]


def test_top_level_tests_come_from_the_test_files(tmp_path: Path) -> None:
    _write(
        tmp_path / "a_test.go",
        "package a\n\nfunc TestMain(m *testing.M) {}\nfunc TestB(t *testing.T) {}\n"
        "func Test(t *testing.T) {}\nfunc Testing(t *testing.T) {}\n"
        "func (s suite) TestMethod(t *testing.T) {}\nfunc helper() {}\n"
        "func ExampleA() {}\nfunc FuzzC(f *testing.F) {}\nfunc BenchmarkD(b *testing.B) {}\n"
        "func Test_under(t *testing.T) {}\n",
    )
    _write(tmp_path / "b_test.go", "package a_test\n\nfunc TestA(t *testing.T) {}\n")
    _write(tmp_path / "c.go", "package a\n\nfunc TestNotATestFile(t *testing.T) {}\n")
    assert go_shards.top_level_tests(tmp_path) == [
        "ExampleA",
        "FuzzC",
        "Test",
        "TestA",
        "TestB",
        "Test_under",
    ]


@pytest.mark.parametrize("parts", [2, 3])
def test_the_part_filters_cover_every_name_once(parts: int) -> None:
    names = [f"Test{c}" for c in "ABCDEFG"]
    filters = [go_shards.part_filter(names, j, parts) for j in range(1, parts + 1)]
    # The parts before the last run their own group; the last skips all the others, so a test
    # the listing does not know still runs there.
    assert all(f[0] == "-run" for f in filters[:-1])
    assert filters[-1][0] == "-skip"
    groups = [go_shards.pattern_names(f[1]) for f in filters[:-1]]
    flat = [n for g in groups for n in g]
    assert len(flat) == len(set(flat))
    assert set(go_shards.pattern_names(filters[-1][1])) == set(flat)
    assert 0 < len(flat) < len(names)


_FIXTURE_TESTS = {
    "a_test.go": """package fixture

import "testing"

func TestMain(m *testing.M) { m.Run() }
func TestAlpha(t *testing.T) { t.Run("sub", func(t *testing.T) {}) }
func TestBeta(t *testing.T) {}
func TestAlphaLonger(t *testing.T) {}
func Test_snake(t *testing.T) {}
func FuzzGamma(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, n int) {}) }
""",
    "b_test.go": """package fixture_test

import (
	"fmt"
	"testing"
)

func TestDelta(t *testing.T) { t.Parallel() }
func TestEpsilon(t *testing.T) { t.Run("TestBeta", func(t *testing.T) {}) }
func Example_zeta() {
	fmt.Println("z")
	// Output: z
}
""",
}


def _go_test_names(module: Path, *args: str) -> list[str]:
    done = subprocess.run(  # nosec B603 B607 - the go tool on a fixture module
        ["go", "test", "-count=1", *args, "."],
        cwd=module,
        capture_output=True,
        text=True,
        check=True,
        env={**os.environ, "GOFLAGS": "-mod=mod", "GOWORK": "off"},
    )
    return done.stdout.splitlines()


@pytest.mark.parametrize("parts", [2, 3])
def test_every_top_level_test_of_a_split_package_runs_in_exactly_one_part(
    tmp_path: Path, parts: int
) -> None:
    """Against the real go tool: the filters of the parts run each test, example and fuzz seed of
    `go test -list` once in all parts together, with its subtests, and a test the listing
    missed still runs (in the last part)."""
    _write(tmp_path / "go.mod", "module example.com/fixture\n\ngo 1.21\n")
    for name, text in _FIXTURE_TESTS.items():
        _write(tmp_path / name, text)
    every = [n for n in _go_test_names(tmp_path, "-list", ".*") if not n.startswith("ok ")]
    listed = go_shards.top_level_tests(tmp_path)
    assert sorted(every) == listed
    missed = [n for n in listed if n != "TestDelta"]
    ran: list[str] = []
    for part in range(1, parts + 1):
        flag, pattern = go_shards.part_filter(missed, part, parts)
        lines = _go_test_names(tmp_path, "-v", flag, pattern)
        ran += [ln.split()[-1] for ln in lines if ln.startswith("=== RUN")]
    top = [n for n in ran if "/" not in n]
    assert sorted(top) == sorted(every)
    assert ran.count("TestAlpha/sub") == 1
    assert ran.count("TestEpsilon/TestBeta") == 1


def _merge(shards: Path, out: Path) -> int:
    return go_shards.main(
        ["merge", "--tags", "contract_validate", "--from", str(shards), "--to", str(out)]
    )


def _part_lines(shards: Path) -> list[list[str]]:
    return [
        line.split("\t")
        for record in sorted(shards.glob(f"*/{RECORD_NAME}"))
        for line in record.read_text(encoding="utf-8").splitlines()
        if line.count("\t") == 5
    ]


@pytest.mark.parametrize("total", [1, 2, 3])
def test_packages_prints_and_records_the_part_filters(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
    total: int,
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, total, capsys, SPLITS)
    parts = _part_lines(shards)
    assert sorted((p[2], p[3]) for p in parts) == [(WIRE, "1/2"), (WIRE, "2/2")]
    by_part = {p[3]: (p[4], p[5]) for p in parts}
    assert by_part["1/2"] == go_shards.part_filter(WIRE_TESTS, 1, 2)
    assert by_part["2/2"] == go_shards.part_filter(WIRE_TESTS, 2, 2)
    assert _merge(shards, tmp_path / "repo") == 0, capsys.readouterr().err


def test_merge_fails_when_a_part_did_not_run(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, 3, capsys, SPLITS)
    for record in shards.glob(f"*/{RECORD_NAME}"):
        lines = record.read_text(encoding="utf-8").splitlines()
        kept = [ln for ln in lines if "\t2/2\t" not in ln]
        record.write_text("".join(f"{ln}\n" for ln in kept), encoding="utf-8")
    assert _merge(shards, tmp_path / "repo") == 1
    assert f"{WIRE} part 2/2 ran in no shard" in capsys.readouterr().err


def test_merge_fails_when_a_split_package_ran_whole(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # The shards split wire, the merge expects it whole: the split and the merge disagree.
    shards = _run_shards(tmp_path, monkeypatch, 2, capsys, SPLITS)
    _fake_tests(monkeypatch, {})
    assert _merge(shards, tmp_path / "repo") == 1
    err = capsys.readouterr().err
    assert f"{WIRE} ran in no shard" in err
    assert f"{WIRE} part 1/2 is not a package of services/server" in err


@pytest.mark.parametrize(
    ("part", "pattern", "message"),
    [
        ("2/2", "^(TestA)$", "the parts of"),
        ("1/2", "^(TestA|TestB|TestC)$", "the parts of"),
        ("1/2", "TestA", "not a filter"),
    ],
)
def test_merge_fails_when_the_part_filters_do_not_cover_the_tests_once(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    capsys: pytest.CaptureFixture[str],
    part: str,
    pattern: str,
    message: str,
) -> None:
    shards = _run_shards(tmp_path, monkeypatch, 2, capsys, SPLITS)
    for record in shards.glob(f"*/{RECORD_NAME}"):
        lines = record.read_text(encoding="utf-8").splitlines()
        out = []
        for line in lines:
            fields = line.split("\t")
            if len(fields) == 6 and fields[3] == part:
                fields[5] = pattern
            out.append("\t".join(fields) + "\n")
        record.write_text("".join(out), encoding="utf-8")
    assert _merge(shards, tmp_path / "repo") == 1
    assert message in capsys.readouterr().err


def test_merged_coverage_of_a_split_package_equals_the_single_run(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    # Every part writes the blocks of its test binary; the merge keeps all of them, so a block
    # one part covers counts as covered as in the single run.
    shards = _run_shards(tmp_path, monkeypatch, 3, capsys, SPLITS)
    holders = sorted({p[0].split("/")[0] for p in _part_lines(shards)})
    assert holders == ["2", "3"]
    folders = {}
    for record in shards.glob(f"*/{RECORD_NAME}"):
        for line in record.read_text(encoding="utf-8").splitlines():
            fields = line.split("\t")
            if len(fields) == 6:
                folders[fields[3]] = record.parent
    blocks = {
        "1/2": "w.go:1.1,2.2 4 1\nw.go:3.1,4.2 6 0\n",
        "2/2": "w.go:1.1,2.2 4 0\nw.go:3.1,4.2 6 2\n",
    }
    for part, folder in folders.items():
        profile = folder / "services/server" / "coverage.out"
        profile.write_text(profile.read_text(encoding="utf-8") + blocks[part], encoding="utf-8")
    out = tmp_path / "repo"
    assert _merge(shards, out) == 0, capsys.readouterr().err
    merged = out / "services/server" / "coverage.out"
    single = _write(tmp_path / "single.out", "mode: atomic\nw.go:1.1,2.2 4 1\nw.go:3.1,4.2 6 2\n")
    only_wire = _write(
        tmp_path / "wire.out",
        "mode: atomic\n"
        + "".join(ln + "\n" for ln in merged.read_text().splitlines()[1:] if ln.startswith("w.go")),
    )
    assert go_shards.total(only_wire) == go_shards.total(single) == 100.0


# --- the test command: go test for the units of a shard --------------------------------------

_GO_FAKE = """#!{python}
import json, os, sys
args = sys.argv[1:]
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"args": args, "cwd": os.getcwd()}}) + "\\n")
if "-c" in args:
    open(args[args.index("-o") + 1], "w").close()
    sys.exit(int(os.environ.get("FAKE_BUILD_EXIT", "0")))
[profile] = [a.split("=", 1)[1] for a in args if a.startswith("-coverprofile=")]
packages = [a for a in args if "/" in a and not a.startswith("-")]
with open(profile, "w", encoding="utf-8") as fh:
    fh.write("mode: atomic\\n" + "".join(f"{{p}}/x.go:1.1,2.2 1 1\\n" for p in packages))
for p in packages:
    print(f"ok  \\t{{p}}\\t2.000s")
sys.exit(1 if os.environ.get("FAKE_FAIL", "-") in args else 0)
"""
GO_FLAGS = ["-race", "-covermode=atomic", "-coverprofile=coverage.out", "-timeout=20m"]


def _go(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    bin_dir = tmp_path / "gobin"
    bin_dir.mkdir()
    go = bin_dir / "go"
    go.write_text(_GO_FAKE.format(python=sys.executable), encoding="utf-8")
    go.chmod(0o755)
    log = tmp_path / "go.log"
    monkeypatch.setenv("PATH", f"{bin_dir}:/usr/bin:/bin")
    monkeypatch.setenv("FAKE_LOG", str(log))
    for module in MODULES:
        (tmp_path / module).mkdir(parents=True, exist_ok=True)
    monkeypatch.chdir(tmp_path)
    return log


def _calls(log: Path) -> list[dict[str, Any]]:
    if not log.exists():
        return []
    return [json.loads(line) for line in log.read_text(encoding="utf-8").splitlines()]


def _test(tmp_path: Path, shard: str, module: str, *flags: str) -> int:
    args = ["test", "--shard", shard, "--timings", str(_timings_file(tmp_path))]
    args += ["--tags", "contract_validate", "--record", str(tmp_path / "rec" / RECORD_NAME)]
    return go_shards.main([*args, "--module", module, *MODULES, "--", *(flags or GO_FLAGS)])


def test_the_test_command_runs_the_packages_and_the_part_of_its_shard(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    log = _go(tmp_path, monkeypatch)
    _fake_list(monkeypatch, PACKAGES)
    _fake_tests(monkeypatch, SPLITS)
    # Shard 3 of 3: the second half of wire and the packages beside it (see the split tests).
    assert _test(tmp_path, "3/3", "services/server") == 0
    out = capsys.readouterr().out
    calls = _calls(log)
    assert {c["cwd"] for c in calls} == {str((tmp_path / "services/server").resolve())}
    # One build first: the go test processes then find every package compiled, instead of
    # compiling the module each on its own at the same time.
    build, *runs = calls
    assert build["args"][:2] == ["test", "-c"]
    assert build["args"][4:] == ["-race", "-covermode=atomic", WIRE]
    [whole] = [c["args"] for c in runs if "-skip" not in c["args"]]
    [part] = [c["args"] for c in runs if "-skip" in c["args"]]
    assert whole[: len(GO_FLAGS) + 1] == ["test", *GO_FLAGS]
    assert WIRE not in whole
    flag, pattern = go_shards.part_filter(WIRE_TESTS, 2, 2)
    assert part == [
        "test",
        "-race",
        "-covermode=atomic",
        "-coverprofile=coverage.part1.out",
        "-timeout=20m",
        "-v",
        flag,
        pattern,
        WIRE,
    ]
    # The part's output follows a marker, so `timings` knows it measured a part; -v adds the
    # durations of its top-level tests, which balance the parts of the next split.
    assert f"go-shard: part 2/2 of {WIRE}\nok  \t{WIRE}\t2.000s\n" in out
    module = tmp_path / "services/server"
    assert not (module / "coverage.part1.out").exists()
    merged = (module / "coverage.out").read_text(encoding="utf-8").splitlines()
    assert merged[0] == "mode: atomic"
    assert f"{WIRE}/x.go:1.1,2.2 1 1" in merged
    assert len(merged) == 1 + len(whole) - len(GO_FLAGS) - 1 + 1


def test_a_shard_of_one_part_runs_it_without_a_build_first(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    log = _go(tmp_path, monkeypatch)
    _fake_list(monkeypatch, {"services/host": [], "services/server": [WIRE]})
    _fake_tests(monkeypatch, SPLITS)
    assert _test(tmp_path, "1/2", "services/server") == 0
    [call] = _calls(log)
    assert go_shards.part_filter(WIRE_TESTS, 1, 2)[1] in call["args"]
    module = tmp_path / "services/server"
    assert (module / "coverage.out").read_text(encoding="utf-8").startswith("mode: atomic\n")
    assert sorted(p.name for p in module.iterdir()) == ["coverage.out"]
    assert _test(tmp_path, "2/2", "services/host") == 0
    assert "no package of services/host in shard 2/2" in capsys.readouterr().out
    assert len(_calls(log)) == 1


@pytest.mark.parametrize("fail", [WIRE, "example.com/server/internal/chat", "build"])
def test_the_test_command_fails_when_a_go_test_fails(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str], fail: str
) -> None:
    log = _go(tmp_path, monkeypatch)
    _fake_list(monkeypatch, {"services/host": [], "services/server": PACKAGES["services/server"]})
    _fake_tests(monkeypatch, SPLITS)
    if fail == "build":
        monkeypatch.setenv("FAKE_BUILD_EXIT", "2")
    else:
        monkeypatch.setenv("FAKE_FAIL", fail)
    assert _test(tmp_path, "1/1", "services/server") == 1
    # A failed build runs nothing; a failed test lets the others finish and report.
    assert len(_calls(log)) == (1 if fail == "build" else 4)
    assert "go-shard: part 2/2" in capsys.readouterr().out or fail == "build"


def test_the_test_command_needs_one_profile_flag(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    log = _go(tmp_path, monkeypatch)
    _fake_list(monkeypatch, PACKAGES)
    _fake_tests(monkeypatch, SPLITS)
    assert _test(tmp_path, "1/1", "services/server", "-race") == 1
    assert "-coverprofile" in capsys.readouterr().err
    assert _calls(log) == []


def test_timings_count_a_part_as_its_share_of_the_package(tmp_path: Path) -> None:
    log = _write(
        tmp_path / "a.log",
        "2026-10-21T01:48:29.0Z ok  \texample.com/a\t1.000s\n"
        f"2026-10-21T01:48:30.0Z go-shard: part 2/2 of {WIRE}\n"
        f"2026-10-21T01:48:30.0Z ok  \t{WIRE}\t300.000s\tcoverage: 1.0% of ...\n",
    )
    other = _write(tmp_path / "b.log", f"go-shard: part 1/2 of {WIRE}\nFAIL\t{WIRE}\t200.000s\n")
    assert go_shards.parse_timings([log, other]) == {"example.com/a": 1.0, WIRE: 500.0}


@pytest.mark.parametrize(("package", "parts"), sorted(go_shards.SPLITS.items()))
def test_every_split_package_is_a_tested_package_of_the_tree(package: str, parts: int) -> None:
    # A renamed or emptied package would leave a split that never applies.
    module = next(m for m in ("services/server", "services/host") if f"/{m}/" in f"{package}/")
    names = go_shards.list_tests(str(REPO / module), package)
    assert len(names) >= 2 * parts
    assert package in json.loads(go_shards.TIMINGS.read_text(encoding="utf-8"))
    first = go_shards.pattern_names(go_shards.part_filter(names, 1, parts)[1])
    assert len(names) // parts <= len(first) <= len(names) // parts + 1


def test_the_parts_balance_the_measured_test_durations() -> None:
    names = ["TestA", "TestB", "TestC", "TestD", "TestE"]
    weights = {"TestC": 100.0, "TestA": 30.0, "TestB": 30.0, "TestD": 30.0}
    # Longest first, each to the lighter part: TestC alone, the others (TestE unmeasured weighs
    # the median) together; the last part still skips the others instead of naming its own.
    first = go_shards.part_filter(names, 1, 2, weights)
    assert first == ("-run", "^(TestC)$")
    assert go_shards.part_filter(names, 2, 2, weights) == ("-skip", "^(TestC)$")
    # Without measurements every test weighs the same: round-robin by name.
    assert go_shards.part_filter(names, 1, 2) == ("-run", "^(TestA|TestC|TestE)$")


def test_timings_keep_the_top_level_tests_of_a_part(tmp_path: Path) -> None:
    log = _write(
        tmp_path / "a.log",
        "2026-10-21T01:48:29.0Z --- PASS: TestOther (9.00s)\n"
        f"2026-10-21T01:48:30.0Z go-shard: part 1/2 of {WIRE}\n"
        "2026-10-21T01:48:30.0Z === RUN   TestA\n"
        "2026-10-21T01:48:30.0Z --- PASS: TestA (12.50s)\n"
        "2026-10-21T01:48:30.0Z     --- PASS: TestA/sub (12.00s)\n"
        "2026-10-21T01:48:30.0Z --- FAIL: TestB (3.00s)\n"
        "2026-10-21T01:48:30.0Z --- SKIP: TestC (0.00s)\n"
        f"2026-10-21T01:48:30.0Z ok  \t{WIRE}\t20.000s\n"
        "2026-10-21T01:48:31.0Z --- PASS: TestLater (5.00s)\n",
    )
    assert go_shards.parse_timings([log]) == {
        WIRE: 40.0,
        f"{WIRE}#TestA": 12.5,
        f"{WIRE}#TestB": 3.0,
        f"{WIRE}#TestC": 0.0,
    }


def test_packages_balances_the_parts_by_the_test_timings(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    _fake_list(monkeypatch, PACKAGES)
    _fake_tests(monkeypatch, SPLITS)
    timings = tmp_path / "timings.json"
    tests = {f"{WIRE}#{n}": 500.0 if n == "TestE" else 10.0 for n in WIRE_TESTS}
    timings.write_text(json.dumps({**TIMINGS, **tests}), encoding="utf-8")
    args = ["packages", "--shard", "1/1", "--timings", str(timings), "--tags", "contract_validate"]
    assert go_shards.main([*args, "--module", "services/server", *MODULES]) == 0
    lines = [ln.split("\t") for ln in capsys.readouterr().out.splitlines() if "\t" in ln]
    assert sorted(ln[1:] for ln in lines) == [["-run", "^(TestE)$"], ["-skip", "^(TestE)$"]]
