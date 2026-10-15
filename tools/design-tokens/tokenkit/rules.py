# SPDX-License-Identifier: Apache-2.0
"""Design gates: WCAG 2.2 contrast for declared colour pairs and palette guard rules.

Contrast thresholds (WCAG 2.2, https://www.w3.org/TR/WCAG22/):
- `text`: SC 1.4.3, 4.5:1
- `large-text`: SC 1.4.3, 3:1 (at least 18 pt, or 14 pt bold)
- `ui`: SC 1.4.11 non-text contrast, 3:1
"""

from __future__ import annotations

from dataclasses import dataclass

from . import color, cvd, dtcg

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
        """Return True if the rounded ratio reaches the required ratio."""
        return round(self.ratio, 2) >= self.required


def _hex(tokens: dict[str, dtcg.Token], path: str, theme: str, what: str = "contrast pair") -> str:
    token = tokens.get(path)
    if token is None:
        raise RuleError(f"{what} references unknown token {path!r} (theme {theme})")
    if token.type != "color":
        raise RuleError(f"{path} is not a colour token")
    alpha = dtcg.color_alpha(token.value)
    if alpha < 1:
        raise RuleError(f"{path} is translucent; {what}s need opaque colours")
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
    """Check every contrast pair in every theme and return one row per pair."""
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


def expand_cvd_pairs(entries: list[dict]) -> list[dict]:
    """Expand the `cvd_pairs` entries of the rules file into one dict per checked pair.

    An entry names `deficiencies` (non-empty, from `cvd.DEFICIENCIES`) and either `sequence`
    (roles in their fixed order; adjacent roles form the pairs, `wrap: true` adds last with
    first) or `pairs` (a list of `[role_a, role_b]`).

    Returns:
        Dicts with `a`, `b`, `usage` and `deficiencies`, in entry order.

    Raises:
        RuleError: unknown or missing deficiency, neither `sequence` nor `pairs`, a sequence
            shorter than two roles, or a pair that is not two roles.
    """
    out: list[dict] = []
    for entry in entries:
        usage = entry.get("usage", "")
        deficiencies = entry.get("deficiencies") or []
        if not deficiencies:
            raise RuleError(f"cvd_pairs entry {usage!r} names no deficiency")
        for deficiency in deficiencies:
            if deficiency not in cvd.DEFICIENCIES:
                known = ", ".join(cvd.DEFICIENCIES)
                raise RuleError(f"cvd_pairs entry {usage!r}: unknown deficiency {deficiency!r} (known: {known})")
        if "sequence" in entry:
            seq = entry["sequence"]
            if len(seq) < 2:
                raise RuleError(f"cvd_pairs entry {usage!r}: a sequence needs at least two roles")
            pairs = list(zip(seq[:-1], seq[1:], strict=True))
            if entry.get("wrap"):
                pairs.append((seq[-1], seq[0]))
        elif "pairs" in entry:
            pairs = []
            for pair in entry["pairs"]:
                if len(pair) != 2:
                    raise RuleError(f"cvd_pairs entry {usage!r}: a pair needs exactly two roles, got {pair!r}")
                pairs.append((pair[0], pair[1]))
        else:
            raise RuleError(f"cvd_pairs entry {usage!r} needs `sequence` or `pairs`")
        out += [{"a": a, "b": b, "usage": usage, "deficiencies": list(deficiencies)} for a, b in pairs]
    return out


def check_cvd(themes: dict[str, dict[str, dtcg.Token]], rule_data: dict) -> list[cvd.CvdRow]:
    """Colour vision deficiency rows: every `cvd_pairs` pair, per theme and deficiency.

    Args:
        themes: resolved token map per theme context.
        rule_data: the whole rules file; reads `cvd_min_delta_e_ok.value` and `cvd_pairs`.

    Returns:
        Rows per theme (in `themes` order), then per entry and deficiency, then pair.
        Empty when the rules file has no `cvd_pairs`.

    Raises:
        RuleError: `cvd_pairs` without `cvd_min_delta_e_ok`, an invalid entry
            (`expand_cvd_pairs`), or a role that is unknown, not a colour or translucent.
    """
    entries = rule_data.get("cvd_pairs")
    if entries is None:
        return []
    minimum = rule_data.get("cvd_min_delta_e_ok", {}).get("value")
    if not isinstance(minimum, (int, float)) or minimum <= 0:
        raise RuleError("cvd_pairs needs cvd_min_delta_e_ok.value, a positive number")
    groups = [(entry, expand_cvd_pairs([entry])) for entry in entries]
    rows: list[cvd.CvdRow] = []
    for theme, tokens in themes.items():
        for entry, pairs in groups:
            colors = {role: _hex(tokens, role, theme, "cvd pair") for p in pairs for role in (p["a"], p["b"])}
            triples = [(p["a"], p["b"], p["usage"]) for p in pairs]
            rows += cvd.check_pairs(theme, colors, triples, entry["deficiencies"], float(minimum))
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
        suffixes = guard.get("scope_suffix", [])
        for path, token in tokens.items():
            if token.type != "color" or not any(path.startswith(prefix) for prefix in scope):
                continue
            if suffixes and not any(path.endswith(suffix) for suffix in suffixes):
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
                below_max = chroma < window.get("max_chroma", float("inf"))
                if chroma >= window["min_chroma"] and below_max and _in_hue_window(hue, window["from"], window["to"]):
                    problems.append(
                        f"{path} {hex_value} lies in the forbidden hue range {window['name']} "
                        f"(hue {hue:.0f}, chroma {chroma:.3f})"
                    )
            if "min_chroma" in guard and chroma < guard["min_chroma"]:
                problems.append(
                    f"{path} {hex_value} has chroma {chroma:.4f} below {guard['min_chroma']} (untinted grey)"
                )
    return problems
