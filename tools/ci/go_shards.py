# SPDX-License-Identifier: Apache-2.0
"""Split the Go tests of `make test` into shards and merge their coverage profiles again.

The CI job `test (server)` runs as a matrix of shards (ci.yml, build-verify.yml): each shard runs
`make test GO_TEST_SHARD=<i>/<n>`, which tests only the packages this script assigns to shard i,
with the same flags as the single run (`-race -covermode=atomic -coverpkg=./...`). The job
`test (server coverage)` downloads the shards' profiles, merges them per module and runs the
coverage gate on the merged profile, which SonarQube reads as before.

Assignment: every package of `go list ./...` in every Go module is one unit; units are sorted by
their measured duration (TIMINGS, longest first, then by name) and each goes to the shard with the
least total so far. A package without a measured duration (a new one) weighs the median, so it
still lands in exactly one shard. The split is deterministic: every shard computes the same one.

Merge: `go test ./...` concatenates the profiles of its test binaries below one mode line; the
merge does the same with the shards' profiles. Every block of the single run is in the merged
profile with the counts its test binary wrote, so `go tool cover` and SonarQube compute the same
coverage (up to blocks that timing-dependent tests reach in one run and not in another); only the
copies with count 0 that the single run repeats for the packages a test binary does not link are
fewer. Before it merges, it checks that every shard
1..n ran, that every package of `go list ./...` ran in exactly one shard, and that every shard
wrote the profile of each module it tested.

Usage:
  go_shards.py packages --shard I/N --module M [--timings F] [--record F] [--tags T] MODULE...
      print the packages of module M in shard I of N (one per line, the longest one first); --record
      appends them to F
  go_shards.py merge --from DIR --to ROOT [--tags T]
      merge the shard folders below DIR (each with RECORD and <module>/coverage.out) into
      ROOT/<module>/coverage.out
  go_shards.py timings LOG...
      print the mean duration per package of `go test` logs as JSON (to refresh TIMINGS)

Exit codes: 0 done, 1 a check failed (the reason on stderr).
"""

from __future__ import annotations

import argparse
import json
import re
import shutil
import statistics
import subprocess  # nosec B404 - runs the fixed command `go list ./...`
import sys
from collections.abc import Iterable, Sequence
from pathlib import Path
from typing import NamedTuple

TIMINGS = Path(__file__).resolve().with_name("go_test_timings.json")
# The list of packages a shard ran, written by `packages --record` and read by `merge`.
RECORD = "go-shard.txt"
PROFILE = "coverage.out"
_RESULT = re.compile(r"^(?:\S+\s)?(?:ok|FAIL)\s+(\S+)\s+(\d+(?:\.\d+)?)s\b")


class Unit(NamedTuple):
    """One Go package of one module: the smallest piece a shard runs."""

    module: str
    package: str


def parse_shard(spec: str) -> tuple[int, int]:
    """Return (index, total) of a shard spec `I/N` with 1 <= I <= N."""
    match = re.fullmatch(r"(\d+)/(\d+)", spec)
    if not match:
        raise ValueError(f"shard {spec!r}: expected I/N, such as 1/4")
    index, total = int(match[1]), int(match[2])
    if not 1 <= index <= total:
        raise ValueError(f"shard {spec!r}: I must be between 1 and N")
    return index, total


def list_packages(module: str, tags: str = "") -> list[str]:
    """The import paths `go test -tags <tags> ./...` tests in `module` (a directory)."""
    args = ["go", "list", *(["-tags", tags] if tags else []), "./..."]
    done = subprocess.run(  # nosec B603 B607 - fixed arguments, the module from the Makefile
        args, cwd=module, capture_output=True, text=True, check=False
    )
    if done.returncode != 0:
        raise ValueError(f"go list ./... in {module} failed: {done.stderr.strip()}")
    return done.stdout.split()


