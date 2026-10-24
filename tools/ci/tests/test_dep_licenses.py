# SPDX-License-Identifier: Apache-2.0
"""The dependency license step of `make licenses` judges licenses only (PL/32).

OSV-Scanner has no mode that checks licenses without vulnerabilities: `scan source
--licenses=...` also matches every package against the advisory database and exits 1 for a
vulnerability. tools/ci/dep_licenses.py reads its JSON report and fails only for license
violations, so a new public advisory turns only the `osv` step red (which judges advisories
against the merge base, PL/25). The fake scanner prints the JSON shape of OSV-Scanner v2.6.0
(`results[].packages[].license_violations`, `vulnerabilities`) and exits like it.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import dep_licenses
import pytest

# Fake OSV-Scanner: each lock file holds lines "<name> <version> <license> [vuln <id>]"; a
# license outside --licenses is a violation. FAKE_OSV_EXIT forces an exit code, FAKE_OSV_RAW
# replaces the report.
FAKE_OSV = r"""
import json, os, sys
args = sys.argv[1:]
assert args[:2] == ["scan", "source"], args
assert "--format" in args and args[args.index("--format") + 1] == "json", args
allowed = [a.split("=", 1)[1] for a in args if a.startswith("--licenses=")][0].split(",")
locks = [args[i + 1] for i, a in enumerate(args) if a == "-L"]
with open(os.environ["FAKE_OSV_LOG"], "a") as fh:
    fh.write(json.dumps(args) + "\n")
results, failing = [], False
for lock in locks:
    pkgs = []
    for line in open(lock).read().splitlines():
        name, version, lic, *rest = line.split()
        pkg = {"package": {"name": name, "version": version, "ecosystem": "npm"},
               "licenses": [lic]}
        if lic not in allowed:
            pkg["license_violations"] = [lic]
            failing = True
        if rest[:1] == ["vuln"]:
            pkg["vulnerabilities"] = [{"id": rest[1]}]
            pkg["groups"] = [{"ids": [rest[1]]}]
            failing = True
        pkgs.append(pkg)
    results.append({"source": {"path": os.path.abspath(lock), "type": "lockfile"},
                    "packages": pkgs})
if "FAKE_OSV_RAW" in os.environ:
    print(os.environ["FAKE_OSV_RAW"])
else:
    print(json.dumps({"results": results}))
sys.exit(int(os.environ.get("FAKE_OSV_EXIT", "1" if failing else "0")))
"""


class Fixture:
    def __init__(self, tmp: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        self.tmp = tmp
        self.osv = tmp / "fake_osv.py"
        self.osv.write_text(f"#!{sys.executable}\n" + FAKE_OSV)
        self.osv.chmod(0o755)
        self.log = tmp / "osv.log"
        monkeypatch.setenv("FAKE_OSV_LOG", str(self.log))

    def lock(self, name: str, lines: list[str]) -> str:
        path = self.tmp / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("\n".join(lines) + "\n")
        return str(path)

    def run(self, *locks: str, allowed: str = "MIT,Apache-2.0") -> int:
        argv = ["--osv-scanner", str(self.osv), "--licenses", allowed]
        for lock in locks:
            argv += ["-L", lock]
        return dep_licenses.main(argv)

    def calls(self) -> list[list[str]]:
        return [json.loads(line) for line in self.log.read_text().splitlines()]


@pytest.fixture
def fx(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Fixture:
    return Fixture(tmp_path, monkeypatch)


def test_an_advisory_alone_passes(fx: Fixture, capsys: pytest.CaptureFixture[str]) -> None:
    # The case of PR #299: no dependency change, the lock of the merge base has a package with a
    # new public advisory and only allowed licenses. The scanner exits 1; the step passes.
    lock = fx.lock("a/package-lock.json", ["fast-uri 3.1.8 BSD-3-Clause vuln GHSA-x", "b 1 MIT"])
    assert fx.run(lock, allowed="BSD-3-Clause,MIT") == 0
    out = capsys.readouterr().out
    assert "::error::" not in out
    assert "GHSA-x" not in out


def test_a_license_violation_fails(fx: Fixture, capsys: pytest.CaptureFixture[str]) -> None:
    lock = fx.lock("a/package-lock.json", ["glob 11.1.0 BlueOak-1.0.0", "b 1 MIT"])
    assert fx.run(lock) == 1
    out = capsys.readouterr().out
    assert "::error::" in out
    assert "glob 11.1.0" in out
    assert "BlueOak-1.0.0" in out
    assert " b 1" not in out


def test_a_violation_fails_next_to_an_advisory(fx: Fixture) -> None:
    lock = fx.lock("a/package-lock.json", ["a 1 GPL-3.0 vuln GHSA-y"])
    assert fx.run(lock) == 1


def test_every_lock_goes_to_one_scan_with_the_allowlist(fx: Fixture) -> None:
    one = fx.lock("a/uv.lock", ["a 1 MIT"])
    two = fx.lock("b/pubspec.lock", ["b 1 Apache-2.0"])
    assert fx.run(one, two) == 0
    [args] = fx.calls()
    assert [args[i + 1] for i, a in enumerate(args) if a == "-L"] == [one, two]
    assert "--licenses=MIT,Apache-2.0" in args


@pytest.mark.parametrize(
    ("env", "why"),
    [
        ({"FAKE_OSV_EXIT": "127"}, "scanner error"),
        ({"FAKE_OSV_EXIT": "128"}, "no package found"),
        ({"FAKE_OSV_EXIT": "1", "FAKE_OSV_RAW": "not json"}, "no JSON"),
        ({"FAKE_OSV_EXIT": "1", "FAKE_OSV_RAW": ""}, "empty report on a failing scan"),
    ],
)
def test_a_scan_that_cannot_decide_fails(
    fx: Fixture, monkeypatch: pytest.MonkeyPatch, env: dict[str, str], why: str
) -> None:
    for key, value in env.items():
        monkeypatch.setenv(key, value)
    lock = fx.lock("a/package-lock.json", ["a 1 MIT"])
    assert fx.run(lock) == 1, why


def test_an_empty_allowlist_fails(fx: Fixture) -> None:
    # An empty --licenses= no longer limits the licenses (M1-05/23).
    lock = fx.lock("a/package-lock.json", ["a 1 GPL-3.0"])
    assert fx.run(lock, allowed="") == 1
    assert not fx.log.exists()
