# SPDX-License-Identifier: Apache-2.0
"""Rerun failed tests in CI and report every rerun that passed: a flake.

Go: in CI (`CI=true`) every `go test` of the Makefile runs through gotestsum (`flakes.py go`). A
failed test runs again alone, as its top-level test (`-run '^Name$'`), up to RERUNS times; a
package that failed without a failed test (TestMain, an exit in a test) runs again as a whole,
with the filters of the first run. The run passes only when every rerun target passed in its last
attempt. It still fails for a test that fails in every attempt, for more than MAX_FAILURES failed
tests of one run (then nothing reruns), for a data race (WARNING: DATA RACE, never rerun), for a
panic or a test timeout, for a build error and for any other `go test` error, and with --budget
(the shards of `make test`, go_shards.py) for a test binary (a package, a part of one or a rerun)
that ran longer than half of its -timeout: the time budget, so a package that creeps towards its
timeout fails by name (PL/39). Locally `make test`
runs plain `go test` without reruns. In a pull request (FLAKE_BASE, the base commit, set by
ci.yml) a failed test of a package the pull request changes fails even when its rerun passed,
and it is not a flake: the pull request may have made it flaky (`changed_packages`).

Coverage: every `go test` gotestsum starts (the first run and each rerun) writes its own profile
FILE.attemptN instead of the -coverprofile=FILE of the flags (`flakes.py exec`, the raw command of
gotestsum), and FILE is their merge (go_shards.merge_profiles: the mode line once, then the blocks
of every attempt). A statement counts as covered when any attempt ran it, so a rerun adds to the
first run's profile instead of replacing it.

Flakes: gotestsum writes the pass and fail events of every attempt; a test with a failed attempt
and a passed last attempt is a flake (a top-level test only when none of its subtests is one; a
package only when none of its tests is one). Dart: `flutter test` runs each test once more after a
failure (`retry: 1` of the preset `ci` in each member's dart_test.yaml, switched on in CI through
DART_TEST_CONFIG); `flakes.py dart` reads its JSON report: a test that reported an error and
passed. Both write the flakes to DIR/<tool>-<id>.json (FLAKE_DIR of the Makefile; the CI jobs
upload it as the artifact flakes-<job>) and as a table to the job summary (GITHUB_STEP_SUMMARY).
`flakes.py issue` (workflow flakes.yml) adds the flakes of finished CI runs to the tracking issue;
a test not seen for AGE moves to its collapsed section "No longer seen", and an issue without a
test seen within AGE is closed.

Usage:
  flakes.py go --dir DIR --gotestsum PATH --packages "PKG..." [--budget] -- FLAGS
      run `go test FLAGS PKG...` through gotestsum with reruns; exit code of gotestsum
  flakes.py exec --packages "PKG..." -- FLAGS [-test.run=PATTERN PKG]
      the raw command of gotestsum: one attempt, `go test -json`
  flakes.py dart --dir DIR --package NAME REPORT
      record the flakes of a JSON report of `flutter test --file-reporter json:REPORT`
  flakes.py issue --repo OWNER/NAME [--workflow ci.yml] [--hours H]
      add the flakes of the runs finished in the last H hours to the open issue labelled `flake`,
      age its list and close it when no test is left in its table

Exit codes: `go` returns the exit code of gotestsum; the others 0 done, 1 an error (on stderr).
"""

from __future__ import annotations

import argparse
import datetime as dt
import itertools
import json
import os
import re
import subprocess  # nosec B404 - runs gotestsum, go and gh with fixed arguments
import sys
import tempfile
import uuid
from collections.abc import Callable, Iterable, Sequence
from pathlib import Path, PurePosixPath
from typing import Any

import go_shards
import yaml

# Reruns after the first attempt, and the most failed tests of a first run that still rerun.
RERUNS = 2
MAX_FAILURES = 10
SELF = Path(__file__).resolve()
SCHEMA = 1
# The tracking issue: one open issue with this label and title.
LABEL = "flake"
TITLE = "Flaky tests (reruns that passed in CI)"
# A row of the issue list not seen for this long moves to the collapsed section "No longer seen";
# when no row is left in the table, `issue` closes the issue.
AGE = dt.timedelta(days=7)
# The author of the comments `issue` writes with the GITHUB_TOKEN.
BOT = "github-actions[bot]"
# The hidden line of a comment of `issue` with the run and its flakes (JSON).
_MARKER = re.compile(r"<!-- flakes run=(\d+) (\{.*?\}) -->")
_ATTEMPT = re.compile(r"\.attempt(\d+)$")
_NAME_MAX = 300

