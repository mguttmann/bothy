# SPDX-License-Identifier: Apache-2.0
"""Subset of the Design Tokens Format Module 2025.10 (https://www.designtokens.org/tr/2025.10/format/).

Supported: groups with inherited `$type`, `$description`, `$extensions`, `$deprecated`,
the reserved token name `$root`, curly-brace aliases (also inside composite values) and the
types color, dimension, duration, cubicBezier, fontFamily, fontWeight, number, shadow,
typography, border, strokeStyle, transition.

Not supported (rejected with an error so nothing is silently ignored): `$extends` and JSON
Pointer `$ref` inside token files. The pipeline does not use them.
"""

from __future__ import annotations

import copy
import re
from dataclasses import dataclass, field
from typing import Any

from . import color

TYPES = {
    "color",
    "dimension",
    "duration",
    "cubicBezier",
    "fontFamily",
    "fontWeight",
    "number",
    "shadow",
    "typography",
    "border",
    "strokeStyle",
    "transition",
}

COLOR_SPACES = {
    "srgb",
    "srgb-linear",
    "hsl",
    "hwb",
    "lab",
    "lch",
    "oklab",
    "oklch",
    "display-p3",
    "a98-rgb",
    "prophoto-rgb",
    "rec2020",
    "xyz-d65",
    "xyz-d50",
}

FONT_WEIGHT_ALIASES = {
    "thin": 100,
    "hairline": 100,
    "extra-light": 200,
    "ultra-light": 200,
    "light": 300,
    "normal": 400,
    "regular": 400,
    "book": 400,
    "medium": 500,
    "semi-bold": 600,
    "demi-bold": 600,
    "bold": 700,
    "extra-bold": 800,
    "ultra-bold": 800,
    "black": 900,
    "heavy": 900,
    "extra-black": 950,
    "ultra-black": 950,
}

STROKE_STYLES = {"solid", "dashed", "dotted", "double", "groove", "ridge", "outset", "inset"}

_GROUP_KEYS = {"$type", "$description", "$extensions", "$deprecated"}
_TOKEN_KEYS = {"$value", "$type", "$description", "$extensions", "$deprecated"}
_ALIAS = re.compile(r"^\{([^{}]+)\}$")


class TokenError(ValueError):
    """Raised for any invalid token document."""


@dataclass
class Token:
    path: str
    value: Any
    type: str | None = None
    own_type: str | None = None
    group_type: str | None = None
    description: str = ""
    extensions: dict = field(default_factory=dict)
    deprecated: bool | str = False
    alias_of: str | None = None
    hex: str | None = None


def _check_name(name: str, where: str) -> None:
    if name == "$root":
        return
    if name.startswith("$") or any(ch in name for ch in "{}."):
        raise TokenError(f"invalid token or group name {name!r} in {where or '<root>'}")


def flatten(doc: dict, prefix: str = "", group_type: str | None = None) -> dict[str, Token]:
    """Return all tokens of a document keyed by dotted path."""
    if not isinstance(doc, dict):
        raise TokenError(f"group {prefix or '<root>'} must be an object")
    if "$extends" in doc or "$ref" in doc:
        raise TokenError(f"$extends/$ref is not supported (at {prefix or '<root>'})")
    current_type = doc.get("$type", group_type)
    out: dict[str, Token] = {}
    for key, node in doc.items():
        if key in _GROUP_KEYS or key == "$schema":
            continue
        _check_name(key, prefix)
        path = f"{prefix}.{key}" if prefix else key
        if not isinstance(node, dict):
            raise TokenError(f"{path}: expected an object")
        if "$value" in node:
            extra = [k for k in node if not k.startswith("$")]
            if extra:
                raise TokenError(f"{path}: a token must not contain child tokens or groups {extra}")
            unknown = [k for k in node if k.startswith("$") and k not in _TOKEN_KEYS]
            if unknown:
                raise TokenError(f"{path}: unknown token properties {unknown}")
            out[path] = Token(
                path=path,
                value=copy.deepcopy(node["$value"]),
                type=node.get("$type", current_type),
                own_type=node.get("$type"),
                group_type=current_type,
                description=node.get("$description", ""),
                extensions=copy.deepcopy(node.get("$extensions", {})),
                deprecated=node.get("$deprecated", False),
            )
        else:
            out.update(flatten(node, path, current_type))
    return out


