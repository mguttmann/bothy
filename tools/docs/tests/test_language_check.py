# SPDX-License-Identifier: Apache-2.0
"""Tests for the language gate (tools/docs/language_check.py).

German sample text lives in data/language/ (excluded by the allow list); words with umlauts
in this module are built with chr(), so that the module itself is English text.
"""

from __future__ import annotations

import json
import subprocess
from pathlib import Path

import language_check
import pytest

DATA = Path(__file__).parent / "data" / "language"
UE, OE, SZ = chr(0xFC), chr(0xF6), chr(0xDF)
CHECKSUM = f"Pr{UE}fsumme"
SIZE = f"Gr{OE}{SZ}e"
NAME = f"M{UE}ller"
NO_WORDS = language_check.AllowList(paths=(), words=frozenset())


def _write(tmp_path: Path, name: str, content: str | bytes) -> Path:
    path = tmp_path / name
    path.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(content, bytes):
        path.write_bytes(content)
    else:
        path.write_text(content, encoding="utf-8")
    return path


def test_german_paragraph_fails() -> None:
    problems = language_check.check_file(DATA / "german-paragraph.md.txt", NO_WORDS)
    prose = [p for p in problems if "German prose" in p]
    assert prose, problems
    assert prose[0].startswith(f"{DATA / 'german-paragraph.md.txt'}:3: ")


def test_german_paragraph_without_umlauts_fails() -> None:
    problems = language_check.check_file(DATA / "german-without-umlauts.txt", NO_WORDS)
    assert len(problems) == 1
    assert ":1: German prose" in problems[0]


def test_english_with_words_shared_with_german_passes() -> None:
    assert language_check.check_file(DATA / "english-technical.txt", NO_WORDS) == []


def test_word_with_umlaut_fails(tmp_path: Path) -> None:
    path = _write(tmp_path, "a.md", f"Intro line.\nThe row keeps the {CHECKSUM} of the file.\n")
    assert language_check.check_file(path, NO_WORDS) == [
        f"{path}:2: word with umlaut or sharp s: '{CHECKSUM}'"
    ]


def test_sharp_s_counts_as_german(tmp_path: Path) -> None:
    path = _write(tmp_path, "a.py", f"# keep the {SIZE} small\n")
    assert language_check.check_file(path, NO_WORDS) == [
        f"{path}:1: word with umlaut or sharp s: '{SIZE}'"
    ]


def test_escaped_umlaut_in_json_fails(tmp_path: Path) -> None:
    path = _write(tmp_path, "a.json", json.dumps({"$comment": f"the {CHECKSUM} of a file"}) + "\n")
    assert language_check.check_file(path, NO_WORDS) == [
        f"{path}:1: word with umlaut or sharp s: '{CHECKSUM}'"
    ]


@pytest.mark.parametrize(
    "line",
    [
        f"See [{CHECKSUM}](x.md).",
        f"{CHECKSUM}/approval",
        f'<span title="{SIZE}">',
        f'print("{CHECKSUM}")',
        f'{{"name":"{SIZE}"}}',
        f"\u201e{CHECKSUM}\u201c",
    ],
)
def test_umlaut_word_inside_markup_or_code_fails(tmp_path: Path, line: str) -> None:
    path = _write(tmp_path, "a.md", line + "\n")
    problems = language_check.check_file(path, NO_WORDS)
    assert len(problems) == 1
    assert problems[0].startswith(f"{path}:1: word with umlaut or sharp s: ")


def test_lone_letter_in_code_is_no_word(tmp_path: Path) -> None:
    path = _write(tmp_path, "a.sql", f"SELECT ${chr(0xE4)}$ concurrently ${chr(0xE4)}$;\n")
    assert language_check.check_file(path, NO_WORDS) == []


def test_allowed_proper_name_passes(tmp_path: Path) -> None:
    path = _write(tmp_path, "a.md", f"Thanks to Jana {NAME} for the review.\n")
    allow = language_check.AllowList(paths=(), words=frozenset({NAME}))
    assert language_check.check_file(path, allow) == []


def test_binary_file_is_skipped(tmp_path: Path) -> None:
    german = (DATA / "german-paragraph.md.txt").read_bytes()
    path = _write(tmp_path, "a.png", b"\x89PNG\x00\x00" + german)
    assert language_check.check_file(path, NO_WORDS) == []


def test_text_that_is_not_utf8_is_skipped(tmp_path: Path) -> None:
    german = (DATA / "german-paragraph.md.txt").read_text(encoding="utf-8")
    path = _write(tmp_path, "a.txt", german.encode("latin-1"))
    assert language_check.check_file(path, NO_WORDS) == []


