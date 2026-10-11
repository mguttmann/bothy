# SPDX-License-Identifier: Apache-2.0
"""CodeQL guard: the matrix covers every tracked component language CodeQL supports.

`.github/workflows/codeql.yml` analyses the repository in the public phase (ADR 0039 rule 1).
A language the repository ships but the matrix lacks would go unanalysed, and no gate would
notice. `codeql_findings` compares the workflow with the tracked component manifests (the
discovery of the root Makefile, docs/spec/23-qualitaet.md "Komponenten-Vertrag") and checks
the build CodeQL needs for Go: Go has no build-free mode, so every tracked Go module (each
Go pattern of LANGUAGE_SOURCES, today the Makefile pathspec `services/*go.mod`) gets its Go
toolchain and a manual `go build ./...` between the CodeQL `init` and `analyze` steps.

The synthetic tests prove each rule against one compliant fixture, so a negative test cannot
pass for a wrong reason; the last tests apply the rules to the real workflow, the real
`git ls-files` and the public-switch runbook.
"""

from __future__ import annotations

import fnmatch
import os
import posixpath
import re
import subprocess
import textwrap
from collections.abc import Iterable
from pathlib import Path
from typing import Any

import check_runners as cr
import pytest

REPO = Path(__file__).resolve().parents[3]
WORKFLOW = REPO / ".github" / "workflows" / "codeql.yml"
RUNBOOK = REPO / "docs" / "quality" / "public-switch.md"
MAKEFILE = REPO / "Makefile"
TOOLS_README = REPO / "tools" / "README.md"

GO_MODULE = "services/*go.mod"
# Tracked path (fnmatch; `*` also matches `/`, like the git pathspecs of the Makefile) to the
# CodeQL language that analyses it. COMPONENT_MANIFESTS are the pathspecs of the component
# discovery of the root Makefile, verbatim (ruling M1-74/5); a test holds them equal to it.
# REPOSITORY_SOURCES is the repository's own code. `tools/node/package.json` is left out on
# purpose: it only pins Node tools and has no source.
COMPONENT_MANIFESTS: dict[str, str] = {
    GO_MODULE: "go",
    "sdk/python/pyproject.toml": "python",
    "plugins/*pyproject.toml": "python",
    "sdk/typescript/package.json": "javascript-typescript",
    "plugins/*package.json": "javascript-typescript",
}
REPOSITORY_SOURCES: dict[str, str] = {
    ".github/workflows/*.yml": "actions",
    ".github/workflows/*.yaml": "actions",
    "tools/pyproject.toml": "python",
}
LANGUAGE_SOURCES: dict[str, str] = {**REPOSITORY_SOURCES, **COMPONENT_MANIFESTS}
# Component discovery variables of the Makefile to the CodeQL language of their manifests;
# None where CodeQL has none: Dart (`pubspec.yaml`) is covered by flutter analyze and
# SonarQube Cloud.
MAKEFILE_DISCOVERY: dict[str, str | None] = {
    "GO_MODS": "go",
    "DART_PKGS": None,
    "PY_PROJECTS": "python",
    "TS_PROJECTS": "javascript-typescript",
}
# A make assignment (`=`, `:=`, `::=`, `:::=`, `+=`, `?=`, `!=`, also after `export` or
# `override`; recipe lines start with a tab), the `$(dir` or `${dir` call that turns manifests
# into component directories, the arguments of each `git ls-files` up to the closing
# parenthesis, brace or pipe, and the shell redirections among them.
_ASSIGNMENT = re.compile(
    r"^ *(?:(?:export|override) +)*([A-Za-z_][A-Za-z0-9_.-]*)\s*(:{1,3}|[+?!])?=(.*)$"
)
_DIR = re.compile(r"\$[({]dir\s")
_LS_FILES = re.compile(r"git ls-files\b([^)}|]*)")
_REDIRECT = re.compile(r"[0-9]*>&?\s*\S*")
# Build modes CodeQL offers per language:
# https://docs.github.com/en/code-security/reference/code-scanning/codeql/build-options-for-compiled-languages
# Go has no `none` mode. `autobuild` is not allowed for Go because the autobuilder first runs
# `make` at the repository root, where the default goal `help` succeeds without building
# anything, and then needs `go list ./...` at a root without go.mod; the outcome would depend
# on fallback heuristics. A manual `go build ./...` per module is deterministic and is the
# build `make typecheck` runs.
ALLOWED_BUILD_MODES: dict[str, frozenset[str]] = {
    "actions": frozenset({"none"}),
    "python": frozenset({"none"}),
    "javascript-typescript": frozenset({"none"}),
    "go": frozenset({"manual"}),
}
GO_ONLY = "matrix.language == 'go'"
GO_BUILD = "go build ./..."
QUERIES = "security-extended"
CHECKOUT = "actions/checkout@"
SETUP_GO = "actions/setup-go@"
INIT = "github/codeql-action/init@"
ANALYZE = "github/codeql-action/analyze@"
MATRIX_LANGUAGE = re.compile(r"\$\{\{\s*matrix\.language\s*\}\}")
CATEGORY = re.compile(r"/language:" + MATRIX_LANGUAGE.pattern)
RUNBOOK_STEP = "Probelauf"
# Closed lists of the options the guard knows: the top-level keys of the workflow, the
# filters of its pull_request trigger, the job, and per kind of step its keys plus the keys
# of its `with` or `env` mapping. Any other option could skip or weaken the analysis of a
# matrix entry without touching a rule below (top-level env such as GOFLAGS or GOTOOLCHAIN,
# a path filter, a condition, continue-on-error, `upload: never`, a CodeQL config with path
# filters, another ref to check out, build tags), so a new option is a deliberate change of
# this guard and its tests.
WORKFLOW_OPTIONS = frozenset({"name", "on", "permissions", "concurrency", "jobs"})
PULL_REQUEST_OPTIONS = frozenset({"branches"})
JOB_OPTIONS = frozenset(
    {"name", "if", "runs-on", "timeout-minutes", "permissions", "strategy", "steps"}
)
STEP_OPTIONS: dict[str, tuple[frozenset[str], str, frozenset[str]]] = {
    "checkout": (
        frozenset({"id", "name", "uses", "with"}),
        "with",
        frozenset({"persist-credentials"}),
    ),
    "setup-go": (
        frozenset({"id", "name", "if", "uses", "with"}),
        "with",
        frozenset({"go-version-file", "cache"}),
    ),
    "init": (
        frozenset({"id", "name", "uses", "with"}),
        "with",
        frozenset({"languages", "build-mode", "queries"}),
    ),
    "Go build": (
        frozenset({"id", "name", "if", "working-directory", "env", "run"}),
        "env",
        frozenset({"CGO_ENABLED"}),
    ),
    "analyze": (frozenset({"id", "name", "uses", "with"}), "with", frozenset({"category"})),
}
# The spellings of false that a boolean action input accepts (YAML 1.2 core schema).
_FALSE_INPUTS = ("false", "False", "FALSE")


