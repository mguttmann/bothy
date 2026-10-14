# SPDX-License-Identifier: Apache-2.0
"""Dart constants and ThemeExtension classes for packages/design_system (Flutter).

Theme-dependent tokens become ThemeExtension subclasses with one const instance per context
(DsColors.light/.dark, DsDensity.comfortable/.compact/.touch, DsMotion.full/.reduced,
DsElevation.light/.dark). Theme-independent tokens become static constants. The accent variants
of `accents.derive` become `DsAccent`, one const instance per theme and accent with a lookup
`DsAccent.of(key, brightness)`.
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
    name: str,
    fields: list[tuple[str, str]],
    instances: dict[str, dict[str, str]],
    lerp_kind: dict[str, str],
    doc: str,
    extra: list[str] | None = None,
) -> str:
    lines = [f"/// {line}".rstrip() for line in doc.split("\n")]
    lines += ["@immutable", f"final class {name} extends ThemeExtension<{name}> {{"]
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
    if extra:
        lines.append("")
        lines += extra
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


def accent_constant(theme: str, accent: str) -> str:
    """Name of the DsAccent constant for a theme and accent: `lightLoch`, `darkSpruce`."""
    return identifier(f"{theme}-{accent}")


def accent_extension(variants: dict[tuple[str, str], dict], default: str) -> str:
    """The `DsAccent` theme extension for the accent variants of `accents.derive`.

    Args:
        variants: (theme, accent) -> role -> Variant; themes must be `light` and `dark`.
        default: the default accent; `DsAccent.of` falls back to it for an unknown key.

    Returns:
        Dart source of the class.

    Raises:
        ValueError: the themes are not exactly `light` and `dark`.
    """
    themes = list(dict.fromkeys(theme for theme, _ in variants))
    if sorted(themes) != ["dark", "light"]:
        raise ValueError(f"DsAccent needs the themes light and dark, got {themes}")
    accent_keys = list(dict.fromkeys(accent for _, accent in variants))
    roles = list(next(iter(variants.values())))
    fields = [(identifier(role, "color"), "Color") for role in roles]
    instances = {
        accent_constant(theme, accent): {
            identifier(role, "color"): color_literal(v.hex, 1.0) for role, v in variants[(theme, accent)].items()
        }
        for accent in accent_keys
        for theme in themes
    }
    of = [
        "  /// Accent roles for the instance accent [key] (`branding.accent_color`) and [brightness].",
        "  ///",
        f"  /// An unknown key gives the default accent `{default}`, so an instance setting written",
        "  /// by a newer server never leaves the app without accent colours.",
        "  static DsAccent of(String key, Brightness brightness) {",
        "    final dark = brightness == Brightness.dark;",
        "    return switch (key) {",
    ]
    for accent in accent_keys:
        if accent != default:
            of.append(
                f"      '{accent}' => dark ? {accent_constant('dark', accent)} : {accent_constant('light', accent)},"
            )
    of += [
        f"      _ => dark ? {accent_constant('dark', default)} : {accent_constant('light', default)},",
        "    };",
        "  }",
    ]
    doc = (
        "Accent roles per theme and instance accent (`branding.accent_color`, ADR 0044).\n"
        "\n"
        "Roles: " + ", ".join(f"`{role}`" for role in roles) + ". Components read these roles only\n"
        "from DsAccent, never from DsColors, so the instance accent applies everywhere. Every\n"
        "constant passed the contrast gate (docs/design/contrast.md); roles that do not follow\n"
        "the accent in a theme carry their base value."
    )
    return _extension_class("DsAccent", fields, instances, {f: "color" for f, _ in fields}, doc, of)


def render(
    model: Model, header: str, variants: dict[tuple[str, str], dict] | None = None, default: str | None = None
) -> str:
    """The Dart file `design_tokens.g.dart`; `variants` and `default` add the class DsAccent."""
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
            "DsPalette", "Primitive tonal scales. Components use DsColors, never these values directly.", palette
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
                "DsColors", fields, instances, {f: "color" for f, _ in fields}, "Semantic color roles per theme."
            )
        )
        if variants:
            parts.append("")
            parts.append(accent_extension(variants, default or ""))
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
                    "Shadows for layers above the content, per theme.",
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
                "Density: comfortable (desktop), compact (selectable), touch (mobile).",
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
                "Motion: full or reduced (system setting or appearance.reduce_motion).",
            )
        )

    for group, cls, doc in (
        ("space", "DsSpace", "Spacing (4 px grid)."),
        ("radius", "DsRadius", "Corner radii."),
        ("border-width", "DsBorderWidth", "Line widths."),
        ("layout", "DsLayout", "Initial widths of the desktop columns."),
        ("size", "DsSize", "Fixed sizes."),
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
        parts.append(_static_class("DsIconSize", "Icon sizes.", icon_sizes))

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
    parts.append(_static_class("DsFonts", "Font families with a fallback list.", families))
    parts.append("")
    parts.append(
        _static_class("DsType", "Type roles. Color comes from DsColors, size scales with MediaQuery.textScaler.", types)
    )

    icons = icon_extension(model)
    if icons:
        parts.append("")
        parts.append("enum DsIconWeight { thin, light, regular, bold, fill, duotone }")
        parts.append("")
        parts.append("/// Semantic icon: name in the Phosphor set and stroke weight.")
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
            _static_class("DsIcons", f"Icons from {icons['source']} {icons['version']} ({icons['license']}).", members)
        )

    return "\n".join(parts) + "\n"


def _sort_key(path: str):
    tail = path.split(".")[-1]
    return (0, int(tail), "") if tail.isdigit() else (1, 0, tail)
