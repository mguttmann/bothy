# SPDX-License-Identifier: Apache-2.0
"""Advisory gates of a pull request fail only for what the pull request adds (PL/25).

The unit tests run tools/ci/advisories.py against a git repository with a base and a head
commit and two fake scanners that print the JSON of OSV-Scanner v2 and govulncheck v1 (the
shapes of their real output) and exit like them: OSV-Scanner with 1 when a called
vulnerability is left, govulncheck with 0 in JSON mode. The workflow tests read the real
Makefile and workflows.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

import advisories
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]

# Fake OSV-Scanner: each lock file holds lines "<ecosystem> <name> <version> <id> [uncalled]";
# a line without an id is a package without a known vulnerability. Like the real scanner it picks
# the parser by the file name (exit 127 for a name it does not know) unless the argument names the
# format (`gradle.lockfile:<path>`), and it reads a Gradle lock file in Gradle's own format
# (`group:artifact:version=configurations`), where an artifact named `bad...` is vulnerable.
FAKE_OSV = r"""
import json, os, sys
args = sys.argv[1:]
assert args[:2] == ["scan", "source"], args
KNOWN = {"uv.lock", "package-lock.json", "pubspec.lock", "go.mod", "gradle.lockfile",
         "buildscript-gradle.lockfile"}
results, failing = [], False
for arg in [args[i + 1] for i, a in enumerate(args) if a == "-L"]:
    form, sep, lock = arg.partition(":")
    if not sep:
        form, lock = os.path.basename(arg), arg
    if form not in KNOWN:
        sys.stderr.write("could not determine extractor suitable to this file: " + arg + "\n")
        sys.exit(127)
    pkgs = []
    for line in open(lock).read().splitlines():
        if form.endswith(".lockfile"):
            if line.startswith("#") or line.startswith("empty="):
                continue
            group, artifact, version = line.split("=")[0].split(":")
            if not artifact.startswith("bad"):
                continue
            parts = ["Maven", group + ":" + artifact, version, "GHSA-" + artifact]
        else:
            parts = line.split()
        if len(parts) < 4:
            continue
        eco, name, version, vid = parts[:4]
        called = "uncalled" not in parts[4:]
        failing = failing or called
        pkgs.append({
            "package": {"name": name, "version": version, "ecosystem": eco},
            "groups": [{"ids": [vid], "aliases": [vid],
                        "experimental_analysis": {vid: {"called": called, "unimportant": False}}}],
            "vulnerabilities": [{"id": vid}],
        })
    if pkgs:
        results.append({"source": {"path": os.path.abspath(lock), "type": "lockfile"},
                        "packages": pkgs})
print(json.dumps({"results": results}))
sys.exit(1 if failing else 0)
"""

# Fake govulncheck: vulns.txt in the module holds lines "<osv> <module> <version> [called]".
FAKE_GOVULNCHECK = r"""
import json, os, sys
print(json.dumps({"config": {"scanner_name": "govulncheck"}}))
if os.path.exists("vulns.txt"):
    for line in open("vulns.txt").read().splitlines():
        osv, mod, version, *rest = line.split()
        frame = {"module": mod, "version": version}
        if "called" in rest:
            frame.update({"package": mod + "/p", "function": "F"})
        print(json.dumps({"finding": {"osv": osv, "trace": [frame]}}))