Flake = dict[str, Any]


# --- Go: gotestsum with reruns ---------------------------------------------------------------


def go_command(
    gotestsum: str, events: Path, packages: Sequence[str], flags: Sequence[str]
) -> list[str]:
    """The gotestsum command of `flakes.py go`; its raw command is `flakes.py exec`.

    The output is that of `go test`, with -v that of `go test -v`.
    """
    verbose = "-v" in flags or "-test.v" in flags
    return [
        gotestsum,
        f"--format=standard-{'verbose' if verbose else 'quiet'}",
        f"--rerun-fails={RERUNS}",
        f"--rerun-fails-max-failures={MAX_FAILURES}",
        "--rerun-fails-abort-on-data-race",
        "--rerun-fails-run-root-test",
        f"--jsonfile-timing-events={events}",
        "--raw-command",
        "--",
        sys.executable,
        str(SELF),
        "exec",
        "--packages",
        " ".join(packages),
        "--",
        *flags,
    ]


def _without_run(flags: Sequence[str]) -> list[str]:
    """`flags` without a -run filter (`-run X`, `-run=X`, also spelled -test.run)."""
    out: list[str] = []
    skip = False
    for flag in flags:
        if skip:
            skip = False
        elif flag in ("-run", "-test.run", "--run"):
            skip = True
        elif not flag.startswith(("-run=", "-test.run=", "--run=")):
            out.append(flag)
    return out


def attempt_args(packages: Sequence[str], flags: Sequence[str], rerun: Sequence[str]) -> list[str]:
    """The `go test -json` arguments of one attempt.

    `rerun` is empty for the first run, else what gotestsum appends: `-test.run=PATTERN` and the
    package. A rerun tests only that package; PATTERN `^$` (a package failed without a failed test)
    reruns it with the first run's filters, any other PATTERN replaces its -run filter.
    """
    if not rerun:
        return ["test", "-json", *flags, *packages]
    run, package = rerun
    pattern = run.split("=", 1)[1]
    if pattern != "^$":
        flags = [*_without_run(flags), f"-run={pattern}"]
    return ["test", "-json", *flags, package]


def next_attempt(profile: str) -> str:
    """Create and return the next free FILE.attemptN of the profile FILE."""
    for number in itertools.count(1):
        path = f"{profile}.attempt{number}"
        try:
            with Path(path).open("x", encoding="utf-8"):
                return path
        except FileExistsError:
            continue
    raise AssertionError("unreachable")  # pragma: no cover


def attempts(profile: Path) -> list[Path]:
    """The attempt profiles FILE.attemptN of FILE, in the order of N."""
    found = [p for p in profile.parent.glob(f"{profile.name}.attempt*") if _ATTEMPT.search(p.name)]
    return sorted(found, key=lambda p: int(_ATTEMPT.search(p.name)[1]))  # type: ignore[index]


def merge_attempts(profile: Path) -> None:
    """Write FILE as the merge of its non-empty attempt profiles and remove them."""
    found = attempts(profile)
    written = [p for p in found if p.stat().st_size > 0]
    try:
        if written:
            go_shards.merge_profiles(written, profile)
    finally:
        for path in found:
            path.unlink()


def _exec(args: argparse.Namespace) -> int:
    argv = list(args.rest)
    rerun = argv[-2:] if len(argv) >= 2 and argv[-2].startswith("-test.run=") else []
    flags = argv[: len(argv) - len(rerun)]
    for i, flag in enumerate(flags):
        if flag.startswith("-coverprofile="):
            flags[i] = "-coverprofile=" + next_attempt(flag.split("=", 1)[1])
    command = ["go", *attempt_args(args.packages.split(), flags, rerun)]
    os.execvp("go", command)  # noqa: S606  # nosec B606 B607 - go test with the Makefile's flags


