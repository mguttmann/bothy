# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/catalog/apple_strings.py, the native strings of the push and the share sheet."""

from __future__ import annotations

import importlib.util
import json
import sys
from pathlib import Path

_SPEC = importlib.util.spec_from_file_location(
    "apple_strings_tool", Path(__file__).resolve().parents[1] / "apple_strings.py"
)
assert _SPEC is not None
assert _SPEC.loader is not None
gen = importlib.util.module_from_spec(_SPEC)
sys.modules[_SPEC.name] = gen
_SPEC.loader.exec_module(gen)

REPO = Path(__file__).resolve().parents[3]


# The placeholders the template declares, in the order that gives their positions.
_PLACEHOLDERS: dict[str, tuple[str, ...]] = {
    "notificationTitle": ("name", "title"),
    "notificationText": ("author", "excerpt"),
    "shareNewPost": ("path",),
    "shareMoreFiles": ("count",),
    "shareFileTooLarge": ("name", "limit"),
    "shareTooManyFiles": ("count",),
    "shareRefused": ("reason",),
    "shareReasonStatus": ("status",),
}


def _arb(locale: str, texts: dict[str, str], *, template: bool) -> dict[str, object]:
    arb: dict[str, object] = {"@@locale": locale}
    for key, text in texts.items():
        arb[key] = text
        if template and key in _PLACEHOLDERS:
            arb[f"@{key}"] = {
                "placeholders": {name: {"type": "String"} for name in _PLACEHOLDERS[key]}
            }
    return arb


def _texts(prefix: str) -> dict[str, str]:
    keys = [*gen.NATIVE_KEYS.values(), *gen.PLURAL_KEYS.values()]
    texts = {arb: f"{prefix} {arb}" for arb in keys}
    texts["notificationTitle"] = "{name} · {title}"
    texts["notificationText"] = '{author}: "{excerpt}" 100%'
    texts["shareNewPost"] = "{path} (new post)"
    texts["shareFileTooLarge"] = "{limit} is less than {name}"
    texts["shareTooManyFiles"] = "At most {count} files"
    texts["shareRefused"] = "Not sent: {reason}"
    texts["shareReasonStatus"] = "error {status}"
    texts["shareMoreFiles"] = (
        f"{{count, plural, =1{{{prefix} 1 more file}} other{{{prefix} {{count}} more files}}}}"
    )
    return texts


def _repo(tmp_path: Path, en: dict[str, str] | None = None) -> Path:
    arb_dir = tmp_path / gen.ARB_DIR
    arb_dir.mkdir(parents=True)
    (arb_dir / "app_en.arb").write_text(
        json.dumps(_arb("en", en or _texts("EN"), template=True)), encoding="utf-8"
    )
    (arb_dir / "app_de.arb").write_text(
        json.dumps(_arb("de", _texts("DE"), template=False)), encoding="utf-8"
    )
    return tmp_path