def tracked_files() -> list[str]:
    """Return the tracked paths of the repository, relative and with `/`.

    Tracked files only, like the component discovery of the Makefile: an untracked
    `go.mod` in a working copy is no component.

    Returns:
        One entry per file in the index of REPO.

    Raises:
        subprocess.CalledProcessError: git fails, for example outside a repository.
    """
    out = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "-z"], check=True, capture_output=True, timeout=60
    ).stdout
    return [p for p in out.decode("utf-8").split("\0") if p]


def required_languages(tracked: Iterable[str]) -> set[str]:
    """Return the CodeQL languages the tracked paths need (LANGUAGE_SOURCES).

    Args:
        tracked: paths relative to the repository root.

    Returns:
        The languages of every matching pattern, for example
        `required_languages(["services/server/go.mod"]) == {"go"}`.
    """
    return {
        language
        for path in tracked
        for pattern, language in LANGUAGE_SOURCES.items()
        if fnmatch.fnmatchcase(path, pattern)
    }


def makefile_discovery(text: str) -> dict[str, list[str]]:
    """Return the git pathspecs of every component discovery variable of a Makefile.

    A discovery variable is an assignment on a line without a leading tab (any operator,
    also after `export` or `override`, across line continuations) whose value calls
    `$(dir ...)` or `${dir ...}` and `git ls-files`, for example `GO_MODS := $(patsubst
    %/,%,$(sort $(dir $(shell git ls-files -- 'services/*go.mod' 2>/dev/null))))`. The
    pathspecs are the arguments of every `git ls-files` in the value without options and
    redirections; `+=` appends to an earlier assignment. Not recognised: a list built
    without `$(dir ...)` or without `git ls-files`, and one built through `define`,
    `$(eval ...)` or `$(call ...)`.

    Args:
        text: the content of the Makefile.

    Returns:
        The pathspecs per variable name in Makefile order, quotes removed and unquoted
        ones kept, for example `{"GO_MODS": ["services/*go.mod"]}`.
    """
    found: dict[str, list[str]] = {}
    for line in text.replace("\\\n", " ").splitlines():
        assignment = _ASSIGNMENT.match(line)
        if assignment is None or not _DIR.search(assignment.group(3)):
            continue
        name, operator, value = assignment.groups()
        pathspecs = [
            token.strip("'\"")
            for arguments in _LS_FILES.findall(value)
            for token in _REDIRECT.sub(" ", arguments).split()
            if not token.startswith("-")
        ]
        if pathspecs:
            found[name] = (found.get(name, []) if operator == "+" else []) + pathspecs
    return found


def codeql_findings(doc: dict[str, Any], tracked: Iterable[str]) -> list[str]:
    """Check the CodeQL workflow against the tracked components.

    Rules: the workflow sets only the top-level keys of WORKFLOW_OPTIONS, has no job but
    `analyze` and runs on pull requests to main (no filter but `branches`, which names main
    and has no `!` pattern); the job `analyze` keeps the public guard; its matrix is an
    `include` list whose entries name `language` and an allowed `build-mode`, with every
    required language; `init` passes the matrix language and build mode and runs
    `security-extended`; `analyze` uploads per language; every tracked Go module gets a
    setup-go step (its go.mod, no cache) before `init` and a `go build ./...` (in the
    module, `CGO_ENABLED` the string "0") between `init` and `analyze`, both only for Go;
    there is no other step, and the job and every step set only the options of JOB_OPTIONS
    and STEP_OPTIONS.

    Args:
        doc: the parsed workflow (`check_runners.load_yaml`, so `on` stays a string).
        tracked: paths relative to the repository root, as `tracked_files` returns them.

    Returns:
        One readable finding per violation; empty if the workflow complies.
    """
    paths = list(tracked)
    # `check_runners.load_yaml` returns None, a list or a string for such files.
    jobs = doc.get("jobs") if isinstance(doc, dict) else None
    job = jobs.get("analyze") if isinstance(jobs, dict) else None
    if not isinstance(job, dict):
        return ["the workflow has no job 'analyze'"]
    findings = _workflow_findings(doc)
    if not cr.has_public_guard(job.get("if")):
        findings.append("the condition of job 'analyze' is not the public guard (ADR 0039 rule 1)")
    findings += _unknown_options("the analyze job", job, JOB_OPTIONS)
    findings += _matrix_findings(job, required_languages(paths))
    # Every Go pattern counts, so a further Go pathspec of the Makefile brings its modules too.
    go_manifests = [pattern for pattern, language in LANGUAGE_SOURCES.items() if language == "go"]
    modules = sorted(
        {
            posixpath.dirname(p)
            for p in paths
            if any(fnmatch.fnmatchcase(p, pattern) for pattern in go_manifests)
        }
    )
    findings += _step_findings(_steps(job), modules)
    return findings


def _expr(value: Any) -> str:
    """Return an expression without `${{ }}` and with single spaces, "" for a non-string."""
    if not isinstance(value, str):
        return ""
    text = value.strip()
    if text.startswith("${{") and text.endswith("}}"):
        text = text[3:-2]
    return " ".join(text.split())


def _mapping(value: Any) -> dict[str, Any]:
    """Return `value` if it is a mapping, else an empty one (a malformed key is no crash)."""
    return value if isinstance(value, dict) else {}


def _steps(job: dict[str, Any]) -> list[dict[str, Any]]:
    """Return the steps of a job; a malformed step becomes `{}` so positions stay intact."""
    steps = job.get("steps")
    return [_mapping(s) for s in steps] if isinstance(steps, list) else []


def _uses(step: dict[str, Any], prefix: str) -> bool:
    """Return whether the step runs an action whose reference starts with `prefix`."""
    uses = step.get("uses")
    return isinstance(uses, str) and uses.startswith(prefix)


def _unknown_options(
    where: str, mapping: dict[str, Any], allowed: frozenset[str], prefix: str = ""
) -> list[str]:
    """Return one finding per key of `mapping` outside `allowed`, named `<prefix><key>`."""
    names = ", ".join(prefix + k for k in sorted(allowed))
    return [
        f"{where} sets {prefix}{key}; allowed are {names}, "
        "so that no option skips or weakens the analysis"
        for key in sorted(str(k) for k in mapping)
        if key not in allowed
    ]


def _workflow_findings(doc: dict[str, Any]) -> list[str]:
    """Findings for the top-level keys, the jobs next to `analyze` and the pull_request trigger."""
    # Top-level env reaches every step without touching one (GOFLAGS, GOTOOLCHAIN, CODEQL_*
    # settings of the action and the extractor); defaults.run sets the shell of the build.
    findings = _unknown_options("the workflow", doc, WORKFLOW_OPTIONS)
    # Another job is outside every rule here and could upload under the categories of analyze.
    findings += [
        f"the workflow has job {name}; only analyze is allowed, "
        "so that no other job uploads results under its categories"
        for name in sorted(str(k) for k in doc["jobs"])
        if name != "analyze"
    ]
    on = doc.get("on")
    if isinstance(on, dict):
        triggers = {str(k): v for k, v in on.items()}
    else:
        triggers = {str(t): None for t in (on if isinstance(on, list) else [on])}
    if "pull_request" not in triggers:
        findings.append(
            "the workflow does not run on pull_request, so pull requests are not analysed"
        )
        return findings
    # A filter would skip the analysis of some pull requests without a red check.
    settings = _mapping(triggers["pull_request"])
    findings += _unknown_options("the pull_request trigger", settings, PULL_REQUEST_OPTIONS)
    branches = settings.get("branches", ["main"])
    if isinstance(branches, str):
        branches = [branches]
    if not isinstance(branches, list) or "main" not in branches:
        findings.append("the pull_request trigger does not list the branch main")
    elif any(str(b).startswith("!") for b in branches):
        # GitHub applies a later ! pattern after main, so it can exclude main again.
        findings.append(
            "the pull_request trigger sets a branch pattern with !, which can exclude main"
        )
    return findings