def outcomes(lines: Iterable[str]) -> dict[tuple[str, str], list[str]]:
    """The results (pass, fail) of every test and package (test "") in the order of the attempts."""
    found: dict[tuple[str, str], list[str]] = {}
    for line in lines:
        if not line.strip():
            continue
        event = json.loads(line)
        if event.get("Action") in ("pass", "fail"):
            key = (str(event.get("Package", "")), str(event.get("Test", "")))
            found.setdefault(key, []).append(event["Action"])
    return found


def still_failing(lines: Iterable[str]) -> list[tuple[str, str]]:
    """The tests and packages whose last attempt failed.

    gotestsum forgets a package that failed again in a rerun without a failed test when another
    test needs one more attempt and passes in it; its exit code is then 0. This list decides.
    """
    return sorted(k for k, v in outcomes(lines).items() if v[-1] == "fail")


def flakes_from_events(lines: Iterable[str]) -> list[Flake]:
    """The flakes of gotestsum's timing events (pass, fail and skip of every attempt)."""
    flaky = {k: v for k, v in outcomes(lines).items() if "fail" in v and v[-1] == "pass"}
    out: list[Flake] = []
    for (package, test), results in sorted(flaky.items()):
        inner = [
            t
            for p, t in flaky
            if p == package and t != test and (not test or t.startswith(test + "/"))
        ]
        if inner:
            continue
        out.append(
            {
                "tool": "go",
                "package": package,
                "test": test,
                "runs": len(results),
                "failures": results.count("fail"),
            }
        )
    return out


_DURATION = re.compile(r"(\d+(?:\.\d+)?)(h|ms|m|s|us|µs|μs|ns)")
_SECONDS = {"h": 3600.0, "m": 60.0, "s": 1.0, "ms": 1e-3, "us": 1e-6, "µs": 1e-6, "μs": 1e-6}
_SECONDS["ns"] = 1e-9


def go_timeout(flags: Sequence[str]) -> float | None:
    """The -timeout of `flags` in seconds (`-timeout=20m`, `-timeout 20m`, also -test.timeout).

    None without one or for 0 (no limit).
    """
    value = ""
    for i, flag in enumerate(flags):
        name, _, given = flag.partition("=")
        if name.lstrip("-") in ("timeout", "test.timeout"):
            value = given if given or i + 1 >= len(flags) else flags[i + 1]
    if value in ("", "0"):
        return None
    parts = _DURATION.findall(value)
    if "".join(n + u for n, u in parts) != value:
        raise ValueError(f"not a Go duration: -timeout {value!r}")
    return sum(float(n) * _SECONDS[u] for n, u in parts) or None


def over_budget(lines: Iterable[str], budget: float) -> list[tuple[str, float]]:
    """The test binary runs longer than `budget` seconds, from gotestsum's timing events.

    Each run of a test binary counts (a whole package, a part of one, a rerun), with its package
    and elapsed time; the durations of single tests do not.
    """
    out: list[tuple[str, float]] = []
    for line in lines:
        if not line.strip():
            continue
        event = json.loads(line)
        if event.get("Action") in ("pass", "fail") and not event.get("Test"):
            elapsed = float(event.get("Elapsed") or 0)
            if elapsed > budget:
                out.append((str(event.get("Package", "")), elapsed))
    return out


def _git(args: Sequence[str], cwd: Path) -> str:
    done = subprocess.run(  # nosec B603 B607 - git with fixed arguments
        ["git", *args], cwd=cwd, capture_output=True, text=True, check=False
    )
    if done.returncode != 0:
        raise ValueError(f"git {' '.join(args)} failed: {done.stderr.strip()}")
    return done.stdout


def _module() -> tuple[Path, str]:
    """The directory and the module path of the Go module of the working directory."""
    here = Path.cwd().resolve()
    for directory in (here, *here.parents):
        gomod = directory / "go.mod"
        if gomod.is_file():
            for line in gomod.read_text(encoding="utf-8").splitlines():
                words = line.split("//", 1)[0].split()
                if len(words) == 2 and words[0] == "module":
                    return directory, words[1].strip('"`')
            raise ValueError(f"{gomod} has no module line")
    raise ValueError(f"no go.mod in {here} or above")


def _package_dir(path: str) -> str:
    """The directory of the package a changed file belongs to: testdata belongs to its package."""
    parts = PurePosixPath(path).parts[:-1]
    if "testdata" in parts:
        parts = parts[: parts.index("testdata")]
    return "/".join(parts) or "."


