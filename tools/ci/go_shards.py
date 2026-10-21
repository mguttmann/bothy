# SPDX-License-Identifier: Apache-2.0
"""Split the Go tests of `make test` into shards and merge their coverage profiles again.

The CI job `test (server)` runs as a matrix of shards (ci.yml, build-verify.yml): each shard runs
`make test GO_TEST_SHARD=<i>/<n>`, which runs `go_shards.py test` for every Go module: it tests
only the packages and package parts this script assigns to shard i, with the same flags as the
single run (`-race -covermode=atomic -coverpkg=./...`). The job `test (server coverage)`
downloads the shards' profiles, merges them per module and runs the coverage gate on the merged
profile, which SonarQube reads as before.

Assignment: every package of `go list ./...` in every Go module is one unit; a package of SPLITS
is K units instead, its parts 1/K to K/K, each weighing 1/K of the package. Units are sorted by
their measured duration (TIMINGS, longest first, then by name) and each goes to the shard with the
least total so far. A package without a measured duration (a new one) weighs the median, so it
still lands in exactly one shard. The split is deterministic: every shard computes the same one.

Parts: the top-level tests of a split package (the Test, Example and Fuzz functions of its
`_test.go` files) go into K groups of about equal measured duration (TIMINGS keeps them as
`<package>#<test>`, from the `-v` output of the parts; without them the names go round-robin);
part J < K runs `-run ^(group J)$`, part K `-skip` with the names of all other groups. So every
top-level test runs in exactly one part, a test the listing missed in the last one. A shard runs
its whole packages in one `go test` and each part in another (with `-v`) at the same time, after
one `go test -c` that compiles the module for all of them, and merges their profiles into the
module's profile.

Merge: `go test ./...` concatenates the profiles of its test binaries below one mode line; the
merge does the same with the shards' profiles. Every block of the single run is in the merged
profile with the counts its test binary wrote, so `go tool cover` and SonarQube compute the same
coverage (up to blocks that timing-dependent tests reach in one run and not in another); only the
copies with count 0 that the single run repeats for the packages a test binary does not link are
fewer, and a split package adds one copy per part. Before it merges, it checks that every shard
1..n ran, that every package of `go list ./...` (every part of a split one) ran in exactly one
shard, that the filters of the parts run each top-level test once, and that every shard wrote the
profile of each module it tested.

Usage:
  go_shards.py test --shard I/N --module M [--timings F] [--record F] [--tags T] MODULE... -- FLAGS
      run `go test FLAGS` in module M for its packages and parts in shard I of N (FLAGS need one
      -coverprofile=FILE); --record appends them to F
  go_shards.py packages --shard I/N --module M [--timings F] [--record F] [--tags T] MODULE...
      print the packages of module M in shard I of N (one per line, the longest one first; a
      part with its filter flag and pattern); --record appends them to F
  go_shards.py merge --from DIR --to ROOT [--tags T]
      merge the shard folders below DIR (each with RECORD and <module>/coverage.out) into
      ROOT/<module>/coverage.out
  go_shards.py timings LOG...
      print the mean duration per package (and per test of a split package) of `go test` logs
      as JSON (to refresh TIMINGS); a part of K counts K times, as an estimate of the package

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
import tempfile
from collections.abc import Iterable, Sequence
from pathlib import Path
from typing import IO, NamedTuple

TIMINGS = Path(__file__).resolve().with_name("go_test_timings.json")
# The list of packages a shard ran, written by `packages --record` and read by `merge`.
RECORD = "go-shard.txt"
PROFILE = "coverage.out"
_RESULT = re.compile(r"^(?:\S+\s)?(?:ok|FAIL)\s+(\S+)\s+(\d+(?:\.\d+)?)s\b")
# The line `test` prints before the output of a part, so `timings` counts the part as its share.
_MARKER = re.compile(r"^(?:\S+\s)?go-shard: part (\d+)/(\d+) of (\S+)$")
# The result line of a top-level test in the -v output of a part.
_TEST_RESULT = re.compile(r"^(?:\S+\s)?--- (?:PASS|FAIL|SKIP): ([^\s/]+) \((\d+(?:\.\d+)?)s\)$")


# Packages whose tests are split across shards, with the number of parts: alone each would set
# the floor of the slowest shard. See part_filter() for how the parts divide the tests.
SPLITS = {"github.com/OWNER/bothy/services/server/internal/wire": 2}
_TEST_FUNC = re.compile(r"^func\s+((?:Test|Example|Fuzz)(?![a-z])\w*)\s*\(", re.MULTILINE)
_NAMES = re.compile(r"\^\(([\w|]*)\)\$")


class Unit(NamedTuple):
    """One Go package of one module, or one part `J/K` of a split package.

    The smallest piece a shard runs.
    """

    module: str
    package: str
    part: str = ""


def expand(units: Iterable[Unit], splits: dict[str, int]) -> list[Unit]:
    """`units` with every package of `splits` replaced by one unit per part (1/K to K/K)."""
    out: list[Unit] = []
    for unit in units:
        parts = splits.get(unit.package, 1)
        if parts < 2:
            out.append(unit)
        else:
            out += [Unit(unit.module, unit.package, f"{j}/{parts}") for j in range(1, parts + 1)]
    return out


def _parts(unit: Unit) -> int:
    return int(unit.part.split("/")[1]) if unit.part else 1


def top_level_tests(directory: Path) -> list[str]:
    """The sorted tests, examples and fuzz targets of the `_test.go` files in `directory`.

    These are the names `go test -run` matches at the top level; TestMain is no test.
    """
    names = {
        name
        for path in directory.glob("*_test.go")
        for name in _TEST_FUNC.findall(path.read_text(encoding="utf-8"))
    }
    return sorted(names - {"TestMain"})


def part_filter(
    names: Sequence[str], part: int, parts: int, weights: dict[str, float] | None = None
) -> tuple[str, str]:
    """The `go test` filter (flag, pattern) of part `part` of `parts` of a package.

    The top-level tests `names` go into `parts` groups of about equal measured duration
    (`weights`, longest first, each to the lighter group; an unmeasured test weighs the median,
    so without measurements the sorted names go round-robin). Part J < K runs group J
    (`-run ^(...)$`), the last part skips the groups of all other parts (`-skip ^(...)$`). Both
    filters match whole top-level names only, so every top-level test runs in exactly one part,
    also one the listing missed: it is in no group and runs in the last part.
    """
    weights = weights or {}
    known = [weights[n] for n in names if n in weights]
    default = statistics.median(known) if known else 1.0
    groups: list[list[str]] = [[] for _ in range(parts)]
    loads = [0.0] * parts
    for negative, name in sorted({(-weights.get(n, default), n) for n in names}):
        target = min(range(parts), key=lambda i: (loads[i], i))
        groups[target].append(name)
        loads[target] -= negative
    groups = [sorted(g) for g in groups]
    if part < parts:
        return "-run", "^(" + "|".join(groups[part - 1]) + ")$"
    return "-skip", "^(" + "|".join(n for g in groups[:-1] for n in g) + ")$"


def pattern_names(pattern: str) -> list[str]:
    """The names of a filter pattern of part_filter()."""
    match = _NAMES.fullmatch(pattern)
    if not match:
        raise ValueError(f"not a filter of whole test names: {pattern!r}")
    return [n for n in match[1].split("|") if n]


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


def list_tests(module: str, package: str) -> list[str]:
    """The top-level tests of `package`, a package of the Go module in the directory `module`."""
    gomod = (Path(module) / "go.mod").read_text(encoding="utf-8")
    match = re.search(r"^module\s+(\S+)", gomod, re.MULTILINE)
    if not match or not package.startswith(match[1] + "/"):
        raise ValueError(f"{package} is not a package of the module in {module}")
    return top_level_tests(Path(module) / package[len(match[1]) + 1 :])


def load_timings(path: Path) -> dict[str, float]:
    """The measured duration in seconds per import path."""
    data = json.loads(path.read_text(encoding="utf-8"))
    return {str(k): float(v) for k, v in data.items()}


def assign(units: Iterable[Unit], timings: dict[str, float], total: int) -> list[list[Unit]]:
    """Split `units` into `total` shards of about equal measured duration (see module docs)."""
    units = list(units)
    known = [timings[u.package] for u in units if u.package in timings]
    default = statistics.median(known) if known else 1.0

    def weight(unit: Unit) -> float:
        # A part weighs its own measured duration, else its share of the package.
        own = timings.get(f"{unit.package}@{unit.part}") if unit.part else None
        return own if own is not None else timings.get(unit.package, default) / _parts(unit)

    weighted = sorted({(-weight(u), u) for u in units})
    shards: list[list[Unit]] = [[] for _ in range(total)]
    loads = [0.0] * total
    for negative, unit in weighted:
        target = min(range(total), key=lambda i: (loads[i], i))
        shards[target].append(unit)
        loads[target] -= negative
    return shards


def parse_timings(logs: Sequence[Path]) -> dict[str, float]:
    """The mean durations of the `go test` logs.

    Per package from its `ok`/`FAIL` result lines, per part of a split package as
    `<package>@<J>/<K>` and per top-level test of a part (from its `-v` output) as
    `<package>#<test>`.
    """
    seen: dict[str, list[float]] = {}
    for log in logs:
        parts: dict[str, str] = {}  # the part J/K whose output follows, per split package
        current = ""  # the split package whose part output this is
        for line in log.read_text(encoding="utf-8", errors="replace").splitlines():
            marker = _MARKER.match(line)
            if marker:
                parts[marker[3]] = f"{marker[1]}/{marker[2]}"
                current = marker[3]
            test = _TEST_RESULT.match(line)
            if test and current:
                seen.setdefault(f"{current}#{test[1]}", []).append(float(test[2]))
            match = _RESULT.match(line)
            if match and match[1] == current:
                current = ""
            if match and match[1] in parts:
                # The part itself, and K times it as an estimate of the whole package.
                seen.setdefault(f"{match[1]}@{parts[match[1]]}", []).append(float(match[2]))
                seen.setdefault(match[1], []).append(
                    float(match[2]) * int(parts.pop(match[1]).split("/")[1])
                )
            elif match:
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


def _shard(args: argparse.Namespace) -> tuple[int, int, list[tuple[Unit, str, str]]]:
    """The units of module `args.module` in the shard, each with its test filter.

    The filter is empty for a whole package; `args.record` gets the units appended.
    """
    index, count = parse_shard(args.shard)
    if args.module not in args.modules:
        raise ValueError(f"module {args.module} is not among the modules {args.modules}")
    units = [Unit(m, p) for m in args.modules for p in list_packages(m, args.tags)]
    # `go test` starts packages roughly in the order given. The longest package of the module in
    # this shard goes first, so it does not start late and end the shard late; the others follow
    # in `go list` order, which mixes long and short ones (all long ones at once slowed every
    # package of a shard down: they compete for the CPUs and the databases).
    timings = load_timings(args.timings)
    shard = assign(expand(units, SPLITS), timings, count)[index - 1]
    own = [u for u in shard if u.module == args.module]  # longest first, as assign() lists them
    order = {(u.module, u.package): i for i, u in enumerate(units)}
    mine = own[:1] + sorted(own[1:], key=lambda u: (order[(u.module, u.package)], u.part))
    planned: list[tuple[Unit, str, str]] = []
    for unit in mine:
        if unit.part:
            part, parts = parse_shard(unit.part)
            names = list_tests(unit.module, unit.package)
            weights = {
                n: timings[f"{unit.package}#{n}"] for n in names if f"{unit.package}#{n}" in timings
            }
            flag, pattern = part_filter(names, part, parts, weights)
            planned.append((unit, flag, pattern))
        else:
            planned.append((unit, "", ""))
    if args.record:
        args.record.parent.mkdir(parents=True, exist_ok=True)
        with args.record.open("a", encoding="utf-8") as fh:
            fh.write(f"{index}/{count}\t{args.module}\n")
            for unit, flag, pattern in planned:
                extra = f"\t{unit.part}\t{flag}\t{pattern}" if unit.part else ""
                fh.write(f"{index}/{count}\t{args.module}\t{unit.package}{extra}\n")
    return index, count, planned


def _packages(args: argparse.Namespace) -> int:
    for unit, flag, pattern in _shard(args)[2]:
        print("\t".join([unit.package, flag, pattern]) if flag else unit.package)
    return 0


def _merge(args: argparse.Namespace) -> int:
    records = sorted(args.source.glob(f"*/{RECORD}"))
    if not records:
        raise ValueError(f"no shard record ({RECORD}) below {args.source}")
    shards: dict[int, tuple[Path, set[str]]] = {}
    tested: dict[int, set[str]] = {}
    counts: set[int] = set()
    ran: dict[Unit, int] = {}
    filters: dict[str, list[tuple[str, str]]] = {}
    for record in records:
        for line in record.read_text(encoding="utf-8").splitlines():
            fields = line.split("\t")
            if len(fields) not in (2, 3, 6):
                raise ValueError(f"{record}: not a record line: {line!r}")
            index, count = parse_shard(fields[0])
            counts.add(count)
            folder, listed = shards.setdefault(index, (record.parent, set()))
            if folder != record.parent:
                raise ValueError(f"shard {index} has two records: {folder} and {record.parent}")
            listed.add(fields[1])
            if len(fields) > 2:
                unit = Unit(fields[1], fields[2], fields[3] if len(fields) == 6 else "")
                if unit.part:
                    parse_shard(unit.part)
                    filters.setdefault(unit.package, []).append((fields[4], fields[5]))
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
    found = [Unit(m, p) for m in modules for p in list_packages(m, args.tags)]
    expected = set(expand(found, SPLITS))
    problems += [f"{_name(u)} ran in no shard" for u in sorted(expected - set(ran))]
    problems += [f"{_name(u)} ran in {n} shards" for u, n in sorted(ran.items()) if n > 1]
    problems += [f"{_name(u)} is not a package of {u.module}" for u in sorted(set(ran) - expected)]
    problems += [
        f"the parts of {package} do not run every test exactly once"
        for package, given in sorted(filters.items())
        if not _cover_once(given)
    ]
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


def _test(args: argparse.Namespace) -> int:
    """Run `go test` with the flags `args.go_flags` for the units of the shard.

    One process for the whole packages and one per part run at the same time; then the profile
    of the module is the merge of theirs, as `go test` writes it for its packages.
    """
    flags = list(args.go_flags)
    profiles = [a for a in flags if a.startswith("-coverprofile=")]
    if len(profiles) != 1:
        raise ValueError("the go test flags need exactly one -coverprofile=FILE")
    index, count, planned = _shard(args)
    if not planned:
        print(f"  go-test: no package of {args.module} in shard {index}/{count}")
        return 0
    module = Path(args.module)
    profile = Path(profiles[0].split("=", 1)[1])
    whole = [u.package for u, flag, _ in planned if not flag]
    parts = [(u, flag, pattern) for u, flag, pattern in planned if flag]
    if len(parts) + bool(whole) > 1:
        # Processes that start at the same time would each compile the module (-coverpkg=./...)
        # on their own; one build first leaves every package compiled for all of them.
        build = [a for a in flags if not a.startswith(("-coverprofile=", "-timeout="))]
        with tempfile.TemporaryDirectory() as tmp:
            binary = str(Path(tmp) / "part.test")
            done = subprocess.run(  # nosec B603 B607 - go test -c with the Makefile's flags
                ["go", "test", "-c", "-o", binary, *build, parts[0][0].package],
                cwd=module,
                check=False,
            )
        if done.returncode != 0:
            return 1
    running: list[tuple[subprocess.Popen[bytes], Unit | None, Path, IO[bytes] | None]] = []
    if whole:
        main = subprocess.Popen(["go", "test", *flags, *whole], cwd=module)  # nosec B603 B607
        running.append((main, None, profile, None))
    for number, (unit, flag, pattern) in enumerate(parts, 1):
        own = profile.with_name(f"{profile.stem}.part{number}{profile.suffix}")
        args_ = [f"-coverprofile={own}" if a == profiles[0] else a for a in flags]
        out = tempfile.TemporaryFile()  # noqa: SIM115 - closed below, after the process ended
        proc = subprocess.Popen(  # nosec B603 B607 - go test with the Makefile's flags
            ["go", "test", *args_, "-v", flag, pattern, unit.package],
            cwd=module,
            stdout=out,
            stderr=subprocess.STDOUT,
        )
        running.append((proc, unit, own, out))
    failed = False
    for proc, owner, _, output in running:
        failed |= proc.wait() != 0
        if owner is not None and output is not None:
            print(f"go-shard: part {owner.part} of {owner.package}", flush=True)
            output.seek(0)
            sys.stdout.buffer.write(output.read())
            sys.stdout.buffer.flush()
            output.close()
    if failed:
        return 1
    written = [module / p for _, _, p, _ in running]
    merged = module / profile.with_name(f"{profile.stem}.merged{profile.suffix}")
    merge_profiles(written, merged)
    for path in written:
        path.unlink()
    merged.replace(module / profile)
    return 0


def _cover_once(filters: Sequence[tuple[str, str]]) -> bool:
    """Whether the filters of the parts of a package run each top-level test exactly once.

    That holds when the -run groups are disjoint and the one -skip part skips exactly them.
    """
    run = [n for flag, pattern in filters if flag == "-run" for n in pattern_names(pattern)]
    skips = [pattern_names(pattern) for flag, pattern in filters if flag == "-skip"]
    return len(run) == len(set(run)) and len(skips) == 1 and sorted(skips[0]) == sorted(run)


def _name(unit: Unit) -> str:
    return f"{unit.package} part {unit.part}" if unit.part else unit.package


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
    test = sub.add_parser("test")
    test.add_argument("--shard", required=True)
    test.add_argument("--module", required=True)
    test.add_argument("--timings", type=Path, default=TIMINGS)
    test.add_argument("--record", type=Path)
    test.add_argument("--tags", default="", help="the build tags of `go test`")
    test.add_argument("modules", nargs="+")
    test.set_defaults(run=_test)
    timings = sub.add_parser("timings")
    timings.add_argument("logs", type=Path, nargs="+")
    timings.set_defaults(run=_timings)
    argv = list(sys.argv[1:] if argv is None else argv)
    go_flags = argv[argv.index("--") + 1 :] if "--" in argv else []
    args = parser.parse_args(argv[: len(argv) - len(go_flags) - 1] if "--" in argv else argv)
    args.go_flags = go_flags
    try:
        code: int = args.run(args)
    except ValueError as exc:
        print(f"go_shards: {exc}", file=sys.stderr)
        return 1
    return code


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
