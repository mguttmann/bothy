# SPDX-License-Identifier: Apache-2.0
"""The accessibility checklists name only tests that exist (briefs M4-32 and M5-10 Design note 3).

`docs/quality/accessibility-checklist.md` (desktop) and `docs/quality/accessibility-mobile.md`
(phones) name their automated evidence as a backticked test file followed by quoted test names.
Every file must exist and every name must be a test of that file: a literal name, a name built
from a template with `$name` or `${...}`, or a screen name of `dsAccessibilityTests` or
`dsPhoneAccessibilityTests` (package:design_system/testing.dart) followed by one of the names the
harness gives its tests. The platform audits of the phones are named the same way, a Swift file
with its XCTest methods (`func <name>(`) and a Kotlin file with its JUnit methods
(`fun <name>(`). So the checklists cannot point at a deleted or renamed test.
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[3]
CHECKLISTS = [
    ROOT / "docs" / "quality" / "accessibility-checklist.md",
    ROOT / "docs" / "quality" / "accessibility-mobile.md",
]
_TESTING = ROOT / "packages" / "design_system" / "lib" / "src" / "testing"
HARNESSES = [_TESTING / "a11y_screen.dart", _TESTING / "phone_a11y_screen.dart"]

_EVIDENCE = re.compile(r"`([\w./-]+_test\.dart)`((?:\s*:\s*|;\s*)?(?:\"[^\"]+\"(?:;\s*)?)*)")
_NATIVE = re.compile(r"`([\w./-]+Tests?\.(?:swift|kt))`((?:\s*:\s*)(?:\"[^\"]+\"(?:;\s*)?)*)")
_NAME = re.compile(r"\"([^\"]+)\"")
# Adjacent string literals ('a' 'b', also across lines) form one string in Dart.
_ADJACENT = re.compile(r"'\s*\n\s*'|'\s+'")
_LITERAL = re.compile(r"'((?:[^'\\\n]|\\.)*)'")
_INTERPOLATION = re.compile(r"\\\$\\\{[^}]*\\\}|\\\$\w+")


def evidence(text: str) -> list[tuple[str, list[str]]]:
    """The (test file, test names) pairs the checklist names."""
    return [(m.group(1), _NAME.findall(m.group(2))) for m in _EVIDENCE.finditer(text)]


def templates(dart: str) -> list[re.Pattern[str]]:
    """Patterns of the string literals of a Dart file, interpolations as wildcards."""
    joined = _ADJACENT.sub("", dart)
    out = []
    for literal in _LITERAL.findall(joined):
        escaped = re.escape(literal.replace("\\'", "'"))
        out.append(re.compile(_INTERPOLATION.sub(".+", escaped)))
    return out


def native_evidence(text: str) -> list[tuple[str, list[str]]]:
    """The (Swift or Kotlin test file, method names) pairs a checklist names."""
    return [(m.group(1), _NAME.findall(m.group(2))) for m in _NATIVE.finditer(text)]


def native_methods(source: str) -> set[str]:
    """The method names of a Swift (`func name(`) or Kotlin (`fun name(`) test file."""
    return set(re.findall(r"\bfunc? (\w+)\(", source))


def harness_suffixes() -> list[str]:
    """The test names of the harnesses after the screen name ("$screen...")."""
    out = []
    for harness in HARNESSES:
        joined = _ADJACENT.sub("", harness.read_text(encoding="utf-8"))
        out += [
            lit[len("$screen") :] for lit in _LITERAL.findall(joined) if lit.startswith("$screen")
        ]
    return out


def is_test_of(name: str, patterns: list[re.Pattern[str]], suffixes: list[str]) -> bool:
    """Whether [name] is a literal or templated test name, or a harness screen name."""
    for pattern in patterns:
        if pattern.fullmatch(name):
            return True
        for suffix in suffixes:
            if name.endswith(suffix) and pattern.fullmatch(name[: -len(suffix)]):
                return True
    return False


def test_harness_names_are_found() -> None:
    suffixes = harness_suffixes()
    assert ": roles, labels and states for screen readers" in suffixes
    assert " on the phone: roles, labels and states for screen readers" in suffixes
    assert len(suffixes) == 3 + 5


def test_parser_reads_files_and_names() -> None:
    text = 'x | `a/b_test.dart`: "one"; "two" | `c/d_test.dart` | y'
    assert evidence(text) == [("a/b_test.dart", ["one", "two"]), ("c/d_test.dart", [])]


def test_templates_match_interpolated_and_adjacent_names() -> None:
    dart = (
        "test('on gives reduced (platform $platform)', () {});\n"
        "testWidgets('only state changes '\n    'are announced', (t) {});\n"
    )
    patterns = templates(dart)
    assert is_test_of("on gives reduced (platform $platform)", patterns, [])
    assert is_test_of("on gives reduced (platform macos)", patterns, [])
    assert is_test_of("only state changes are announced", patterns, [])
    assert not is_test_of("a deleted test", patterns, [])


def test_harness_screen_names_match() -> None:
    dart = "dsAccessibilityTests('the sidebar', build: sidebar);"
    suffix = ": roles, labels and states for screen readers"
    assert is_test_of("the sidebar" + suffix, templates(dart), [suffix])
    assert not is_test_of("the post list" + suffix, templates(dart), [suffix])


def _candidates(path: str) -> list[Path]:
    """The test files [path] names: the file at that path, or for a bare file name every test
    file of that name in the members (a section may name a file of its own member briefly)."""
    direct = ROOT / path
    if direct.is_file():
        return [direct]
    if "/" in path:
        return []
    return [
        p
        for p in ROOT.glob(f"*/*/test/**/{path}")
        if "node_modules" not in p.parts and ".dart_tool" not in p.parts
    ]


def test_a_bare_file_name_may_name_several_files() -> None:
    files = _candidates("accessibility_test.dart")
    assert files
    assert all(f.name == "accessibility_test.dart" for f in files)
    assert _candidates("no_such_file_test.dart") == []


def test_native_methods_are_found() -> None:
    swift = "final class A: XCTestCase {\n  func testM1Spaces() throws { }\n}"
    kotlin = "class A {\n  @Test fun m1Spaces() = audit()\n}"
    assert native_methods(swift) == {"testM1Spaces"}
    assert native_methods(kotlin) == {"m1Spaces"}
    text = 'x `a/AuditTests.swift`: "testA"; "testB" | `b/ChecksTest.kt`: "c" |'
    assert native_evidence(text) == [
        ("a/AuditTests.swift", ["testA", "testB"]),
        ("b/ChecksTest.kt", ["c"]),
    ]


@pytest.mark.parametrize("checklist", CHECKLISTS, ids=lambda p: p.name)
def test_every_native_evidence_exists(checklist: Path) -> None:
    problems = []
    for path, names in native_evidence(checklist.read_text(encoding="utf-8")):
        file = ROOT / path
        if not file.is_file():
            problems.append(f"{path}: no such test file")
            continue
        methods = native_methods(file.read_text(encoding="utf-8"))
        problems += [f"{path}: no test {n!r}" for n in names if n not in methods]
    assert problems == [], "\n".join(problems)


@pytest.mark.parametrize("checklist", CHECKLISTS, ids=lambda p: p.name)
def test_every_evidence_of_the_checklist_exists(checklist: Path) -> None:
    text = checklist.read_text(encoding="utf-8")
    pairs = evidence(text)
    assert len(pairs) > 40, 'the checklist names its evidence as `file`: "name"'
    suffixes = harness_suffixes()
    problems = []
    for path, names in pairs:
        files = _candidates(path)
        if not files:
            problems.append(f"{path}: no such test file")
            continue
        # One of the files of that name holds every named test.
        missing = min(
            (
                [
                    n
                    for n in names
                    if not is_test_of(n, templates(f.read_text(encoding="utf-8")), suffixes)
                ]
                for f in files
            ),
            key=len,
        )
        problems.extend(f"{path}: no test {name!r}" for name in missing)
    assert problems == [], "\n".join(problems)


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(pytest.main([__file__]))