def changed_packages_of(base: str) -> Callable[[Iterable[str]], set[str]]:
    """A function from import paths to those of them the pull request changes.

    A package is changed when a file of its directory (or of a testdata folder below it) differs
    between the merge base of HEAD and `base` and HEAD, also a removed or renamed file. Only the
    packages of the module of the working directory count. Raises ValueError when git cannot
    compare, for example when the checkout lacks the base commit (fetch-depth: 0).
    """
    module_dir, module_path = _module()
    root = Path(_git(["rev-parse", "--show-toplevel"], module_dir).strip()).resolve()
    names = _git(["diff", "--name-only", "--no-renames", "-z", f"{base}...HEAD", "--"], root)
    dirs = {_package_dir(name) for name in names.split("\0") if name}

    def changed(packages: Iterable[str]) -> set[str]:
        out: set[str] = set()
        for package in packages:
            if package == module_path:
                directory = module_dir
            elif package.startswith(module_path + "/"):
                directory = module_dir / package[len(module_path) + 1 :]
            else:
                continue
            if Path(os.path.relpath(directory, root)).as_posix() in dirs:
                out.add(package)
        return out

    return changed


def changed_packages(base: str, packages: Iterable[str]) -> set[str]:
    """The import paths of `packages` the pull request with the base commit `base` changes."""
    return changed_packages_of(base)(packages)


def _go(args: argparse.Namespace) -> int:
    packages = args.packages.split()
    flags = list(args.rest)
    # The time budget (PL/39), read first so a bad -timeout fails at once.
    timeout = go_timeout(flags) if args.budget else None
    # FLAKE_BASE (ci.yml sets it in a pull request): a rerun that passed in a package the pull
    # request changes does not count, the pull request may have made the test flaky (PL/26).
    base = os.environ.get("FLAKE_BASE", "")
    changed = changed_packages_of(base) if base else (lambda _: set())
    profiles = [Path(f.split("=", 1)[1]) for f in flags if f.startswith("-coverprofile=")]
    for profile in profiles:
        for stale in attempts(profile):
            stale.unlink()
    with tempfile.TemporaryDirectory() as tmp:
        events = Path(tmp) / "events.json"
        done = subprocess.run(  # nosec B603 - the pinned gotestsum with the Makefile's flags
            go_command(args.gotestsum, events, packages, flags), check=False
        )
        for profile in profiles:
            merge_attempts(profile)
        lines = events.read_text(encoding="utf-8").splitlines() if events.exists() else []
    found = flakes_from_events(lines)
    strict = changed({f["package"] for f in found})
    held = [f for f in found if f["package"] in strict]
    record(args.dir, "go", [f for f in found if f["package"] not in strict])
    failing = still_failing(lines)
    for package, test in failing:
        print(f"FAIL after the reruns: {package} {test or '(package)'}", file=sys.stderr)
    # With --budget (the shards of `make test`) a test binary that needs more than half of its
    # -timeout fails with its name and time, long before a slow creep ends in a killed binary
    # that blames whichever test happened to run.
    slow = over_budget(lines, timeout / 2) if timeout else []
    for package, elapsed in slow:
        print(
            f"FAIL over the time budget: {package} ran {elapsed:.0f}s, more than half of its "
            f"-timeout of {timeout:.0f}s; split it (SPLITS in tools/ci/go_shards.py), make it "
            "faster, or refresh tools/ci/go_test_timings.json when its shard is overloaded",
            file=sys.stderr,
        )
    for f in held:
        print(
            f"FAIL in a package the pull request changes (its reruns do not count): "
            f"{f['package']} {_test_name(f)}",
            file=sys.stderr,
        )
    return done.returncode or (1 if failing or held or slow else 0)


# --- Dart: the JSON report of flutter test -----------------------------------------------------


