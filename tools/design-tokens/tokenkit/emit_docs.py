# SPDX-License-Identifier: Apache-2.0
"""Generated documentation pages: docs/design/palette.md, contrast.md and tokens.md."""

from __future__ import annotations

from . import color, dtcg, emit_css
from .model import Model
from .rules import ContrastRow


def _step_key(step: str):
    return int(step) if step.isdigit() else 10**6


def _plain_value(token: dtcg.Token) -> str:
    decls = emit_css.declarations(token)
    if token.type == "typography":
        parts = {name.rsplit("-", 2)[-2] + "-" + name.rsplit("-", 2)[-1]: val for name, val in decls}
        return ", ".join(f"{k} {v}" for k, v in parts.items())
    return "; ".join(val for _, val in decls).replace("|", "\\|")


def tokens_md(model: Model, header: str) -> str:
    """Reference of every non-colour token with its value, per modifier context."""
    lines = [
        "# Token-Referenz (ohne Farben)",
        "",
        f"> {header}",
        "",
        "Alle Werte außer Farben, wie sie die Apps und `tokens.css` bekommen. Farben: [palette.md](palette.md).",
        "Hand geschriebene Seiten nennen nur Token-Namen; die Zahlen stehen hier und sind durch",
        "`tokens.py check` an die Quelle gebunden.",
        "",
        "## Basis (in allen Kontexten gleich)",
        "",
        "| Token | Typ | Wert |",
        "|---|---|---|",
    ]
    for path, token in sorted(model.base.items()):
        if token.type == "color":
            continue
        lines.append(f"| `{path}` | {token.type} | {_plain_value(token)} |")
    lines.append("")
    for name, contexts in model.contexts.items():
        paths = sorted(p for p, t in next(iter(contexts.values())).items() if t.type != "color")
        if not paths:
            continue
        ctx_names = list(contexts)
        lines.append(f"## Modifikator `{name}` (Standard: `{model.defaults[name]}`)")
        lines.append("")
        lines.append("| Token | Typ | " + " | ".join(f"`{c}`" for c in ctx_names) + " |")
        lines.append("|---|---|" + "---|" * len(ctx_names))
        for path in paths:
            kind = contexts[ctx_names[0]][path].type
            cells = " | ".join(_plain_value(contexts[c][path]) for c in ctx_names)
            lines.append(f"| `{path}` | {kind} | {cells} |")
        lines.append("")
    return "\n".join(lines)


def _alias_chain(token: dtcg.Token) -> str:
    return token.alias_of.removeprefix("palette.") if token.alias_of else "eigener Wert"


def palette_md(model: Model, header: str) -> str:
    lines = [
        "# Farbpalette",
        "",
        f"> {header}",
        "",
        "Primitive Tonleitern in OKLCH und ihre sRGB-Werte. Komponenten verwenden nur die",
        "semantischen Rollen weiter unten. Bilder: [Palette hell](previews/palette-light.png),",
        "[Palette dunkel](previews/palette-dark.png). Begründung der Farbwahl: [README.md](README.md).",
        "",
        'Spalten "zu Weiß" und "zu Schwarz": WCAG-Kontrastverhältnis gegen #ffffff bzw. #000000.',
        "",
    ]
    scales: dict[str, list[tuple[str, dtcg.Token]]] = {}
    for path, token in model.base.items():
        if path.startswith("palette."):
            _, scale, step = path.split(".")
            scales.setdefault(scale, []).append((step, token))
    for scale in scales:
        lines.append(f"## {scale}")
        lines.append("")
        lines.append("| Stufe | Hex | OKLCH (L, C, H) | zu Weiß | zu Schwarz |")
        lines.append("|---|---|---|---|---|")
        for step, token in sorted(scales[scale], key=lambda s: _step_key(s[0])):
            comps = token.value["components"]
            oklch = ", ".join(f"{c:g}" for c in comps)
            lines.append(
                f"| {step} | `{token.hex}` | {oklch} | {color.contrast(token.hex, '#ffffff'):.2f} | "
                f"{color.contrast(token.hex, '#000000'):.2f} |"
            )
        lines.append("")
    theme = model.contexts.get("theme", {})
    if theme:
        names = list(theme)
        lines.append("## Semantische Rollen")
        lines.append("")
        lines.append(
            "Jede Rolle verweist je Theme auf eine Stufe der Palette. Durchscheinende Rollen tragen ihren Alphawert."
        )
        lines.append("")
        lines.append("| Rolle | " + " | ".join(names) + " |")
        lines.append("|---|" + "---|" * len(names))
        for path in sorted(theme[names[0]]):
            if theme[names[0]][path].type != "color":
                continue
            cells = []
            for ctx in names:
                token = theme[ctx][path]
                alpha = dtcg.color_alpha(token.value)
                suffix = f" (Alpha {alpha:g})" if alpha < 1 else ""
                cells.append(f"`{token.hex}` {_alias_chain(token)}{suffix}")
            lines.append(f"| `{path}` | " + " | ".join(cells) + " |")
        lines.append("")
    return "\n".join(lines)


