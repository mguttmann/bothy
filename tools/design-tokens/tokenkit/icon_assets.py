# SPDX-License-Identifier: Apache-2.0
"""Icon assets of packages/design_system: compiled vectors, their lookup file, LICENSE and MANIFEST.json.

Source: the mapping `ds.icons` of `icon.tokens.json` (resolved model) and the vendored SVGs of
`vendor/phosphor/`. Every mapped glyph is built in its mapped weight and in `fill` (the active
form, docs/design/icons.md "Rules"); every space icon in `regular`. Each pair becomes
`assets/icons/<weight>/<glyph>.vec`, compiled by `vector_graphics_compiler` from the workspace
lock (docs/DECISIONS.md D-M4-05-1 and D-M4-05-2). The compiler is passed in as a function, so the
build logic is testable without Dart and `tokens.py` decides where the compiler runs.
"""

from __future__ import annotations

import hashlib
import json
import re
import shutil
import subprocess  # nosec B404 - runs only the pinned dart executable with fixed arguments
import tempfile
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from .emit_dart import identifier

ASSET_DIR = "assets/icons"
LICENSE_FILE = f"{ASSET_DIR}/LICENSE"
MANIFEST_FILE = f"{ASSET_DIR}/MANIFEST.json"
LOOKUP_FILE = "lib/src/generated/ds_icon_assets.g.dart"
PACKAGE_NAME = "design_system"
FILL = "fill"
COMPILER = "vector_graphics_compiler"
# The optimizers need the native path-ops library of the Flutter engine cache; Phosphor glyphs are
# single paths without masks or clips, so turning them off costs nothing and keeps the output
# independent of the host's native libraries.
COMPILER_FLAGS = ("--no-optimize-masks", "--no-optimize-clips", "--no-optimize-overdraw", "--concurrency", "1")

# Maps `<weight>/<glyph>` to the SVG bytes and returns `<weight>/<glyph>` to the compiled bytes.
Compiler = Callable[[dict[str, bytes]], dict[str, bytes]]


class IconError(ValueError):
    """A mapped glyph cannot be built (its SVG is missing, or the compiler failed)."""


@dataclass(frozen=True)
class Entry:
    """One asset: weight, Phosphor name and the semantic names that need it."""

    weight: str
    name: str
    users: tuple[str, ...]

    @property
    def key(self) -> str:
        """`<weight>/<glyph>`, the sort key and the lookup key of the Dart file."""
        return f"{self.weight}/{self.name}"

    @property
    def asset(self) -> str:
        """Asset path relative to the package: `assets/icons/<weight>/<glyph>.vec`."""
        return f"{ASSET_DIR}/{self.weight}/{self.name}.vec"

    @property
    def svg_file(self) -> str:
        """File name in vendor/phosphor/."""
        return svg_file(self.name, self.weight)


def svg_file(name: str, weight: str) -> str:
    """File name of a Phosphor SVG in vendor/phosphor/: `<glyph>.svg` for regular, `<glyph>-<weight>.svg` otherwise."""
    return f"{name}.svg" if weight == "regular" else f"{name}-{weight}.svg"


def entries(icons: dict) -> list[Entry]:
    """Every (weight, glyph) pair the assets hold, sorted by `<weight>/<glyph>`.

    Args:
        icons: the `ds.icons` extension (`emit_dart.icon_extension`).

    Returns:
        One entry per pair with the semantic names (or `space-icons:<glyph>`) that need it.
    """
    users: dict[tuple[str, str], list[str]] = {}
    for semantic, g in sorted(icons.get("glyphs", {}).items()):
        for weight in dict.fromkeys((g["weight"], FILL)):
            users.setdefault((weight, g["name"]), []).append(semantic)
    for name in icons.get("space-icons", []):
        users.setdefault(("regular", name), []).append(f"space-icons:{name}")
    found = [Entry(w, n, tuple(u)) for (w, n), u in users.items()]
    return sorted(found, key=lambda e: e.key)


def needed_svgs(icons: dict) -> list[tuple[str, str]]:
    """The (glyph, weight) pairs whose SVGs vendor/phosphor/ must hold, sorted."""
    return sorted((e.name, e.weight) for e in entries(icons))


def _sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def compiler_version(lock: Path) -> str:
    """The locked version of `vector_graphics_compiler` in a pubspec.lock, or "" when it is absent."""
    if not lock.is_file():
        return ""
    match = re.search(rf'\n  {COMPILER}:\n(?:    .*\n)*?    version: "([^"]+)"', lock.read_text(encoding="utf-8"))
    return match.group(1) if match else ""


def vector_compiler(package_dir: Path) -> Compiler:
    """A compiler that runs `dart run vector_graphics_compiler` in `package_dir` (one call for all SVGs).

    Args:
        package_dir: a member of the Dart workspace that has the compiler as a dependency.

    Returns:
        The compiler function; it raises IconError when dart is missing or the compiler fails.
    """

    def run(svgs: dict[str, bytes]) -> dict[str, bytes]:
        dart = shutil.which("dart")
        if dart is None:
            raise IconError("dart not found on PATH; the icon build needs the pinned Flutter SDK (R-QA-02)")
        with tempfile.TemporaryDirectory(prefix="ds-icons-") as tmp:
            src = Path(tmp)
            for key, data in svgs.items():
                path = src / f"{key}.svg"
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(data)
            proc = subprocess.run(  # nosec B603 - fixed argument list, no shell
                [dart, "run", COMPILER, "--input-dir", str(src), *COMPILER_FLAGS],
                cwd=package_dir,
                capture_output=True,
                text=True,
                check=False,
            )
            if proc.returncode != 0:
                raise IconError(f"{COMPILER} failed ({proc.returncode}): {proc.stdout[-2000:]}{proc.stderr[-2000:]}")
            out: dict[str, bytes] = {}
            for key in svgs:
                compiled = src / f"{key}.svg.vec"
                if compiled.is_file():
                    out[key] = compiled.read_bytes()
            return out

    return run


