# SPDX-License-Identifier: Apache-2.0
"""Wiring of `make ops-check`: Makefile, Renovate and the image versions of the Compose stack.

Renovate itself cannot run offline. The regex managers of renovate.json are applied here with
Python's `re` (the patterns use only syntax both engines share, after renaming the named
groups), so a change of the files or the patterns that loses an image shows up in the tests.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
from pathlib import Path
from typing import Any

import pytest

REPO = Path(__file__).resolve().parents[3]
ENV_EXAMPLE = REPO / "infra" / "compose" / ".env.example"
COMPOSE = REPO / "infra" / "compose" / "compose.yaml"
VALIDATORS = ["otelcol-contrib", "promtool", "amtool", "loki", "tempo"]
_COMMENT = re.compile(r"^# renovate: datasource=docker depName=(\S+)$")
_VERSION_LINE = re.compile(r"^(BOTHY_[A-Z0-9_]+_VERSION)=(\S*)$")
_IMAGE_DEFAULT = re.compile(r"image: (\S+):\$\{(BOTHY_[A-Z0-9_]+_VERSION):-([^}]+)\}")
# make must not inherit the flags of an outer `make test`.
_DROPPED = {"MAKEFLAGS", "MAKELEVEL", "MFLAGS"}


def make(*args: str) -> subprocess.CompletedProcess[str]:
    env = {k: v for k, v in os.environ.items() if k not in _DROPPED}
    return subprocess.run(
        ["make", *args], cwd=REPO, env=env, capture_output=True, text=True, check=False
    )


def renovate() -> dict[str, Any]:
    data: dict[str, Any] = json.loads((REPO / "renovate.json").read_text(encoding="utf-8"))
    return data


def manager_for(path: str) -> dict[str, Any]:
    """The one custom regex manager whose file pattern matches `path`."""
    found = [
        m
        for m in renovate()["customManagers"]
        if any(re.search(p.strip("/"), path) for p in m["managerFilePatterns"])
    ]
    assert len(found) == 1, path
    return found[0]


def extract(manager: dict[str, Any], text: str) -> set[tuple[str, str, str]]:
    """(datasource, depName, currentValue) of every match, as the regex manager reads them."""
    deps = set()
    for pattern in manager["matchStrings"]:
        for match in re.finditer(pattern.replace("(?<", "(?P<"), text):
            groups = match.groupdict()
            datasource = groups.get("datasource") or manager.get("datasourceTemplate", "")
            deps.add((datasource, groups["depName"], groups["currentValue"]))
    return deps


def compose_images() -> dict[str, tuple[str, str]]:
    """Variable -> (image, default version) of every `${BOTHY_<X>_VERSION:-...}` image."""
    text = COMPOSE.read_text(encoding="utf-8")
    return {var: (image, default) for image, var, default in _IMAGE_DEFAULT.findall(text)}


# ---- Makefile -----------------------------------------------------------------------------


def test_ops_check_is_a_gate_after_api_lint() -> None:
    database = make("-pq", "gates").stdout  # -q: question mode, no recipe runs
    [line] = [x for x in database.splitlines() if x.startswith("gates:")]
    gates = line.split(":", 1)[1].split()
    assert "ops-check" in gates
    assert gates.index("ops-check") > gates.index("api-lint")
    [phony] = [x for x in database.splitlines() if x.startswith(".PHONY:")]
    assert "ops-check" in phony.split()


def test_ops_check_runs_the_tool() -> None:
    result = make("-n", "ops-check")
    assert result.returncode == 0, result.stderr
    assert result.stdout.strip().endswith("python tools/opscheck/opscheck.py")
    assert "## Operations configuration valid with the pinned upstream tools" in (
        REPO / "Makefile"
    ).read_text(encoding="utf-8")


def test_bandit_scans_opscheck() -> None:
    lines = make("-n", "lint").stdout.splitlines()
    [bandit] = [x for x in lines if " bandit -q -c tools/pyproject.toml " in x]
    assert "tools/opscheck" in bandit.split()


# ---- .env.example and compose.yaml --------------------------------------------------------


def test_every_image_version_has_exactly_one_renovate_comment() -> None:
    """Each image variable has the comment directly above it, naming the image of compose.yaml.

    This covers the section "Images der Hilfsdienste" and every other image variable whose
    default compose.yaml reads (Tailscale), so both regex managers see the same images.
    """
    lines = ENV_EXAMPLE.read_text(encoding="utf-8").splitlines()
    images = compose_images()
    start = next(i for i, x in enumerate(lines) if x.startswith("# --- Images der Hilfsdienste"))
    section = []
    for line in lines[start + 1 :]:
        if line.startswith("# ---"):
            break
        if match := _VERSION_LINE.match(line):
            section.append(match.group(1))
    assert len(section) == 8
    assert set(section) <= set(images)
    for index, line in enumerate(lines):
        match = _VERSION_LINE.match(line)
        if match is not None and match.group(1) in images:
            above = _COMMENT.match(lines[index - 1])
            assert above is not None, f"{match.group(1)}: no renovate comment directly above"
            assert above.group(1) == images[match.group(1)][0]
    comments = [i for i, x in enumerate(lines) if _COMMENT.match(x)]
    below = [_VERSION_LINE.match(lines[i + 1]) for i in comments]
    assert all(m is not None and m.group(1) in images for m in below)
    assert len(comments) == len(images)


def test_env_example_and_compose_defaults_agree() -> None:
    text = ENV_EXAMPLE.read_text(encoding="utf-8")
    env = dict(m.groups() for x in text.splitlines() if (m := _VERSION_LINE.match(x)))
    for variable, (_, default) in compose_images().items():
        assert env.get(variable) == default, variable


# ---- renovate.json ------------------------------------------------------------------------


def test_regex_managers_extract_the_same_images_from_both_files() -> None:
    env_text = ENV_EXAMPLE.read_text(encoding="utf-8")
    compose_text = COMPOSE.read_text(encoding="utf-8")
    env_deps = extract(manager_for("infra/compose/.env.example"), env_text)
    compose_deps = extract(manager_for("infra/compose/compose.yaml"), compose_text)
    expected = {("docker", image, version) for image, version in compose_images().values()}
    assert env_deps == expected
    assert compose_deps == expected
    assert ("docker", "otel/opentelemetry-collector-contrib", "0.162.0") in expected


def test_compose_images_are_one_group_with_the_validator_note() -> None:
    rules = [
        r
        for r in renovate()["packageRules"]
        if set(r.get("matchFileNames", []))
        == {"infra/compose/.env.example", "infra/compose/compose.yaml"}
    ]
    assert len(rules) == 1
    [rule] = rules
    assert rule["groupName"] == "Compose-Images"
    notes = " ".join(rule["prBodyNotes"])
    assert "make tools-lock" in notes
    for validator in VALIDATORS:
        assert validator in notes


def test_validator_pins_are_off_for_renovate() -> None:
    [rule] = [r for r in renovate()["packageRules"] if r.get("matchDepNames") == VALIDATORS]
    assert rule["enabled"] is False
    assert rule["matchFileNames"] == ["tools/bin/tools.lock.json"]
    assert "make tools-lock" in rule["description"]


@pytest.mark.parametrize("tool", ["sloth", "tofu"])
def test_other_validators_stay_with_the_release_binaries(tool: str) -> None:
    disabled = [r for r in renovate()["packageRules"] if r.get("enabled") is False]
    assert all(tool not in r.get("matchDepNames", []) for r in disabled)
    lock = (REPO / "tools" / "bin" / "tools.lock.json").read_text(encoding="utf-8")
    version = json.loads(lock)["tools"][tool]["version"]
    deps = extract(manager_for("tools/bin/tools.lock.json"), lock)
    assert ("github-releases", tool, version) in deps


def test_renovate_description_names_the_tag_based_compose_images() -> None:
    """The file summary must not promise digest pins the Compose-Images rule turns off."""
    description = renovate()["description"]
    assert "Compose-Images" in description
    assert "version tags" in description
