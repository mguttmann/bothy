# SPDX-License-Identifier: Apache-2.0
"""Dart constants and ThemeExtension classes for packages/design_system (Flutter).

Theme-dependent tokens become ThemeExtension subclasses with one const instance per context
(DsColors.light/.dark, DsDensity.comfortable/.compact/.touch, DsMotion.full/.reduced,
DsElevation.light/.dark). Theme-independent tokens become static constants.
"""

from __future__ import annotations

import re

from . import dtcg
from .model import Model

_WEIGHTS = {100, 200, 300, 400, 500, 600, 700, 800, 900}


def identifier(path: str, drop: str | None = None) -> str:
    """Return the lowerCamelCase Dart identifier of a token path, without the `drop` prefix."""
    parts = [p for p in path.split(".") if p != "$root"]
    if drop and parts and parts[0] == drop:
        parts = parts[1:]
    words: list[str] = []
    for part in parts:
        words += [w for w in re.split(r"[-_]", part) if w]
    if not words:
        raise ValueError(f"no identifier for {path}")
    ident = words[0].lower() + "".join(w[:1].upper() + w[1:] for w in words[1:])
    if ident[0].isdigit():
        ident = (drop[0] if drop else "v") + ident
    return ident


def color_literal(hex_value: str, alpha: float) -> str:
    """Return a Dart `Color(0xAARRGGBB)` literal."""
    a = round(alpha * 255)
    return f"Color(0x{a:02X}{hex_value.lstrip('#').upper()})"


def _double(value: float) -> str:
    text = f"{float(value):.4f}".rstrip("0")
    return text + "0" if text.endswith(".") else text


def _px(dim: dict) -> str:
    if dim["unit"] == "rem":
        return _double(dim["value"] * 16)
    return _double(dim["value"])


def _ms(dur: dict) -> int:
    return int(round(dur["value"] * (1000 if dur["unit"] == "s" else 1)))


def _font_weight(value) -> str:
    num = dtcg.font_weight_number(value)
    nearest = min(_WEIGHTS, key=lambda w: abs(w - num))
    return f"FontWeight.w{nearest}"


def _string_list(values) -> str:
    return "[" + ", ".join(f"'{v}'" for v in values) + "]"


def _color_expr(token: dtcg.Token) -> str:
    return color_literal(token.hex, dtcg.color_alpha(token.value))


def _shadow_expr(token: dtcg.Token) -> str:
    layers = token.value if isinstance(token.value, list) else [token.value]
    out = []
    for layer in layers:
        probe = dtcg.Token(path=token.path, value=layer["color"], type="color")
        dtcg._validate_color(probe)
        args = [
            f"color: {color_literal(probe.hex, dtcg.color_alpha(layer['color']))}",
            f"offset: Offset({_px(layer['offsetX'])}, {_px(layer['offsetY'])})",
            f"blurRadius: {_px(layer['blur'])}",
        ]
        if layer["spread"]["value"] != 0:
            args.append(f"spreadRadius: {_px(layer['spread'])}")
        out.append("BoxShadow(" + ", ".join(args) + ")")
    return "[\n      " + ",\n      ".join(out) + ",\n    ]"


def _extension_class(
    name: str, fields: list[tuple[str, str]], instances: dict[str, dict[str, str]], lerp_kind: dict[str, str], doc: str
) -> str:
    lines = [f"/// {doc}", "@immutable", f"final class {name} extends ThemeExtension<{name}> {{"]
    lines.append(f"  const {name}({{")
    for field, _ in fields:
        lines.append(f"    required this.{field},")
    lines.append("  });")
    lines.append("")
    for field, dart_type in fields:
        lines.append(f"  final {dart_type} {field};")
    for inst, values in instances.items():
        lines.append("")
        lines.append(f"  static const {name} {inst} = {name}(")
        for field, _ in fields:
            lines.append(f"    {field}: {values[field]},")
        lines.append("  );")
    lines.append("")
    lines.append("  @override")
    lines.append(f"  {name} copyWith({{")
    for field, dart_type in fields:
        lines.append(f"    {dart_type}? {field},")
    lines.append("  }) {")
    lines.append(f"    return {name}(")
    for field, _ in fields:
        lines.append(f"      {field}: {field} ?? this.{field},")
    lines.append("    );")
    lines.append("  }")
    lines.append("")
    lines.append("  @override")
    lines.append(f"  {name} lerp(covariant ThemeExtension<{name}>? other, double t) {{")
    lines.append(f"    if (other is! {name}) {{")
    lines.append("      return this;")
    lines.append("    }")
    lines.append(f"    return {name}(")
    for field, _ in fields:
        kind = lerp_kind[field]
        if kind == "color":
            expr = f"Color.lerp({field}, other.{field}, t)!"
        elif kind == "double":
            expr = f"lerpDouble({field}, other.{field}, t)!"
        elif kind == "shadows":
            expr = f"BoxShadow.lerpList({field}, other.{field}, t)!"
        else:
            expr = f"t < 0.5 ? {field} : other.{field}"
        lines.append(f"      {field}: {expr},")
    lines.append("    );")
    lines.append("  }")
    lines.append("}")
    return "\n".join(lines)


