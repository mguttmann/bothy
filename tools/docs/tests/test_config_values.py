# SPDX-License-Identifier: Apache-2.0
"""Number guard: every named value of docs/spec/23-quality.md equals the value that the
tool configuration or community file actually uses. A changed threshold must change both."""

from __future__ import annotations

import json
import re
import subprocess
import sys
import tomllib
from pathlib import Path

import pytest
import yaml

REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO / "tools" / "hooks"))
sys.path.insert(0, str(REPO / "tools" / "ci"))

import commit_msg  # noqa: E402
import license_check  # noqa: E402

_ROW = re.compile(r"^\| `(?P<name>[a-z0-9_.]+)` \| `(?P<value>[^`]+)` \|", re.MULTILINE)
_DOC_ROW = re.compile(r"\(`(?P<name>[a-z0-9_.]+)`\) \| (?P<value>\d+)\b")


def chapter_values() -> dict[str, str]:
    text = (REPO / "docs" / "spec" / "23-quality.md").read_text(encoding="utf-8")
    table = text.split("## 11. Configuration values of this chapter", 1)[1]
    return {m.group("name"): m.group("value") for m in _ROW.finditer(table)}


def _makefile_var(name: str) -> str:
    text = (REPO / "Makefile").read_text(encoding="utf-8")
    m = re.search(rf"^{name} \?= (\S+)$", text, re.MULTILINE)
    assert m, f"{name} not set in Makefile"
    return m.group(1)


def _gate_value(key: str) -> str:
    gate = json.loads((REPO / "quality" / "sonar" / "quality-gate.json").read_text())
    [cond] = [c for c in gate["conditions"] if c["config_key"] == key]
    return str(cond["error"])


def _size_hooks() -> list[dict]:
    config = yaml.safe_load((REPO / ".pre-commit-config.yaml").read_text())
    return [h for r in config["repos"] for h in r["hooks"] if h["id"] == "check-added-large-files"]


def _maxkb(hook: dict) -> str:
    [arg] = [a for a in hook["args"] if a.startswith("--maxkb=")]
    return arg.split("=", 1)[1]


def _max_file_kb() -> str:
    """The hook over every file except the REST contract and its single-file copies."""
    [hook] = [h for h in _size_hooks() if "files" not in h]
    return _maxkb(hook)


def _contract_max_file_kb() -> str:
    """The hook over the REST contract and its single-file copies (docs/DECISIONS.md PL/9)."""
    [hook] = [h for h in _size_hooks() if "files" in h]
    return _maxkb(hook)


def test_size_hooks_split_the_files_between_them() -> None:
    """The contract hook checks exactly the files the general hook leaves out, and they exist."""
    general = [h for h in _size_hooks() if "files" not in h]
    contract = [h for h in _size_hooks() if "files" in h]
    assert len(general) == len(contract) == 1
    assert general[0]["exclude"] == contract[0]["files"]
    assert "--enforce-all" in general[0]["args"]
    assert "--enforce-all" in contract[0]["args"]
    tracked = subprocess.run(
        ["git", "ls-files"], cwd=REPO, check=True, capture_output=True, text=True
    ).stdout.split()
    matched = [f for f in tracked if re.search(contract[0]["files"], f)]
    assert sorted(matched) == sorted(CONTRACT_FILES), matched


# The REST contract and the files generated from it as one file each (docs/DECISIONS.md PL/9).
CONTRACT_FILES = [
    "docs/api/openapi.yaml",
    "sdk/typescript/src/generated/openapi.ts",
    "services/server/api/rest/openapi.gen.yaml",
]


def _sonar_port() -> str:
    compose = (REPO / "tools" / "sonarqube" / "compose.yaml").read_text()
    m = re.search(r'"127\.0\.0\.1:\$\{SONARQUBE_PORT:-(\d+)\}:9000"', compose)
    assert m, "port mapping not found in compose.yaml"
    host_url = _makefile_var("SONAR_HOST_URL")
    assert host_url == f"http://127.0.0.1:{m.group(1)}", host_url
    return m.group(1)