def _matrix_findings(job: dict[str, Any], required: set[str]) -> list[str]:
    """Findings for `strategy.matrix`: include entries, build modes, missing languages."""
    matrix = _mapping(_mapping(job.get("strategy")).get("matrix"))
    findings: list[str] = []
    present: set[str] = set()
    # Next to an axis, GitHub merges the include entries into the axis combinations: an
    # axis value without an entry runs without a build mode, and with an unrelated axis a
    # later entry overwrites values an earlier one added. One include list keeps every job
    # explicit. The languages of a `language` axis count as present, so that the finding
    # names the real cause.
    axes = sorted(str(k) for k in matrix if k != "include")
    if axes:
        findings.append(
            f"the matrix sets {', '.join(axes)} outside include; "
            "list every language with its build-mode under include"
        )
        axis = matrix.get("language")
        if isinstance(axis, list):
            present.update(str(x) for x in axis)
    include = matrix.get("include")
    if not isinstance(include, list):
        findings.append("the analyze job has no strategy.matrix.include list")
        include = []
    for number, raw in enumerate(include, start=1):
        entry = _mapping(raw)
        language, mode = entry.get("language"), entry.get("build-mode")
        if language:
            present.add(str(language))
        if not language or not mode:
            findings.append(f"matrix entry {number} lacks language or build-mode")
            continue
        allowed = ALLOWED_BUILD_MODES.get(str(language))
        if allowed is None:
            known = ", ".join(sorted(ALLOWED_BUILD_MODES))
            findings.append(
                f"{language}: no build mode is allowed for this language (known: {known})"
            )
        elif not isinstance(mode, str) or mode not in allowed:
            options = ", ".join(sorted(allowed))
            findings.append(f"{language}: build-mode {mode} is not allowed (allowed: {options})")
    findings += [f"{language} missing from the matrix" for language in sorted(required - present)]
    return findings


def _step_findings(steps: list[dict[str, Any]], modules: list[str]) -> list[str]:
    """Findings for the CodeQL steps and the Go steps of every module."""
    findings: list[str] = []
    init = next((i for i, s in enumerate(steps) if _uses(s, INIT)), None)
    analyze = next((i for i, s in enumerate(steps) if _uses(s, ANALYZE)), None)
    if init is None:
        findings.append("no github/codeql-action/init step")
    else:
        options = _mapping(steps[init].get("with"))
        if _expr(options.get("languages")) != "matrix.language":
            findings.append("init does not pass the matrix language")
        if _expr(options.get("build-mode")) != "matrix.build-mode":
            findings.append("init does not pass the matrix build mode")
        if _expr(options.get("queries")) != QUERIES:
            findings.append(f"init does not run the query suite {QUERIES}")
    if analyze is None:
        findings.append("no github/codeql-action/analyze step")
    else:
        if init is not None and analyze < init:
            findings.append("the analyze step is not after the init step")
        # One category for all entries would let each upload replace the others' results.
        category = _mapping(steps[analyze].get("with")).get("category")
        if not isinstance(category, str) or not CATEGORY.fullmatch(category):
            findings.append(
                "analyze does not upload under the category /language:${{ matrix.language }}"
            )
    kinds: dict[int, str] = {}
    for module in modules:
        module_findings, setup, build = _go_findings(steps, module, init, analyze)
        findings += module_findings
        kinds.update(
            {i: kind for i, kind in ((setup, "setup-go"), (build, "Go build")) if i is not None}
        )
    checkout = next((i for i, s in enumerate(steps) if _uses(s, CHECKOUT)), None)
    kinds.update(
        {
            i: kind
            for i, kind in ((checkout, "checkout"), (init, "init"), (analyze, "analyze"))
            if i is not None
        }
    )
    for i, step in enumerate(steps):
        kind = kinds.get(i)
        if kind is not None:
            keys, inner, inner_keys = STEP_OPTIONS[kind]
            findings += _unknown_options(f"the {kind} step", step, keys)
            findings += _unknown_options(
                f"the {kind} step", _mapping(step.get(inner)), inner_keys, f"{inner}."
            )
        elif _uses(step, SETUP_GO):
            # A further setup-go step would replace the module toolchain or bring back the cache.
            findings.append(
                f"setup-go step {i + 1} is not the setup of a tracked Go module "
                f"(if: {GO_ONLY}, its go-version-file, cache: false)"
            )
        else:
            # Another step could change the code under analysis before init or the build.
            findings.append(
                f"step {i + 1} is none of checkout, the Go setup of a module, init, "
                "the Go build of a module and analyze"
            )
    return findings


def _go_findings(
    steps: list[dict[str, Any]], module: str, init: int | None, analyze: int | None
) -> tuple[list[str], int | None, int | None]:
    """Return the findings for the Go steps of one module and its setup and build index."""
    findings: list[str] = []
    go_mod = f"{module}/go.mod"
    setup = next(
        (
            i
            for i, s in enumerate(steps)
            if _uses(s, SETUP_GO)
            and _expr(s.get("if")) == GO_ONLY
            and _mapping(s.get("with")).get("go-version-file") == go_mod
        ),
        None,
    )
    if setup is None:
        findings.append(
            f"{module}: no actions/setup-go step with if: {GO_ONLY} and go-version-file: {go_mod}"
        )
    else:
        # A restored cache is not verified again; an analysis must not trust state written
        # by another run. Inputs reach setup-go as strings and it reads `cache` as a boolean
        # input, so a quoted "false" disables the cache as well.
        if _mapping(steps[setup].get("with")).get("cache") not in (False, *_FALSE_INPUTS):
            findings.append(f"{module}: the setup-go step does not set cache: false")
        if init is not None and setup > init:
            findings.append(f"{module}: the setup-go step is not before the init step")
    build = next(
        (
            i
            for i, s in enumerate(steps)
            if " ".join(str(s.get("run", "")).split()) == GO_BUILD
            and s.get("working-directory") == module
            and _expr(s.get("if")) == GO_ONLY
        ),
        None,
    )
    if build is None:
        findings.append(
            f"{module}: no run step '{GO_BUILD}' with working-directory: {module} and if: {GO_ONLY}"
        )
        return findings, setup, None
    # The server ships as a static binary (ADR 0005); CodeQL sees the shipped configuration.
    # Only the string "0": load_yaml reads 0b0, 0_0 or 00 as the number 0 (YAML 1.1), GitHub
    # passes 0b0 and 0_0 on as text (YAML 1.2), and Go takes nothing but "0" as off.
    cgo = _mapping(steps[build].get("env")).get("CGO_ENABLED")
    if not isinstance(cgo, str) or cgo != "0":
        findings.append(f'{module}: the build step does not set CGO_ENABLED: "0"')
    if init is None or analyze is None or not init < build < analyze:
        findings.append(f"{module}: the build step is not after the init step and before analyze")
    return findings, setup, build