def dart_flakes(lines: Iterable[str], package: str) -> list[Flake]:
    """The tests of a `flutter test` JSON report that reported an error and then passed."""
    names: dict[int, str] = {}
    suites: dict[int, str] = {}
    files: dict[int, str] = {}
    errors: dict[int, int] = {}
    out: list[Flake] = []
    for line in lines:
        if not line.strip().startswith("{"):
            continue
        event = json.loads(line)
        kind = event.get("type")
        if kind == "suite":
            suites[event["suite"]["id"]] = str(event["suite"].get("path") or "")
        elif kind == "testStart":
            test = event["test"]
            names[test["id"]] = str(test.get("name", ""))
            files[test["id"]] = suites.get(test.get("suiteID", -1), "")
        elif kind == "error":
            errors[event["testID"]] = errors.get(event["testID"], 0) + 1
        elif (
            kind == "testDone" and event.get("result") == "success" and errors.get(event["testID"])
        ):
            test_id = event["testID"]
            out.append(
                {
                    "tool": "dart",
                    "package": package,
                    "file": _relative(files.get(test_id, "")),
                    "test": names.get(test_id, ""),
                    "runs": 2,
                    "failures": 1,
                }
            )
    return out


def _relative(path: str) -> str:
    """`path` from the repository root when it lies below it."""
    root = str(SELF.parents[2]) + os.sep
    real = os.path.realpath(path) if path else ""
    return real[len(root) :] if real.startswith(root) else path


def _dart(args: argparse.Namespace) -> int:
    lines = args.report.read_text(encoding="utf-8").splitlines() if args.report.exists() else []
    record(args.dir, "dart", dart_flakes(lines, args.package))
    return 0


# --- reporting ---------------------------------------------------------------------------------


def _cell(text: str) -> str:
    """`text` as the content of a Markdown code span in a table cell."""
    flat = " ".join(str(text).split())[:_NAME_MAX]
    # No angle brackets: they could open or close an HTML comment such as the hidden line.
    flat = flat.replace("`", "'").replace("<", "\u2039").replace(">", "\u203a")
    return "`" + flat.replace("|", "\\|") + "`"


def _test_name(flake: Flake) -> str:
    return str(flake.get("test") or "(package: TestMain or an exit outside a test)")


def table(flakes: Sequence[Flake]) -> str:
    """A Markdown table of flakes."""
    rows = ["| Tool | Package | Test | Runs | Failures |", "|---|---|---|---|---|"]
    for f in flakes:
        cells = [
            _cell(f["tool"]),
            _cell(f["package"]),
            _cell(_test_name(f)),
            str(f["runs"]),
            str(f["failures"]),
        ]
        rows.append("| " + " | ".join(cells) + " |")
    return "\n".join(rows) + "\n"


def record(directory: Path, tool: str, flakes: Sequence[Flake]) -> None:
    """Write the flakes to DIR/<tool>-<id>.json and the job summary; nothing without flakes."""
    if not flakes:
        return
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / f"{tool}-{uuid.uuid4().hex}.json"
    path.write_text(json.dumps({"schema": SCHEMA, "flakes": list(flakes)}, indent=2) + "\n")
    for f in flakes:
        name, failures = _test_name(f), f["failures"]
        print(f"FLAKE ({f['tool']}): {f['package']} {name}: passed after {failures} failed run(s)")
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with Path(summary).open("a", encoding="utf-8") as fh:
            fh.write(f"### Flaky tests ({tool}): failed, then passed in a rerun\n\n")
            fh.write(table(flakes) + "\nThe list: docs/quality/gates.md, section Flakes.\n\n")


def load_flakes(paths: Iterable[Path]) -> list[Flake]:
    """The flakes of the JSON files `record` wrote; other files and shapes are skipped."""
    out: list[Flake] = []
    for path in sorted(paths):
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        if not isinstance(data, dict) or data.get("schema") != SCHEMA:
            continue
        out += clean(data.get("flakes", []))
    return out


def clean(items: Any) -> list[Flake]:
    """The well-formed flakes of `items` with their names cut to a bounded length."""
    out: list[Flake] = []
    for f in items if isinstance(items, list) else []:
        if not isinstance(f, dict) or not {"tool", "package", "test", "runs", "failures"} <= set(f):
            continue
        try:
            runs, failures = int(f["runs"]), int(f["failures"])
        except (TypeError, ValueError):
            continue
        out.append(
            {
                "tool": str(f["tool"])[:10],
                "package": str(f["package"])[:_NAME_MAX],
                "test": str(f["test"])[:_NAME_MAX],
                "runs": runs,
                "failures": failures,
            }
        )
    return out


# --- the tracking issue ------------------------------------------------------------------------

Gh = Callable[[Sequence[str]], str]