def _python_min_percent() -> str:
    """The one fail_under of every tracked Python project under sdk/ and plugins/."""
    patterns = ["sdk/*pyproject.toml", "plugins/*pyproject.toml"]
    tracked = subprocess.run(
        ["git", "-C", str(REPO), "ls-files", "--", *patterns],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split()
    values = {
        str(tomllib.loads((REPO / name).read_text())["tool"]["coverage"]["report"]["fail_under"])
        for name in tracked
    }
    assert tracked, "no Python project under sdk/ or plugins/"
    assert len(values) == 1, f"the Python projects disagree on fail_under: {sorted(values)}"
    return values.pop()


def _doc_values(path: str) -> dict[str, str]:
    text = (REPO / path).read_text(encoding="utf-8")
    return {m.group("name"): m.group("value") for m in _DOC_ROW.finditer(text)}


def _sdk_typescript_min() -> str:
    """The one value of the four Vitest thresholds (lines, functions, branches, statements)."""
    text = (REPO / "sdk" / "typescript" / "vitest.config.ts").read_text(encoding="utf-8")
    block = re.search(r"thresholds: \{(?P<body>[^}]*)\}", text)
    assert block, "coverage.thresholds not found in sdk/typescript/vitest.config.ts"
    values = dict(re.findall(r"(\w+): (\d+)", block.group("body")))
    assert set(values) == {"lines", "functions", "branches", "statements"}, values
    assert len(set(values.values())) == 1, values
    return next(iter(values.values()))


def _testkit_patience() -> str:
    """Patience of the Go test helper, as its configuration text PatienceSetting."""
    text = (REPO / "services/server/internal/platform/testkit/testkit.go").read_text(
        encoding="utf-8"
    )
    match = re.search(r'^const PatienceSetting = "([^"]+)"$', text, re.MULTILINE)
    assert match, "testkit.go has no PatienceSetting"
    return match.group(1)


SOURCES = {
    "quality.coverage.tools_min_percent": lambda: str(
        tomllib.loads((REPO / "tools" / "pyproject.toml").read_text())["tool"]["coverage"][
            "report"
        ]["fail_under"]
    ),
    "quality.coverage.go_min_percent": lambda: _makefile_var("COVERAGE_GO_MIN"),
    "quality.coverage.dart_min_percent": lambda: _makefile_var("COVERAGE_DART_MIN"),
    "quality.coverage.python_min_percent": _python_min_percent,
    "quality.coverage.typescript_min_percent": _sdk_typescript_min,
    "quality.contract.max_examples": lambda: _makefile_var("CONTRACT_MAX_EXAMPLES"),
    "quality.contract.timeout": lambda: _makefile_var("CONTRACT_TIMEOUT"),
    "quality.contract.seed": lambda: _makefile_var("CONTRACT_SEED"),
    "quality.perf.timeout": lambda: _makefile_var("PERF_TIMEOUT"),
    "quality.test.go_timeout": lambda: _makefile_var("GO_TEST_TIMEOUT"),
    "quality.test.patience": _testkit_patience,
    "quality.sonar.new_issues_max": lambda: _gate_value("quality.sonar.new_issues_max"),
    "quality.sonar.new_hotspots_reviewed_min_percent": lambda: _gate_value(
        "quality.sonar.new_hotspots_reviewed_min_percent"
    ),
    "quality.sonar.new_coverage_min_percent": lambda: _gate_value(
        "quality.sonar.new_coverage_min_percent"
    ),
    "quality.sonar.new_duplication_max_percent": lambda: _gate_value(
        "quality.sonar.new_duplication_max_percent"
    ),
    "quality.sonar.startup_timeout_s": lambda: _makefile_var("SONAR_STARTUP_TIMEOUT_S"),
    "quality.sonar.local_port": _sonar_port,
    "commit.header_max_length": lambda: str(commit_msg.HEADER_MAX_LENGTH),
    "repo.max_file_kb": _max_file_kb,
    "api.contract_max_file_kb": _contract_max_file_kb,
    "license.spdx_header_lines": lambda: str(license_check.HEADER_SEARCH_LINES),
    "deps.min_release_age": lambda: json.loads((REPO / "renovate.json").read_text())[
        "minimumReleaseAge"
    ],
    "security.report_ack_target": lambda: _doc_values("SECURITY.md")["security.report_ack_target"],
    "security.assessment_target": lambda: _doc_values("SECURITY.md")["security.assessment_target"],
    "security.fix_target_critical": lambda: _doc_values("SECURITY.md")[
        "security.fix_target_critical"
    ],
    "security.fix_target_high": lambda: _doc_values("SECURITY.md")["security.fix_target_high"],
    "security.disclosure_deadline": lambda: _doc_values("SECURITY.md")[
        "security.disclosure_deadline"
    ],
    "triage.first_response_target": lambda: _doc_values("GOVERNANCE.md")[
        "triage.first_response_target"
    ],
    "triage.needs_info_close_after": lambda: _doc_values("GOVERNANCE.md")[
        "triage.needs_info_close_after"
    ],
}


def test_every_named_value_has_a_source() -> None:
    assert set(chapter_values()) == set(SOURCES)


@pytest.mark.parametrize("name", sorted(SOURCES))
def test_named_value_matches_its_source(name: str) -> None:
    assert chapter_values()[name] == SOURCES[name](), name


def test_coverage_threshold_in_makefile_is_not_duplicated() -> None:
    """The tools threshold lives only in tools/pyproject.toml."""
    assert "--fail-under" not in (REPO / "Makefile").read_text(encoding="utf-8")
