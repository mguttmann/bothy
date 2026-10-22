# SPDX-License-Identifier: Apache-2.0
"""Advisory gate of a pull request: fail only for vulnerable dependencies the pull request adds.

A new public advisory turns the full scan red for every branch at once, although none of them
changed the affected dependency. In a pull request (`make osv` with `OSV_BASE`, set by ci.yml)
this tool keeps the scanners' own verdict but drops the findings that the merge base of the
pull request has as well (docs/DECISIONS.md PL/25, docs/quality/gates.md "Advisories"):

- OSV-Scanner over the given lock files. When it fails, every failing package (a group with a
  called vulnerability) counts as new unless the same lock file of the merge base holds the
  same package at the same version: added packages, changed versions and lock files new to the
  pull request fail. The base lock files (with the `osv-scanner.toml` files of the base) are
  scanned from a temporary tree with the same database at the same time, so a package of the
  base shows up there exactly when it has the vulnerability today.
- govulncheck per Go module (JSON mode). A called finding (a trace with a function) counts as
  new unless the merge base, checked out in a temporary worktree, has the same finding: same
  OSV entry, module and version. So a pull request that adds a call into a vulnerable
  dependency fails too.

Findings the base already has are printed as GitHub warnings; the full scan on pushes to main
and the daily schedule (advisories.yml) still fail on them. A scanner error, a failing scan
without a readable finding and a failed base scan never pass: the head findings then all count
as new.

Usage:
  advisories.py --base REF --osv-scanner PATH --govulncheck CMD
                [--lock PATH]... [--go-module DIR]... [--repo DIR]
Exit codes: 0 nothing new, 1 a new finding or an error.
"""

from __future__ import annotations

import argparse
import json
import shlex
import subprocess
import sys
import tempfile
from collections.abc import Iterable
from pathlib import Path
from typing import Any

# (lock file relative to the tree, ecosystem, name, version)
OsvKey = tuple[str, str, str, str]
# (OSV id, module, version)
GoKey = tuple[str, str, str]
# Prefix of a finding the merge base has too; the full scan of main and the daily one fail on it.
OLD_NOTE = "advisory the base has too (main and the daily scan fail on it)"


class GateError(Exception):
    """A scanner or git failed; the gate cannot decide and fails."""


def _git(repo: Path, *args: str) -> str:
    done = subprocess.run(
        ["git", "-C", str(repo), *args], check=False, capture_output=True, text=True
    )
    if done.returncode != 0:
        raise GateError(f"git {' '.join(args)}: {done.stderr.strip()}")
    return done.stdout


def _rel(path: str, root: Path) -> str:
    try:
        return Path(path).resolve().relative_to(root.resolve()).as_posix()
    except ValueError:
        return path


def _called(group: dict[str, Any]) -> bool:
    """A group fails the scan unless its analysis marks every id as not called or unimportant."""
    analysis = group.get("experimental_analysis") or {}
    for vid in group.get("ids") or []:
        a = analysis.get(vid)
        if not a or (a.get("called", True) and not a.get("unimportant", False)):
            return True
    return False


def osv_findings(
    report: dict[str, Any], root: Path, *, failing_only: bool
) -> dict[OsvKey, list[str]]:
    """Packages of an OSV-Scanner JSON report with their vulnerability ids."""
    found: dict[OsvKey, list[str]] = {}
    for result in report.get("results") or []:
        src = _rel((result.get("source") or {}).get("path", ""), root)
        for pkg in result.get("packages") or []:
            groups = pkg.get("groups") or []
            if failing_only:
                groups = [g for g in groups if _called(g)]
            ids = sorted({i for g in groups for i in g.get("ids") or []})
            if failing_only and not ids:
                continue
            p = pkg.get("package") or {}
            key = (src, p.get("ecosystem", ""), p.get("name", ""), p.get("version", ""))
            found[key] = sorted(set(found.get(key, [])) | set(ids))
    return found


def run_osv(cmd: list[str], locks: list[str], cwd: Path) -> tuple[int, dict[str, Any]]:
    """Run OSV-Scanner in JSON mode; exit code 0 or 1 and the report, else a GateError."""
    args = [*cmd, "scan", "source", "--format", "json"]
    for lock in locks:
        args += ["-L", lock]
    done = subprocess.run(args, check=False, cwd=cwd, capture_output=True, text=True)
    sys.stderr.write(done.stderr)
    if done.returncode not in (0, 1):
        raise GateError(f"osv-scanner exited with {done.returncode}")
    try:
        return done.returncode, json.loads(done.stdout or "{}")
    except json.JSONDecodeError as err:
        raise GateError(f"osv-scanner printed no JSON: {err}") from err


