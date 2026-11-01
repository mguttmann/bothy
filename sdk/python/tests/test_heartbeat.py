# SPDX-License-Identifier: Apache-2.0
"""The heartbeat building block for `agent_heartbeat.v1` (docs/spec/09-agents.md 4.8).

The server's decoder (M2-20) refuses an unknown key and a value outside its form; the SDK
refuses the same before anything is sent, and its error names the field, never the value.
"""

from __future__ import annotations

import re
from collections.abc import Callable
from datetime import datetime, timedelta, timezone
from functools import cache
from pathlib import Path
from typing import Any

import pytest
import yaml

from bothy_sdk import heartbeat
from bothy_sdk.generated.models import (
    AgentRateLimitState,
    BillingEvidence,
    Heartbeat,
    ProjectReadiness,
    ProjectTrashCheckReport,
    ProjectTrashResult,
    ProjectTrashResultResult,
)
from bothy_sdk.heartbeat import build_heartbeat
from bothy_sdk.ids import uuid7

RUN_A, RUN_B, CHANNEL = uuid7(), uuid7(), uuid7()

FULL_DETAILS: dict[str, Any] = {
    "capabilities": {
        "followups": "queue",
        "components": True,
        "voice_input": False,
        "title": True,
        "files_out": False,
    },
    "active_runs": [RUN_A, str(RUN_B)],
    "degraded_reasons": ["runtime_unreachable", "billing_evidence_unknown"],
    "adapter": {"name": "bothy-sdk-python", "version": "0.1.0+local"},
    "runtime": {"name": "example-runtime"},
    "protocol_version": 1,
}


def repo_root() -> Path:
    """The repository root: the nearest parent directory that holds docs/api."""
    for parent in Path(__file__).resolve().parents:
        if (parent / "docs" / "api").is_dir():
            return parent
    raise AssertionError("repository root with docs/api not found")


@cache
def openapi() -> dict[str, Any]:
    """docs/api/openapi.yaml of the repository."""
    with (repo_root() / "docs" / "api" / "openapi.yaml").open(encoding="utf-8") as fh:
        loaded: dict[str, Any] = yaml.safe_load(fh)
    return loaded


def details_with(path: str, value: object) -> dict[str, Any]:
    """FULL_DETAILS with `value` at the dotted `path` below details."""
    details: dict[str, Any] = {
        k: (dict(v) if isinstance(v, dict) else v) for k, v in FULL_DETAILS.items()
    }
    *parents, last = path.split(".")
    target = details
    for key in parents:
        target = target[key]
    target[last] = value
    return details


def raises_naming(field: str, call: Callable[[], object], value: object = None) -> None:
    """`call` raises ValueError whose text names `field` and never shows `value`."""
    with pytest.raises(ValueError, match=re.escape(field)) as caught:
        call()
    if isinstance(value, str) and value:
        assert value not in str(caught.value)
        assert value not in repr(caught.value.args)