# ---- synthetic fixture: one workflow that satisfies every rule --------------------------

# Any full commit SHA: pinning is the rule of check_runners, not of this module.
SHA = "3d3c42e5aac5ba805825da76410c181273ba90b1"
INCLUDE = """\
        include:
          - language: actions
            build-mode: none
          - language: go
            build-mode: manual
          - language: python
            build-mode: none
"""
HEADER = """\
name: codeql
on:
  pull_request:
    branches: [main]
permissions:
  contents: read
concurrency:
  group: codeql-${{ github.ref }}
  cancel-in-progress: true
jobs:
  analyze:
    name: analyze (${{ matrix.language }})
    if: <guard>
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    permissions:
      contents: read
      security-events: write
    strategy:
      fail-fast: false
      matrix:
<include>    steps:
"""
STEPS: dict[str, str] = {
    "checkout": """
        - uses: actions/checkout@<sha>
          with:
            persist-credentials: false
    """,
    "setup-go": """
        - if: matrix.language == 'go'
          uses: actions/setup-go@<sha>
          with:
            go-version-file: services/server/go.mod
            cache: false
    """,
    "init": """
        - uses: github/codeql-action/init@<sha>
          with:
            languages: ${{ matrix.language }}
            build-mode: ${{ matrix.build-mode }}
            queries: security-extended
    """,
    "build": """
        - name: Build the server module for CodeQL
          if: matrix.language == 'go'
          working-directory: services/server
          env:
            CGO_ENABLED: "0"
          run: go build ./...
    """,
    "analyze": """
        - uses: github/codeql-action/analyze@<sha>
          with:
            category: /language:${{ matrix.language }}
    """,
}
# Further setup-go steps for the tests of the rule that every setup-go step is a module setup.
STEPS["setup-go-stable"] = """
    - if: matrix.language == 'go'
      uses: actions/setup-go@<sha>
      with:
        go-version: stable
"""
STEPS["cleanup"] = """
    - run: rm -rf services/server/internal
"""
STEPS["setup-go-unguarded"] = """
    - uses: actions/setup-go@<sha>
      with:
        go-version-file: services/server/go.mod
        cache: false
"""
ORDER = ("checkout", "setup-go", "init", "build", "analyze")
STEP_KEYS = "id, name, uses, with"
TRACKED = [
    ".github/workflows/codeql.yml",
    "apps/client/pubspec.yaml",
    "services/server/go.mod",
    "services/server/go.sum",
    "tools/node/package.json",
    "tools/pyproject.toml",
]
NO_SETUP = (
    "services/server: no actions/setup-go step with if: matrix.language == 'go' "
    "and go-version-file: services/server/go.mod"
)
NO_BUILD = (
    "services/server: no run step 'go build ./...' with working-directory: services/server "
    "and if: matrix.language == 'go'"
)
BUILD_ORDER = "services/server: the build step is not after the init step and before analyze"
NOT_GUARDED = "the condition of job 'analyze' is not the public guard (ADR 0039 rule 1)"
NO_CGO = 'services/server: the build step does not set CGO_ENABLED: "0"'
WORKFLOW_KEYS = "concurrency, jobs, name, on, permissions"
NO_PULL_REQUEST = "the workflow does not run on pull_request, so pull requests are not analysed"
NO_MAIN = "the pull_request trigger does not list the branch main"
NEGATED_BRANCH = "the pull_request trigger sets a branch pattern with !, which can exclude main"
RUNBOOK_TEXT = (
    "18. **Probelauf:** `analyze (actions)`, `analyze (go)` und `analyze (python)` sind grün.\n"
)


def option(where: str, key: str, allowed: str) -> str:
    """Return the finding for an option `key` that the guard does not know at `where`."""
    return (
        f"{where} sets {key}; allowed are {allowed}, "
        "so that no option skips or weakens the analysis"
    )


def unknown_step(position: int) -> str:
    """Return the finding for a step at `position` (1-based) that is none of the known steps."""
    return (
        f"step {position} is none of checkout, the Go setup of a module, init, "
        "the Go build of a module and analyze"
    )


def extra_setup(position: int) -> str:
    """Return the finding for a setup-go step at `position` (1-based) that no module owns."""
    return (
        f"setup-go step {position} is not the setup of a tracked Go module "
        "(if: matrix.language == 'go', its go-version-file, cache: false)"
    )


def workflow(order: Iterable[str] = ORDER, include: str = INCLUDE) -> str:
    """Return the fixture workflow with the steps in `order`."""
    steps = "".join(textwrap.dedent(STEPS[name]).lstrip("\n") for name in order)
    text = HEADER.replace("<include>", include) + textwrap.indent(steps, " " * 6)
    return text.replace("<guard>", cr.PUBLIC_GUARD).replace("<sha>", SHA)


def edit(old: str, new: str, text: str | None = None) -> str:
    """Replace the single occurrence of `old` in the fixture, so a mutation cannot miss."""
    text = workflow() if text is None else text
    assert text.count(old) == 1, f"the fixture holds {text.count(old)} copies of {old!r}"
    return text.replace(old, new)


def check(text: str, tracked: Iterable[str] = TRACKED) -> list[str]:
    """Return the findings for a workflow text and a tracked file list."""
    return codeql_findings(cr.load_yaml(text), tracked)


def runbook_step(text: str, title: str) -> str:
    """Return the numbered step `**<title>:**` of a runbook text, whitespace collapsed.

    The step ends at the next numbered item, heading or blank line, so text further down
    the file never counts for it.
    """
    start = text.index(f"**{title}:**")
    end = re.search(r"\n(?:\d+\. |#|[ \t]*\n)", text[start:])
    return " ".join(text[start : start + end.start() if end else None].split())


def runbook_findings(doc: Any, runbook: str) -> list[str]:
    """Check that step "Probelauf" of the runbook names the check of every matrix entry.

    The check names come from the job name of `analyze` with the language of each entry of
    `strategy.matrix.include`, for example `analyze (go)`.

    Args:
        doc: the parsed workflow (`check_runners.load_yaml`).
        runbook: the text of docs/quality/public-switch.md.

    Returns:
        One readable finding per gap; empty if the step names every check.
    """
    jobs = doc.get("jobs") if isinstance(doc, dict) else None
    job = _mapping(jobs).get("analyze")
    if not isinstance(job, dict):
        return ["the workflow has no job 'analyze'"]
    name = job.get("name")
    # Without the language in the job name the check names would differ.
    if not isinstance(name, str) or not MATRIX_LANGUAGE.search(name):
        return [f"the job name {name!r} does not name the matrix language"]
    include = _mapping(_mapping(job.get("strategy")).get("matrix")).get("include")
    if not isinstance(include, list) or not include:
        return ["the analyze job has no strategy.matrix.include entries to name the checks of"]
    if f"**{RUNBOOK_STEP}:**" not in runbook:
        return [f"the runbook has no step **{RUNBOOK_STEP}:**"]
    step = runbook_step(runbook, RUNBOOK_STEP)

    def check_name(language: str) -> str:
        return MATRIX_LANGUAGE.sub(lambda _: language, name)

    checks = [check_name(str(_mapping(e).get("language"))) for e in include]
    return [f"step {RUNBOOK_STEP} does not name `{c}`" for c in checks if f"`{c}`" not in step]


