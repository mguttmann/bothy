# SPDX-License-Identifier: Apache-2.0
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Design-token pipeline. One source (design/tokens, DTCG 2025.10), several outputs.

Usage (from the repository root):
  uv run tools/design-tokens/tokens.py build      # gates, then write all generated files
  uv run tools/design-tokens/tokens.py check      # gates, then fail if a generated file is stale
  uv run tools/design-tokens/tokens.py previews   # render PNG previews with Playwright (needs npx)
  uv run tools/design-tokens/tokens.py vendor --phosphor DIR --sans DIR --mono DIR
"""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

from tokenkit import accents, dtcg, emit_css, emit_dart, emit_docs, emit_preview, model, rules  # noqa: E402

ROOT = HERE.parents[1]
RESOLVER = ROOT / "design" / "tokens" / "theme.resolver.json"
RULES = ROOT / "design" / "design-rules.json"
BUILD = ROOT / "design" / "build"
DOCS = ROOT / "docs" / "design"
VENDOR = HERE / "vendor"
HEADER = (
    "GENERIERT von tools/design-tokens/tokens.py aus design/tokens. Nicht bearbeiten; Quelle ändern und neu erzeugen."
)
PLAYWRIGHT = "playwright@1.56.1"

PREVIEWS = [
    # (view, theme, viewport, file stem)
    ("palette", "light", "1280,1180", "palette-light"),
    ("palette", "dark", "1280,1180", "palette-dark"),
    ("layout", "light", "1440,1000", "layout-light"),
    ("layout", "dark", "1440,1000", "layout-dark"),
    ("components", "light", "1280,1840", "components-light"),
    ("components", "dark", "1280,1840", "components-dark"),
    ("mobile", "light", "820,900", "mobile-light"),
    ("mobile", "dark", "820,900", "mobile-dark"),
]


def _accents_json(variants: dict, default: str) -> dict:
    """Member `accents` of tokens.resolved.json: accent -> theme -> role -> hex, alias, moved_from, swapped."""
    out: dict = {}
    for (theme, accent), roles in variants.items():
        out.setdefault(accent, {})[theme] = {
            role: {"hex": v.hex, "alias": v.alias, "moved_from": v.moved_from, "swapped": v.swapped}
            for role, v in roles.items()
        }
    return {"default": default, "variants": out}


def _resolved_json(m: model.Model, variants: dict | None = None, default: str = "") -> str:
    """The file tokens.resolved.json; `variants` and `default` add the member `accents`."""

    def value(token: dtcg.Token):
        if token.type == "color":
            alpha = dtcg.color_alpha(token.value)
            return token.hex if alpha >= 1 else {"hex": token.hex, "alpha": alpha}
        return token.value

    data = {
        "$comment": HEADER,
        "defaults": m.defaults,
        "base": {p: {"type": t.type, "value": value(t)} for p, t in sorted(m.base.items())},
        "contexts": {
            name: {
                ctx: {p: {"type": t.type, "value": value(t)} for p, t in sorted(tokens.items())}
                for ctx, tokens in ctxs.items()
            }
            for name, ctxs in m.contexts.items()
        },
        "icons": emit_dart.icon_extension(m),
    }
    if variants:
        data["accents"] = _accents_json(variants, default)
    return json.dumps(data, indent=2, ensure_ascii=False) + "\n"


def _build_readme() -> str:
    return (
        "# design/build\n\n"
        f"{HEADER}\n\n"
        "| Datei | Inhalt | Verbraucher |\n"
        "|---|---|---|\n"
        "| `tokens.css` | CSS-Variablen (`--ds-*`), Theme, Dichte und Bewegung per Attribut oder Systemeinstellung "
        "| Doku-Site, Vorschau, Statusseite |\n"
        "| `dart/design_tokens.g.dart` | Konstanten und `ThemeExtension`-Klassen | `packages/design_system` |\n"
        "| `tokens.resolved.json` | alle Tokens aufgelöst, je Kontext "
        "| Server-Prüfung erlaubter Farb- und Icon-Schlüssel, Design-Werkzeuge |\n"
        "| `preview.html` | eigenständige Vorschau (Schriften und Icons eingebettet) "
        "| Menschen, Playwright für `docs/design/previews/` |\n\n"
        "Erzeugen: `uv run tools/design-tokens/tokens.py build`. Prüfen: `make tokens`.\n"
    )


def generate() -> tuple[dict[Path, str], list[str]]:
    m = model.build(RESOLVER)
    rule_data = json.loads(RULES.read_text(encoding="utf-8"))
    themes = m.full["theme"]
    rows = rules.check_contrast(themes, rule_data["contrast_pairs"])
    default = rule_data.get("accent_variants", {}).get("default", "")
    try:
        variants = accents.derive(themes, rule_data)
        accent_problems: list[str] = []
    except accents.AccentContrastError as exc:
        variants, accent_problems = {}, [str(exc)]
    accent_rows = rules.check_accent_contrast(themes, rule_data["contrast_pairs"], variants, default)
    guard_problems: list[str] = []
    for ctx, tokens in themes.items():
        guard_problems += [f"[{ctx}] {p}" for p in rules.check_guards(tokens, rule_data["palette_guards"])]
    problems = [
        f"[{r.theme}] {r.fg} on {r.bg}: {r.ratio:.2f} < {r.required} ({r.fg_hex} on {r.bg_hex})"
        for r in rows
        if not r.passed
    ]
    problems += [
        f"[{r.theme}, accent {r.accent}] {r.fg} on {r.bg}: {r.ratio:.2f} < {r.required} ({r.fg_hex} on {r.bg_hex})"
        for r in accent_rows
        if not r.passed
    ]
    problems += accent_problems + guard_problems
    outputs = {
        BUILD / "README.md": _build_readme(),
        BUILD / "tokens.css": emit_css.render(m, HEADER, variants, default),
        BUILD / "dart" / "design_tokens.g.dart": emit_dart.render(m, HEADER, variants, default),
        BUILD / "tokens.resolved.json": _resolved_json(m, variants, default),
        BUILD / "preview.html": emit_preview.render(m, VENDOR, HEADER),
        DOCS / "palette.md": emit_docs.palette_md(m, HEADER),
        DOCS / "contrast.md": emit_docs.contrast_md(rows, guard_problems, HEADER, accent_rows, variants, default),
        DOCS / "tokens.md": emit_docs.tokens_md(m, HEADER),
    }
    return outputs, problems


def cmd_build(_args) -> int:
    outputs, problems = generate()
    if problems:
        print("Design gates failed:", file=sys.stderr)
        for p in problems:
            print("  " + p, file=sys.stderr)
        return 1
    for path, text in outputs.items():
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        print(f"wrote {path.relative_to(ROOT)}")
    return 0


def cmd_check(_args) -> int:
    outputs, problems = generate()
    stale = [p for p, text in outputs.items() if not p.exists() or p.read_text(encoding="utf-8") != text]
    vendor = vendor_problems(VENDOR)
    for p in problems:
        print("gate: " + p, file=sys.stderr)
    for p in stale:
        print(f"stale: {p.relative_to(ROOT)} (run: uv run tools/design-tokens/tokens.py build)", file=sys.stderr)
    for p in vendor:
        print("vendor: " + p, file=sys.stderr)
    if problems or stale or vendor:
        return 1
    print(f"ok: {len(outputs)} generated files up to date, all design gates pass, vendor/ matches MANIFEST.json")
    return 0


def cmd_previews(args) -> int:
    html = BUILD / "preview.html"
    if not html.exists():
        print("design/build/preview.html missing; run build first", file=sys.stderr)
        return 1
    out_dir = DOCS / "previews"
    out_dir.mkdir(parents=True, exist_ok=True)
    npx = shutil.which("npx")
    if npx is None:
        print("npx not found; install Node.js", file=sys.stderr)
        return 1
    for view, theme, viewport, stem in PREVIEWS:
        url = f"{html.as_uri()}?view={view}&theme={theme}"
        target = out_dir / f"{stem}.png"
        cmd = [
            npx,
            "-y",
            args.playwright,
            "screenshot",
            "--browser",
            "chromium",
            "--color-scheme",
            theme,
            "--viewport-size",
            viewport,
            "--wait-for-timeout",
            "400",
            url,
            str(target),
        ]
        print(" ".join(cmd[2:]))
        subprocess.run(cmd, check=True)
    return 0


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def as_stored(data: bytes) -> bytes:
    """Bytes as the repository stores them: text with LF line endings (.gitattributes), binary as is.

    A file without NUL byte counts as text, as in git's `text=auto`. Hashing the stored form
    keeps MANIFEST.json valid in every checkout, whatever line endings the upstream package used.
    """
    return data if b"\0" in data else data.replace(b"\r\n", b"\n")


def _copy_as_stored(src: Path, dst: Path) -> None:
    dst.write_bytes(as_stored(src.read_bytes()))


def vendor_problems(vendor: Path) -> list[str]:
    """Every file under fonts/ and phosphor/ is listed in MANIFEST.json with its sha256, and nothing else."""
    recorded: dict[str, str] = json.loads((vendor / "MANIFEST.json").read_text(encoding="utf-8"))["sha256"]
    present = {
        f.relative_to(vendor).as_posix(): f
        for sub in ("fonts", "phosphor")
        if (vendor / sub).is_dir()
        for f in (vendor / sub).iterdir()
        if f.is_file()
    }
    problems = [f"{name}: not listed in MANIFEST.json" for name in sorted(present.keys() - recorded.keys())]
    problems += [f"{name}: listed in MANIFEST.json but missing" for name in sorted(recorded.keys() - present.keys())]
    problems += [
        f"{name}: sha256 differs from MANIFEST.json (re-vendor with `tokens.py vendor`)"
        for name in sorted(present.keys() & recorded.keys())
        if _sha256(present[name]) != recorded[name]
    ]
    return problems


def cmd_vendor(args) -> int:
    """Copy the font files and the used Phosphor SVGs into vendor/ and write MANIFEST.json.

    Text files are stored with LF line endings (`as_stored`); MANIFEST.json records the sha256 of
    the stored bytes, and `check` verifies it.
    """
    m = model.build(RESOLVER)
    icons = emit_dart.icon_extension(m)
    phosphor = Path(args.phosphor)
    fonts_dir, icons_dir = VENDOR / "fonts", VENDOR / "phosphor"
    files: dict[str, str] = {}
    with tempfile.TemporaryDirectory() as tmp:
        staging = Path(tmp)
        (staging / "fonts").mkdir()
        (staging / "phosphor").mkdir()
        for src_dir, family in (
            (Path(args.sans), "atkinson-hyperlegible-next"),
            (Path(args.mono), "atkinson-hyperlegible-mono"),
        ):
            for name in (f"{family}-latin-wght-normal.woff2", f"{family}-latin-ext-wght-normal.woff2"):
                _copy_as_stored(src_dir / "files" / name, staging / "fonts" / name)
            _copy_as_stored(src_dir / "LICENSE", staging / "fonts" / f"{family}.OFL.txt")
        needed = {(g["name"], g["weight"]) for g in icons["glyphs"].values()} | {
            (n, "regular") for n in icons["space-icons"]
        }
        for name, weight in sorted(needed):
            suffix = "" if weight == "regular" else f"-{weight}"
            _copy_as_stored(
                phosphor / "assets" / weight / f"{name}{suffix}.svg", staging / "phosphor" / f"{name}{suffix}.svg"
            )
        _copy_as_stored(phosphor / "LICENSE", staging / "phosphor" / "LICENSE")
        for sub in ("fonts", "phosphor"):
            target = VENDOR / sub
            if target.exists():
                shutil.rmtree(target)
            shutil.copytree(staging / sub, target)
    for sub_dir in (fonts_dir, icons_dir):
        for f in sorted(sub_dir.iterdir()):
            files[f.relative_to(VENDOR).as_posix()] = _sha256(f)
    manifest = {
        "$comment": (
            "Herkunft der eingebetteten Dateien. Nur für Vorschau und Doku; "
            "die Apps beziehen Schriften und Icons im Paket-Build aus denselben Quellen. "
            "sha256 der gespeicherten Bytes (Textdateien mit LF wie in .gitattributes); "
            "tokens.py check prüft sie."
        ),
        "sources": {
            "fonts/atkinson-hyperlegible-next-*": (
                f"npm @fontsource-variable/atkinson-hyperlegible-next@{args.sans_version} (OFL-1.1)"
            ),
            "fonts/atkinson-hyperlegible-mono-*": (
                f"npm @fontsource-variable/atkinson-hyperlegible-mono@{args.mono_version} (OFL-1.1)"
            ),
            "phosphor/*": f"npm @phosphor-icons/core@{icons['version']} (MIT)",
        },
        "sha256": files,
    }
    (VENDOR / "MANIFEST.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    print(f"vendored {len(files)} files")
    return 0


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("build").set_defaults(func=cmd_build)
    sub.add_parser("check").set_defaults(func=cmd_check)
    p = sub.add_parser("previews")
    p.add_argument("--playwright", default=PLAYWRIGHT, help="npx package spec of the Playwright CLI")
    p.set_defaults(func=cmd_previews)
    v = sub.add_parser("vendor")
    v.add_argument("--phosphor", required=True, help="unpacked @phosphor-icons/core package")
    v.add_argument("--sans", required=True, help="unpacked @fontsource-variable/atkinson-hyperlegible-next")
    v.add_argument("--mono", required=True, help="unpacked @fontsource-variable/atkinson-hyperlegible-mono")
    v.add_argument("--sans-version", default="5.3.0")
    v.add_argument("--mono-version", default="5.3.0")
    v.set_defaults(func=cmd_vendor)
    args = parser.parse_args(argv)
    try:
        return args.func(args)
    except (dtcg.TokenError, rules.RuleError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
