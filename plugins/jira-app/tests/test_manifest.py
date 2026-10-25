# SPDX-License-Identifier: Apache-2.0
"""The manifest of the app (brief M2-50 Design note 17; docs/spec/11-extensions.md 3.1).

Proves the members of Design note 17, the version against ``VERSION`` and ``pyproject.toml``,
``min_api_version`` against ``info.version`` of ``docs/api/openapi.yaml``, the command ``issue``,
both addresses on ``https`` and one host, no ``homepage_url`` and no member outside the field
table of 11 3.1 (parity with the manifest test of the GitHub app, M2-49).
"""

from __future__ import annotations

import json
import re
import tomllib
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

APP = Path(__file__).resolve().parents[1]
REPO = APP.parents[1]


def manifest() -> dict[str, Any]:
    """The parsed ``manifest.json`` of the app."""
    data: dict[str, Any] = json.loads((APP / "manifest.json").read_text(encoding="utf-8"))
    return data


def repo_text(relative: str) -> str:
    """A file of the repository as text."""
    return (REPO / relative).read_text(encoding="utf-8")


def field_table() -> list[str]:
    """The top-level member names of the field table of 11 3.1."""
    chapter = repo_text("docs/spec/11-extensions.md")
    section = chapter.split("### 3.1 Format", 1)[1].split("### 3.2 ", 1)[0]
    names: list[str] = []
    for line in section.splitlines():
        if not line.startswith("| `"):
            continue
        cell = line.split("|")[1]
        names.extend(m.split(".")[0] for m in re.findall(r"`([a-z_.]+)`", cell))
    return list(dict.fromkeys(names))


def openapi_major_minor() -> tuple[int, int]:
    """Major and minor of the first ``version:`` in the block below the top-level ``info:``."""
    lines = repo_text("docs/api/openapi.yaml").splitlines()
    start = lines.index("info:")
    version = ""
    for line in lines[start + 1 :]:
        if not line.startswith(" "):
            break
        if line.strip().startswith("version:"):
            version = line.strip().removeprefix("version:").strip().strip("'\"")
            break
    match = re.fullmatch(r"(\d+)\.(\d+)(?:\.\d+.*)?", version)
    assert match is not None, version
    return int(match.group(1)), int(match.group(2))


def description_max_chars() -> int:
    """``command.description_max_chars`` from the limit table of 09-agents.md."""
    text = repo_text("docs/spec/09-agents.md")
    match = re.search(r"^\| `command\.description_max_chars` \| [^|]+ \| `(\d+)` \|", text, re.M)
    assert match is not None
    return int(match.group(1))


def test_members_of_design_note_17() -> None:
    m = manifest()
    assert m["schema_version"] == 1
    assert m["slug"] == "jira"
    assert m["name"] == "Jira"
    assert m["publisher"] == "Reference integration"
    assert m["scopes"] == {
        "required": ["messages:write", "commands:write", "components:write"],
        "optional": ["messages:read", "events:subscribe"],
    }
    assert m["events"] == ["message.created"]
    assert m["components"] == ["form"]
    grants = m["requested_grants"]
    assert len(grants) == 1
    assert set(grants[0]) == {"target_kind", "level", "capabilities", "purpose"}
    assert grants[0]["target_kind"] == "channel"
    assert grants[0]["level"] == "write"
    assert grants[0]["capabilities"] == []
    assert isinstance(grants[0]["purpose"], str)
    assert grants[0]["purpose"].strip()
    assert isinstance(m["data_egress"], str)
    assert m["data_egress"].strip()
    assert "homepage_url" not in m


def test_description_is_one_english_sentence() -> None:
    description = manifest()["description"]
    assert isinstance(description, str)
    assert description.isascii()
    assert description.endswith(".")
    # One sentence: no sentence end before the last character.
    assert re.search(r"[.!?]\s", description) is None
    assert description[0].isupper()


def test_data_egress_names_what_leaves_and_where() -> None:
    egress = manifest()["data_egress"]
    assert egress.isascii()
    for fact in ("forum", "/issue", "messages:read", "events:subscribe", "display name"):
        assert fact in egress


def test_version_equals_version_file_and_pyproject() -> None:
    version = (APP / "VERSION").read_text(encoding="utf-8").strip()
    pyproject = tomllib.loads((APP / "pyproject.toml").read_text(encoding="utf-8"))
    assert manifest()["version"] == version
    assert pyproject["project"]["version"] == version


def test_needs_no_newer_api_than_openapi() -> None:
    major, minor = openapi_major_minor()
    want = manifest()["min_api_version"]
    assert isinstance(want, str)
    match = re.fullmatch(r"(\d+)\.(\d+)", want)
    assert match is not None, want
    assert (int(match.group(1)), int(match.group(2))) <= (major, minor)
    # Design note 17: equal to major and minor of info.version.
    assert want == f"{major}.{minor}"


def test_declares_issue_with_option_title_and_required_level_trigger() -> None:
    commands = manifest()["commands"]
    assert commands == [
        {
            "name": "issue",
            "description": "Create a Jira issue",
            "options": [
                {
                    "name": "title",
                    "description": "Issue title",
                    "type": "string",
                    "required": True,
                }
            ],
            "required_level": "trigger",
        }
    ]
    limit = description_max_chars()
    assert len(commands[0]["description"]) <= limit
    assert len(commands[0]["options"][0]["description"]) <= limit


def test_both_addresses_on_https_and_one_host() -> None:
    delivery = manifest()["delivery"]
    assert set(delivery) == {"mode", "events_url", "interactions_url"}
    assert delivery["mode"] == "webhook"
    events = urlsplit(delivery["events_url"])
    interactions = urlsplit(delivery["interactions_url"])
    assert events.scheme == "https"
    assert interactions.scheme == "https"
    assert events.netloc == interactions.netloc
    assert events.hostname == "jira-app.example.com"
    assert events.path == "/chat/events"
    assert interactions.path == "/chat/interactions"


def test_no_member_outside_the_field_table_of_11_3_1() -> None:
    allowed = field_table()
    assert "data_egress" in allowed
    assert "homepage_url" in allowed
    for key in manifest():
        assert key in allowed, key
