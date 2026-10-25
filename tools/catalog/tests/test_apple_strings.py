# SPDX-License-Identifier: Apache-2.0
"""Tests for tools/catalog/apple_strings.py, the native strings of the push placeholders."""

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


def _arb(locale: str, texts: dict[str, str], *, template: bool) -> dict[str, object]:
    arb: dict[str, object] = {"@@locale": locale}
    for key, text in texts.items():
        arb[key] = text
        if template and "{" in text:
            arb[f"@{key}"] = {
                "placeholders": {name: {"type": "String"} for name in ("name", "title")}
                if key == "notificationTitle"
                else {name: {"type": "String"} for name in ("author", "excerpt")}
            }
    return arb


def _texts(prefix: str) -> dict[str, str]:
    texts = {arb: f"{prefix} {arb}" for arb in gen.NATIVE_KEYS.values()}
    texts["notificationTitle"] = "{name} · {title}"
    texts["notificationText"] = '{author}: "{excerpt}" 100%'
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
    for native in gen.NATIVE_KEYS:
        assert f'<string name="{native.lower()}">' in en
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