def base_tree(repo: Path, base: str, locks: Iterable[str], dest: Path) -> Path:
    """Write the lock files and osv-scanner.toml files of `base` to `dest` at their paths."""
    tracked = set(_git(repo, "ls-tree", "-r", "--name-only", base).splitlines())
    wanted = [p for p in locks if p in tracked]
    wanted += [p for p in tracked if Path(p).name == "osv-scanner.toml"]
    for path in wanted:
        blob = subprocess.run(
            ["git", "-C", str(repo), "show", f"{base}:{path}"], capture_output=True, check=False
        )
        if blob.returncode != 0:
            raise GateError(f"git show {base}:{path} failed")
        target = dest / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(blob.stdout)
    return dest


def osv_gate(repo: Path, base: str, cmd: list[str], locks: list[str]) -> list[str]:
    """Return the error lines of new OSV-Scanner findings; print the old ones as warnings."""
    if not locks:
        return []
    rc, report = run_osv(cmd, locks, repo)
    if rc == 0:
        return []
    head = osv_findings(report, repo, failing_only=True)
    if not head:
        return ["osv-scanner failed, but its report names no failing package"]
    old: set[OsvKey] = set()
    with tempfile.TemporaryDirectory(prefix="osv-base-") as tmp:
        tree = base_tree(repo, base, locks, Path(tmp))
        present = [lock for lock in locks if (tree / lock).is_file()]
        if present:
            try:
                _, base_report = run_osv(cmd, [str(tree / p) for p in present], tree)
                old = set(osv_findings(base_report, tree, failing_only=False))
            except GateError as err:
                print(f"::warning::osv base scan failed ({err}); every finding counts as new")
    errors = []
    for key, ids in sorted(head.items()):
        src, eco, name, version = key
        line = f"{src}: {eco} {name} {version}: {', '.join(ids)}"
        if key in old:
            print(f"::warning::{OLD_NOTE}: {line}")
        else:
            errors.append(f"vulnerable dependency added or changed by this pull request: {line}")
    return errors


def govulncheck_findings(cmd: list[str], module: Path) -> dict[GoKey, str]:
    """Called findings of govulncheck in JSON mode, with the fixed version where known."""
    done = subprocess.run(
        [*cmd, "-format", "json", "./..."], check=False, cwd=module, capture_output=True, text=True
    )
    sys.stderr.write(done.stderr)
    if done.returncode != 0:
        raise GateError(f"govulncheck in {module} exited with {done.returncode}")
    found: dict[GoKey, str] = {}
    dec, text, i = json.JSONDecoder(), done.stdout, 0
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            break
        try:
            msg, i = dec.raw_decode(text, i)
        except json.JSONDecodeError as err:
            raise GateError(f"govulncheck in {module} printed no JSON: {err}") from err
        finding = msg.get("finding") if isinstance(msg, dict) else None
        if not finding:
            continue
        trace = finding.get("trace") or [{}]
        if not trace[0].get("function"):
            continue
        key = (finding.get("osv", ""), trace[0].get("module", ""), trace[0].get("version", ""))
        found[key] = finding.get("fixed_version", "")
    return found


def go_gate(repo: Path, base: str, cmd: list[str], modules: list[str]) -> list[str]:
    """Return the error lines of new called govulncheck findings; print the old ones."""
    heads = {m: govulncheck_findings(cmd, repo / m) for m in modules}
    if not any(heads.values()):
        return []
    errors = []
    with tempfile.TemporaryDirectory(prefix="govuln-base-") as tmp:
        wt = Path(tmp) / "base"
        _git(repo, "worktree", "add", "-q", "--detach", str(wt), base)
        try:
            for module, head in heads.items():
                old: set[GoKey] = set()
                if head and (wt / module / "go.mod").is_file():
                    try:
                        old = set(govulncheck_findings(cmd, wt / module))
                    except GateError as err:
                        print(
                            f"::warning::govulncheck base scan failed ({err}); "
                            "every finding counts as new"
                        )
                for key, fixed in sorted(head.items()):
                    osv, mod, version = key
                    line = f"{module}: {osv} in {mod} {version}" + (
                        f" (fixed in {fixed})" if fixed else ""
                    )
                    if key in old:
                        print(f"::warning::{OLD_NOTE}: {line}")
                    else:
                        errors.append(f"reachable vulnerability added by this pull request: {line}")
        finally:
            _git(repo, "worktree", "remove", "--force", str(wt))
    return errors


def main(argv: list[str] | None = None) -> int:
    """Command line entry; see the module docstring."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--repo", type=Path, default=Path.cwd())
    parser.add_argument("--base", required=True)
    parser.add_argument("--osv-scanner", required=True)
    parser.add_argument("--govulncheck", required=True)
    parser.add_argument("--lock", action="append", default=[])
    parser.add_argument("--go-module", action="append", default=[])
    args = parser.parse_args(argv)
    repo = args.repo.resolve()
    try:
        merge_base = _git(repo, "merge-base", "HEAD", args.base).strip()
        print(f"advisories: findings of the merge base {merge_base} do not fail this pull request")
        errors = osv_gate(repo, merge_base, [args.osv_scanner], args.lock)
        errors += go_gate(repo, merge_base, shlex.split(args.govulncheck), args.go_module)
    except GateError as err:
        errors = [str(err)]
    for line in errors:
        print(f"::error::{line}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
