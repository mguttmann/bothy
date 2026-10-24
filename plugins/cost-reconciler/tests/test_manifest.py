# SPDX-License-Identifier: Apache-2.0
"""The manifest asks for exactly the scope the app needs and nothing else."""

from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def test_manifest_has_only_the_reconcile_scope() -> None:
    manifest = json.loads((ROOT / "manifest.json").read_text(encoding="utf-8"))
    assert manifest["schema_version"] == 1
    assert manifest["slug"] == "cost-reconciler"
    assert manifest["scopes"] == {"required": ["costs:reconcile"], "optional": []}
    assert manifest["version"] == (ROOT / "VERSION").read_text(encoding="utf-8").strip()
    for absent in (
        "events",
        "delivery",
        "commands",
        "components",
        "requested_grants",
        "incoming_webhooks",
        "data_egress",
        "homepage_url",
    ):
        assert absent not in manifest
    assert set(manifest) == {
        "schema_version",
        "slug",
        "name",
        "description",
        "publisher",
        "version",
        "min_api_version",
        "scopes",
    }
    for text in ("name", "description", "publisher"):
        assert manifest[text].strip()


def test_package_version_equals_version_file() -> None:
    import tomllib

    pyproject = tomllib.loads((ROOT / "pyproject.toml").read_text(encoding="utf-8"))
    version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
    assert pyproject["project"]["version"] == version
    from cost_reconciler import __version__

    assert __version__ == version


def test_min_api_version_is_not_above_the_contract() -> None:
    # The server refuses a manifest whose min_api_version (major.minor) is above the version
    # of the API it implements, info.version of docs/api/openapi.yaml.
    import re

    contract = (ROOT.parents[1] / "docs" / "api" / "openapi.yaml").read_text(encoding="utf-8")
    found = re.search(r"^  version: (\d+)\.(\d+)\.\d+", contract, re.MULTILINE)
    assert found
    manifest = json.loads((ROOT / "manifest.json").read_text(encoding="utf-8"))
    need = re.fullmatch(r"(\d+)\.(\d+)", manifest["min_api_version"])
    assert need
    have = (int(found.group(1)), int(found.group(2)))
    assert (int(need.group(1)), int(need.group(2))) <= have
