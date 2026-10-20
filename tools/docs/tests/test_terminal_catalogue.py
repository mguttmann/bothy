# SPDX-License-Identifier: Apache-2.0
"""Tests for the docs gate of the terminal test catalogue (brief M2-48 Design note 12).

Every test builds a small repository in `tmp_path` that passes the gate and then breaks one
thing, so each failure names exactly the check under test. The last tests run the gate against
this repository.
"""

from __future__ import annotations

import copy
from pathlib import Path
from typing import Any

import pytest
import terminal_catalogue
import yaml

REPO = Path(__file__).resolve().parents[3]

CHAPTER_IDS = ["TM-T01", "TM-T07", "TM-T12", "TM-T13", "TM-T14", "TM-T16", "TM-T24", "TM-T29"]

PLAN = """# Plan

## 7. M1 Foundation

| ID | Title | Result | Scope | Done when | Depends on |
|---|---|---|---|---|---|
| M2-38 | Noise core | x | x | x | none |
| M2-48 | Conformance suite | x | x | TM-T12 to TM-T13 and TM-T16 green | M2-38 |
| M4-35 | Terminal view | x | x | x | M2-48 |
| M6-18 | Console terminal | x | x | x | M2-48 |

## 8. M2 Agent platform

### Exit criteria

7. Something else names TM-T14.
8. Terminal (M2-48): TM-T01, TM-T07, TM-T24, TM-T29,
   AZ-M54 and ID-T28 without the console part are green; the threat model names its tests.
9. The red team says `JA`.

## 9. M3 Platform services
"""

THREATS = """# Threat model

### 6.7 Terminal

| ID | Threat | L | I | Mitigations | Tests | Residual risk |
|---|---|---|---|---|---|---|
| T-TM-01 | Reading content | medium | high | Noise | TM-T01, `TestVectorsTMT01` | low |
| T-TM-03 | Unauthorized opening | medium | high | step-up | TM-T07, TM-T12, AZ-M54 | low |
| T-TM-07 | Console | medium | high | passkey | ID-T28, origin test (PLAN M6-18) | low |
| T-TM-10 | Renderer | low | medium | widget | TM-T14 | low |

### 6.8 Visualization
"""

GO_FILES = {
    "services/host/internal/noise/vectors_test.go": ["TestVectorsTMT01"],
    "services/host/test/platform/platform_test.go": [
        "TestPlatform_TMT13",
        "TestPlatform_TMT24",
        "TestPlatform_TMT29_Parts",
    ],
    "services/server/internal/terminal/app/open_test.go": ["TestOpen_TMT07_StepUp"],
    "services/server/test/chaos/terminal_test.go": ["TestChaosTMT16"],
    "services/server/test/terminal/e2e/conformance/scenarios_test.go": ["TestScenarios"],
    "services/server/test/terminal/e2e/restart/scenarios_test.go": ["TestScenarios"],
}

SCENARIOS = ["tm-t07.yaml", "tm-t12.yaml", "tm-t16.yaml", "id-t28.yaml", "az-m54-61.yaml"]

DART = """import 'package:test/test.dart';

void main() {
  for (final name in ['cacophony']) {
    test('TM-T01 $name', () async {});
  }
  test(
    'TM-T01 literal name',
    () async {},
  );
}
"""

ALL_OS = ["linux", "macos", "windows10", "windows11"]


def _platform(ref: str) -> list[dict[str, str]]:
    return [{"kind": "platform", "ref": ref, "os": os} for os in ALL_OS]


def _entry(
    id_: str, runs: list[dict[str, str]], threats: list[str], **extra: Any
) -> dict[str, Any]:
    entry: dict[str, Any] = {
        "id": id_,
        "title": f"Title of {id_}",
        "expected": f"Expected of {id_}",
        "runs": runs,
        "implemented_by": [{"ticket": "M2-38", "tests": ["TestVectorsTMT01"]}],
        "threats": threats,
    }
    entry.update(extra)
    return entry