def merge(docs: list[dict]) -> dict:
    """Deep-merge token documents in order; a later token replaces an earlier one entirely."""
    result: dict = {}
    for doc in docs:
        _merge_into(result, copy.deepcopy(doc))
    return result


def _merge_into(target: dict, source: dict) -> None:
    for key, value in source.items():
        existing = target.get(key)
        is_group = isinstance(value, dict) and "$value" not in value
        if is_group and isinstance(existing, dict) and "$value" not in existing:
            _merge_into(existing, value)
        else:
            target[key] = value


def _alias_target(value: Any) -> str | None:
    if isinstance(value, str):
        match = _ALIAS.match(value)
        if match:
            return match.group(1)
    return None


def resolve(tokens: dict[str, Token]) -> dict[str, Token]:
    """Resolve aliases, determine types and validate every value."""
    resolved: dict[str, Token] = {}

    def resolve_token(path: str, stack: tuple[str, ...]) -> Token:
        if path in resolved:
            return resolved[path]
        if path in stack:
            raise TokenError(f"circular reference: {' -> '.join(stack + (path,))}")
        if path not in tokens:
            raise TokenError(f"unknown reference {{{path}}} (from {stack[-1] if stack else '?'})")
        token = copy.copy(tokens[path])
        target = _alias_target(token.value)
        if target is not None:
            target_token = resolve_token(target, stack + (path,))
            token.alias_of = target
            token.value = copy.deepcopy(target_token.value)
            token.type = token.own_type or target_token.type or token.group_type
        else:
            token.value = resolve_inner(token.value, stack + (path,))
            token.type = token.own_type or token.group_type
        if token.type is None:
            raise TokenError(f"{path}: no $type (own, referenced or inherited)")
        if token.type not in TYPES:
            raise TokenError(f"{path}: unknown $type {token.type!r}")
        _validate(token)
        resolved[path] = token
        return token

    def resolve_inner(value: Any, stack: tuple[str, ...]) -> Any:
        target = _alias_target(value)
        if target is not None:
            return copy.deepcopy(resolve_token(target, stack).value)
        if isinstance(value, dict):
            return {k: resolve_inner(v, stack) for k, v in value.items()}
        if isinstance(value, list):
            return [resolve_inner(v, stack) for v in value]
        return value

    for path in tokens:
        resolve_token(path, ())
    return resolved


def _is_number(value: Any) -> bool:
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def _validate_dimension(path: str, value: Any) -> None:
    if not (isinstance(value, dict) and _is_number(value.get("value")) and value.get("unit") in ("px", "rem")):
        raise TokenError(f"{path}: dimension must be {{value: number, unit: px|rem}}, got {value!r}")


def _validate_duration(path: str, value: Any) -> None:
    if not (isinstance(value, dict) and _is_number(value.get("value")) and value.get("unit") in ("ms", "s")):
        raise TokenError(f"{path}: duration must be {{value: number, unit: ms|s}}, got {value!r}")


def _validate_cubic(path: str, value: Any) -> None:
    if not (isinstance(value, list) and len(value) == 4 and all(_is_number(v) for v in value)):
        raise TokenError(f"{path}: cubicBezier must be four numbers")
    if not (0 <= value[0] <= 1 and 0 <= value[2] <= 1):
        raise TokenError(f"{path}: cubicBezier x values must lie in [0, 1]")


def _validate_font_weight(path: str, value: Any) -> None:
    if _is_number(value) and 1 <= value <= 1000:
        return
    if isinstance(value, str) and value in FONT_WEIGHT_ALIASES:
        return
    raise TokenError(f"{path}: invalid fontWeight {value!r}")


def font_weight_number(value: Any) -> int:
    if _is_number(value):
        return int(value)
    return FONT_WEIGHT_ALIASES[value]