def test_closed_details_keys() -> None:
    beat = build_heartbeat(
        "degraded",
        runtime_version="2.3.1",
        reported_billing_mode="organization",
        billing_modes_supported=["organization", "personal"],
        billing_evidence={"provider": "anthropic", "key_type": "api_key", "method": "env"},
        reported_tool_class="scoped",
        details=FULL_DETAILS,
        project_readiness=[ProjectReadiness(channel_id=CHANNEL, ready=True)],
    )

    assert isinstance(beat, Heartbeat)
    body = beat.to_dict()
    assert body["details"]["active_runs"] == [str(RUN_A), str(RUN_B)]
    assert body["details"]["capabilities"] == FULL_DETAILS["capabilities"]
    assert body["billing_evidence"] == {
        "provider": "anthropic",
        "key_type": "api_key",
        "method": "env",
    }
    assert body["state"] == "degraded"
    assert body["project_readiness"] == [{"channel_id": str(CHANNEL), "ready": True}]
    assert Heartbeat.from_dict(body).to_dict() == body
    assert build_heartbeat().to_dict() == {"state": "online"}
    assert build_heartbeat(
        billing_evidence=BillingEvidence.from_dict({"key_type": "unknown"})
    ).to_dict() == {"state": "online", "billing_evidence": {"key_type": "unknown"}}

    for path in (
        "rate_limit",
        "billing_mode",
        "capabilities.streaming",
        "adapter.path",
        "runtime.version",
    ):
        raises_naming(
            f"details.{path}",
            lambda p=path: build_heartbeat(details=details_with(p, "x")),
        )
    raises_naming(
        "billing_evidence.key_value",
        lambda: build_heartbeat(billing_evidence={"key_type": "api_key", "key_value": "secret"}),
    )
    # An unknown key that is not even a name is not echoed either.
    raises_naming(
        "details", lambda: build_heartbeat(details={"/home/alex/.token": "x"}), "/home/alex/.token"
    )
    assert set(heartbeat.DETAILS_KEYS) == {
        "capabilities",
        "active_runs",
        "degraded_reasons",
        "adapter",
        "runtime",
        "protocol_version",
    }
    assert set(heartbeat.CAPABILITY_KEYS) == {
        "followups",
        "components",
        "voice_input",
        "title",
        "files_out",
    }

    # Required members, types, vocabularies and duplicates.
    raises_naming(
        "details.adapter.name", lambda: build_heartbeat(details={"adapter": {"version": "1.0"}})
    )
    raises_naming("details.runtime.name", lambda: build_heartbeat(details={"runtime": {}}))
    raises_naming(
        "details.capabilities.followups",
        lambda: build_heartbeat(details={"capabilities": {"followups": "interrupt"}}),
    )
    raises_naming(
        "details.capabilities.components",
        lambda: build_heartbeat(details={"capabilities": {"components": 1}}),
    )
    for bad_version in (0, -1, True, "1", 1.0, 2**31):
        raises_naming(
            "details.protocol_version",
            lambda v=bad_version: build_heartbeat(details={"protocol_version": v}),
        )
    raises_naming(
        "details.active_runs", lambda: build_heartbeat(details={"active_runs": [RUN_A, str(RUN_A)]})
    )
    raises_naming(
        "details.active_runs",
        lambda: build_heartbeat(details={"active_runs": ["not-a-uuid"]}),
        "not-a-uuid",
    )
    raises_naming(
        "details.degraded_reasons",
        lambda: build_heartbeat(
            details={"degraded_reasons": ["busy_mode_invalid", "busy_mode_invalid"]}
        ),
    )
    raises_naming(
        "billing_modes_supported",
        lambda: build_heartbeat(billing_modes_supported=["personal", "personal"]),
    )
    raises_naming(
        "project_readiness",
        lambda: build_heartbeat(
            project_readiness=[
                ProjectReadiness(channel_id=CHANNEL, ready=True),
                ProjectReadiness(channel_id=CHANNEL, ready=False),
            ]
        ),
    )
    raises_naming("state", lambda: build_heartbeat("offline"))
    raises_naming(
        "reported_billing_mode", lambda: build_heartbeat(reported_billing_mode="corporate_example")
    )
    raises_naming("reported_tool_class", lambda: build_heartbeat(reported_tool_class="root"))
    raises_naming(
        "billing_evidence.key_type",
        lambda: build_heartbeat(billing_evidence={"provider": "anthropic"}),
    )
    raises_naming(
        "billing_evidence.key_type",
        lambda: build_heartbeat(billing_evidence={"key_type": "prefix_example"}),
    )


# Values a heartbeat must never carry: paths, secrets, content, whitespace, control and bidi
# characters, and the Python `$` pitfall (a trailing newline).
BAD_VALUES = [
    "/opt/agents/bin/runtime",
    "..\\runtime",
    "C:\\agents\\runtime",
    "sk-ant-api03-AbCd/Ef+Gh==",
    "Bearer abc.def",
    "token=abcdef",
    "two words",
    "runtime\n",
    "run\ttime",
    "r\u00e9ntime",
    "runtime\u202e",
    "",
    "-runtime",
]

FIELDS: dict[str, Callable[[str], object]] = {
    "runtime_version": lambda v: build_heartbeat(runtime_version=v),
    "billing_evidence.provider": lambda v: build_heartbeat(
        billing_evidence={"key_type": "api_key", "provider": v}
    ),
    "billing_evidence.method": lambda v: build_heartbeat(
        billing_evidence={"key_type": "api_key", "method": v}
    ),
    "details.degraded_reasons": lambda v: build_heartbeat(details={"degraded_reasons": [v]}),
    "details.adapter.name": lambda v: build_heartbeat(details={"adapter": {"name": v}}),
    "details.adapter.version": lambda v: build_heartbeat(
        details={"adapter": {"name": "a", "version": v}}
    ),
    "details.runtime.name": lambda v: build_heartbeat(details={"runtime": {"name": v}}),
}


@pytest.mark.parametrize("field", sorted(FIELDS))
@pytest.mark.parametrize("value", BAD_VALUES)
def test_forms_reject_paths_and_secrets(field: str, value: str) -> None:
    raises_naming(field, lambda: FIELDS[field](value), value)


@pytest.mark.parametrize("field", sorted(FIELDS))
def test_forms_reject_non_strings(field: str) -> None:
    raises_naming(field, lambda: FIELDS[field](42))


def test_heartbeat_forms_match_spec() -> None:
    schemas = openapi()["components"]["schemas"]
    beat, evidence = schemas["Heartbeat"]["properties"], schemas["BillingEvidence"]["properties"]

    assert beat["runtime_version"]["pattern"] == heartbeat.VERSION_FORM
    assert evidence["provider"]["pattern"] == heartbeat.CODE_FORM
    assert evidence["method"]["pattern"] == heartbeat.CODE_FORM
    # The name form and the details forms exist only in the description of `details`.
    described = beat["details"]["description"]
    forms = dict(re.findall(r"(Names|versions|codes)(?: match)? `([^`]+)`", described))
    assert forms == {
        "Names": heartbeat.NAME_FORM,
        "versions": heartbeat.VERSION_FORM,
        "codes": heartbeat.CODE_FORM,
    }
    # The closed key set of `details` as the description lists it.
    listed = re.search(r"Keys: (.*?)\. Names", described)
    assert listed is not None
    top = re.findall(r"(?:^|\), )`([a-z_]+)` \(", listed.group(1))
    assert set(top) == set(heartbeat.DETAILS_KEYS)
    # The top-level members of Heartbeat that the builder offers: all of them (M6-14).
    assert set(beat) == set(heartbeat.HEARTBEAT_MEMBERS)