# ---- language mapping --------------------------------------------------------------------


@pytest.mark.parametrize(
    ("path", "expected"),
    [
        (".github/workflows/ci.yml", {"actions"}),
        (".github/workflows/release.yaml", {"actions"}),
        ("services/server/go.mod", {"go"}),
        ("tools/pyproject.toml", {"python"}),
        ("sdk/python/pyproject.toml", {"python"}),
        ("plugins/example-adapter/pyproject.toml", {"python"}),
        ("sdk/typescript/package.json", {"javascript-typescript"}),
        ("plugins/example-panel/package.json", {"javascript-typescript"}),
        ("tools/node/package.json", set()),
        ("apps/client/pubspec.yaml", set()),
        ("services/server/go.sum", set()),
        # Ruling M1-74/3: `*` crosses `/` like a git pathspec, matching is anchored at the
        # start and case-sensitive on every operating system.
        ("plugins/group/example/pyproject.toml", {"python"}),
        ("x/tools/pyproject.toml", set()),
        ("Services/server/go.mod", set()),
        # Ruling M1-74/5: the pathspecs of the Makefile also find a manifest directly under
        # services/ or plugins/.
        ("services/go.mod", {"go"}),
        ("plugins/pyproject.toml", {"python"}),
        ("plugins/package.json", {"javascript-typescript"}),
    ],
)
def test_required_languages_mapping(path: str, expected: set[str]) -> None:
    assert required_languages([path]) == expected


def test_matching_is_case_sensitive_on_every_os(monkeypatch: pytest.MonkeyPatch) -> None:
    # Ruling M1-74/3: Windows lowercases in os.path.normcase, which fnmatch.fnmatch applies.
    monkeypatch.setattr(os.path, "normcase", str.lower)
    assert required_languages(["Services/server/go.mod", "TOOLS/pyproject.toml"]) == set()


def test_go_module_detection_is_case_sensitive_on_every_os(monkeypatch: pytest.MonkeyPatch) -> None:
    # Ruling M1-74/3 for the second match: `Services/host` is no Go module, as for the Makefile.
    monkeypatch.setattr(os.path, "normcase", str.lower)
    assert check(workflow(), [*TRACKED, "Services/host/go.mod"]) == []


def test_makefile_discovery_reads_the_pathspecs() -> None:
    text = (
        "GO_MODS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'services/*go.mod'"
        " 2>/dev/null))))\n"
        "PY_PROJECTS := $(patsubst %/,%,$(sort $(dir $(shell git ls-files -- 'a/pyproject.toml'"
        " 'b/*pyproject.toml' 2>/dev/null))))\n"
        "PRODUCT_SOURCES := $(shell git ls-files -- 'services/*.go' 2>/dev/null | head -n 1)\n"
    )
    assert makefile_discovery(text) == {
        "GO_MODS": ["services/*go.mod"],
        "PY_PROJECTS": ["a/pyproject.toml", "b/*pyproject.toml"],
    }


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        # Any assignment that takes `$(dir ...)` of `git ls-files` is a discovery, however
        # it is spelled, and an unquoted pathspec still counts.
        (
            "RS_CRATES = $(sort $(dir $(shell git ls-files -- 'crates/*Cargo.toml')))\n",
            {"RS_CRATES": ["crates/*Cargo.toml"]},
        ),
        (
            "RS_CRATES += $(dir $(shell git ls-files -- crates/*Cargo.toml 2>/dev/null))\n",
            {"RS_CRATES": ["crates/*Cargo.toml"]},
        ),
        (
            "RS_CRATES\t:= $(dir $(shell git ls-files -z -- 'a/x' \\\n  \"b/y\" 2>/dev/null))\n",
            {"RS_CRATES": ["a/x", "b/y"]},
        ),
        (
            "  export RS := ${dir ${shell git ls-files 'a/x' 2> /dev/null}}\n"
            "override KT ?= $(dir $(shell git ls-files -- 'k/x') $(shell git ls-files -- 'k/y'))\n",
            {"RS": ["a/x"], "KT": ["k/x", "k/y"]},
        ),
        # `+=` appends to the pathspecs of an earlier assignment, `:=` replaces them.
        (
            "RS := $(dir $(shell git ls-files -- 'a/x'))\n"
            "RS += $(dir $(shell git ls-files -- 'b/y'))\n"
            "KT := $(dir $(shell git ls-files -- 'old/x'))\n"
            "KT := $(dir $(shell git ls-files -- 'new/x'))\n",
            {"RS": ["a/x", "b/y"], "KT": ["new/x"]},
        ),
        # A recipe line and a list without `$(dir ...)` or without git ls-files are no discovery.
        ("check:\n\t@files=$$(git ls-files -- '*Dockerfile*' | head -n 1)\n", {}),
        ("SOURCES := $(shell git ls-files -- 'services/*.go' 2>/dev/null | head -n 1)\n", {}),
        ("DIRS := $(dir $(wildcard services/*/go.mod))\n", {}),
    ],
)
def test_makefile_discovery_finds_every_spelling(text: str, expected: dict[str, list[str]]) -> None:
    assert makefile_discovery(text) == expected


def test_component_manifests_are_the_makefile_discovery() -> None:
    # Ruling M1-74/5: the guard finds exactly the component manifests the Makefile finds.
    discovery = makefile_discovery(MAKEFILE.read_text(encoding="utf-8"))
    assert sorted(discovery) == sorted(MAKEFILE_DISCOVERY), (
        f"Makefile discovery variables {sorted(discovery)}: map every one to the CodeQL "
        "language of its manifests in MAKEFILE_DISCOVERY"
    )
    manifests = {
        spec: language
        for variable, language in MAKEFILE_DISCOVERY.items()
        if language is not None
        for spec in discovery[variable]
    }
    assert manifests == COMPONENT_MANIFESTS


# ---- compliant fixture ---------------------------------------------------------------------


def test_compliant_synthetic_workflow_passes() -> None:
    assert check(workflow()) == []


def test_expression_spellings_are_accepted() -> None:
    text = edit(
        "if: matrix.language == 'go'\n        working-directory",
        "if: ${{ matrix.language  ==  'go' }}\n        working-directory",
    )
    text = edit("${{ matrix.build-mode }}", "${{matrix.build-mode}}", text)
    text = edit("run: go build ./...\n", "run: |\n          go build ./...\n", text)
    text = edit("/language:${{ matrix.language }}", "/language:${{matrix.language}}", text)
    text = edit('CGO_ENABLED: "0"', "CGO_ENABLED: '0'", text)
    assert check(text) == []


@pytest.mark.parametrize("cache", ["'false'", "'False'", "'FALSE'", "False"])
def test_cache_false_spellings_are_accepted(cache: str) -> None:
    # Action inputs reach setup-go as strings; it reads `cache` as a boolean input.
    assert check(edit("cache: false", f"cache: {cache}")) == []


