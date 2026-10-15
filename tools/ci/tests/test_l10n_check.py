# SPDX-License-Identifier: Apache-2.0
"""Tests for l10n_check: a fresh `flutter gen-l10n` in a temporary copy (brief M4-06, note 2).

A fake `flutter` stands in for the SDK: called as `flutter gen-l10n` in the copy it writes the
files of FAKE_GEN_DIR into the output directory of the copy's l10n.yaml and the text of
FAKE_UNTRANSLATED into the untranslated-messages-file; FAKE_EXIT sets its exit code. It logs
its working directory and the l10n.yaml it saw, so the tests can prove that the generator ran
in a copy and never in the package itself.
"""

from __future__ import annotations

import json
import os
import sys
from pathlib import Path

import l10n_check as lc
import pytest

_FAKE = """#!{python}
import json, os, pathlib, sys
cwd = pathlib.Path.cwd()
config = (cwd / "l10n.yaml").read_text(encoding="utf-8")
with open(os.environ["FAKE_LOG"], "a", encoding="utf-8") as fh:
    fh.write(json.dumps({{"args": sys.argv[1:], "cwd": str(cwd), "config": config}}) + "\\n")
if os.environ.get("FAKE_EXIT"):
    sys.exit(int(os.environ["FAKE_EXIT"]))
options = dict(line.split(": ", 1) for line in config.splitlines() if ": " in line)
out = cwd / options["output-dir"].strip()
out.mkdir(parents=True, exist_ok=True)
for src in pathlib.Path(os.environ["FAKE_GEN_DIR"]).iterdir():
    (out / src.name).write_text(src.read_text(encoding="utf-8"), encoding="utf-8")
target = options.get("untranslated-messages-file")
if target is not None and os.environ.get("FAKE_UNTRANSLATED") is not None:
    path = pathlib.Path(target.strip().strip('"'))
    path.write_text(os.environ["FAKE_UNTRANSLATED"], encoding="utf-8")
"""

L10N_YAML = """arb-dir: lib/l10n
template-arb-file: app_en.arb
output-dir: lib/src/generated
"""


class Case:
    """A package with ARB files, checked-in generated code and a recording fake flutter."""

    def __init__(self, tmp: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        self.package = tmp / "pkg"
        self.gen = tmp / "fresh"
        self.log = tmp / "calls.jsonl"
        self.flutter = tmp / "bin" / "flutter"
        for rel, text in {
            "pubspec.yaml": "name: pkg\nflutter:\n  generate: true\n",
            "l10n.yaml": L10N_YAML,
            "lib/l10n/app_en.arb": '{"a": "A"}\n',
            "lib/l10n/app_de.arb": '{"a": "A"}\n',
            "lib/src/generated/app_localizations.dart": "// generated\n",
            "lib/src/hand.dart": "// hand-written\n",
        }.items():
            path = self.package / rel
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text, encoding="utf-8")
        self.gen.mkdir()
        (self.gen / "app_localizations.dart").write_text("// generated\n", encoding="utf-8")
        self.flutter.parent.mkdir()
        self.flutter.write_text(_FAKE.format(python=sys.executable), encoding="utf-8")
        self.flutter.chmod(0o755)
        monkeypatch.setenv("FAKE_LOG", str(self.log))
        monkeypatch.setenv("FAKE_GEN_DIR", str(self.gen))
        monkeypatch.setenv("FAKE_UNTRANSLATED", "{}")
        monkeypatch.delenv("FAKE_EXIT", raising=False)

    def run(self) -> int:
        return lc.main(["--dir", str(self.package), "--flutter", str(self.flutter)])

    def calls(self) -> list[dict[str, str]]:
        return [json.loads(line) for line in self.log.read_text(encoding="utf-8").splitlines()]


@pytest.fixture
def case(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Case:
    return Case(tmp_path, monkeypatch)


def test_fresh_generation_equal_to_checked_in_passes(
    case: Case, capsys: pytest.CaptureFixture[str]
) -> None:
    assert case.run() == 0
    assert "l10n-check: pkg ok" in capsys.readouterr().out
    [call] = case.calls()
    assert call["args"] == ["gen-l10n"]
    assert Path(call["cwd"]).resolve() != case.package.resolve()
    assert "untranslated-messages-file: " in call["config"]
    # The package itself is untouched and the temporary copy is gone.
    assert (case.package / "l10n.yaml").read_text(encoding="utf-8") == L10N_YAML
    assert not Path(call["cwd"]).exists()


def test_untranslated_message_fails(case: Case, capsys: pytest.CaptureFixture[str]) -> None:
    os.environ["FAKE_UNTRANSLATED"] = json.dumps({"de": ["onlyEnglish"]})
    assert case.run() == 1
    out = capsys.readouterr().out
    assert "untranslated messages" in out
    assert "de: onlyEnglish" in out


def test_missing_untranslated_file_fails(case: Case, capsys: pytest.CaptureFixture[str]) -> None:
    del os.environ["FAKE_UNTRANSLATED"]
    assert case.run() == 1
    assert "wrote no untranslated-messages-file" in capsys.readouterr().out


def test_stale_generated_file_fails(case: Case, capsys: pytest.CaptureFixture[str]) -> None:
    (case.gen / "app_localizations.dart").write_text("// newer\n", encoding="utf-8")
    assert case.run() == 1
    out = capsys.readouterr().out
    assert "changed: lib/src/generated/app_localizations.dart" in out
    assert "stale" in out


def test_added_and_removed_generated_files_fail(
    case: Case, capsys: pytest.CaptureFixture[str]
) -> None:
    (case.gen / "app_localizations_fr.dart").write_text("// fr\n", encoding="utf-8")
    (case.package / "lib/src/generated/old.dart").write_text("// old\n", encoding="utf-8")
    assert case.run() == 1
    out = capsys.readouterr().out
    assert "added: lib/src/generated/app_localizations_fr.dart" in out
    assert "removed: lib/src/generated/old.dart" in out


def test_generator_failure_passes_its_exit_code(case: Case) -> None:
    os.environ["FAKE_EXIT"] = "3"
    assert case.run() == 3


def test_missing_config_is_a_usage_error(case: Case, capsys: pytest.CaptureFixture[str]) -> None:
    (case.package / "l10n.yaml").unlink()
    assert case.run() == 2
    assert "l10n.yaml" in capsys.readouterr().err


def test_config_that_already_names_an_untranslated_file_is_rejected(
    case: Case, capsys: pytest.CaptureFixture[str]
) -> None:
    (case.package / "l10n.yaml").write_text(
        L10N_YAML + "untranslated-messages-file: x.json\n", encoding="utf-8"
    )
    assert case.run() == 2
    assert "untranslated-messages-file" in capsys.readouterr().err