def _gh(args: Sequence[str]) -> str:
    done = subprocess.run(  # nosec B603 B607 - the GitHub CLI with fixed arguments
        ["gh", *args], capture_output=True, text=True, check=False
    )
    if done.returncode != 0:
        raise ValueError(f"gh {' '.join(args[:3])} failed: {done.stderr.strip()}")
    return done.stdout


def _label_args() -> list[str]:
    """Colour and description of LABEL from .github/labels.yml (the source of the labels)."""
    labels = yaml.safe_load(
        (SELF.parents[2] / ".github" / "labels.yml").read_text(encoding="utf-8")
    )
    [item] = [i for i in labels if i.get("name") == LABEL]
    return ["--color", str(item["color"]), "--description", str(item["description"])]


def _key(flake: Flake) -> str:
    return f"{flake['tool']} {flake['package']} {flake['test']}"


def marker(run: int, flakes: Sequence[Flake], seen: str | None = None) -> str:
    """The hidden line of a comment: run, flakes and finish time, safe in an HTML comment."""
    data: dict[str, Any] = {"flakes": list(flakes)}
    if seen:
        data["seen"] = seen
    payload = json.dumps(data, sort_keys=True)
    payload = payload.replace("<", "\\u003c").replace(">", "\\u003e").replace("&", "\\u0026")
    return f"<!-- flakes run={run} {payload} -->"


def parse_comments(bodies: Iterable[str]) -> dict[int, list[Flake]]:
    """The runs already in the issue with their flakes, from the hidden lines of the comments."""
    seen: dict[int, list[Flake]] = {}
    for body in bodies:
        for match in _MARKER.finditer(body or ""):
            try:
                payload = json.loads(match[2])
            except ValueError:
                continue
            if isinstance(payload, dict):
                seen[int(match[1])] = clean(payload.get("flakes"))
    return seen


def _parse_time(text: Any) -> dt.datetime | None:
    try:
        when = dt.datetime.fromisoformat(str(text).replace("Z", "+00:00"))
    except ValueError:
        return None
    return when if when.tzinfo else when.replace(tzinfo=dt.UTC)


def parse_seen(comments: Iterable[tuple[str, str]]) -> dict[int, dt.datetime]:
    """When each run of the comments (body, created_at) was seen.

    That is the `seen` of its hidden line, or the time of its comment for a comment written
    before the hidden line had one.
    """
    seen: dict[int, dt.datetime] = {}
    for text, created in comments:
        for match in _MARKER.finditer(text or ""):
            try:
                payload = json.loads(match[2])
            except ValueError:
                continue
            if not isinstance(payload, dict):
                continue
            when = _parse_time(payload.get("seen")) or _parse_time(created)
            if when:
                seen[int(match[1])] = when
    return seen


def comment(run: dict[str, Any], flakes: Sequence[Flake], known: set[str]) -> str:
    """The comment for one CI run: its flakes, the ones not in the issue before marked new."""
    rows = ["| New | Tool | Package | Test | Runs | Failures |", "|---|---|---|---|---|---|"]
    for f in flakes:
        new = "new" if _key(f) not in known else ""
        cells = [
            new,
            _cell(f["tool"]),
            _cell(f["package"]),
            _cell(_test_name(f)),
            str(f["runs"]),
            str(f["failures"]),
        ]
        rows.append("| " + " | ".join(cells) + " |")
    head = (
        f"Run [{run['id']}]({run['html_url']}) ({run.get('event', '')}, "
        f"{_cell(run.get('head_branch') or '')}, {str(run.get('head_sha', ''))[:12]}): "
        f"{len(flakes)} flaky test(s)\n\n"
    )
    seen = _parse_time(run.get("updated_at"))
    stamp = seen.strftime("%Y-%m-%dT%H:%M:%SZ") if seen else None
    return head + "\n".join(rows) + "\n\n" + marker(int(run["id"]), flakes, stamp) + "\n"