def load_timings(path: Path) -> dict[str, float]:
    """The measured duration in seconds per import path."""
    data = json.loads(path.read_text(encoding="utf-8"))
    return {str(k): float(v) for k, v in data.items()}


def assign(units: Iterable[Unit], timings: dict[str, float], total: int) -> list[list[Unit]]:
    """Split `units` into `total` shards of about equal measured duration (see module docs)."""
    known = [timings[u.package] for u in units if u.package in timings]
    default = statistics.median(known) if known else 1.0
    weighted = sorted({(-timings.get(u.package, default), u) for u in units})
    shards: list[list[Unit]] = [[] for _ in range(total)]
    loads = [0.0] * total
    for negative, unit in weighted:
        target = min(range(total), key=lambda i: (loads[i], i))
        shards[target].append(unit)
        loads[target] -= negative
    return shards


def parse_timings(logs: Sequence[Path]) -> dict[str, float]:
    """The mean duration per package of the `ok`/`FAIL` result lines of `go test` logs."""
    seen: dict[str, list[float]] = {}
    for log in logs:
        for line in log.read_text(encoding="utf-8", errors="replace").splitlines():
            match = _RESULT.match(line)
            if match:
                seen.setdefault(match[1], []).append(float(match[2]))
    return {pkg: round(statistics.fmean(v), 3) for pkg, v in sorted(seen.items())}


def merge_profiles(profiles: Sequence[Path], out: Path) -> None:
    """Write one profile: the mode line once, then the blocks of every profile in order."""
    modes = set()
    for profile in profiles:
        with profile.open(encoding="utf-8") as fh:
            modes.add(fh.readline().strip())
    if len(modes) != 1 or not next(iter(modes)).startswith("mode: "):
        raise ValueError(f"the profiles have different or no mode lines: {sorted(modes)}")
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("w", encoding="utf-8") as dst:
        dst.write(next(iter(modes)) + "\n")
        for profile in profiles:
            with profile.open(encoding="utf-8") as src:
                src.readline()
                shutil.copyfileobj(src, dst)


def total(profile: Path) -> float:
    """The total statement coverage in percent, as `go tool cover -func` computes it."""
    blocks: dict[str, tuple[int, int]] = {}
    with profile.open(encoding="utf-8") as fh:
        fh.readline()
        for line in fh:
            block, statements, count = line.rsplit(" ", 2)
            old = blocks.get(block, (0, 0))[1]
            blocks[block] = (int(statements), old + int(count))
    every = sum(s for s, _ in blocks.values())
    covered = sum(s for s, c in blocks.values() if c)
    return 100.0 * covered / every if every else 0.0


def _packages(args: argparse.Namespace) -> int:
    index, count = parse_shard(args.shard)
    if args.module not in args.modules:
        raise ValueError(f"module {args.module} is not among the modules {args.modules}")
    units = [Unit(m, p) for m in args.modules for p in list_packages(m, args.tags)]
    # `go test` starts packages roughly in the order given. The longest package of the module in
    # this shard goes first, so it does not start late and end the shard late; the others follow
    # in `go list` order, which mixes long and short ones (all long ones at once slowed every
    # package of a shard down: they compete for the CPUs and the databases).
    shard = assign(units, load_timings(args.timings), count)[index - 1]
    own = [u for u in shard if u.module == args.module]  # longest first, as assign() lists them
    mine = [u.package for u in own[:1] + sorted(own[1:], key=units.index)]
    if args.record:
        args.record.parent.mkdir(parents=True, exist_ok=True)
        with args.record.open("a", encoding="utf-8") as fh:
            fh.write(f"{index}/{count}\t{args.module}\n")
            fh.writelines(f"{index}/{count}\t{args.module}\t{p}\n" for p in mine)
    for package in mine:
        print(package)
    return 0