def _catalogue() -> dict[str, Any]:
    e2e = "e2e/conformance:scenarios/"
    return {
        "format": 1,
        "entries": [
            _entry(
                "TM-T01",
                [
                    {"kind": "unit", "ref": "services/host/internal/noise:TestVectorsTMT01"},
                    {
                        "kind": "unit",
                        "ref": "packages/bothy_noise/test/vectors_test.dart:TM-T01 cacophony",
                    },
                    {
                        "kind": "unit",
                        "ref": "packages/bothy_noise/test/vectors_test.dart:TM-T01 literal name",
                    },
                ],
                ["T-TM-01"],
            ),
            _entry(
                "TM-T07",
                [
                    {"kind": "e2e", "ref": e2e + "tm-t07.yaml"},
                    {
                        "kind": "unit",
                        "ref": "services/server/internal/terminal/app:TestOpen_TMT07_StepUp",
                    },
                ],
                ["T-TM-03"],
                deferred_parts=[{"part": "console with a password step-up", "ticket": "M6-18"}],
            ),
            _entry(
                "TM-T12",
                [{"kind": "e2e", "ref": e2e + "tm-t12.yaml"}],
                ["T-TM-03"],
                deferred_parts=[{"part": "web console without a passkey", "ticket": "M6-18"}],
            ),
            _entry("TM-T13", _platform("TestPlatform_TMT13"), []),
            _entry(
                "TM-T14",
                [],
                ["T-TM-10"],
                implemented_by=[],
                deferred={"ticket": "M4-35", "reason": "renderer of the client"},
            ),
            _entry(
                "TM-T16",
                [
                    {"kind": "e2e", "ref": "e2e/restart:scenarios/tm-t16.yaml"},
                    {"kind": "perf", "ref": "services/server/test/chaos:TestChaosTMT16"},
                ],
                [],
            ),
            _entry(
                "TM-T24",
                [
                    {"kind": "e2e", "ref": "e2e/conformance:TestScenarios"},
                    *_platform("TestPlatform_TMT24"),
                ],
                [],
            ),
            _entry(
                "TM-T29",
                [
                    {"kind": "e2e", "ref": "e2e/conformance:TestScenarios"},
                    *_platform("TestPlatform_TMT29_Parts"),
                ],
                [],
            ),
            _entry("AZ-M54", [{"kind": "e2e", "ref": e2e + "az-m54-61.yaml"}], ["T-TM-03"]),
            _entry(
                "ID-T28",
                [{"kind": "e2e", "ref": e2e + "id-t28.yaml"}],
                ["T-TM-07"],
                deferred_parts=[{"part": "console passkey step-up", "ticket": "M6-18"}],
            ),
        ],
    }


def _chapter(ids: list[str], header: str = "| ID | Test | Expected |") -> str:
    rows = "\n".join(f"| {i} | Title of {i} | Expected of {i} |" for i in ids)
    return f"# Terminal\n\n## 14. Tests\n\n{header}\n|---|---|---|\n{rows}\n\n## 15. Next\n"


def _write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _repo(tmp_path: Path, catalogue: dict[str, Any] | None = None) -> Path:
    _write(tmp_path / "docs/spec/24-terminal.md", _chapter(CHAPTER_IDS))
    _write(tmp_path / "docs/PLAN.md", PLAN)
    _write(tmp_path / "docs/security/threat-model.md", THREATS)
    for rel, names in GO_FILES.items():
        body = "".join(f"func {n}(t *testing.T) {{}}\n\n" for n in names)
        _write(tmp_path / rel, f'package x\n\nimport "testing"\n\n{body}')
    for name in SCENARIOS:
        _write(tmp_path / "services/server/test/terminal/scenarios" / name, "id: x\n")
    _write(tmp_path / "packages/bothy_noise/test/vectors_test.dart", DART)
    _save(tmp_path, catalogue if catalogue is not None else _catalogue())
    return tmp_path