def rows(
    seen: dict[int, list[Flake]], when: dict[int, dt.datetime], now: dt.datetime
) -> tuple[list[str], list[str]]:
    """The rows of the issue list, the most frequent first: (seen within AGE, not seen since).

    A run without a time counts as seen `now`.
    """
    count: dict[str, int] = {}
    last: dict[str, int] = {}
    last_seen: dict[str, dt.datetime] = {}
    sample: dict[str, Flake] = {}
    for run, flakes in sorted(seen.items()):
        at = when.get(run, now)
        for f in flakes:
            key = _key(f)
            count[key] = count.get(key, 0) + 1
            if key not in last_seen or at >= last_seen[key]:
                last[key] = run
                last_seen[key] = at
            sample[key] = f
    recent: list[str] = []
    aged: list[str] = []
    for key in sorted(count, key=lambda k: (-count[k], k)):
        f = sample[key]
        cells = [
            str(count[key]),
            str(last[key]),
            last_seen[key].strftime("%Y-%m-%d"),
            _cell(f["tool"]),
            _cell(f["package"]),
            _cell(_test_name(f)),
        ]
        row = "| " + " | ".join(cells) + " |"
        (aged if now - last_seen[key] > AGE else recent).append(row)
    return recent, aged


_HEADER = [
    "| Runs with the flake | Last run | Last seen | Tool | Package | Test |",
    "|---|---|---|---|---|---|",
]


def body(
    seen: dict[int, list[Flake]],
    when: dict[int, dt.datetime] | None = None,
    now: dt.datetime | None = None,
) -> str:
    """The issue body: the flaky tests seen within AGE, then the older ones collapsed.

    Both lists put the most frequent first; the collapsed one is "No longer seen".
    """
    recent, aged = rows(seen, when or {}, now or dt.datetime.now(dt.UTC))
    text = (
        "Tests that failed and then passed in a rerun of CI (docs/quality/gates.md, section "
        "Flakes). The workflow flakes.yml adds one comment per CI run with flakes and keeps this "
        f'list current. A test not seen for {AGE.days} days moves to "No longer seen"; when '
        "none is left in the table, the workflow closes the issue, and the next flake opens a "
        "new one.\n\n" + "\n".join([*_HEADER, *recent]) + "\n"
    )
    if aged:
        text += (
            f"\n<details>\n<summary>No longer seen ({len(aged)})</summary>\n\n"
            + "\n".join([*_HEADER, *aged])
            + "\n\n</details>\n"
        )
    return text


def _issue(args: argparse.Namespace, gh: Gh = _gh, now: dt.datetime | None = None) -> int:
    now = now or dt.datetime.now(dt.UTC)
    since = (now - dt.timedelta(hours=args.hours)).strftime("%Y-%m-%dT%H:%M:%SZ")
    runs: list[dict[str, Any]] = _json_lines(
        gh(
            [
                "api",
                "--paginate",
                f"repos/{args.repo}/actions/workflows/{args.workflow}/runs"
                f"?status=completed&created=>={since}&per_page=100",
                "--jq",
                ".workflow_runs[] | {id, html_url, event, head_branch, head_sha, updated_at}",
            ]
        )
    )
    issues = json.loads(
        gh(
            [
                "issue",
                "list",
                "-R",
                args.repo,
                "--label",
                LABEL,
                "--state",
                "all",
                "--json",
                "number,title,state,body",
            ]
        )
    )
    issues = [i for i in issues if i.get("title") == TITLE]
    number = next((int(i["number"]) for i in issues if i.get("state") == "OPEN"), 0)
    current = next((str(i.get("body") or "") for i in issues if int(i["number"]) == number), "")
    # The runs of every issue, also of a closed one, are reported; the open one lists its own.
    seen: dict[int, list[Flake]] = {}
    when: dict[int, dt.datetime] = {}
    reported: set[int] = set()
    for issue in issues:
        listed = gh(
            [
                "api",
                "--paginate",
                f"repos/{args.repo}/issues/{issue['number']}/comments",
                "--jq",
                ".[] | {body, user: .user.login, created_at}",
            ]
        )
        # Only the comments of this workflow: anyone can comment on the public issue.
        own = [c for c in _json_lines(listed) if c.get("user") == BOT]
        parsed = parse_comments(str(c.get("body") or "") for c in own)
        reported |= set(parsed)
        if int(issue["number"]) == number:
            seen = parsed
            when = parse_seen((str(c.get("body") or ""), str(c.get("created_at"))) for c in own)
    known = {_key(f) for flakes in seen.values() for f in flakes}
    added = 0
    for run in sorted(runs, key=lambda r: int(r["id"])):
        if int(run["id"]) in reported:
            continue
        listed = gh(
            [
                "api",
                "--paginate",
                f"repos/{args.repo}/actions/runs/{run['id']}/artifacts?per_page=100",
                "--jq",
                ".artifacts[] | select(.expired | not) | {name}",
            ]
        )
        # A rerun of failed jobs keeps the artifact of each attempt under the same name, and a
        # second download of a name into its directory fails (file exists): each name once.
        names = sorted(
            {str(a["name"]) for a in _json_lines(listed) if str(a["name"]).startswith("flakes-")}
        )
        if not names:
            continue
        with tempfile.TemporaryDirectory() as tmp:
            for name in names:
                gh(
                    [
                        "run",
                        "download",
                        str(run["id"]),
                        "-R",
                        args.repo,
                        "-n",
                        str(name),
                        "-D",
                        str(Path(tmp) / str(name)),
                    ]
                )
            flakes = load_flakes(Path(tmp).rglob("*.json"))
        if not flakes:
            continue
        if not number:
            gh(["label", "create", LABEL, "-R", args.repo, "--force", *_label_args()])
            url = gh(
                [
                    "issue",
                    "create",
                    "-R",
                    args.repo,
                    "--title",
                    TITLE,
                    "--label",
                    LABEL,
                    "--body",
                    body({}),
                ]
            )
            number = int(url.strip().rstrip("/").rsplit("/", 1)[1])
        text = comment(run, flakes, known)
        gh(["issue", "comment", str(number), "-R", args.repo, "--body", text])
        seen[int(run["id"])] = flakes
        when.update(parse_seen([(text, "")]))
        known |= {_key(f) for f in flakes}
        added += 1
    print(
        f"flakes: {added} run(s) with flakes added to issue #{number}"
        if added
        else "flakes: no new flakes"
    )
    if number:
        _age(args.repo, number, current, seen, when, now, gh)
    return 0