def _validate_color(token: Token) -> None:
    value = token.value
    if not isinstance(value, dict) or "colorSpace" not in value or "components" not in value:
        raise TokenError(f"{token.path}: color must be {{colorSpace, components[, alpha, hex]}}")
    space = value["colorSpace"]
    if space not in COLOR_SPACES:
        raise TokenError(f"{token.path}: unknown colorSpace {space!r}")
    comps = value["components"]
    if not (isinstance(comps, list) and len(comps) == 3):
        raise TokenError(f"{token.path}: components must have three entries")
    alpha = value.get("alpha", 1)
    if not (_is_number(alpha) and 0 <= alpha <= 1):
        raise TokenError(f"{token.path}: alpha must lie in [0, 1]")
    nums = [0.0 if c == "none" else c for c in comps]
    if not all(_is_number(c) for c in nums):
        raise TokenError(f"{token.path}: components must be numbers or 'none'")
    if space == "oklch":
        rgb = color.oklch_to_srgb(*nums)
        if not color.in_srgb_gamut(rgb):
            raise TokenError(f"{token.path}: oklch{tuple(nums)} lies outside the sRGB gamut")
    elif space == "srgb":
        rgb = tuple(nums)
        if not color.in_srgb_gamut(rgb):
            raise TokenError(f"{token.path}: srgb components must lie in [0, 1]")
    elif "hex" in value:
        rgb = color.hex_to_srgb(value["hex"])
    else:
        raise TokenError(f"{token.path}: colorSpace {space!r} needs a hex fallback for this tool")
    token.hex = color.to_hex(rgb)


def _validate(token: Token) -> None:
    path, value, kind = token.path, token.value, token.type
    if kind == "color":
        _validate_color(token)
    elif kind == "dimension":
        _validate_dimension(path, value)
    elif kind == "duration":
        _validate_duration(path, value)
    elif kind == "cubicBezier":
        _validate_cubic(path, value)
    elif kind == "fontFamily":
        if not (
            isinstance(value, str) or (isinstance(value, list) and value and all(isinstance(v, str) for v in value))
        ):
            raise TokenError(f"{path}: fontFamily must be a string or a list of strings")
    elif kind == "fontWeight":
        _validate_font_weight(path, value)
    elif kind == "number":
        if not _is_number(value):
            raise TokenError(f"{path}: number expected")
    elif kind == "typography":
        _validate_typography(path, value)
    elif kind == "shadow":
        layers = value if isinstance(value, list) else [value]
        for layer in layers:
            _validate_shadow(path, layer)
    elif kind == "border":
        if not isinstance(value, dict) or set(value) != {"color", "width", "style"}:
            raise TokenError(f"{path}: border must be {{color, width, style}}")
        _validate_dimension(path, value["width"])
    elif kind == "strokeStyle":
        if not (value in STROKE_STYLES or isinstance(value, dict)):
            raise TokenError(f"{path}: invalid strokeStyle")
    elif kind == "transition":
        if not isinstance(value, dict) or set(value) != {"duration", "delay", "timingFunction"}:
            raise TokenError(f"{path}: transition must be {{duration, delay, timingFunction}}")
        _validate_duration(path, value["duration"])
        _validate_duration(path, value["delay"])
        _validate_cubic(path, value["timingFunction"])


def _validate_typography(path: str, value: Any) -> None:
    required = {"fontFamily", "fontSize", "fontWeight", "letterSpacing", "lineHeight"}
    if not isinstance(value, dict) or set(value) != required:
        raise TokenError(f"{path}: typography needs exactly {sorted(required)}")
    _validate_dimension(path, value["fontSize"])
    _validate_dimension(path, value["letterSpacing"])
    _validate_font_weight(path, value["fontWeight"])
    if not _is_number(value["lineHeight"]):
        raise TokenError(f"{path}: lineHeight must be a number (multiplier)")


def _validate_shadow(path: str, layer: Any) -> None:
    required = {"color", "offsetX", "offsetY", "blur", "spread"}
    if not isinstance(layer, dict) or not required <= set(layer) or not set(layer) <= required | {"inset"}:
        raise TokenError(f"{path}: shadow layer needs {sorted(required)} (optional inset)")
    for key in ("offsetX", "offsetY", "blur", "spread"):
        _validate_dimension(path, layer[key])
    probe = Token(path=f"{path}.color", value=layer["color"], type="color")
    _validate_color(probe)


def color_hex(token: Token) -> str:
    if token.type != "color" or token.hex is None:
        raise TokenError(f"{token.path} is not a colour")
    return token.hex


def color_alpha(value: dict) -> float:
    return float(value.get("alpha", 1))