sys.exit(int(os.environ.get("FAKE_GV_EXIT", "0")))
"""


def _git(repo: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    ).stdout.strip()


class Fixture:
    def __init__(self, tmp: Path) -> None:
        self.repo = tmp / "repo"
        self.repo.mkdir()
        _git(self.repo, "init", "-q", "-b", "main")
        _git(self.repo, "config", "user.name", "t")
        _git(self.repo, "config", "user.email", "t@example.org")
        _git(self.repo, "config", "commit.gpgsign", "false")
        self.osv = tmp / "fake_osv.py"
        self.osv.write_text(f"#!{sys.executable}\n" + FAKE_OSV)
        self.osv.chmod(0o755)
        self.gv = tmp / "fake_gv.py"
        self.gv.write_text(FAKE_GOVULNCHECK)

    def write(self, files: dict[str, str]) -> None:
        for name, text in files.items():
            path = self.repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text)

    def commit(self, files: dict[str, str]) -> str:
        self.write(files)
        _git(self.repo, "add", "-A")
        _git(self.repo, "commit", "-q", "-m", "c", "--allow-empty")
        return _git(self.repo, "rev-parse", "HEAD")

    def run(
        self,
        base: str | None,
        locks: list[str],
        mods: tuple[str, ...] | list[str] = (),
    ) -> int:
        argv = ["--repo", str(self.repo), "--osv-scanner", str(self.osv)]
        if base is not None:
            argv += ["--base", base, "--govulncheck", f"{sys.executable} {self.gv}"]
        for lock in locks:
            argv += ["--lock", lock]
        for mod in mods:
            argv += ["--go-module", mod]
        return advisories.main(argv)


@pytest.fixture
def fx(tmp_path: Path) -> Fixture:
    return Fixture(tmp_path)


def test_a_pull_request_that_adds_a_vulnerable_dependency_fails(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    base = fx.commit({"a/uv.lock": "PyPI ok 1.0\n"})
    fx.commit({"a/uv.lock": "PyPI ok 1.0\nPyPI bad 2.0 GHSA-new\n"})
    assert fx.run(base, ["a/uv.lock"]) == 1
    err = capsys.readouterr()
    assert "bad 2.0" in err.out + err.err
    assert "GHSA-new" in err.out + err.err


def test_a_new_advisory_for_a_dependency_of_the_base_does_not_fail(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    base = fx.commit({"a/uv.lock": "PyPI old 1.0 GHSA-fresh\n"})
    fx.commit({"a/uv.lock": "PyPI old 1.0 GHSA-fresh\n", "x.txt": "code\n"})
    assert fx.run(base, ["a/uv.lock"]) == 0
    out = capsys.readouterr().out
    assert "::warning" in out
    assert "GHSA-fresh" in out


def test_a_pull_request_that_changes_a_dependency_to_a_vulnerable_version_fails(
    fx: Fixture,
) -> None:
    base = fx.commit({"a/uv.lock": "PyPI lib 1.0 GHSA-one\n"})
    fx.commit({"a/uv.lock": "PyPI lib 1.1 GHSA-one\n"})
    assert fx.run(base, ["a/uv.lock"]) == 1


def test_a_vulnerable_dependency_in_a_new_lock_file_fails(fx: Fixture) -> None:
    base = fx.commit({"a/uv.lock": "PyPI ok 1.0\n"})
    fx.commit({"b/package-lock.json": "npm bad 2.0 GHSA-x\n"})
    assert fx.run(base, ["a/uv.lock", "b/package-lock.json"]) == 1


def test_a_dependency_moved_into_another_lock_file_counts_as_added(fx: Fixture) -> None:
    base = fx.commit({"a/uv.lock": "PyPI old 1.0 GHSA-fresh\n"})
    fx.commit({"b/uv.lock": "PyPI old 1.0 GHSA-fresh\n"})
    assert fx.run(base, ["a/uv.lock", "b/uv.lock"]) == 1


def test_an_uncalled_vulnerability_passes_as_the_scanner_decides(fx: Fixture) -> None:
    base = fx.commit({"a/uv.lock": "PyPI ok 1.0\n"})
    fx.commit({"a/uv.lock": "Go mod 1.0 GO-1 uncalled\n"})
    assert fx.run(base, ["a/uv.lock"]) == 0


def test_a_scanner_error_fails(fx: Fixture) -> None:
    base = fx.commit({"a/uv.lock": "PyPI ok 1.0\n"})
    fx.commit({"x.txt": "y\n"})
    fx.osv.write_text(f"#!{sys.executable}\nimport sys\nsys.exit(128)\n")
    assert fx.run(base, ["a/uv.lock"]) == 1


def test_a_failing_scan_without_a_readable_finding_fails(fx: Fixture) -> None:
    base = fx.commit({"a/uv.lock": "PyPI ok 1.0\n"})
    fx.commit({"x.txt": "y\n"})
    fx.osv.write_text(
        f"#!{sys.executable}\nimport sys\nprint('{{\"results\": []}}')\nsys.exit(1)\n"
    )
    assert fx.run(base, ["a/uv.lock"]) == 1


def test_a_failed_base_scan_counts_every_finding_as_new(fx: Fixture) -> None:
    base = fx.commit({"a/uv.lock": "PyPI old 1.0 GHSA-fresh\n"})
    fx.commit({"x.txt": "y\n"})
    # The base scan reads the lock files of the base from a tree outside the repository.
    guard = (
        f"#!{sys.executable}\nimport os, sys\n"
        f"if any(os.path.isabs(a) and not a.startswith({str(fx.repo)!r}) for a in sys.argv[1:]):\n"
        "    sys.exit(2)\n"
    )
    fx.osv.write_text(guard + FAKE_OSV)
    assert fx.run(base, ["a/uv.lock"]) == 1


def test_the_base_scan_reads_the_scanner_config_of_the_base(fx: Fixture) -> None:
    # A lock file and its osv-scanner.toml of the base land in the base tree at the same place.
    base = fx.commit({"a/uv.lock": "PyPI old 1.0 GHSA-fresh\n", "a/osv-scanner.toml": "# c\n"})
    fx.commit({"x.txt": "y\n"})
    tree = advisories.base_tree(fx.repo, base, ["a/uv.lock"], Path(fx.repo.parent / "t"))
    assert (tree / "a" / "uv.lock").read_text() == "PyPI old 1.0 GHSA-fresh\n"
    assert (tree / "a" / "osv-scanner.toml").read_text() == "# c\n"


def test_govulncheck_fails_for_a_call_the_pull_request_adds(fx: Fixture) -> None:
    base = fx.commit({"m/go.mod": "module m\n", "m/vulns.txt": "GO-1 x.org/y v1.0.0\n"})
    fx.commit({"m/vulns.txt": "GO-1 x.org/y v1.0.0 called\n"})
    assert fx.run(base, [], ["m"]) == 1


def test_govulncheck_passes_a_new_advisory_the_base_calls_too(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    base = fx.commit({"m/go.mod": "module m\n", "m/vulns.txt": "GO-1 stdlib v1.27.2 called\n"})
    fx.commit({"m/x.go": "package m\n"})
    assert fx.run(base, [], ["m"]) == 0
    assert "GO-1" in capsys.readouterr().out
    assert not (fx.repo.parent / "wt").exists()
    assert _git(fx.repo, "worktree", "list").count("\n") == 0


def test_govulncheck_fails_for_a_new_module_with_a_called_vulnerability(fx: Fixture) -> None:
    base = fx.commit({"a/uv.lock": "PyPI ok 1.0\n"})
    fx.commit({"m/go.mod": "module m\n", "m/vulns.txt": "GO-1 x.org/y v1.0.0 called\n"})
    assert fx.run(base, [], ["m"]) == 1


def test_a_govulncheck_error_fails(fx: Fixture, monkeypatch: pytest.MonkeyPatch) -> None:
    base = fx.commit({"m/go.mod": "module m\n"})
    fx.commit({"m/x.go": "package m\n"})
    monkeypatch.setenv("FAKE_GV_EXIT", "1")
    assert fx.run(base, [], ["m"]) == 1


def test_an_unknown_base_fails(fx: Fixture) -> None:
    fx.commit({"a/uv.lock": "PyPI ok 1.0\n"})
    assert fx.run("0" * 40, ["a/uv.lock"]) == 1


# Gradle lock files: only what ships counts, in a pull request and in the full scan.

GRADLE_HEAD = "# generated\n"
SHIPPED = "com.example:bad-lib:1.0=debugRuntimeClasspath,releaseRuntimeClasspath\n"
BUILD_ONLY = "com.example:bad-tool:2.0=_internal-unified-test-platform-core,debugRuntimeClasspath\n"
CLEAN = "com.example:fine:3.0=releaseRuntimeClasspath\n"
LOCK = "app/android/gradle.lockfile"


def test_a_vulnerable_package_a_pull_request_adds_to_a_release_classpath_fails(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    base = fx.commit({LOCK: GRADLE_HEAD + CLEAN})
    fx.commit({LOCK: GRADLE_HEAD + CLEAN + SHIPPED})
    assert fx.run(base, [LOCK]) == 1
    out = capsys.readouterr().out
    assert f"{LOCK}: Maven com.example:bad-lib 1.0: GHSA-bad-lib" in out


def test_a_vulnerable_package_of_a_build_or_test_configuration_does_not_count(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    base = fx.commit({LOCK: GRADLE_HEAD + CLEAN})
    fx.commit({LOCK: GRADLE_HEAD + CLEAN + BUILD_ONLY})
    assert fx.run(base, [LOCK]) == 0
    out = capsys.readouterr().out
    assert "::error::" not in out
    assert f"{LOCK}: 1 package(s) with a known vulnerability" in out
    assert "do not ship" in out


def test_a_shipped_finding_of_the_base_is_a_warning_in_a_pull_request(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    base = fx.commit({LOCK: GRADLE_HEAD + SHIPPED})
    fx.commit({LOCK: GRADLE_HEAD + SHIPPED + BUILD_ONLY})
    assert fx.run(base, [LOCK]) == 0
    out = capsys.readouterr().out
    assert "::warning::" in out
    assert "GHSA-bad-lib" in out


def test_a_variant_lock_file_is_scanned_as_a_gradle_lock_file(fx: Fixture) -> None:
    """The scanner does not know the name `gradle-firebase.lockfile`; the argument names it."""
    variant = "app/android/gradle-firebase.lockfile"
    base = fx.commit({variant: GRADLE_HEAD + CLEAN})
    fx.commit({variant: GRADLE_HEAD + CLEAN + SHIPPED})
    assert fx.run(base, [variant]) == 1
    fx.commit({variant: GRADLE_HEAD + CLEAN})
    assert fx.run(base, [variant]) == 0


def test_a_gradle_lock_file_without_a_configuration_that_ships_fails(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    line = "com.example:bad-lib:1.0=stagingRuntimeClasspath\n"
    base = fx.commit({LOCK: GRADLE_HEAD + line})
    fx.commit({LOCK: GRADLE_HEAD + line, "x.txt": "y\n"})
    assert fx.run(base, [LOCK]) == 1
    assert "names no configuration that ships" in capsys.readouterr().out


def test_the_full_scan_fails_for_every_shipped_finding(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    """Without a base (main, the daily scan, a local run) nothing is excused as old."""
    fx.commit({LOCK: GRADLE_HEAD + SHIPPED + BUILD_ONLY})
    assert fx.run(None, [LOCK]) == 1
    out = capsys.readouterr().out
    assert f"::error::vulnerable dependency: {LOCK}: Maven com.example:bad-lib 1.0" in out
    assert "bad-tool" not in out
    assert "merge base" not in out


def test_the_full_scan_passes_when_only_build_and_test_packages_are_vulnerable(
    fx: Fixture, capsys: pytest.CaptureFixture[str]
) -> None:
    fx.commit({LOCK: GRADLE_HEAD + CLEAN + BUILD_ONLY})
    assert fx.run(None, [LOCK]) == 0
    assert "::error::" not in capsys.readouterr().out


def test_the_full_scan_takes_no_go_module() -> None:
    """govulncheck needs the base; the full scan of `make osv` runs it in its own step."""
    with pytest.raises(SystemExit):
        advisories.main(["--osv-scanner", "x", "--go-module", "m"])


# Workflow and Makefile wiring.


def _workflow(name: str) -> dict:
    return yaml.safe_load((REPO / ".github" / "workflows" / name).read_text(encoding="utf-8"))


def test_ci_gives_the_security_gate_the_base_only_for_a_pull_request_or_the_queue() -> None:
    steps = _workflow("ci.yml")["jobs"]["gates-security"]["steps"]
    (step,) = [s for s in steps if "osv" in s.get("run", "")]
    assert step["env"]["OSV_BASE"] == (
        "${{ github.event_name == 'pull_request' && github.event.pull_request.base.sha"
        " || github.event_name == 'merge_group' && github.event.merge_group.base_sha || '' }}"
    )
    assert 'OSV_BASE="$OSV_BASE"' in step["run"]


def _make_osv(tmp: Path, env_base: str | None) -> tuple[int, list[str], str]:
    """Run the real `make osv` with a fake OSV-Scanner (through FETCH) and a fake `go`."""
    calls = tmp / "go-calls.txt"
    bin_dir = tmp / "bin"
    bin_dir.mkdir()
    osv = bin_dir / "osv-scanner"
    osv.write_text(f"#!{sys.executable}\nprint('{{}}')\n")
    fetch = bin_dir / "fetch"
    fetch.write_text(f"#!/bin/sh\necho {osv}\n")
    go = bin_dir / "go"
    go.write_text(
        f'#!/bin/sh\necho "$*" >> {calls}\n'
        'echo \'{"config": {"scanner_name": "govulncheck"}}\'\n'
    )
    for f in (osv, fetch, go):
        f.chmod(0o755)
    env = {**os.environ, "PATH": f"{bin_dir}:{os.environ['PATH']}"}
    env.pop("OSV_BASE", None)
    if env_base is not None:
        env["OSV_BASE"] = env_base
    done = subprocess.run(
        ["make", "-s", "-C", str(REPO), "osv", f"FETCH={fetch}", "COMPONENTS=all"],
        check=False,
        capture_output=True,
        text=True,
        env=env,
    )
    lines = calls.read_text().splitlines() if calls.exists() else []
    return done.returncode, lines, done.stdout + done.stderr


def test_the_osv_target_with_a_base_runs_only_the_diff(tmp_path: Path) -> None:
    rc, go_calls, out = _make_osv(tmp_path, "HEAD")
    assert rc == 0, out
    assert "advisories: findings of the merge base" in out
    # govulncheck runs once per Go module, in JSON mode for the diff, never in full text mode.
    assert go_calls
    assert all("-format json" in c for c in go_calls), go_calls


def test_the_osv_target_without_a_base_scans_in_full(tmp_path: Path) -> None:
    rc, go_calls, out = _make_osv(tmp_path, None)
    assert rc == 0, out
    assert "advisories:" not in out
    assert go_calls
    assert not any("-format json" in c for c in go_calls), go_calls


def test_a_daily_schedule_runs_the_full_scan() -> None:
    wf = _workflow("advisories.yml")
    on = wf[True]  # YAML 1.1 reads the key `on` as true
    assert "schedule" in on
    assert "workflow_dispatch" in on
    runs = [s.get("run", "") for job in wf["jobs"].values() for s in job["steps"]]
    assert any("make osv" in r for r in runs)
    assert not any("OSV_BASE" in r for r in runs)


def test_the_fake_scanner_prints_the_shape_of_the_real_one(fx: Fixture, tmp_path: Path) -> None:
    lock = tmp_path / "uv.lock"
    lock.write_text("PyPI bad 2.0 GHSA-new\n")
    out = subprocess.run(
        [sys.executable, str(fx.osv), "scan", "source", "--format", "json", "-L", str(lock)],
        check=False,
        capture_output=True,
        text=True,
    )
    pkg = json.loads(out.stdout)["results"][0]["packages"][0]
    assert set(pkg) >= {"package", "groups"}
    assert set(pkg["groups"][0]) >= {"ids", "experimental_analysis"}