def _age(
    repo: str,
    number: int,
    current: str,
    seen: dict[int, list[Flake]],
    when: dict[int, dt.datetime],
    now: dt.datetime,
    gh: Gh,
) -> None:
    """Keep the body of the open issue current and close it when its table is empty."""
    # The list ages every hour, also without a new run: edit the body when it changed.
    text = body(seen, when, now)
    if text != current:
        gh(["issue", "edit", str(number), "-R", repo, "--body", text])
    recent, aged = rows(seen, when, now)
    if aged and not recent:
        note = (
            f"No flaky test of this list was seen in the last {AGE.days} days, so the workflow "
            "flakes.yml closes the issue. The next flake opens a new one."
        )
        gh(
            [
                "issue",
                "close",
                str(number),
                "-R",
                repo,
                "--reason",
                "completed",
                "--comment",
                note,
            ]
        )
        print(f"flakes: issue #{number} closed, no flake in the last {AGE.days} days")


def _json_lines(text: str) -> list[Any]:
    return [json.loads(line) for line in text.splitlines() if line.strip()]


def main(argv: list[str] | None = None) -> int:
    """Entry point; returns the exit code."""
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = parser.add_subparsers(dest="command", required=True)
    go = sub.add_parser("go")
    go.add_argument("--dir", type=Path, required=True)
    go.add_argument("--gotestsum", required=True)
    go.add_argument("--packages", required=True)
    go.add_argument(
        "--budget",
        action="store_true",
        help="fail a test binary that runs longer than half of its -timeout (PL/39)",
    )
    go.set_defaults(run=_go)
    run = sub.add_parser("exec")
    run.add_argument("--packages", required=True)
    run.set_defaults(run=_exec)
    dart = sub.add_parser("dart")
    dart.add_argument("--dir", type=Path, required=True)
    dart.add_argument("--package", required=True)
    dart.add_argument("report", type=Path)
    dart.set_defaults(run=_dart)
    issue = sub.add_parser("issue")
    issue.add_argument("--repo", required=True)
    issue.add_argument("--workflow", default="ci.yml")
    issue.add_argument("--hours", type=int, default=48)
    issue.set_defaults(run=_issue)
    argv = list(sys.argv[1:] if argv is None else argv)
    rest = argv[argv.index("--") + 1 :] if "--" in argv else []
    args = parser.parse_args(argv[: len(argv) - len(rest) - 1] if "--" in argv else argv)
    args.rest = rest
    try:
        code: int = args.run(args)
    except ValueError as exc:
        print(f"flakes: {exc}", file=sys.stderr)
        return 1
    return code


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
