# SPDX-License-Identifier: Apache-2.0
"""Design gates: WCAG 2.2 contrast for declared colour pairs and palette guard rules.

Contrast thresholds (WCAG 2.2, https://www.w3.org/TR/WCAG22/):
- `text`: SC 1.4.3, 4.5:1
- `large-text`: SC 1.4.3, 3:1 (at least 18 pt, or 14 pt bold)
- `ui`: SC 1.4.11 non-text contrast, 3:1
"""

from __future__ import annotations

from dataclasses import dataclass

from . import color, dtcg

THRESHOLDS = {"text": 4.5, "large-text": 3.0, "ui": 3.0}


class RuleError(ValueError):
    """Raised when the rules themselves are invalid (unknown token or kind)."""


@dataclass
class ContrastRow:
    """One checked pairing: colours, ratio and threshold; `accent` is empty for the base roles."""

    theme: str
    fg: str
    bg: str
    kind: str
    fg_hex: str
    bg_hex: str
    ratio: float
    required: float
    usage: str
    accent: str = ""

    @property
    def passed(self) -> bool:
        return round(self.ratio, 2) >= self.required


def _hex(tokens: dict[str, dtcg.Token], path: str, theme: str) -> str:
    token = tokens.get(path)
    if token is None:
        raise RuleError(f"contrast pair references unknown token {path!r} (theme {theme})")
    if token.type != "color":
        raise RuleError(f"{path} is not a colour token")
    alpha = dtcg.color_alpha(token.value)
    if alpha < 1:
        raise RuleError(f"{path} is translucent; contrast pairs need opaque colours")
    return dtcg.color_hex(token)


def _as_list(value: str | list[str]) -> list[str]:
    return value if isinstance(value, list) else [value]


def expand_pairs(pairs: list[dict]) -> list[dict]:
    """Expand pairs whose fg or bg is a list into one pair per combination."""
    out = []
    for pair in pairs:
        if pair["kind"] not in THRESHOLDS:
            raise RuleError(f"unknown contrast kind {pair['kind']!r}")
        for fg in _as_list(pair["fg"]):
            for bg in _as_list(pair["bg"]):
                out.append({**pair, "fg": fg, "bg": bg})
    return out


def check_contrast(themes: dict[str, dict[str, dtcg.Token]], pairs: list[dict]) -> list[ContrastRow]:
    rows: list[ContrastRow] = []
    expanded = expand_pairs(pairs)
    for theme, tokens in themes.items():
        for pair in expanded:
            fg_hex = _hex(tokens, pair["fg"], theme)
            bg_hex = _hex(tokens, pair["bg"], theme)
            rows.append(
                ContrastRow(
                    theme=theme,
                    fg=pair["fg"],
                    bg=pair["bg"],
                    kind=pair["kind"],
                    fg_hex=fg_hex,
                    bg_hex=bg_hex,
                    ratio=color.contrast(fg_hex, bg_hex),
                    required=THRESHOLDS[pair["kind"]],
                    usage=pair.get("usage", ""),
                )
            )
    return rows


def check_accent_contrast(
    themes: dict[str, dict[str, dtcg.Token]],
    pairs: list[dict],
    variants: dict[tuple[str, str], dict],
    default: str,
) -> list[ContrastRow]:
    """Contrast rows of every non-default accent, one per pairing that contains a swapped role.

    The default accent adds no rows: its variants equal the base roles, which `check_contrast`
    already covers.

    Args:
        themes: resolved token map per theme context.
        pairs: `contrast_pairs` of the rules file (lists are expanded).
        variants: `accents.derive` output, (theme, accent) -> role -> Variant.
        default: the default accent.

    Returns:
        Rows with `accent` set, in the order of `variants`, then of the expanded pairs.

    Raises:
        RuleError: a pairing names an unknown, non-colour or translucent token.
    """
    rows: list[ContrastRow] = []
    expanded = expand_pairs(pairs)
    for (theme, accent), roles in variants.items():
        if accent == default:
            continue
        swapped = {role: v.hex for role, v in roles.items() if v.swapped}
        tokens = themes[theme]
        for pair in expanded:
            if pair["fg"] not in swapped and pair["bg"] not in swapped:
                continue
            fg_hex = swapped.get(pair["fg"]) or _hex(tokens, pair["fg"], theme)
            bg_hex = swapped.get(pair["bg"]) or _hex(tokens, pair["bg"], theme)
            rows.append(
                ContrastRow(
                    theme=theme,
                    fg=pair["fg"],
                    bg=pair["bg"],
                    kind=pair["kind"],
                    fg_hex=fg_hex,
                    bg_hex=bg_hex,
                    ratio=color.contrast(fg_hex, bg_hex),
                    required=THRESHOLDS[pair["kind"]],
                    usage=pair.get("usage", ""),
                    accent=accent,
                )
            )
    return rows


def _in_hue_window(hue: float, start: float, end: float) -> bool:
    if start <= end:
        return start <= hue <= end
    return hue >= start or hue <= end


def check_guards(tokens: dict[str, dtcg.Token], guards: list[dict]) -> list[str]:
    """Return human-readable problems; an empty list means every guard holds."""
    problems: list[str] = []
    for guard in guards:
        scope = guard["scope"]
        for path, token in tokens.items():
            if token.type != "color" or not any(path.startswith(prefix) for prefix in scope):
                continue
            hex_value = dtcg.color_hex(token)
            _, chroma, hue = color.srgb_to_oklch(color.hex_to_srgb(hex_value))
            for ref in guard.get("forbidden_colors", []):
                distance = color.delta_e_ok(hex_value, ref["hex"])
                if distance < ref["min_delta_e_ok"]:
                    problems.append(
                        f"{path} {hex_value} is too close to {ref['name']} {ref['hex']} "
                        f"(deltaE_ok {distance:.3f} < {ref['min_delta_e_ok']})"
                    )
            for window in guard.get("forbidden_hue_ranges", []):
                if chroma >= window["min_chroma"] and _in_hue_window(hue, window["from"], window["to"]):
                    problems.append(
                        f"{path} {hex_value} lies in the forbidden hue range {window['name']} "
                        f"(hue {hue:.0f}, chroma {chroma:.3f})"
                    )
            if "min_chroma" in guard and chroma < guard["min_chroma"]:
                problems.append(
                    f"{path} {hex_value} has chroma {chroma:.4f} below {guard['min_chroma']} (untinted grey)"
                )
    return problems