# ---- matrix ------------------------------------------------------------------------------


def test_matrix_without_include_is_rejected() -> None:
    found = check(workflow(include="        language: [go]\n"))
    assert "the analyze job has no strategy.matrix.include list" in found
    assert "go missing from the matrix" not in found


def test_axis_outside_include_is_rejected() -> None:
    text = workflow(include="        language: [actions, python]\n" + INCLUDE)
    assert check(text) == [
        "the matrix sets language outside include; "
        "list every language with its build-mode under include"
    ]


def test_matrix_entry_without_build_mode_is_rejected() -> None:
    text = edit("- language: go\n            build-mode: manual\n", "- language: go\n")
    assert check(text) == ["matrix entry 2 lacks language or build-mode"]


def test_matrix_entry_without_language_is_rejected() -> None:
    text = edit("- language: go\n            build-mode: manual\n", "- build-mode: manual\n")
    assert check(text) == [
        "matrix entry 2 lacks language or build-mode",
        "go missing from the matrix",
    ]


def test_go_build_mode_none_is_rejected() -> None:
    text = edit("build-mode: manual", "build-mode: none")
    assert check(text) == ["go: build-mode none is not allowed (allowed: manual)"]


def test_go_autobuild_is_rejected() -> None:
    text = edit("build-mode: manual", "build-mode: autobuild")
    assert check(text) == ["go: build-mode autobuild is not allowed (allowed: manual)"]


def test_unknown_language_is_rejected() -> None:
    text = workflow(include=INCLUDE + "          - language: ruby\n            build-mode: none\n")
    assert check(text) == [
        "ruby: no build mode is allowed for this language "
        "(known: actions, go, javascript-typescript, python)"
    ]


def test_missing_required_language_is_rejected() -> None:
    found = check(workflow(), [*TRACKED, "sdk/typescript/package.json"])
    assert found == ["javascript-typescript missing from the matrix"]


# ---- CodeQL steps ------------------------------------------------------------------------


@pytest.mark.parametrize(
    "build_mode",
    ["\n          build-mode: none", "\n          build-mode: ${{ matrix.language }}", ""],
)
def test_init_must_pass_matrix_build_mode(build_mode: str) -> None:
    text = edit("\n          build-mode: ${{ matrix.build-mode }}", build_mode)
    assert check(text) == ["init does not pass the matrix build mode"]


def test_init_must_pass_matrix_language() -> None:
    text = edit("languages: ${{ matrix.language }}", "languages: go")
    assert check(text) == ["init does not pass the matrix language"]


@pytest.mark.parametrize(
    ("order", "expected"),
    [
        (("checkout", "setup-go", "build", "analyze"), "no github/codeql-action/init step"),
        (("checkout", "setup-go", "init", "build"), "no github/codeql-action/analyze step"),
        (
            ("checkout", "setup-go", "analyze", "init", "build"),
            "the analyze step is not after the init step",
        ),
    ],
)
def test_codeql_steps_must_run_init_then_analyze(order: tuple[str, ...], expected: str) -> None:
    # Without both steps in order the Go build cannot sit between them either.
    assert check(workflow(order)) == [expected, BUILD_ORDER]


@pytest.mark.parametrize(
    ("old", "new", "expected"),
    [
        # A condition on init or analyze lets a matrix entry end green without an analysis.
        (
            "- uses: github/codeql-action/init@",
            "- if: matrix.language != 'go'\n        uses: github/codeql-action/init@",
            option("the init step", "if", STEP_KEYS),
        ),
        (
            "- uses: github/codeql-action/analyze@",
            "- continue-on-error: true\n        uses: github/codeql-action/analyze@",
            option("the analyze step", "continue-on-error", STEP_KEYS),
        ),
        (
            "queries: security-extended\n",
            "queries: security-extended\n          config-file: ./.github/codeql/codeql.yml\n",
            option(
                "the init step", "with.config-file", "with.build-mode, with.languages, with.queries"
            ),
        ),
        (
            "category: /language:${{ matrix.language }}\n",
            "category: /language:${{ matrix.language }}\n          upload: never\n",
            option("the analyze step", "with.upload", "with.category"),
        ),
        (
            "persist-credentials: false\n",
            "persist-credentials: false\n          ref: main\n",
            option("the checkout step", "with.ref", "with.persist-credentials"),
        ),
        (
            "cache: false\n",
            "cache: false\n          check-latest: true\n",
            option("the setup-go step", "with.check-latest", "with.cache, with.go-version-file"),
        ),
        (
            'CGO_ENABLED: "0"\n',
            'CGO_ENABLED: "0"\n          GOFLAGS: -tags=example\n',
            option("the Go build step", "env.GOFLAGS", "env.CGO_ENABLED"),
        ),
        (
            "        run: go build ./...\n",
            "        run: go build ./...\n        continue-on-error: true\n",
            option(
                "the Go build step",
                "continue-on-error",
                "env, id, if, name, run, working-directory",
            ),
        ),
    ],
)
def test_options_outside_the_guard_are_rejected(old: str, new: str, expected: str) -> None:
    assert check(edit(old, new)) == [expected]


@pytest.mark.parametrize("key", ["continue-on-error: true", "env:\n      GOFLAGS: -tags=example"])
def test_analyze_job_options_outside_the_guard_are_rejected(key: str) -> None:
    text = edit("    timeout-minutes: 30\n", f"    timeout-minutes: 30\n    {key}\n")
    allowed = "if, name, permissions, runs-on, steps, strategy, timeout-minutes"
    assert check(text) == [option("the analyze job", key.split(":", maxsplit=1)[0], allowed)]


@pytest.mark.parametrize(
    ("order", "position"),
    [
        (("checkout", "cleanup", "setup-go", "init", "build", "analyze"), 2),
        (("checkout", "setup-go", "init", "init", "build", "analyze"), 4),
        (("checkout", "checkout", "setup-go", "init", "build", "analyze"), 2),
    ],
)
def test_unknown_steps_are_rejected(order: tuple[str, ...], position: int) -> None:
    # Another step could change the code under analysis before init or before the build.
    assert check(workflow(order)) == [unknown_step(position)]


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ("queries: security-extended", "queries: security-and-quality"),
        ("\n          queries: security-extended", ""),
    ],
)
def test_init_must_run_security_extended(old: str, new: str) -> None:
    assert check(edit(old, new)) == ["init does not run the query suite security-extended"]


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ("category: /language:${{ matrix.language }}", "category: /language:go"),
        ("category: /language:${{ matrix.language }}", "category: /language:<l>"),
        # The whole value must be the expression, not only contain it.
        (
            "category: /language:${{ matrix.language }}",
            "category: /language:${{ matrix.language }}-x",
        ),
        (
            "category: /language:${{ matrix.language }}",
            "category: x/language:${{ matrix.language }}",
        ),
        ("\n        with:\n          category: /language:${{ matrix.language }}", ""),
    ],
)
def test_analyze_must_upload_per_language(old: str, new: str) -> None:
    # One category for all entries would let each upload replace the results of the others.
    expected = "analyze does not upload under the category /language:${{ matrix.language }}"
    assert check(edit(old, new)) == [expected]