def _static_class(name: str, doc: str, members: list[tuple[str, str, str]]) -> str:
    lines = [f"/// {doc}", f"abstract final class {name} {{"]
    for ident, dart_type, expr in members:
        lines.append(f"  static const {dart_type} {ident} = {expr};")
    lines.append("}")
    return "\n".join(lines)


def _typography_expr(value: dict) -> str:
    families = value["fontFamily"] if isinstance(value["fontFamily"], list) else [value["fontFamily"]]
    primary, fallback = families[0], families[1:]
    return (
        "TextStyle("
        f"fontFamily: '{primary}', "
        f"fontFamilyFallback: {_string_list(fallback)}, "
        f"fontSize: {_px(value['fontSize'])}, "
        f"fontWeight: {_font_weight(value['fontWeight'])}, "
        f"height: {_double(value['lineHeight'])}, "
        f"letterSpacing: {_px(value['letterSpacing'])})"
    )


def icon_extension(model: Model) -> dict:
    """Return the icon set extension (`ds.icons`) of the default token document."""
    merged = model.document.merged_document(model.defaults)
    return merged.get("icon", {}).get("$extensions", {}).get("ds.icons", {})


def render(model: Model, header: str) -> str:
    """Render the Dart token library with theme extensions for light and dark."""
    parts = [
        f"// {header}",
        "// ignore_for_file: public_member_api_docs, constant_identifier_names",
        "",
        "import 'dart:ui' show lerpDouble;",
        "",
        "import 'package:flutter/material.dart';",
        "",
    ]
    base = model.base

    palette = [
        (identifier(p, "palette"), "Color", _color_expr(t)) for p, t in sorted(base.items()) if p.startswith("palette.")
    ]
    parts.append(
        _static_class(
            "DsPalette", "Primitive Tonleitern. Komponenten nutzen DsColors, nie diese Werte direkt.", palette
        )
    )

    theme = model.contexts.get("theme", {})
    if theme:
        any_ctx = next(iter(theme.values()))
        color_paths = [p for p, t in any_ctx.items() if t.type == "color"]
        fields = [(identifier(p, "color"), "Color") for p in color_paths]
        instances = {
            ctx: {identifier(p, "color"): _color_expr(tokens[p]) for p in color_paths} for ctx, tokens in theme.items()
        }
        parts.append("")
        parts.append(
            _extension_class(
                "DsColors", fields, instances, {f: "color" for f, _ in fields}, "Semantische Farbrollen je Theme."
            )
        )
        shadow_paths = [p for p, t in any_ctx.items() if t.type == "shadow"]
        if shadow_paths:
            sfields = [(identifier(p, "elevation"), "List<BoxShadow>") for p in shadow_paths]
            sinst = {
                ctx: {identifier(p, "elevation"): _shadow_expr(tokens[p]) for p in shadow_paths}
                for ctx, tokens in theme.items()
            }
            parts.append("")
            parts.append(
                _extension_class(
                    "DsElevation",
                    sfields,
                    sinst,
                    {f: "shadows" for f, _ in sfields},
                    "Schatten für Ebenen über dem Inhalt, je Theme.",
                )
            )

    density = model.contexts.get("density", {})
    if density:
        any_ctx = next(iter(density.values()))
        paths = sorted(any_ctx)
        fields = [(identifier(p, "density"), "double") for p in paths]
        inst = {ctx: {identifier(p, "density"): _px(tokens[p].value) for p in paths} for ctx, tokens in density.items()}
        parts.append("")
        parts.append(
            _extension_class(
                "DsDensity",
                fields,
                inst,
                {f: "double" for f, _ in fields},
                "Dichte: comfortable (Desktop), compact (wählbar), touch (Mobil).",
            )
        )

    motion = model.contexts.get("motion", {})
    if motion:
        any_ctx = next(iter(motion.values()))
        paths = sorted(any_ctx)
        fields, lerp, inst = [], {}, {ctx: {} for ctx in motion}
        for p in paths:
            ident = identifier(p, "motion")
            kind = any_ctx[p].type
            dart_type = "Duration" if kind == "duration" else "Cubic"
            fields.append((ident, dart_type))
            lerp[ident] = "step"
            for ctx, tokens in motion.items():
                v = tokens[p].value
                inst[ctx][ident] = (
                    ("Duration.zero" if _ms(v) == 0 else f"Duration(milliseconds: {_ms(v)})")
                    if kind == "duration"
                    else "Cubic(" + ", ".join(_double(x) for x in v) + ")"
                )
        parts.append("")
        parts.append(
            _extension_class(
                "DsMotion",
                fields,
                inst,
                lerp,
                "Bewegung: full oder reduced (Systemeinstellung bzw. appearance.reduce_motion).",
            )
        )

    for group, cls, doc in (
        ("space", "DsSpace", "Abstände (Raster 4 px)."),
        ("radius", "DsRadius", "Eckenradien."),
        ("border-width", "DsBorderWidth", "Linienstärken."),
        ("layout", "DsLayout", "Startbreiten der Desktop-Spalten."),
        ("size", "DsSize", "Feste Maße."),
    ):
        members = [
            (identifier(p, group), "double", _px(t.value))
            for p, t in sorted(base.items(), key=lambda kv: _sort_key(kv[0]))
            if p.split(".")[0] == group
        ]
        if members:
            parts.append("")
            parts.append(_static_class(cls, doc, members))

    icon_sizes = [
        (identifier(p, "icon"), "double", _px(t.value)) for p, t in sorted(base.items()) if p.startswith("icon.size.")
    ]
    if icon_sizes:
        parts.append("")
        parts.append(_static_class("DsIconSize", "Icon-Größen.", icon_sizes))

    families = [
        (identifier(p, "font"), "List<String>", _string_list(t.value if isinstance(t.value, list) else [t.value]))
        for p, t in sorted(base.items())
        if p.startswith("font.family.")
    ]
    types = [
        (identifier(p, "type"), "TextStyle", _typography_expr(t.value))
        for p, t in sorted(base.items())
        if t.type == "typography"
    ]
    parts.append("")
    parts.append(_static_class("DsFonts", "Schriftfamilien mit Rückfall-Liste.", families))
    parts.append("")
    parts.append(
        _static_class(
            "DsType", "Typo-Rollen. Farbe kommt aus DsColors, Größe skaliert mit MediaQuery.textScaler.", types
        )
    )

    icons = icon_extension(model)
    if icons:
        parts.append("")
        parts.append("enum DsIconWeight { thin, light, regular, bold, fill, duotone }")
        parts.append("")
        parts.append("/// Semantisches Icon: Name im Phosphor-Set und Strichstärke.")
        parts.append("@immutable")
        parts.append("final class DsGlyph {")
        parts.append("  const DsGlyph(this.name, this.weight, {this.spins = false});")
        parts.append("  final String name;")
        parts.append("  final DsIconWeight weight;")
        parts.append("  final bool spins;")
        parts.append("}")
        members = []
        for key, glyph in sorted(icons["glyphs"].items()):
            spins = ", spins: true" if glyph.get("motion") == "spin" else ""
            members.append(
                (
                    identifier(key.replace(".", "-")),
                    "DsGlyph",
                    f"DsGlyph('{glyph['name']}', DsIconWeight.{glyph['weight']}{spins})",
                )
            )
        members.append(("spaceIcons", "List<String>", _string_list(icons["space-icons"])))
        parts.append("")
        parts.append(
            _static_class("DsIcons", f"Icons aus {icons['source']} {icons['version']} ({icons['license']}).", members)
        )

    return "\n".join(parts) + "\n"


def _sort_key(path: str):
    tail = path.split(".")[-1]
    return (0, int(tail), "") if tail.isdigit() else (1, 0, tail)