def build(icons: dict, phosphor: Path, header: str, compile_svgs: Compiler, version: str) -> dict[str, bytes]:
    """All icon assets, relative to packages/design_system, as bytes.

    Args:
        icons: the `ds.icons` extension of the resolved model.
        phosphor: vendor/phosphor/ with the SVGs and LICENSE.
        header: the "generated, do not edit" line for the Dart file and the manifest.
        compile_svgs: turns SVG bytes into compiled vector bytes (`vector_compiler`).
        version: the compiler version recorded in MANIFEST.json.

    Returns:
        Every `.vec` asset, LOOKUP_FILE, LICENSE_FILE and MANIFEST_FILE mapped to their bytes.

    Raises:
        IconError: an SVG or the LICENSE is missing (the message names the semantic name, the
            glyph and the weight), or the compiler gave no output for an SVG.
    """
    items = entries(icons)
    svgs: dict[str, bytes] = {}
    for entry in items:
        path = phosphor / entry.svg_file
        if not path.is_file():
            raise IconError(
                f"{', '.join(entry.users)}: glyph {entry.name!r} in weight {entry.weight!r} has no SVG "
                f"vendor/phosphor/{entry.svg_file} (run tokens.py vendor)"
            )
        svgs[entry.key] = path.read_bytes()
    license_path = phosphor / "LICENSE"
    if not license_path.is_file():
        raise IconError("vendor/phosphor/LICENSE is missing (run tokens.py vendor)")
    compiled = compile_svgs(svgs)
    missing = [key for key in svgs if not compiled.get(key)]
    if missing:
        raise IconError(f"{COMPILER} gave no output for {', '.join(missing)}")
    license_text = license_path.read_bytes()
    out: dict[str, bytes] = {entry.asset: compiled[entry.key] for entry in items}
    manifest = {
        "$comment": header,
        "package": f"{icons.get('source', '@phosphor-icons/core')}@{icons.get('version', '')}",
        "license": icons.get("license", "MIT"),
        "license_file": "LICENSE",
        "license_sha256": _sha256(license_text),
        "compiler": f"{COMPILER}@{version}",
        "total_bytes": sum(len(compiled[e.key]) for e in items),
        "glyphs": {
            entry.key: {
                "asset": entry.asset,
                "asset_sha256": _sha256(compiled[entry.key]),
                "source": f"tools/design-tokens/vendor/phosphor/{entry.svg_file}",
                "source_sha256": _sha256(svgs[entry.key]),
                "used_by": list(entry.users),
            }
            for entry in items
        },
    }
    out[LOOKUP_FILE] = _lookup_dart(icons, items, header).encode("utf-8")
    out[LICENSE_FILE] = license_text
    out[MANIFEST_FILE] = (json.dumps(manifest, indent=2, ensure_ascii=False) + "\n").encode("utf-8")
    return out


def _lookup_dart(icons: dict, items: list[Entry], header: str) -> str:
    version = icons.get("version", "")
    lines = [
        f"// {header}",
        "// SPDX-License-Identifier: Apache-2.0",
        "",
        f"/// Asset lookup of the icons: Phosphor glyphs of @phosphor-icons/core {version} (MIT),",
        "/// compiled by vector_graphics_compiler into `assets/icons/<weight>/<glyph>.vec`.",
        "library;",
        "",
        "import 'design_tokens.g.dart';",
        "",
        "/// The package that bundles the icon assets (the `packageName` of their loader).",
        f"const String dsIconPackage = '{PACKAGE_NAME}';",
        "",
        "/// Asset paths of the compiled icons.",
        "abstract final class DsIconAssets {",
        "  /// The asset of [glyph] in its mapped weight, or in `fill` when [filled] is true; null",
        "  /// when the assets lack the pair.",
        "  static String? lookup(DsGlyph glyph, {bool filled = false}) =>",
        "      byKey['${filled ? DsIconWeight.fill.name : glyph.weight.name}/${glyph.name}'];",
        "",
        "  /// The asset of the space icon [name] (`DsIcons.spaceIcons`, weight `regular`).",
        "  static String? space(String name) => byKey['regular/$name'];",
        "",
        "  /// Every asset by `<weight>/<glyph>`, in sorted order.",
        "  static const Map<String, String> byKey = {",
    ]
    lines += [f"    '{e.key}': '{e.asset}'," for e in items]
    lines += [
        "  };",
        "}",
        "",
        "/// Every semantic icon of `ds.icons` by its token name, in sorted order.",
        "const Map<String, DsGlyph> dsSemanticIcons = {",
    ]
    lines += [f"  '{s}': DsIcons.{identifier(s.replace('.', '-'))}," for s in sorted(icons.get("glyphs", {}))]
    lines += ["};", ""]
    return "\n".join(lines)