# ---- Go steps ----------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ("go-version-file: services/server/go.mod", "go-version-file: go.mod"),
        ("go-version-file: services/server/go.mod", "go-version: stable"),
    ],
)
def test_go_setup_step_must_use_module_go_mod(old: str, new: str) -> None:
    assert check(edit(old, new)) == [NO_SETUP, extra_setup(2)]


def test_go_setup_step_is_required() -> None:
    assert check(workflow(tuple(n for n in ORDER if n != "setup-go"))) == [NO_SETUP]


def test_go_setup_step_must_be_setup_go() -> None:
    text = edit("uses: actions/setup-go@", "uses: actions/setup-node@")
    assert check(text) == [NO_SETUP, unknown_step(2)]


@pytest.mark.parametrize(
    ("order", "position"),
    [
        (("checkout", "setup-go", "setup-go-stable", "init", "build", "analyze"), 3),
        (("checkout", "setup-go-unguarded", "setup-go", "init", "build", "analyze"), 2),
        (("checkout", "setup-go", "setup-go", "init", "build", "analyze"), 3),
    ],
)
def test_every_setup_go_step_must_be_a_module_setup(order: tuple[str, ...], position: int) -> None:
    # A further setup-go step would replace the module toolchain or bring back the cache.
    assert check(workflow(order)) == [extra_setup(position)]


@pytest.mark.parametrize(
    ("old", "new"), [("cache: false", "cache: true"), ("\n          cache: false", "")]
)
def test_go_setup_step_must_disable_cache(old: str, new: str) -> None:
    text = edit(old, new)
    assert check(text) == ["services/server: the setup-go step does not set cache: false"]


def test_go_setup_step_must_precede_init() -> None:
    text = workflow(("checkout", "init", "setup-go", "build", "analyze"))
    assert check(text) == ["services/server: the setup-go step is not before the init step"]


@pytest.mark.parametrize(
    ("old", "new", "expected"),
    [
        (
            "- if: matrix.language == 'go'\n        uses: actions/setup-go",
            "- uses: actions/setup-go",
            [NO_SETUP, extra_setup(2)],
        ),
        (
            "if: matrix.language == 'go'\n        working-directory",
            "working-directory",
            [NO_BUILD, unknown_step(4)],
        ),
    ],
)
def test_go_steps_must_be_go_only(old: str, new: str, expected: list[str]) -> None:
    assert check(edit(old, new)) == expected


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ("working-directory: services/server\n", "working-directory: services\n"),
        ("\n        working-directory: services/server", ""),
    ],
)
def test_go_build_step_must_run_in_module(old: str, new: str) -> None:
    assert check(edit(old, new)) == [NO_BUILD, unknown_step(4)]


@pytest.mark.parametrize("run", ["run: go build ./cmd/...", "run: go build ./... && go vet ./..."])
def test_go_build_step_must_build_every_package(run: str) -> None:
    assert check(edit("run: go build ./...", run)) == [NO_BUILD, unknown_step(4)]


@pytest.mark.parametrize(
    ("old", "new"),
    [
        ('CGO_ENABLED: "0"', 'CGO_ENABLED: "1"'),
        ('\n        env:\n          CGO_ENABLED: "0"', ""),
        # Ruling M1-74/4: load_yaml reads all of these as the number 0 (YAML 1.1), so the
        # unquoted 0 and 00 cannot be told apart from 0b0 and 0_0, which GitHub passes on as
        # text (YAML 1.2) and Go does not take as off.
        ('CGO_ENABLED: "0"', "CGO_ENABLED: 0b0"),
        ('CGO_ENABLED: "0"', "CGO_ENABLED: 0_0"),
        ('CGO_ENABLED: "0"', "CGO_ENABLED: 0"),
        ('CGO_ENABLED: "0"', "CGO_ENABLED: 00"),
        # Go takes " 0" as on, so the string is compared as it stands.
        ('CGO_ENABLED: "0"', 'CGO_ENABLED: " 0"'),
    ],
)
def test_go_build_step_must_disable_cgo(old: str, new: str) -> None:
    text = edit(old, new)
    assert check(text) == [NO_CGO]


@pytest.mark.parametrize(
    "order",
    [
        ("checkout", "setup-go", "build", "init", "analyze"),
        ("checkout", "setup-go", "init", "analyze", "build"),
    ],
)
def test_go_build_step_must_sit_between_init_and_analyze(order: tuple[str, ...]) -> None:
    assert check(workflow(order)) == [BUILD_ORDER]


@pytest.mark.parametrize("module", ["services", "services/host", "services/worker"])
def test_second_go_module_needs_its_own_steps(module: str) -> None:
    # Modules sort before and after services/server, so every module is checked, not the first;
    # `services` itself is a module for the Makefile too (ruling M1-74/5).
    found = check(workflow(), [*TRACKED, f"{module}/go.mod"])
    assert found == [
        f"{module}: no actions/setup-go step with if: matrix.language == 'go' "
        f"and go-version-file: {module}/go.mod",
        f"{module}: no run step 'go build ./...' with working-directory: {module} "
        "and if: matrix.language == 'go'",
    ]


def test_every_go_pattern_finds_its_modules(monkeypatch: pytest.MonkeyPatch) -> None:
    # A further Go pathspec of the Makefile (ruling M1-74/5) puts its modules under the same rules.
    monkeypatch.setitem(LANGUAGE_SOURCES, "tools/*go.mod", "go")
    assert check(workflow(), [*TRACKED, "tools/gen/go.mod"]) == [
        "tools/gen: no actions/setup-go step with if: matrix.language == 'go' "
        "and go-version-file: tools/gen/go.mod",
        "tools/gen: no run step 'go build ./...' with working-directory: tools/gen "
        "and if: matrix.language == 'go'",
    ]


# ---- workflow ----------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("key", "block"),
    [
        ("env", "env:\n  GOFLAGS: -tags=example\n"),
        ("env", "env:\n  GOTOOLCHAIN: local\n"),
        ("env", 'env:\n  CODEQL_ACTION_DEPENDENCY_CACHING: "true"\n'),
        ("env", 'env:\n  CODEQL_EXTRACTOR_GO_OPTION_EXTRACT_TESTS: "true"\n'),
        ("defaults", "defaults:\n  run:\n    shell: bash\n"),
    ],
)
def test_workflow_keys_outside_the_guard_are_rejected(key: str, block: str) -> None:
    # Top-level env reaches every step, defaults.run the build step (ruling M1-74/6).
    old = "permissions:\n  contents: read\nconcurrency:"
    assert check(edit(old, block + old)) == [option("the workflow", key, WORKFLOW_KEYS)]


def test_second_job_is_rejected() -> None:
    # Another job is outside every rule here and could upload under the categories of analyze.
    text = workflow() + "  other:\n    runs-on: ubuntu-24.04\n    steps:\n      - run: echo other\n"
    assert check(text) == [
        "the workflow has job other; only analyze is allowed, "
        "so that no other job uploads results under its categories"
    ]