def _merge(args: argparse.Namespace) -> int:
    records = sorted(args.source.glob(f"*/{RECORD}"))
    if not records:
        raise ValueError(f"no shard record ({RECORD}) below {args.source}")
    shards: dict[int, tuple[Path, set[str]]] = {}
    tested: dict[int, set[str]] = {}
    counts: set[int] = set()
    ran: dict[Unit, int] = {}
    for record in records:
        for line in record.read_text(encoding="utf-8").splitlines():
            fields = line.split("\t")
            if len(fields) not in (2, 3):
                raise ValueError(f"{record}: not a record line: {line!r}")
            index, count = parse_shard(fields[0])
            counts.add(count)
            folder, listed = shards.setdefault(index, (record.parent, set()))
            if folder != record.parent:
                raise ValueError(f"shard {index} has two records: {folder} and {record.parent}")
            listed.add(fields[1])
            if len(fields) == 3:
                unit = Unit(fields[1], fields[2])
                ran[unit] = ran.get(unit, 0) + 1
                tested.setdefault(index, set()).add(unit.module)
    if len(counts) != 1:
        raise ValueError(f"the shard counts differ: {sorted(counts)}")
    count = counts.pop()
    problems = [f"shard {i}/{count} is missing" for i in range(1, count + 1) if i not in shards]
    module_sets = {frozenset(m) for _, m in shards.values()}
    if len(module_sets) > 1:
        problems.append(f"the shards tested different modules: {sorted(map(sorted, module_sets))}")
    modules = sorted(set().union(*module_sets))
    expected = {Unit(m, p) for m in modules for p in list_packages(m, args.tags)}
    problems += [f"{u.package} ran in no shard" for u in sorted(expected - set(ran))]
    problems += [f"{u.package} ran in {n} shards" for u, n in sorted(ran.items()) if n > 1]
    problems += [f"{u.package} is not a package of {u.module}" for u in sorted(set(ran) - expected)]
    profiles: dict[str, list[Path]] = {m: [] for m in modules}
    for index, (folder, _) in sorted(shards.items()):
        for module in sorted(tested.get(index, set())):
            profile = folder / module / PROFILE
            if profile.is_file():
                profiles[module].append(profile)
            else:
                problems.append(f"shard {index}/{count}: {module}/{PROFILE} is missing")
    problems += [f"{m}: no shard wrote {PROFILE}" for m, paths in profiles.items() if not paths]
    if problems:
        raise ValueError("\n".join(problems))
    for module, paths in profiles.items():
        merge_profiles(paths, args.target / module / PROFILE)
        print(f"{module}/{PROFILE}: merged from {len(paths)} shards")
    return 0


def _timings(args: argparse.Namespace) -> int:
    print(json.dumps(parse_timings(args.logs), indent=2, sort_keys=True))
    return 0


def main(argv: list[str] | None = None) -> int:
    """Entry point; returns the exit code."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = parser.add_subparsers(dest="command", required=True)
    packages = sub.add_parser("packages")
    packages.add_argument("--shard", required=True)
    packages.add_argument("--module", required=True)
    packages.add_argument("--timings", type=Path, default=TIMINGS)
    packages.add_argument("--record", type=Path)
    packages.add_argument("--tags", default="", help="the build tags of `go test`")
    packages.add_argument("modules", nargs="+")
    packages.set_defaults(run=_packages)
    merge = sub.add_parser("merge")
    merge.add_argument("--from", dest="source", type=Path, required=True)
    merge.add_argument("--to", dest="target", type=Path, required=True)
    merge.add_argument("--tags", default="", help="the build tags the shards tested with")
    merge.set_defaults(run=_merge)
    timings = sub.add_parser("timings")
    timings.add_argument("logs", type=Path, nargs="+")
    timings.set_defaults(run=_timings)
    args = parser.parse_args(argv)
    try:
        code: int = args.run(args)
    except ValueError as exc:
        print(f"go_shards: {exc}", file=sys.stderr)
        return 1
    return code


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