def _save(repo: Path, catalogue: dict[str, Any]) -> None:
    _write(repo / terminal_catalogue.CATALOGUE, yaml.safe_dump(catalogue, sort_keys=False))


def _find(catalogue: dict[str, Any], id_: str) -> dict[str, Any]:
    entries: list[dict[str, Any]] = catalogue["entries"]
    return next(e for e in entries if e["id"] == id_)


def _broken(edit: Any) -> dict[str, Any]:
    doc = copy.deepcopy(_catalogue())
    edit(doc)
    return doc


def test_the_fixture_passes(tmp_path: Path) -> None:
    assert terminal_catalogue.check(_repo(tmp_path)) == []


# Check 1: chapter 24 section 14.


def test_missing_chapter_row_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/spec/24-terminal.md", _chapter([i for i in CHAPTER_IDS if i != "TM-T07"]))
    problems = terminal_catalogue.check(repo)
    assert problems == ["TM-T07: catalogue entry has no row in docs/spec/24-terminal.md section 14"]


def test_chapter_row_without_entry_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/spec/24-terminal.md", _chapter([*CHAPTER_IDS, "TM-T30"]))
    assert terminal_catalogue.check(repo) == [
        "TM-T30: row of docs/spec/24-terminal.md section 14 has no catalogue entry"
    ]


def test_cells_of_an_english_chapter_must_be_equal(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T12").update(title="Another title"))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T12: title differs from the cell of docs/spec/24-terminal.md section 14"
    ]


def test_cells_of_a_german_chapter_are_not_compared(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T12").update(expected="Anders"))
    repo = _repo(tmp_path, doc)
    _write(repo / "docs/spec/24-terminal.md", _chapter(CHAPTER_IDS, "| ID | Test | Erwartet |"))
    assert terminal_catalogue.check(repo) == []


def test_chapter_without_section_14_table_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/spec/24-terminal.md", "# Terminal\n\n## 14. Tests\n\nNo table.\n")
    problems = terminal_catalogue.check(repo)
    assert problems == ["docs/spec/24-terminal.md: section 14 has no test table"]


# Check 2: PLAN row M2-48 and exit criterion 8.


def test_plan_ids_parse_lists_and_ranges() -> None:
    row, exit8 = terminal_catalogue.plan_ids(PLAN)
    assert row == {"TM-T12", "TM-T13", "TM-T16"}
    assert exit8 == {"TM-T01", "TM-T07", "TM-T24", "TM-T29", "AZ-M54", "ID-T28"}


def test_plan_ids_parse_german_ranges() -> None:
    text = PLAN.replace("TM-T12 to TM-T13", "TM-T10 bis TM-T13").replace(
        "AZ-M54 and", "AZ-M54 bis AZ-M56 und"
    )
    row, exit8 = terminal_catalogue.plan_ids(text)
    assert row == {"TM-T10", "TM-T11", "TM-T12", "TM-T13", "TM-T16"}
    assert {"AZ-M54", "AZ-M55", "AZ-M56"} <= exit8


def test_plan_without_row_or_criterion_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/PLAN.md", "# Plan\n\n| M2-38 | x |\n| M4-35 | x |\n| M6-18 | x |\n")
    problems = terminal_catalogue.check(repo)
    assert "docs/PLAN.md: no row `| M2-48 |`" in problems
    assert "docs/PLAN.md: no exit criterion `8. Terminal (M2-48):` in section 8" in problems


def test_plan_id_without_run_is_reported(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T12").update(runs=[]))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T12: named by the PLAN row M2-48 or exit criterion 8 but has no run of kind "
        "e2e or unit or platform run on linux"
    ]


def test_plan_id_without_entry_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/PLAN.md", PLAN.replace("TM-T16 green", "TM-T16 and TM-T11 green"))
    assert terminal_catalogue.check(repo) == [
        "TM-T11: named by the PLAN row M2-48 or exit criterion 8 but has no catalogue entry"
    ]


def test_platform_run_counts_only_on_linux(tmp_path: Path) -> None:
    def edit(d: dict[str, Any]) -> None:
        _find(d, "TM-T13")["runs"] = [r for r in _find(d, "TM-T13")["runs"] if r["os"] != "linux"]

    assert terminal_catalogue.check(_repo(tmp_path, _broken(edit))) == [
        "TM-T13: named by the PLAN row M2-48 or exit criterion 8 but has no run of kind "
        "e2e or unit or platform run on linux"
    ]


def test_plan_id_with_deferred_is_rejected(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T07").update(deferred={"ticket": "M6-18", "reason": "x"}))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T07: named by the PLAN row M2-48 or exit criterion 8 and must not be deferred"
    ]


def test_tmt16_needs_a_perf_run(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T16")["runs"].pop())
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == ["TM-T16: needs a run of kind perf"]


def test_doctor_matrix_needs_every_platform(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T24")["runs"].pop())
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T24: doctor matrix ID needs platform runs on windows11"
    ]


def test_console_part_must_be_deferred_to_m6_18(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "ID-T28").pop("deferred_parts"))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "ID-T28: needs a deferred_parts entry for its console part with ticket M6-18"
    ]