@pytest.mark.parametrize(
    "on", ["on:\n  workflow_dispatch:\n", "on: [schedule, workflow_dispatch]\n", "on: schedule\n"]
)
def test_pull_request_trigger_is_required(on: str) -> None:
    assert check(edit("on:\n  pull_request:\n    branches: [main]\n", on)) == [NO_PULL_REQUEST]


@pytest.mark.parametrize(
    "on",
    [
        "on: pull_request\n",
        "on: [pull_request, workflow_dispatch]\n",
        "on:\n  pull_request:\n",
        "on:\n  pull_request:\n    branches: main\n",
    ],
)
def test_pull_request_trigger_spellings_are_accepted(on: str) -> None:
    assert check(edit("on:\n  pull_request:\n    branches: [main]\n", on)) == []


@pytest.mark.parametrize(
    ("new", "expected"),
    [
        # A filter would skip the analysis of some pull requests without a red check.
        (
            "    branches: [main]\n    paths: ['docs/**']\n",
            option("the pull_request trigger", "paths", "branches"),
        ),
        (
            "    branches: [main]\n    types: [labeled]\n",
            option("the pull_request trigger", "types", "branches"),
        ),
        ("    branches: [release]\n", NO_MAIN),
        ("    branches: release\n", NO_MAIN),
        ("    branches: []\n", NO_MAIN),
        ("    branches:\n", NO_MAIN),
        # GitHub applies a later ! pattern after main, so it can exclude main again.
        ("    branches: [main, '!main']\n", NEGATED_BRANCH),
        ("    branches: [main, '!**']\n", NEGATED_BRANCH),
        ("    branches: ['!release/**', main]\n", NEGATED_BRANCH),
    ],
)
def test_pull_request_trigger_must_cover_main(new: str, expected: str) -> None:
    assert check(edit("    branches: [main]\n", new)) == [expected]


# ---- job ---------------------------------------------------------------------------------


@pytest.mark.parametrize(
    "condition",
    [
        None,
        f"{cr.PUBLIC_GUARD} || true",
        "github.event.repository.visibility == 'public'",
        "github.event_name == 'pull_request'",
    ],
)
def test_job_must_keep_public_guard(condition: str | None) -> None:
    line = f"    if: {cr.PUBLIC_GUARD}\n"
    assert check(edit(line, "" if condition is None else f"    if: {condition}\n")) == [NOT_GUARDED]


@pytest.mark.parametrize(
    "doc", [None, [], "x", {}, {"jobs": []}, {"jobs": {"analyze": "x"}}, {"jobs": {"build": {}}}]
)
def test_workflow_without_analyze_job_is_rejected(doc: Any) -> None:
    # `check_runners.load_yaml` returns None, a list or a string for such files.
    assert codeql_findings(doc, TRACKED) == ["the workflow has no job 'analyze'"]


def test_malformed_matrix_and_steps_yield_findings() -> None:
    doc = cr.load_yaml(workflow())
    job = doc["jobs"]["analyze"]
    job["strategy"]["matrix"]["include"][1] = "go"
    job["strategy"]["matrix"]["include"][2]["build-mode"] = ["none"]
    job["steps"] = "go build ./..."
    found = codeql_findings(doc, TRACKED)
    assert "matrix entry 2 lacks language or build-mode" in found
    assert "python: build-mode ['none'] is not allowed (allowed: none)" in found
    assert "go missing from the matrix" in found
    assert "no github/codeql-action/init step" in found
    assert NO_BUILD in found


# ---- the repository ----------------------------------------------------------------------


def test_repository_codeql_workflow_has_no_findings() -> None:
    tracked = tracked_files()
    assert any(fnmatch.fnmatchcase(p, GO_MODULE) for p in tracked), f"no {GO_MODULE} tracked"
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    found = codeql_findings(doc, tracked)
    assert found == [], "codeql.yml:\n" + "\n".join(found)


def test_runbook_step_ends_at_the_next_item_heading_or_blank_line() -> None:
    text = "1. **A:** one\n   two\n2. **B:** three\n\n## Next\n4. **C:** four\n## Tail\n"
    assert runbook_step(text, "A") == "**A:** one two"
    assert runbook_step(text, "B") == "**B:** three"
    assert runbook_step(text, "C") == "**C:** four"
    assert runbook_step("5. **D:** last", "D") == "**D:** last"
    assert runbook_step("6. **E:** five\n\nA paragraph after the list.\n", "E") == "**E:** five"


@pytest.mark.parametrize(
    ("old", "new", "runbook", "expected"),
    [
        ("", "", RUNBOOK_TEXT, []),
        (
            INCLUDE,
            "        language: [actions, go, python]\n",
            RUNBOOK_TEXT,
            ["the analyze job has no strategy.matrix.include entries to name the checks of"],
        ),
        (
            INCLUDE,
            "        include: []\n",
            RUNBOOK_TEXT,
            ["the analyze job has no strategy.matrix.include entries to name the checks of"],
        ),
        (
            "name: analyze (${{ matrix.language }})",
            "name: analyze",
            RUNBOOK_TEXT,
            ["the job name 'analyze' does not name the matrix language"],
        ),
        (
            "    name: analyze (${{ matrix.language }})\n",
            "",
            RUNBOOK_TEXT,
            ["the job name None does not name the matrix language"],
        ),
        (
            "",
            "",
            edit("`analyze (go)`", "analyze (go)", RUNBOOK_TEXT),
            ["step Probelauf does not name `analyze (go)`"],
        ),
        (
            "",
            "",
            edit(", `analyze (go)`", "", RUNBOOK_TEXT),
            ["step Probelauf does not name `analyze (go)`"],
        ),
        (
            "",
            "",
            edit("**Probelauf:**", "**Test:**", RUNBOOK_TEXT),
            ["the runbook has no step **Probelauf:**"],
        ),
    ],
)
def test_runbook_findings(old: str, new: str, runbook: str, expected: list[str]) -> None:
    # A gap in the workflow is a readable finding, not a KeyError in the runbook check.
    text = edit(old, new) if old else workflow()
    assert runbook_findings(cr.load_yaml(text), runbook) == expected


@pytest.mark.parametrize("doc", [None, [], {"jobs": {"build": {}}}, {"jobs": {"analyze": "x"}}])
def test_runbook_findings_without_analyze_job(doc: Any) -> None:
    assert runbook_findings(doc, RUNBOOK_TEXT) == ["the workflow has no job 'analyze'"]


def test_public_switch_runbook_names_every_codeql_job() -> None:
    """Step "Probelauf" of the runbook names the check of every matrix entry to look at."""
    doc = cr.load_yaml(WORKFLOW.read_text(encoding="utf-8"))
    found = runbook_findings(doc, RUNBOOK.read_text(encoding="utf-8"))
    assert found == [], "public-switch.md:\n" + "\n".join(found)


def test_tools_readme_names_this_guard() -> None:
    # tools/README.md, "Regeln für neue Werkzeuge": this guard has its row in the table.
    module = Path(__file__).resolve().relative_to(REPO / "tools").as_posix()
    rows = [r for r in TOOLS_README.read_text(encoding="utf-8").splitlines() if r.startswith("|")]
    assert any(f"`{module}`" in row.split("|")[1] for row in rows), (
        f"tools/README.md has no table row for `{module}`"
    )
