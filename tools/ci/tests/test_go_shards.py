# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/ci/go_shards.py: the Go test shards of the job test (server) and their merge."""

from __future__ import annotations

import json
import subprocess
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


def _run_shards(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, total: int, capsys: pytest.CaptureFixture[str]
) -> Path:
    """Run `packages` for every shard and module like the Makefile, into one folder per shard."""
    _fake_list(monkeypatch, PACKAGES)
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
            printed = capsys.readouterr().out.split()
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


def test_the_checked_in_timings_are_package_durations() -> None:
    data = json.loads(go_shards.TIMINGS.read_text(encoding="utf-8"))
    assert data
    assert all(
        isinstance(k, str) and isinstance(v, int | float) and v >= 0 for k, v in data.items()
    )


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