def _variant_label(variant) -> str:
    """`loch.700`, `loch.700, moved from 600`, or the kept value with "not swapped"."""
    name = f"{variant.scale}.{variant.step}" if variant.scale else variant.hex
    if not variant.swapped:
        return f"{name}, not swapped"
    return f"{name}, moved from {variant.moved_from}" if variant.moved_from else name


def _accent_sections(accent_rows: list[ContrastRow], variants: dict[tuple[str, str], dict], default: str) -> list[str]:
    lines: list[str] = []
    if not variants:
        return lines
    accent_keys = list(dict.fromkeys(accent for _, accent in variants))
    themes = list(dict.fromkeys(theme for theme, _ in variants))
    lines += [
        "## Accent variants",
        "",
        "An instance chooses its accent (`branding.accent_color`, list in `accent_variants` of",
        "`design/design-rules.json`). Roles that alias the default scale take the same step of the",
        "chosen scale; a step that fails a pairing moves along the scale until every pairing passes",
        '("moved from"). Roles that alias another scale keep their value ("not swapped"). Per accent',
        "and theme only the pairings that contain a swapped role are listed; all other pairings are",
        "those of the theme sections above.",
        "",
        f"Accent `{default}` is the default: its variants are the base roles of the theme sections above.",
        "",
    ]
    for accent in accent_keys:
        if accent == default:
            continue
        for theme in themes:
            roles = variants[(theme, accent)]
            rows = [r for r in accent_rows if r.theme == theme and r.accent == accent]
            labels = {role: _variant_label(v) for role, v in roles.items() if v.swapped}
            lines.append(f"### Accent {accent}, theme {theme}")
            lines.append("")
            lines.append("| Role | Step | Value |")
            lines.append("|---|---|---|")
            for role, variant in roles.items():
                lines.append(f"| `{role}` | {_variant_label(variant)} | `{variant.hex}` |")
            lines.append("")
            lines.append("| Foreground | Background | Kind | Values | Ratio | Threshold | Result | Usage |")
            lines.append("|---|---|---|---|---|---|---|---|")
            for r in rows:
                fg = f"`{r.fg}` ({labels[r.fg]})" if r.fg in labels else f"`{r.fg}`"
                bg = f"`{r.bg}` ({labels[r.bg]})" if r.bg in labels else f"`{r.bg}`"
                verdict = "passed" if r.passed else "FAILED"
                lines.append(
                    f"| {fg} | {bg} | {r.kind} | `{r.fg_hex}` on `{r.bg_hex}` | {r.ratio:.2f} | "
                    f"{r.required:g} | {verdict} | {r.usage} |"
                )
            lines.append("")
    return lines


def contrast_md(
    rows: list[ContrastRow],
    guards: list[str],
    header: str,
    accent_rows: list[ContrastRow] | None = None,
    variants: dict[tuple[str, str], dict] | None = None,
    default: str = "",
) -> str:
    """The contrast table: base rows per theme, then one section per non-default accent and theme."""
    accent_rows = accent_rows or []
    failed = [r for r in rows + accent_rows if not r.passed]
    lines = [
        "# Kontrasttabelle (WCAG 2.2)",
        "",
        f"> {header}",
        "",
        "Jede Text- und Flächen-Paarung, die eine Komponente nutzt (Liste: `design/design-rules.json`),",
        "für jedes Theme. Schwellen: `text` 4.5:1 (SC 1.4.3), `large-text` 3:1 (SC 1.4.3),",
        "`ui` 3:1 (SC 1.4.11), Quelle https://www.w3.org/TR/WCAG22/ (abgerufen 2026-10-10).",
        "Das Werkzeug bricht ab, wenn eine Paarung darunter liegt; diese Seite entsteht nur bei Erfolg.",
        "",
        f"Ergebnis: {len(rows) + len(accent_rows)} Paarungen geprüft, {len(failed)} unter der Schwelle"
        + (f" ({len(accent_rows)} davon in den Akzent-Varianten)." if accent_rows else "."),
        "",
        "Palettenwächter (Verbotsliste als Test): "
        + ("alle eingehalten." if not guards else f"{len(guards)} Verstöße."),
        "",
    ]
    for theme in dict.fromkeys(r.theme for r in rows):
        lines.append(f"## Theme {theme}")
        lines.append("")
        lines.append("| Vordergrund | Hintergrund | Art | Werte | Verhältnis | Schwelle | Ergebnis | Verwendung |")
        lines.append("|---|---|---|---|---|---|---|---|")
        for r in rows:
            if r.theme != theme:
                continue
            verdict = "bestanden" if r.passed else "NICHT BESTANDEN"
            lines.append(
                f"| `{r.fg}` | `{r.bg}` | {r.kind} | `{r.fg_hex}` auf `{r.bg_hex}` | {r.ratio:.2f} | "
                f"{r.required:g} | {verdict} | {r.usage} |"
            )
        lines.append("")
    lines += _accent_sections(accent_rows, variants or {}, default)
    return "\n".join(lines)