def test_deferred_parts_name_plan_tickets(tmp_path: Path) -> None:
    doc = _broken(
        lambda d: _find(d, "TM-T12")["deferred_parts"].append({"part": "x", "ticket": "M9-99"})
    )
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T12: deferred_parts ticket M9-99 is no row of docs/PLAN.md"
    ]


# Check 3: threat model section 6.7.


def test_threat_without_test_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    text = THREATS.replace("| TM-T14 | low |", "| none yet | low |")
    _write(repo / "docs/security/threat-model.md", text)
    assert terminal_catalogue.check(repo) == [
        "T-TM-10: row of docs/security/threat-model.md section 6.7 names no test"
    ]


def test_threat_test_without_threat_in_catalogue_is_reported(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "AZ-M54").update(threats=[]))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "T-TM-03: names AZ-M54, whose catalogue entry does not list the threat"
    ]


def test_threat_test_without_entry_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/security/threat-model.md", THREATS.replace("AZ-M54 |", "AZ-M54, SEC-T05 |"))
    assert terminal_catalogue.check(repo) == [
        "T-TM-03: names SEC-T05, which has no catalogue entry"
    ]


def test_threat_ranges_are_expanded(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(
        repo / "docs/security/threat-model.md",
        THREATS.replace("TM-T07, TM-T12", "TM-T07 bis TM-T08"),
    )
    assert terminal_catalogue.check(repo) == ["T-TM-03: names TM-T08, which has no catalogue entry"]


def test_threat_go_test_must_exist(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(
        repo / "docs/security/threat-model.md",
        THREATS.replace("`TestVectorsTMT01`", "`TestVectorsTMT01`, `TestGone_TMT01`"),
    )
    assert terminal_catalogue.check(repo) == [
        "T-TM-01: Go test TestGone_TMT01 is no `func TestGone_TMT01(` under services/"
    ]


def test_threat_plan_reference_must_exist(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/security/threat-model.md", THREATS.replace("PLAN M6-18", "PLAN M6-99"))
    assert terminal_catalogue.check(repo) == ["T-TM-07: PLAN M6-99 is no row of docs/PLAN.md"]


def test_threat_model_without_section_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / "docs/security/threat-model.md", "# Threat model\n")
    assert terminal_catalogue.check(repo) == [
        "docs/security/threat-model.md: section 6.7 has no threat table"
    ]


# Check 4: every ref and ticket exists.


def test_ref_to_missing_function_is_reported(tmp_path: Path) -> None:
    def edit(d: dict[str, Any]) -> None:
        _find(d, "TM-T07")["runs"][1]["ref"] = "services/server/internal/terminal/app:TestGone"

    assert terminal_catalogue.check(_repo(tmp_path, _broken(edit))) == [
        "TM-T07: ref services/server/internal/terminal/app:TestGone names no "
        "`func TestGone(` in services/server/internal/terminal/app"
    ]


@pytest.mark.parametrize(
    ("id_", "index", "ref", "problem"),
    [
        (
            "TM-T12",
            0,
            "e2e/conformance:scenarios/tm-t99.yaml",
            "TM-T12: ref e2e/conformance:scenarios/tm-t99.yaml names no file "
            "services/server/test/terminal/scenarios/tm-t99.yaml",
        ),
        (
            "TM-T24",
            0,
            "e2e/conformance:TestGone",
            "TM-T24: ref e2e/conformance:TestGone names no `func TestGone(` in "
            "services/server/test/terminal/e2e/conformance",
        ),
        (
            "TM-T13",
            0,
            "TestPlatform_Gone",
            "TM-T13: ref TestPlatform_Gone names no `func TestPlatform_Gone(` in "
            "services/host/test/platform",
        ),
        (
            "TM-T16",
            1,
            "services/server/test/chaos:TestChaosGone",
            "TM-T16: ref services/server/test/chaos:TestChaosGone names no "
            "`func TestChaosGone(` in services/server/test/chaos",
        ),
        (
            "TM-T01",
            1,
            "packages/bothy_noise/test/vectors_test.dart:TM-T02 gone",
            "TM-T01: ref packages/bothy_noise/test/vectors_test.dart:TM-T02 gone names no "
            "Dart test or group",
        ),
        (
            "TM-T01",
            1,
            "packages/bothy_noise/test/missing_test.dart:TM-T01 x",
            "TM-T01: ref packages/bothy_noise/test/missing_test.dart:TM-T01 x names no "
            "Dart test or group",
        ),
    ],
)
def test_missing_refs_are_reported(
    tmp_path: Path, id_: str, index: int, ref: str, problem: str
) -> None:
    doc = _broken(lambda d: _find(d, id_)["runs"][index].update(ref=ref))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [problem]


def test_implemented_by_ticket_must_exist(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T01")["implemented_by"][0].update(ticket="M2-99"))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T01: implemented_by ticket M2-99 is no row of docs/PLAN.md"
    ]


def test_implemented_by_test_must_exist(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T01")["implemented_by"][0].update(tests=["TestGone"]))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T01: implemented_by test TestGone is neither a Go test under services/ nor a "
        "Dart test of packages/bothy_noise/test"
    ]


def test_deferred_ticket_must_exist(tmp_path: Path) -> None:
    doc = _broken(lambda d: _find(d, "TM-T14")["deferred"].update(ticket="M4-99"))
    assert terminal_catalogue.check(_repo(tmp_path, doc)) == [
        "TM-T14: deferred ticket M4-99 is no row of docs/PLAN.md"
    ]


# Structure.


@pytest.mark.parametrize(
    ("edit", "problem"),
    [
        (lambda d: d.update(extra=1), "catalogue: unknown field extra"),
        (lambda d: d.update(format=2), "catalogue: format must be 1"),
        (lambda d: d.pop("entries"), "catalogue: missing field entries"),
        (lambda d: _find(d, "TM-T01").update(note="x"), "TM-T01: unknown field note"),
        (lambda d: _find(d, "TM-T01").pop("threats"), "TM-T01: missing field threats"),
        (
            lambda d: d["entries"].insert(1, copy.deepcopy(d["entries"][0])),
            "TM-T01: duplicate ID",
        ),
        (
            lambda d: d["entries"].insert(0, d["entries"].pop(1)),
            "TM-T01: entries are not ordered by ID (after TM-T07)",
        ),
        (lambda d: _find(d, "TM-T01").update(id="TM-1"), "TM-1: ID has an unknown form"),
        (
            lambda d: _find(d, "TM-T01")["runs"][0].update(kind="manual"),
            "TM-T01: run 1 has unknown kind manual",
        ),
        (
            lambda d: _find(d, "TM-T13")["runs"][0].update(os="freebsd"),
            "TM-T13: run 1 has unknown os freebsd",
        ),
        (
            lambda d: _find(d, "TM-T13")["runs"][0].pop("os"),
            "TM-T13: run 1 of kind platform needs os",
        ),
        (
            lambda d: _find(d, "TM-T01")["runs"][0].update(os="linux"),
            "TM-T01: run 1 of kind unit must not have os",
        ),
        (
            lambda d: _find(d, "TM-T01")["runs"][0].update(extra="x"),
            "TM-T01: run 1 has unknown field extra",
        ),
        (
            lambda d: _find(d, "TM-T07")["runs"][0].update(ref="e2e/other:TestScenarios"),
            "TM-T07: run 1 ref e2e/other:TestScenarios has an unknown form",
        ),
        (
            lambda d: _find(d, "TM-T16")["runs"][1].update(ref="services/server/test/perf:TestX"),
            "TM-T16: run 2 ref services/server/test/perf:TestX has an unknown form",
        ),
        (
            lambda d: _find(d, "TM-T12")["deferred_parts"][0].pop("ticket"),
            "TM-T12: deferred_parts entry 1 missing field ticket",
        ),
        (
            lambda d: _find(d, "TM-T12")["deferred_parts"][0].update(owner="x"),
            "TM-T12: deferred_parts entry 1 has unknown field owner",
        ),
        (
            lambda d: _find(d, "TM-T14")["deferred"].pop("reason"),
            "TM-T14: deferred missing field reason",
        ),
        (
            lambda d: _find(d, "TM-T01")["implemented_by"][0].pop("tests"),
            "TM-T01: implemented_by entry 1 missing field tests",
        ),
        (
            lambda d: _find(d, "TM-T01").update(threats=["TM-01"]),
            "TM-T01: threat TM-01 has an unknown form",
        ),
        (lambda d: _find(d, "TM-T01").update(title=7), "TM-T01: title must be a text"),
        (
            lambda d: _find(d, "TM-T01").update(implemented_by=5),
            "TM-T01: implemented_by must be a list",
        ),
        (
            lambda d: _find(d, "TM-T12")["deferred_parts"][0].update(ticket=["M6-18"]),
            "TM-T12: deferred_parts entry 1 ticket ['M6-18'] has an unknown form",
        ),
        (
            lambda d: _find(d, "TM-T14")["deferred"].update(ticket="M4"),
            "TM-T14: deferred ticket M4 has an unknown form",
        ),
    ],
)
def test_structure_is_validated(tmp_path: Path, edit: Any, problem: str) -> None:
    assert problem in terminal_catalogue.check(_repo(tmp_path, _broken(edit)))


def test_duplicate_yaml_keys_are_rejected(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    _write(repo / terminal_catalogue.CATALOGUE, "format: 1\nformat: 1\nentries: []\n")
    problems = terminal_catalogue.check(repo)
    assert len(problems) == 1
    assert problems[0].startswith("catalogue: duplicate key format")


def test_missing_catalogue_is_reported(tmp_path: Path) -> None:
    repo = _repo(tmp_path)
    (repo / terminal_catalogue.CATALOGUE).unlink()
    assert terminal_catalogue.check(repo) == [
        f"catalogue: cannot read {terminal_catalogue.CATALOGUE}"
    ]


# Command line.


def test_check_mode_prints_one_line_per_problem(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    doc = _broken(lambda d: _find(d, "TM-T16")["runs"].pop())
    repo = _repo(tmp_path, doc)
    assert terminal_catalogue.main(["--check", "--repo", str(repo)]) == 1
    assert capsys.readouterr().err == "terminal_catalogue: TM-T16: needs a run of kind perf\n"
    _save(repo, _catalogue())
    assert terminal_catalogue.main(["--check", "--repo", str(repo)]) == 0


# This repository.


def test_this_repository_passes() -> None:
    assert terminal_catalogue.check(REPO) == []