def test_one_finding_per_german_paragraph(tmp_path: Path) -> None:
    german = (DATA / "german-without-umlauts.txt").read_text(encoding="utf-8")
    english = "This paragraph is English and names nothing else of interest here at all.\n"
    path = _write(tmp_path, "a.md", german + "\n" + english * 3 + "\n" + german)
    problems = language_check.check_file(path, NO_WORDS)
    lines = [p.split(":")[1] for p in problems]
    assert lines == ["1", "8"]


def test_marked_region_is_skipped(tmp_path: Path) -> None:
    german = (DATA / "german-paragraph.md.txt").read_text(encoding="utf-8")
    text = f"Intro.\n\n<!-- language-check: off -->\n{german}<!-- language-check: on -->\n\nEnd.\n"
    path = _write(tmp_path, "a.md", text)
    assert language_check.check_file(path, NO_WORDS) == []


def test_text_after_a_marked_region_is_checked(tmp_path: Path) -> None:
    german = (DATA / "german-without-umlauts.txt").read_text(encoding="utf-8")
    text = f"# language-check: off\n{german}# language-check: on\n{german}"
    path = _write(tmp_path, "a.py", text)
    problems = language_check.check_file(path, NO_WORDS)
    assert [p.split(":")[1] for p in problems] == ["5"]


def test_line_naming_both_markers_is_no_marker(tmp_path: Path) -> None:
    german = (DATA / "german-without-umlauts.txt").read_text(encoding="utf-8")
    text = f"Mark German with `language-check: off` and `language-check: on`.\n{german}"
    path = _write(tmp_path, "a.md", text)
    assert [p.split(":")[1] for p in language_check.check_file(path, NO_WORDS)] == ["2"]


def test_marker_named_in_prose_is_no_marker(tmp_path: Path) -> None:
    german = (DATA / "german-without-umlauts.txt").read_text(encoding="utf-8")
    text = f"Text marked `language-check: off` is skipped.\n{german}"
    path = _write(tmp_path, "a.md", text)
    assert [p.split(":")[1] for p in language_check.check_file(path, NO_WORDS)] == ["2"]


def test_region_without_end_marker_fails(tmp_path: Path) -> None:
    path = _write(tmp_path, "a.md", "Intro.\n<!-- language-check: off -->\nRest.\n")
    assert language_check.check_file(path, NO_WORDS) == [
        f"{path}:2: 'language-check: off' without a later 'language-check: on'"
    ]


def test_load_allow_list_reads_paths_words_and_skips_comments(tmp_path: Path) -> None:
    path = _write(
        tmp_path,
        "allow.txt",
        f"# comment\n\npath: docs/glossary.md\npath: tools/x/data/*\n{NAME}\n  Z{UE}rich  \n",
    )
    allow = language_check.load_allow_list(path)
    assert allow.paths == ("docs/glossary.md", "tools/x/data/*")
    assert allow.words == frozenset({NAME, f"Z{UE}rich"})


@pytest.mark.parametrize(
    ("name", "excluded"),
    [
        ("docs/glossary.md", True),
        ("tools/x/data/german.txt", True),
        ("docs/spec/00-glossary.md", False),
        ("tools/x/datax/german.txt", False),
    ],
)
def test_path_allow_list_matches_globs(name: str, excluded: bool) -> None:
    allow = language_check.AllowList(
        paths=("docs/glossary.md", "tools/x/data/*"), words=frozenset()
    )
    assert allow.excludes(name) is excluded


def _git_repo(tmp_path: Path, files: dict[str, str]) -> Path:
    for name, text in files.items():
        _write(tmp_path, name, text)
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    subprocess.run(["git", "-C", str(tmp_path), "add", "-A"], check=True)
    return tmp_path


def test_main_scans_tracked_files_and_honours_the_allow_list(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    german = (DATA / "german-paragraph.md.txt").read_text(encoding="utf-8")
    root = _git_repo(
        tmp_path,
        {
            "allow.txt": "path: docs/glossary.md\n",
            "docs/glossary.md": german,
            "docs/page.md": german,
            "README.md": "English only.\n",
        },
    )
    _write(root, "untracked.md", german)
    code = language_check.main(["--root", str(root), "--allow", str(root / "allow.txt")])
    err = capsys.readouterr().err
    assert code == 1
    assert "docs/page.md:3: German prose" in err
    assert "glossary" not in err
    assert "untracked" not in err


def test_main_passes_on_english_repository(tmp_path: Path) -> None:
    root = _git_repo(tmp_path, {"allow.txt": "", "README.md": "All content is English.\n"})
    assert language_check.main(["--root", str(root), "--allow", str(root / "allow.txt")]) == 0
