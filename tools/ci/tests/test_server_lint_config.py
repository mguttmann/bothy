# SPDX-License-Identifier: Apache-2.0
"""The lint exemptions of the server module follow the exclusions of the analysis.

`make lint` checks the rules S3776 and S1192 of the quality gate with gocognit and goconst
(docs/DECISIONS.md M1-07/23). Go code that `sonar.exclusions` leaves out as generated is exempt
from both linters as well; otherwise `make lint` fails on code the analysis never sees. Every
entry of `sonar.exclusions` is sorted below, so a new entry needs a decision here (M1-07/28).
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
GOLANGCI = REPO / "services" / "server" / ".golangci.yml"
SONAR = REPO / "sonar-project.properties"
LINTERS = ("gocognit", "goconst")

# Entries that name generated code and can hold Go files of the module.
GENERATED_GO = {
    "**/generated/**",
    "**/*.gen.go",
    "**/*_gen.go",
    "**/*.pb.go",
    "**/sqlcgen/**",
}
# Entries without Go code that golangci-lint reads (no Go files, or testdata, which the Go tools
# skip); where Go code could sit (build/), the lint gate stays stricter than the analysis.
OTHER = {
    "**/node_modules/**",
    "**/build/**",
    "**/.dart_tool/**",
    "**/*.g.dart",
    "**/*.freezed.dart",
    # Generated drift code of the Dart workspace (docs/DECISIONS.md D-M4-01-4).
    "**/*.drift.dart",
    "**/testdata/**",
    # The embedded copy of the API document (api/rest/openapi.gen.yaml), written by the route
    # generator; docs/api/openapi.yaml is its source.
    "**/*.gen.yaml",
    # The Pigeon output of the plugin members, Swift, Kotlin and C++ (docs/DECISIONS.md D-M4-10-7).
    "**/*.g.swift",
    "**/*.g.kt",
    "**/*.g.cpp",
    "**/*.g.h",
}
# Files whose path looks like generated code. Each one that a lint rule exempts must be one that
# sonar.exclusions leaves out too: dropping an entry there must narrow the rule.
LOOKALIKE = [
    "generated/x.go",
    "internal/p/generated/q/x.go",
    "internal/p/sqlcgen/x.go",
    "x.gen.go",
    "internal/p/x_gen.go",
    "internal/p/x.pb.go",
]
# Product code whose path only resembles generated code; the exemption must not reach it. A
# directory named gen holds hand-written code, such as the route generator in api/routes/gen
# (docs/DECISIONS.md M1-17/15 and M1-17/31).
PRODUCT = [
    "gen/x.go",
    "internal/gen/x.go",
    "api/routes/gen/x.go",
    "internal/generator/x.go",
    "internal/agen/x.go",
    "internal/regenerated/x.go",
    "internal/p/mysqlcgen/x.go",
    "internal/chat/gen.go",
    "internal/chat/generated.go",
    "internal/chat/app/x.go",
]


def _glob(pattern: str) -> re.Pattern[str]:
    """A sonar wildcard pattern as a regular expression: `**/` any directories, `*` and `?` within
    one path segment."""
    parts: list[str] = []
    i = 0
    while i < len(pattern):
        if pattern.startswith("**/", i):
            parts.append("(?:.*/)?")
            i += 3
        elif pattern.startswith("/**", i) and i + 3 == len(pattern):
            parts.append("(?:/.*)?")
            i += 3
        else:
            parts.append({"*": "[^/]*", "?": "[^/]"}.get(pattern[i], re.escape(pattern[i])))
            i += 1
    return re.compile("".join(parts))


def _sonar_excluded(path: str) -> bool:
    """Whether sonar.exclusions leaves out the file of the module; patterns are relative to the
    repository root, where sonar-project.properties lies."""
    full = f"services/server/{path}"
    return any(_glob(entry).fullmatch(full) for entry in _sonar_exclusions())


def _sonar_exclusions() -> list[str]:
    for line in SONAR.read_text(encoding="utf-8").splitlines():
        key, sep, value = line.partition("=")
        if sep and key.strip() == "sonar.exclusions":
            assert not value.rstrip().endswith("\\"), "continuation lines are not read here"
            return [entry.strip() for entry in value.split(",") if entry.strip()]
    raise AssertionError(f"{SONAR.name} has no sonar.exclusions")


def _rules() -> list[dict[str, Any]]:
    config = yaml.safe_load(GOLANGCI.read_text(encoding="utf-8"))
    rules: list[dict[str, Any]] = config["linters"]["exclusions"]["rules"]
    return rules


def _exempt(path: str, linter: str) -> bool:
    """Whether a rule with only `path` and `linters` exempts the file from the linter.

    golangci-lint matches `path` as an unanchored regular expression against the path relative
    to the configuration file; the patterns use no syntax in which Go and Python differ.
    """
    return any(
        set(rule) == {"path", "linters"}
        and linter in rule["linters"]
        and re.search(rule["path"], path) is not None
        for rule in _rules()
    )


def _samples(pattern: str) -> list[str]:
    """Files of the module that the sonar.exclusions entry leaves out."""
    if match := re.fullmatch(r"\*\*/([^*/]+)/\*\*", pattern):
        name = match.group(1)
        return [f"{name}/x.go", f"internal/{name}/x.go", f"internal/p/{name}/q/x.go"]
    if match := re.fullmatch(r"\*\*/\*([^*/]+)", pattern):
        suffix = match.group(1)
        return [f"x{suffix}", f"internal/p/x{suffix}"]
    raise AssertionError(f"no samples for the pattern {pattern!r}")


def test_every_sonar_exclusion_is_sorted() -> None:
    assert sorted(_sonar_exclusions()) == sorted(GENERATED_GO | OTHER)


@pytest.mark.parametrize("linter", LINTERS)
@pytest.mark.parametrize("pattern", sorted(GENERATED_GO))
def test_generated_go_code_is_exempt(pattern: str, linter: str) -> None:
    for path in _samples(pattern):
        assert _exempt(path, linter), f"{path} ({pattern} in sonar.exclusions) is not exempt"


@pytest.mark.parametrize("linter", LINTERS)
@pytest.mark.parametrize("path", PRODUCT)
def test_product_code_stays_checked(path: str, linter: str) -> None:
    assert not _exempt(path, linter)
    assert not _sonar_excluded(path)


@pytest.mark.parametrize("linter", LINTERS)
@pytest.mark.parametrize("path", LOOKALIKE)
def test_lint_exempts_nothing_the_analysis_checks(path: str, linter: str) -> None:
    """For paths that look like generated code the lint gate may be stricter than the analysis,
    never laxer; the two M1-05 files of docs/DECISIONS.md M1-07/23 are a documented exception."""
    if _exempt(path, linter):
        assert _sonar_excluded(path), f"{linter} skips {path}, which the analysis checks"


def test_identity_bans_math_rand() -> None:
    """ID-T27: depguard keeps math/rand and math/rand/v2 out of internal/identity (M1-34)."""
    config: dict[str, Any] = yaml.safe_load(GOLANGCI.read_text(encoding="utf-8"))
    assert "depguard" in config["linters"]["enable"]
    rule = config["linters"]["settings"]["depguard"]["rules"]["crypto-rand-only-in-identity"]
    assert rule["files"] == ["**/internal/identity/**"]
    assert {d["pkg"] for d in rule["deny"]} == {"math/rand", "math/rand/v2"}
