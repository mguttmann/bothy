# SPDX-License-Identifier: Apache-2.0
"""The table of third-party services in docs/quality/licenses.md names every image of
infra/compose/compose.yaml that the project does not build, with the Compose profile that
starts it (AUD-PKG-P10-repo-ops-host). No network."""

from __future__ import annotations

import re
from pathlib import Path

import yaml

REPO = Path(__file__).resolve().parents[3]
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
LICENSES = REPO / "docs" / "quality" / "licenses.md"
SECTION = "## Third-party services of the installation"
_ROW = re.compile(r"^\| `(?P<image>[^`]+)` \| (?P<profile>[^|]+?) \| `(?P<license>[^`]+)` \|")


def compose_images() -> dict[str, str]:
    """Image repository (without tag) to its profile, for every image not built by the project."""
    services = yaml.safe_load(COMPOSE.read_text(encoding="utf-8"))["services"]
    out: dict[str, str] = {}
    for svc in services.values():
        image = svc.get("image", "")
        if not image or image.startswith("${BOTHY_REGISTRY"):
            continue
        profiles = svc.get("profiles") or []
        out[image.split(":", 1)[0]] = ", ".join(f"`{p}`" for p in profiles) or "none"
    return out


def table_rows() -> dict[str, tuple[str, str]]:
    text = LICENSES.read_text(encoding="utf-8")
    assert SECTION in text, f"{LICENSES.name} has no section {SECTION!r}"
    body = text.split(SECTION, 1)[1].split("\n## ", 1)[0]
    rows = {}
    for line in body.splitlines():
        m = _ROW.match(line)
        if m:
            rows[m.group("image")] = (m.group("profile").strip(), m.group("license"))
    return rows


def test_every_third_party_image_has_a_row_with_its_profile() -> None:
    images = compose_images()
    rows = table_rows()
    assert images, "compose.yaml has no third-party image"
    assert sorted(rows) == sorted(images)
    for image, profile in images.items():
        assert rows[image][0] == profile, image


def test_the_agpl_services_are_only_in_an_optional_profile() -> None:
    images = compose_images()
    for image, (_, spdx) in table_rows().items():
        if spdx.startswith(("AGPL", "GPL")):
            assert images[image] != "none", f"{image} ({spdx}) starts without a profile"
