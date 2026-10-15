# SPDX-License-Identifier: Apache-2.0
"""Colour vision deficiency: simulation and the distance check over role pairs.

Model: Machado, Oliveira and Fernandes, "A Physiologically-based Model for Simulation of Color
Vision Deficiency", IEEE Transactions on Visualization and Computer Graphics 15(6), 2009,
doi:10.1109/TVCG.2009.113. The matrices are those for severity 1.0 (dichromacy) from the table of
precomputed matrices in the paper's supplementary material. They are applied in linear sRGB:
decode sRGB, multiply, clip to [0, 1], encode (ruling D-M6-10-1).

Distance: Euclidean distance in OKLab of the two simulated colours (`delta_e_ok`, the measure of
the palette guards; ruling D-M6-10-2).

Example:
    >>> simulate_hex("#ffffff", "deuteranopia")
    '#ffffff'
"""

from __future__ import annotations

import math
from collections.abc import Iterable, Mapping
from dataclasses import dataclass

from . import color

MODEL = "Machado, Oliveira and Fernandes 2009, severity 1.0, applied in linear sRGB"
SOURCE = "IEEE TVCG 15(6), 2009, doi:10.1109/TVCG.2009.113"

Matrix = tuple[tuple[float, float, float], tuple[float, float, float], tuple[float, float, float]]

# Machado et al. 2009, supplementary table, rows "severity 1.0". Each row sums to 1, which is why
# white and every grey stay unchanged.
MATRICES: dict[str, Matrix] = {
    "protanopia": (
        (0.152286, 1.052583, -0.204868),
        (0.114503, 0.786281, 0.099216),
        (-0.003882, -0.048116, 1.051998),
    ),
    "deuteranopia": (
        (0.367322, 0.860646, -0.227968),
        (0.280085, 0.672501, 0.047413),
        (-0.011820, 0.042940, 0.968881),
    ),
    "tritanopia": (
        (1.255528, -0.076749, -0.178779),
        (-0.078411, 0.930809, 0.147602),
        (0.004733, 0.691367, 0.303900),
    ),
}
DEFICIENCIES = tuple(MATRICES)


def _matrix(deficiency: str) -> Matrix:
    try:
        return MATRICES[deficiency]
    except KeyError:
        raise ValueError(f"unknown colour vision deficiency {deficiency!r}; known: {', '.join(DEFICIENCIES)}") from None


def apply_matrix(linear: color.RGB, deficiency: str) -> color.RGB:
    """Multiply a linear sRGB triple with the matrix of `deficiency`, without clipping.

    Args:
        linear: linear-light sRGB components.
        deficiency: one of `DEFICIENCIES`.

    Returns:
        The transformed linear triple; components may lie outside [0, 1].

    Raises:
        ValueError: unknown deficiency.
    """
    m = _matrix(deficiency)
    return tuple(sum(m[i][j] * linear[j] for j in range(3)) for i in range(3))  # type: ignore[return-value]


def simulate_linear(hex_value: str, deficiency: str) -> color.RGB:
    """Simulated colour of `hex_value` as linear sRGB, clipped to [0, 1]."""
    linear = tuple(color.srgb_decode(v) for v in color.hex_to_srgb(hex_value))
    return tuple(min(1.0, max(0.0, v)) for v in apply_matrix(linear, deficiency))  # type: ignore[return-value]


def simulate(hex_value: str, deficiency: str) -> color.RGB:
    """Simulated colour of `hex_value` as gamma-encoded sRGB components in [0, 1].

    Raises:
        ValueError: unknown deficiency or malformed hex value.
    """
    return tuple(color.srgb_encode(v) for v in simulate_linear(hex_value, deficiency))  # type: ignore[return-value]


def simulate_hex(hex_value: str, deficiency: str) -> str:
    """Simulated colour of `hex_value` as `#rrggbb` (for tables and messages)."""
    return color.to_hex(simulate(hex_value, deficiency))


def distance(hex_a: str, hex_b: str, deficiency: str) -> float:
    """OKLab distance of the two colours after simulating `deficiency` on both.

    The distance uses the unrounded simulated values, so it does not depend on hex rounding.
    """
    lab_a = color.linear_srgb_to_oklab(simulate_linear(hex_a, deficiency))
    lab_b = color.linear_srgb_to_oklab(simulate_linear(hex_b, deficiency))
    return math.dist(lab_a, lab_b)


@dataclass
class CvdRow:
    """One checked pair under one deficiency in one theme."""

    theme: str
    deficiency: str
    a: str
    b: str
    a_hex: str
    b_hex: str
    a_sim: str
    b_sim: str
    distance: float
    required: float
    usage: str

    @property
    def passed(self) -> bool:
        """True when the distance, rounded to the three places the docs show, reaches the minimum."""
        return round(self.distance, 3) >= self.required


def check_pairs(
    theme: str,
    colors: Mapping[str, str],
    pairs: Iterable[tuple[str, str, str]],
    deficiencies: Iterable[str],
    minimum: float,
) -> list[CvdRow]:
    """Distance rows for every pair under every deficiency.

    Args:
        theme: theme name written into the rows.
        colors: role -> `#rrggbb` for every role the pairs name.
        pairs: `(role_a, role_b, usage)` triples.
        deficiencies: names from `DEFICIENCIES`.
        minimum: required distance (`cvd_min_delta_e_ok`).

    Returns:
        Rows ordered by deficiency, then pair.

    Raises:
        ValueError: unknown deficiency.
        KeyError: a role missing from `colors`.
    """
    pairs = list(pairs)
    rows: list[CvdRow] = []
    for deficiency in deficiencies:
        _matrix(deficiency)
        for a, b, usage in pairs:
            rows.append(
                CvdRow(
                    theme=theme,
                    deficiency=deficiency,
                    a=a,
                    b=b,
                    a_hex=colors[a],
                    b_hex=colors[b],
                    a_sim=simulate_hex(colors[a], deficiency),
                    b_sim=simulate_hex(colors[b], deficiency),
                    distance=distance(colors[a], colors[b], deficiency),
                    required=minimum,
                    usage=usage,
                )
            )
    return rows
