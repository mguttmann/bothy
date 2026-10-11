# SPDX-License-Identifier: Apache-2.0
"""Colour maths: OKLCH <-> sRGB, WCAG 2.x relative luminance and contrast, OKLab distance.

Formulas:
- OKLab/OKLCH after Bjoern Ottosson (https://bottosson.github.io/posts/oklab/).
- sRGB transfer function per IEC 61966-2-1 (threshold 0.04045, see the WAI errata note
  https://www.w3.org/WAI/GL/wiki/Relative_luminance).
- Contrast ratio (L1 + 0.05) / (L2 + 0.05) per WCAG 2.2.
"""

from __future__ import annotations

import math

RGB = tuple[float, float, float]

_GAMUT_EPS = 1e-4


def _srgb_encode(x: float) -> float:
    if x <= 0.0031308:
        return 12.92 * x
    return 1.055 * (x ** (1 / 2.4)) - 0.055


def _srgb_decode(x: float) -> float:
    if x <= 0.04045:
        return x / 12.92
    return ((x + 0.055) / 1.055) ** 2.4


def oklab_to_linear_srgb(lightness: float, a: float, b: float) -> RGB:
    """Convert OKLab to linear sRGB; the result may lie outside the sRGB gamut."""
    l_ = lightness + 0.3963377774 * a + 0.2158037573 * b
    m_ = lightness - 0.1055613458 * a - 0.0638541728 * b
    s_ = lightness - 0.0894841775 * a - 1.2914855480 * b
    l3, m3, s3 = l_**3, m_**3, s_**3
    return (
        4.0767416621 * l3 - 3.3077115913 * m3 + 0.2309699292 * s3,
        -1.2684380046 * l3 + 2.6097574011 * m3 - 0.3413193965 * s3,
        -0.0041960863 * l3 - 0.7034186147 * m3 + 1.7076147010 * s3,
    )


def linear_srgb_to_oklab(rgb: RGB) -> tuple[float, float, float]:
    """Convert linear sRGB to OKLab as (lightness, a, b)."""
    r, g, b = rgb
    lms = (
        0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b,
        0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b,
        0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b,
    )
    l_, m_, s_ = (math.copysign(abs(v) ** (1 / 3), v) for v in lms)
    return (
        0.2104542553 * l_ + 0.7936177850 * m_ - 0.0040720468 * s_,
        1.9779984951 * l_ - 2.4285922050 * m_ + 0.4505937099 * s_,
        0.0259040371 * l_ + 0.7827717662 * m_ - 0.8086757660 * s_,
    )


def oklch_to_srgb(lightness: float, chroma: float, hue: float) -> RGB:
    """Return gamma-encoded sRGB components (may lie outside 0..1 if out of gamut)."""
    rad = math.radians(hue)
    linear = oklab_to_linear_srgb(lightness, chroma * math.cos(rad), chroma * math.sin(rad))
    return tuple(math.copysign(_srgb_encode(abs(v)), v) for v in linear)  # type: ignore[return-value]


def srgb_to_oklch(rgb: RGB) -> tuple[float, float, float]:
    """Convert gamma-encoded sRGB to OKLCH as (lightness, chroma, hue in degrees)."""
    linear = tuple(_srgb_decode(v) for v in rgb)
    lightness, a, b = linear_srgb_to_oklab(linear)  # type: ignore[arg-type]
    chroma = math.hypot(a, b)
    hue = math.degrees(math.atan2(b, a)) % 360.0
    return lightness, chroma, hue


def in_srgb_gamut(rgb: RGB) -> bool:
    """Return True if every channel lies in [0, 1] within a small tolerance."""
    return all(-_GAMUT_EPS <= v <= 1 + _GAMUT_EPS for v in rgb)


def to_hex(rgb: RGB) -> str:
    """Return `#rrggbb` for sRGB channels, clamped to [0, 1]."""
    return "#" + "".join(f"{round(min(1.0, max(0.0, v)) * 255):02x}" for v in rgb)


def hex_to_srgb(value: str) -> RGB:
    """Parse `#rrggbb` into sRGB channels in [0, 1]; raises ValueError for other forms."""
    value = value.lstrip("#")
    if len(value) != 6:
        raise ValueError(f"expected #rrggbb, got #{value}")
    return tuple(int(value[i : i + 2], 16) / 255 for i in (0, 2, 4))  # type: ignore[return-value]


def relative_luminance(hex_value: str) -> float:
    """Return the WCAG relative luminance of a `#rrggbb` colour."""
    r, g, b = (_srgb_decode(v) for v in hex_to_srgb(hex_value))
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def contrast(fg_hex: str, bg_hex: str) -> float:
    """Return the WCAG contrast ratio of two `#rrggbb` colours."""
    l1, l2 = sorted((relative_luminance(fg_hex), relative_luminance(bg_hex)), reverse=True)
    return (l1 + 0.05) / (l2 + 0.05)


def delta_e_ok(hex_a: str, hex_b: str) -> float:
    """Euclidean distance in OKLab (0 = identical; about 0.02 is barely noticeable)."""
    lab_a = linear_srgb_to_oklab(tuple(_srgb_decode(v) for v in hex_to_srgb(hex_a)))  # type: ignore[arg-type]
    lab_b = linear_srgb_to_oklab(tuple(_srgb_decode(v) for v in hex_to_srgb(hex_b)))  # type: ignore[arg-type]
    return math.dist(lab_a, lab_b)
