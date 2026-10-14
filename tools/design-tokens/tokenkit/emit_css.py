# SPDX-License-Identifier: Apache-2.0
"""CSS custom properties for the docs site, the preview page and any HTML surface (status page).

Selection rules:
- Theme: `data-theme="light|dark"` on the root element wins; without it the system setting
  (`prefers-color-scheme`) decides.
- Density: `data-density="comfortable|compact|touch"`.
- Motion: `data-motion="full|reduced"` wins; without it `prefers-reduced-motion` decides.
- Accent: `data-accent="<key>"` on the same element as `data-theme` (the root) swaps the accent
  roles for each non-default accent; without `data-theme` the system setting picks the theme of
  the variant, as for the base roles. The default accent needs no attribute.
"""

from __future__ import annotations

from . import color, dtcg
from .model import Model

PREFIX = "--ds-"
GENERIC_FAMILIES = {
    "serif",
    "sans-serif",
    "monospace",
    "cursive",
    "fantasy",
    "system-ui",
    "ui-serif",
    "ui-sans-serif",
    "ui-monospace",
    "ui-rounded",
    "-apple-system",
    "math",
    "emoji",
}


def var_name(path: str) -> str:
    parts = [p for p in path.split(".") if p != "$root"]
    return PREFIX + "-".join(parts)


def _num(value: float) -> str:
    text = f"{value:.4f}".rstrip("0").rstrip(".")
    return text if text not in ("-0", "") else "0"


def font_family(value) -> str:
    names = [value] if isinstance(value, str) else value
    return ", ".join(n if n in GENERIC_FAMILIES else f'"{n}"' for n in names)


def color_value(hex_value: str, alpha: float) -> str:
    if alpha >= 1:
        return hex_value
    r, g, b = (round(v * 255) for v in color.hex_to_srgb(hex_value))
    return f"rgb({r} {g} {b} / {_num(alpha)})"


def cubic(value) -> str:
    return "cubic-bezier(" + ", ".join(_num(v) for v in value) + ")"


def dimension(value) -> str:
    return f"{_num(value['value'])}{value['unit']}"


def duration(value) -> str:
    return f"{_num(value['value'])}{value['unit']}"


def _shadow_layer(layer: dict) -> str:
    probe = dtcg.Token(path="shadow", value=layer["color"], type="color")
    dtcg._validate_color(probe)
    col = color_value(probe.hex, dtcg.color_alpha(layer["color"]))
    inset = "inset " if layer.get("inset") else ""
    dims = " ".join(dimension(layer[k]) for k in ("offsetX", "offsetY", "blur", "spread"))
    return f"{inset}{dims} {col}"


def declarations(token: dtcg.Token) -> list[tuple[str, str]]:
    name, value, kind = var_name(token.path), token.value, token.type
    if kind == "color":
        return [(name, color_value(token.hex, dtcg.color_alpha(value)))]
    if kind == "dimension":
        return [(name, dimension(value))]
    if kind == "duration":
        return [(name, duration(value))]
    if kind == "cubicBezier":
        return [(name, cubic(value))]
    if kind == "fontFamily":
        return [(name, font_family(value))]
    if kind == "fontWeight":
        return [(name, str(dtcg.font_weight_number(value)))]
    if kind == "number":
        return [(name, _num(value))]
    if kind == "shadow":
        layers = value if isinstance(value, list) else [value]
        return [(name, ", ".join(_shadow_layer(layer) for layer in layers))]
    if kind == "typography":
        return [
            (f"{name}-font-family", font_family(value["fontFamily"])),
            (f"{name}-font-size", dimension(value["fontSize"])),
            (f"{name}-font-weight", str(dtcg.font_weight_number(value["fontWeight"]))),
            (f"{name}-line-height", _num(value["lineHeight"])),
            (f"{name}-letter-spacing", dimension(value["letterSpacing"])),
        ]
    return []


def _block(selector: str, tokens: dict[str, dtcg.Token], indent: str = "") -> str:
    lines = [f"{indent}{selector} {{"]
    for path in sorted(tokens):
        for name, val in declarations(tokens[path]):
            lines.append(f"{indent}  {name}: {val};")
    lines.append(f"{indent}}}")
    return "\n".join(lines)


def _decl_block(selector: str, decls: list[tuple[str, str]], indent: str = "") -> str:
    lines = [f"{indent}{selector} {{"]
    lines += [f"{indent}  {name}: {val};" for name, val in sorted(decls)]
    lines.append(f"{indent}}}")
    return "\n".join(lines)


def accent_blocks(variants: dict[tuple[str, str], dict], default: str, default_theme: str) -> list[str]:
    """CSS blocks of every non-default accent: only the swapped `--ds-*` variables per theme.

    Args:
        variants: `accents.derive` output, (theme, accent) -> role -> Variant.
        default: the default accent (no block; the base roles are its values).
        default_theme: theme that applies without `data-theme` and without a system preference.

    Returns:
        Blocks in accent order: the default theme, the other theme, then the
        `prefers-color-scheme` block for an element without `data-theme`.
    """
    out: list[str] = []
    accent_keys = list(dict.fromkeys(accent for _, accent in variants))
    themes = list(dict.fromkeys(theme for theme, _ in variants))
    for accent in accent_keys:
        if accent == default:
            continue
        sel = f'[data-accent="{accent}"]'
        for theme in sorted(themes, key=lambda t: t != default_theme):
            decls = [(var_name(role), v.hex) for role, v in variants[(theme, accent)].items() if v.swapped]
            if theme == default_theme:
                out.append(_decl_block(f'{sel}:not([data-theme]), {sel}[data-theme="{theme}"]', decls))
            else:
                out.append(_decl_block(f'{sel}[data-theme="{theme}"]', decls))
                media = _decl_block(f"{sel}:not([data-theme])", decls, indent="  ")
                out.append(f"@media (prefers-color-scheme: {theme}) {{\n{media}\n}}")
    return out


def render(
    model: Model, header: str, variants: dict[tuple[str, str], dict] | None = None, default_accent: str | None = None
) -> str:
    """The file `tokens.css`; `variants` and `default_accent` add the `data-accent` blocks."""
    out = [f"/* {header} */", ""]
    out.append(_block(":root", model.base))
    theme = model.contexts.get("theme", {})
    if theme:
        default = model.defaults["theme"]
        other = next(c for c in theme if c != default)
        out.append("")
        out.append(_block(f':root, [data-theme="{default}"]', theme[default]))
        out.append("")
        out.append(_block(f'[data-theme="{other}"]', theme[other]))
        out.append("")
        media = "dark" if other == "dark" else "light"
        out.append(f"@media (prefers-color-scheme: {media}) {{")
        out.append(_block(":root:not([data-theme])", theme[other], indent="  "))
        out.append("}")
        for block in accent_blocks(variants or {}, default_accent or "", default):
            out.append("")
            out.append(block)
    for name in ("density", "motion"):
        ctxs = model.contexts.get(name, {})
        if not ctxs:
            continue
        default = model.defaults[name]
        out.append("")
        out.append(_block(f':root, [data-{name}="{default}"]', ctxs[default]))
        for ctx, tokens in ctxs.items():
            if ctx == default:
                continue
            out.append("")
            out.append(_block(f'[data-{name}="{ctx}"]', tokens))
        if name == "motion" and "reduced" in ctxs:
            out.append("")
            out.append("@media (prefers-reduced-motion: reduce) {")
            out.append(_block(":root:not([data-motion])", ctxs["reduced"], indent="  "))
            out.append("}")
    return "\n".join(out) + "\n"