def test_render_maps_every_native_key_with_positional_arguments(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    assert gen.main(["--root", str(root)]) == 0
    en = (root / gen.OUTPUT_DIR / "en.lproj/Localizable.strings").read_text(encoding="utf-8")
    for native in gen.NATIVE_KEYS:
        assert f'"{native}" = ' in en
    assert '"PUSH_PREVIEW_TITLE" = "%1$@ · %2$@";' in en
    assert '"PUSH_PREVIEW_TEXT" = "%1$@: \\"%2$@\\" 100%%";' in en
    assert '"PUSH_TITLE" = "EN pushTitle";' in en
    de = (root / gen.OUTPUT_DIR / "de.lproj/Localizable.strings").read_text(encoding="utf-8")
    assert '"PUSH_TITLE" = "DE pushTitle";' in de


def test_check_fails_when_stale_and_passes_after_generation(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    assert gen.main(["--root", str(root), "--check"]) == 1
    assert gen.main(["--root", str(root)]) == 0
    assert gen.main(["--root", str(root), "--check"]) == 0
    target = root / gen.OUTPUT_DIR / "de.lproj/Localizable.strings"
    target.write_text(target.read_text(encoding="utf-8") + "\n", encoding="utf-8")
    assert gen.main(["--root", str(root), "--check"]) == 1


def test_missing_key_or_unknown_placeholder_fails(tmp_path: Path) -> None:
    texts = _texts("EN")
    del texts["pushTitle"]
    assert gen.main(["--root", str(_repo(tmp_path / "a", texts))]) == 1
    texts = _texts("EN")
    texts["notificationTitle"] = "{name} · {other}"
    assert gen.main(["--root", str(_repo(tmp_path / "b", texts))]) == 1


def test_repository_files_are_current() -> None:
    assert gen.main(["--root", str(REPO), "--check"]) == 0


def test_android_output_has_every_key_with_positional_arguments(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    assert gen.main(["--root", str(root)]) == 0
    en = (root / gen.ANDROID_DIR / "values/strings.xml").read_text(encoding="utf-8")
    for native in gen.ANDROID_KEYS:
        assert f'<string name="{native.lower()}">' in en
    assert len(gen.ANDROID_KEYS) == 12
    assert "share_send" not in en
    assert '<string name="push_preview_title">%1$s · %2$s</string>' in en
    # The format strings escape a literal percent; quotes and apostrophes are escaped for aapt.
    assert '<string name="push_preview_text">%1$s: \\"%2$s\\" 100%%</string>' in en
    assert '<string name="push_title">EN pushTitle</string>' in en
    de = (root / gen.ANDROID_DIR / "values-de/strings.xml").read_text(encoding="utf-8")
    assert '<string name="push_title">DE pushTitle</string>' in de


def test_android_escaping_of_xml_and_resource_syntax(tmp_path: Path) -> None:
    texts = _texts("EN")
    texts["pushTitle"] = "@Alex's <b> & 50%"
    root = _repo(tmp_path, texts)
    assert gen.main(["--root", str(root)]) == 0
    en = (root / gen.ANDROID_DIR / "values/strings.xml").read_text(encoding="utf-8")
    # No placeholders: the percent stays as it is, because nothing formats this string.
    assert '<string name="push_title">\\@Alex\\\'s &lt;b&gt; &amp; 50%</string>' in en
    target = root / gen.ANDROID_DIR / "values-de/strings.xml"
    target.write_text("stale", encoding="utf-8")
    assert gen.main(["--root", str(root), "--check"]) == 1


def test_share_keys_follow_the_push_keys_with_positional_arguments(tmp_path: Path) -> None:
    natives = list(gen.NATIVE_KEYS)
    assert natives[:12] == [
        "PUSH_TITLE",
        "PUSH_BODY_MESSAGE",
        "PUSH_BODY_MENTION",
        "PUSH_BODY_DM",
        "PUSH_BODY_RUN_DONE",
        "PUSH_BODY_RUN_FAILED",
        "PUSH_BODY_INPUT_NEEDED",
        "PUSH_BODY_SECURITY",
        "PUSH_BODY_ADMIN",
        "PUSH_MESSAGE_NOT_AVAILABLE",
        "PUSH_PREVIEW_TITLE",
        "PUSH_PREVIEW_TEXT",
    ]
    assert all(native.startswith("SHARE_") for native in natives[12:])
    assert len(natives) == 34
    root = _repo(tmp_path)
    assert gen.main(["--root", str(root)]) == 0
    en = (root / gen.OUTPUT_DIR / "en.lproj/Localizable.strings").read_text(encoding="utf-8")
    for native, key in gen.NATIVE_KEYS.items():
        assert f'"{native}" = ' in en, native
        if key not in _PLACEHOLDERS:
            assert f'"{native}" = "EN {key}";' in en
    assert '"SHARE_NEW_POST" = "%1$@ (new post)";' in en
    assert '"SHARE_FILE_TOO_LARGE" = "%2$@ is less than %1$@";' in en
    assert '"SHARE_TOO_MANY_FILES" = "At most %1$@ files";' in en
    assert '"SHARE_REFUSED" = "Not sent: %1$@";' in en
    assert '"SHARE_REASON_STATUS" = "error %1$@";' in en
    assert '"SHARE_UNAVAILABLE" = "EN shareUnavailable";' in en


def test_plural_becomes_one_and_other_lines_in_each_language(tmp_path: Path) -> None:
    root = _repo(tmp_path)
    assert gen.main(["--root", str(root)]) == 0
    for lang, prefix in (("en", "EN"), ("de", "DE")):
        text = (root / gen.OUTPUT_DIR / f"{lang}.lproj/Localizable.strings").read_text(
            encoding="utf-8"
        )
        assert f'"SHARE_MORE_FILES_ONE" = "{prefix} 1 more file";' in text
        assert f'"SHARE_MORE_FILES_OTHER" = "{prefix} %1$@ more files";' in text
        assert '"SHARE_MORE_FILES" =' not in text


def test_plural_branches_nest_placeholders_and_fall_back_to_one() -> None:
    one, other = gen.plural_branches(
        "{count, plural, one{{count} file {x}} =0{none} other{{count} files}}"
    )
    assert (one, other) == ("{count} file {x}", "{count} files")
    one, _ = gen.plural_branches("{n,plural,one{a}=1{exact}other{b}}")
    assert one == "exact"


def test_plural_without_other_or_one_or_not_a_plural_fails(tmp_path: Path) -> None:
    bad = {
        "no_other": "{count, plural, =1{one file}}",
        "no_one": "{count, plural, =0{none} other{{count} files}}",
        "not_plural": "{count} files",
        "trailing": "{count, plural, =1{a} other{b}} more",
        "unbalanced": "{count, plural, =1{a} other{b}",
        "duplicate": "{count, plural, =1{a} =1{b} other{c}}",
    }
    for name, text in bad.items():
        texts = _texts("EN")
        texts["shareMoreFiles"] = text
        assert gen.main(["--root", str(_repo(tmp_path / name, texts))]) == 1, name
