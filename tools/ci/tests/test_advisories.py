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
import subprocess
import sys
from pathlib import Path

import advisories
import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]

# Fake OSV-Scanner: each lock file holds lines "<ecosystem> <name> <version> <id> [uncalled]";
# a line without an id is a package without a known vulnerability.
FAKE_OSV = r"""
import json, sys
args = sys.argv[1:]
assert args[:2] == ["scan", "source"], args
locks = [args[i + 1] for i, a in enumerate(args) if a == "-L"]
results, failing = [], False
for lock in locks:
    pkgs = []
    for line in open(lock).read().splitlines():
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
        import os
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
        base: str,
        locks: list[str],
        mods: tuple[str, ...] | list[str] = (),
    ) -> int:
        argv = ["--repo", str(self.repo), "--base", base]
        argv += ["--osv-scanner", str(self.osv)]
        argv += ["--govulncheck", f"{sys.executable} {self.gv}"]
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


# Workflow and Makefile wiring.


def _workflow(name: str) -> dict:
    return yaml.safe_load((REPO / ".github" / "workflows" / name).read_text(encoding="utf-8"))


def test_ci_gives_the_security_gate_the_base_only_for_a_pull_request() -> None:
    steps = _workflow("ci.yml")["jobs"]["gates-security"]["steps"]
    (step,) = [s for s in steps if "osv" in s.get("run", "")]
    assert step["env"]["OSV_BASE"] == (
        "${{ github.event_name == 'pull_request' && github.event.pull_request.base.sha || '' }}"
    )
    assert 'OSV_BASE="$OSV_BASE"' in step["run"]


def test_the_osv_target_diffs_only_with_a_base() -> None:
    make = (REPO / "Makefile").read_text()
    recipe = make.split("\nosv:", 1)[1].split("\n\n", 1)[0]
    assert "OSV_BASE" in recipe
    assert "tools/ci/advisories.py" in recipe


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