def test_rate_limit() -> None:
    """``rate_limit`` carries ``state`` and an aware ``resets_at`` in UTC, nothing else (M6-14)."""
    berlin = timezone(timedelta(hours=2))
    body = build_heartbeat(
        rate_limit={"state": "limited", "resets_at": datetime(2026, 10, 10, 11, 30, tzinfo=berlin)}
    ).to_dict()
    assert body == {
        "state": "online",
        "rate_limit": {"state": "limited", "resets_at": "2026-10-10T09:30:00+00:00"},
    }
    assert build_heartbeat(rate_limit={"state": "ok"}).to_dict()["rate_limit"] == {"state": "ok"}
    model = AgentRateLimitState.from_dict({"state": "near_limit"})
    assert build_heartbeat(rate_limit=model).to_dict()["rate_limit"] == {"state": "near_limit"}
    raises_naming("rate_limit.state", lambda: build_heartbeat(rate_limit={"state": "exhausted"}))
    raises_naming("rate_limit.state", lambda: build_heartbeat(rate_limit={}))
    raises_naming(
        "rate_limit.quota", lambda: build_heartbeat(rate_limit={"state": "ok", "quota": 5})
    )
    raises_naming(
        "rate_limit.resets_at",
        lambda: build_heartbeat(rate_limit={"state": "ok", "resets_at": datetime(2026, 10, 10)}),
    )
    raises_naming(
        "rate_limit.resets_at",
        lambda: build_heartbeat(rate_limit={"state": "ok", "resets_at": "2026-10-10T09:30:00Z"}),
    )


def test_project_folders() -> None:
    """``project_folders`` carries distinct one-segment folder names (09 18.13, M4-44)."""
    beat = build_heartbeat(project_folders=["docs-site", "website"])
    assert beat.to_dict() == {"state": "online", "project_folders": ["docs-site", "website"]}
    assert build_heartbeat(project_folders=[]).to_dict() == {
        "state": "online",
        "project_folders": [],
    }
    assert "project_folders" not in build_heartbeat().to_dict()
    raises_naming("project_folders", lambda: build_heartbeat(project_folders=["a", "a"]))
    for bad in (".git", "a/b", "-x", "", "my project", "x" * 256):
        raises_naming("project_folders", lambda bad=bad: build_heartbeat(project_folders=[bad]))


def test_project_trash_results() -> None:
    """``project_trash_results`` carries one outcome per forum with a code (09 18.15)."""
    other = uuid7()
    beat = build_heartbeat(
        project_trash_results=[
            ProjectTrashResult(channel_id=CHANNEL, result=ProjectTrashResultResult.TRASHED),
            ProjectTrashResult(
                channel_id=other, result=ProjectTrashResultResult.REFUSED, reason="symlink"
            ),
        ]
    )
    assert beat.to_dict()["project_trash_results"] == [
        {"channel_id": str(CHANNEL), "result": "trashed"},
        {"channel_id": str(other), "result": "refused", "reason": "symlink"},
    ]
    assert "project_trash_results" not in build_heartbeat().to_dict()
    done = ProjectTrashResult(channel_id=CHANNEL, result=ProjectTrashResultResult.TRASHED)
    raises_naming(
        "project_trash_results", lambda: build_heartbeat(project_trash_results=[done, done])
    )
    raises_naming(
        "project_trash_results",
        lambda: build_heartbeat(project_trash_results=[{"channel_id": str(CHANNEL)}]),  # type: ignore[list-item]
    )
    for bad in ("/home/alex/website", "Symlink", "x" * 65, ""):
        refused = ProjectTrashResult(
            channel_id=CHANNEL, result=ProjectTrashResultResult.REFUSED, reason=bad
        )
        raises_naming(
            "project_trash_results.reason",
            lambda refused=refused: build_heartbeat(project_trash_results=[refused]),
            bad,
        )


def test_project_readiness_trash_check() -> None:
    """A readiness entry carries the folder check; its reason is a code (09 18.15)."""
    entry = ProjectReadiness(
        channel_id=CHANNEL,
        ready=True,
        trash_check=ProjectTrashCheckReport(ok=False, reason="uncommitted_changes"),
    )
    assert build_heartbeat(project_readiness=[entry]).to_dict()["project_readiness"] == [
        {
            "channel_id": str(CHANNEL),
            "ready": True,
            "trash_check": {"ok": False, "reason": "uncommitted_changes"},
        }
    ]
    bad = "/srv/projects/website"
    entry = ProjectReadiness(
        channel_id=CHANNEL, ready=True, trash_check=ProjectTrashCheckReport(ok=False, reason=bad)
    )
    raises_naming(
        "project_readiness.trash_check.reason",
        lambda: build_heartbeat(project_readiness=[entry]),
        bad,
    )
