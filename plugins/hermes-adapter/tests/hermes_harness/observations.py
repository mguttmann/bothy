# SPDX-License-Identifier: Apache-2.0
"""Facts and verdicts of the six points of docs/spec/09-agenten.md 17.10.

:func:`observe` computes, from the raw recordings of the scenarios, the facts of each point
and whether its claim holds, so no verdict is written by hand. Facts hold only booleans,
counts of exact events, enumerations, surface names and placeholders: no credential value,
no ID of the run and no third-party prose. :func:`surface_facts`, :func:`egress_facts`,
:func:`store_call_facts` and :func:`discovery_facts` describe the run as a whole.

Example:
    >>> points = observe(results, scenarios)
    >>> points["steer"]["holds"]
    False
"""

from __future__ import annotations

import itertools
import re
from collections.abc import Callable, Mapping, Sequence
from typing import Any, Final

from hermes_harness.calls import digest
from hermes_harness.scenarios import PLUGIN_PLACE, SURFACES, Scenario, ScenarioResult

__all__ = ["POINT_SCENARIOS", "SLUGS", "SURFACES", "observe"]

SLUGS: Final = (
    "progress_hooks",
    "steer",
    "credential_type",
    "interrupt_outcome",
    "session_switch",
    "own_access_policy",
)
#: The scenarios of each point (Design note 9 of the brief, plus the extra evidence scenarios).
POINT_SCENARIOS: Final[Mapping[str, tuple[str, ...]]] = {
    "progress_hooks": ("tool_and_text", "tool_without_progress", "text_without_draft"),
    "steer": (
        "busy_steer",
        "busy_queue",
        "busy_interrupt",
        "busy_steer_global",
        "busy_queue_global",
        "busy_steer_command",
    ),
    "credential_type": (
        "credential_api_key_only",
        "credential_oauth_only",
        "credential_both",
        "credential_none",
        "credential_login_file_only",
        "credential_login_file_optout",
        "credential_other_provider",
        "credential_per_turn",
    ),
    "interrupt_outcome": ("interrupt", "interrupt_command"),
    "session_switch": (
        "session_new",
        "session_resume",
        "session_resume_admin",
        "session_resume_same_thread",
        "session_resume_missing",
    ),
    "own_access_policy": (
        "access_policy_own",
        "access_policy_default",
        "access_policy_upstream",
    ),
}
SESSION_ID_SURFACE: Final = "ctx.register_hook('pre_llm_call') keyword session_id"
PER_PLATFORM_BUSY_KEY: Final = "display.platforms.<platform>.busy_input_mode"
GLOBAL_BUSY_KEY: Final = "display.busy_input_mode"
SEEDED_EXTRA: Final = "seeded extra key"
_IDENTIFIER: Final = re.compile(r"[A-Za-z0-9_.:-]{1,64}")
_INTERRUPT_WAYS: Final = (
    ("interrupt", "BasePlatformAdapter.cancel_session_processing(session_key)"),
    ("interrupt_command", "/stop through handle_message"),
)
_EXPECTED_BUSY: Final = {"steer": "steered", "queue": "new_turn", "interrupt": "interrupted"}
_SOURCE_LABELS: Final = {
    "env": "process environment",
    "dotenv": ".env of the Hermes home",
    "login_file": "login file of another tool (~/.claude/.credentials.json)",
    "config_api_key": "config.yaml model.api_key",
}

Results = Mapping[str, ScenarioResult]
Scenarios = Mapping[str, Scenario]


def _calls(result: ScenarioResult, method: str) -> list[dict[str, Any]]:
    return [c for c in result.calls if c["method"] == method]


def _scripted(result: ScenarioResult) -> list[dict[str, Any]]:
    return [r for r in result.model_requests if r.get("scripted")]


def _strip(text: Any) -> str:
    return str(text or "").replace("▉", "").strip()


def _event_message(call: Mapping[str, Any]) -> Any:
    return (call["args"].get("event") or {}).get("message_id")


def _outcome(call: Mapping[str, Any]) -> Any:
    outcome = call["args"].get("outcome")
    return outcome.get("value") if isinstance(outcome, Mapping) else outcome


def _label(value: Any) -> Any:
    """A label Hermes returned: kept when it is an identifier, else only its digest."""
    if value is None or (isinstance(value, str) and _IDENTIFIER.fullmatch(value)):
        return value
    return digest(str(value))


def _answer(scenario: Scenario) -> str:
    return scenario.texts.get("answer", "")


# ---- progress_hooks ----------------------------------------------------------------------
def _progress(results: Results, scenarios: Scenarios) -> dict[str, Any]:
    tat = results["tool_and_text"]
    twp = results["tool_without_progress"]
    twd = results["text_without_draft"]
    answer = _answer(scenarios["tool_and_text"])
    tool_events = _calls(tat, "format_tool_event")
    drafts = [_strip(c["args"].get("content")) for c in _calls(tat, "send_draft")]
    raw_drafts = [str(c["args"].get("content") or "") for c in _calls(tat, "send_draft")]
    statuses = [c["args"].get("text") for c in _calls(tat, "set_status_text")]
    answer_sends = [
        i
        for i, c in enumerate(tat.calls)
        if c["method"] == "send" and _strip(c["args"].get("content")) == answer
    ]
    draft_positions = [i for i, c in enumerate(tat.calls) if c["method"] == "send_draft"]

    def progress_send(result: ScenarioResult) -> bool:
        return any(
            c["args"].get("reply_to") is None and _strip(c["args"].get("content")) != answer
            for c in _calls(result, "send")
        )

    no_draft_answer = _answer(scenarios["text_without_draft"])
    edits = _calls(twd, "edit_message")
    final_edits = [e for e in edits if e["args"].get("finalize")]
    previews = [
        c for c in _calls(twd, "send") if (c["args"].get("metadata") or {}).get("expect_edits")
    ]
    facts: dict[str, Any] = {
        "format_tool_event": {
            "calls": len(tool_events),
            "event_fields": sorted(
                ((tool_events[0]["args"].get("event") or {}).get("fields") or {}).keys()
            )
            if tool_events
            else [],
        },
        "render_message_event": {"calls": len(_calls(tat, "render_message_event"))},
        "set_status_text": {
            "called": bool(statuses),
            "phrase_at_tool_start_then_cleared": len(statuses) >= 2
            and statuses[0] is not None
            and statuses[-1] is None,
        },
        "send_draft": {
            "called": bool(drafts),
            "cumulative": all(b.startswith(a) for a, b in itertools.pairwise(drafts)),
            "last_equals_answer": bool(drafts) and drafts[-1] == answer,
            "leading_whitespace_after_tool": bool(raw_drafts) and raw_drafts[0][:1].isspace(),
            "before_answer_send": bool(draft_positions and answer_sends)
            and max(draft_positions) < min(answer_sends),
        },
        "answer_with_draft": {
            "sent_with_send": bool(answer_sends),
            "edit_finalize_calls": sum(
                1 for e in _calls(tat, "edit_message") if e["args"].get("finalize")
            ),
        },
        "answer_without_draft": {
            "preview_send_with_cursor": bool(previews)
            and "▉" in str(previews[0]["args"].get("content")),
            "edit_finalize_calls": len(final_edits),
            "final_edit_is_last_and_equals_answer": bool(edits)
            and bool(edits[-1]["args"].get("finalize"))
            and _strip(edits[-1]["args"].get("content")) == no_draft_answer,
        },
        "progress_message_sent": {
            "default_display": progress_send(tat),
            "tool_progress_off": progress_send(twp),
        },
    }
    holds = (
        facts["format_tool_event"]["calls"] >= 2
        and not facts["progress_message_sent"]["default_display"]
        and facts["send_draft"]["called"]
        and facts["send_draft"]["last_equals_answer"]
        and facts["set_status_text"]["called"]
        and facts["answer_without_draft"]["edit_finalize_calls"] == 1
    )
    return {"facts": facts, "holds": bool(holds)}


# ---- steer -------------------------------------------------------------------------------
def _busy(result: ScenarioResult, scenario: Scenario) -> str:
    followup = scenario.texts.get("followup", "")
    if any(_event_message(c) == "m-2" for c in _calls(result, "on_processing_start")):
        return "new_turn"
    scripted = _scripted(result)
    later = [
        r
        for r in scripted
        if followup in (r.get("scenario_texts") or []) and (r.get("turn") or 0) > 1
    ]
    if not later:
        return "lost"
    # Steered: the held tool call of the first answer ran, so its result precedes the follow-up.
    # Interrupted: the first answer was dropped; the follow-up comes without a tool result.
    roles = list(later[0].get("roles") or [])
    return "steered" if "tool" in roles else "interrupted"


def _steer(results: Results, scenarios: Scenarios) -> dict[str, Any]:
    def b(name: str) -> str:
        return _busy(results[name], scenarios[name])

    per_platform = {mode: b(f"busy_{mode}") for mode in ("steer", "queue", "interrupt")}
    accepted = all(per_platform[m] == want for m, want in _EXPECTED_BUSY.items())
    facts = {
        "per_platform_key": PER_PLATFORM_BUSY_KEY,
        "per_platform_behaviour": per_platform,
        "per_platform_key_accepted": accepted,
        "global_key": GLOBAL_BUSY_KEY,
        "global_behaviour": {"steer": b("busy_steer_global"), "queue": b("busy_queue_global")},
        "steer_command_behaviour": b("busy_steer_command"),
    }
    return {"facts": facts, "holds": accepted and per_platform["steer"] == "steered"}


# ---- credential_type ---------------------------------------------------------------------
def _sources(scenario: Scenario) -> list[str]:
    found = []
    for key, label in _SOURCE_LABELS.items():
        value = scenario.credentials.get(key)
        if isinstance(value, Mapping):
            found += [f"{label}: {name}" for name in sorted(value)]
        elif value:
            found.append(label)
    return found


def _credential(results: Results, scenarios: Scenarios) -> dict[str, Any]:
    profiles: dict[str, Any] = {}
    for name in POINT_SCENARIOS["credential_type"]:
        result = results[name]
        truth = [str(r.get("credential")) for r in _scripted(result)] or ["none"]
        answers = [c["result"] for c in result.commands if c["command"] == "credential_type"]
        kinds = [str(a.get("kind")) for a in answers]
        profiles[name] = {
            "ground_truth": truth,
            "surface_answers": kinds,
            "surface": answers[0].get("surface") if answers else None,
            "public": bool(answers) and all(a.get("public") is True for a in answers),
            "resolver_source": [_label(a.get("source")) for a in answers],
            "sources": _sources(scenarios[name]),
            "matches": kinds == truth,
        }
    all_match = all(p["matches"] for p in profiles.values())
    public = all(p["public"] for p in profiles.values())
    facts = {
        "profiles": profiles,
        "all_profiles_match": all_match,
        "public_surface": public,
        "oauth_wins_over_api_key": profiles["credential_both"]["ground_truth"] == ["oauth"],
        "rechosen_per_turn": profiles["credential_per_turn"]["ground_truth"]
        == ["api_key", "oauth"],
        "login_file_of_another_tool_used": profiles["credential_login_file_only"]["ground_truth"]
        == ["oauth"],
        "login_file_opt_out_honoured": profiles["credential_login_file_optout"]["ground_truth"]
        == ["none"],
    }
    holds = public and all_match and facts["oauth_wins_over_api_key"]
    return {"facts": facts, "holds": bool(holds)}


# ---- interrupt_outcome -------------------------------------------------------------------
def _interrupt(results: Results, scenarios: Scenarios) -> dict[str, Any]:
    ways: dict[str, Any] = {}
    used = None
    for name, way in _INTERRUPT_WAYS:
        result = results[name]
        answer = _answer(scenarios[name])
        first = [c for c in _calls(result, "on_processing_complete") if _event_message(c) == "m-1"]
        scripted = _scripted(result)
        entry = {
            "outcome": _outcome(first[0]) if first else None,
            "outcome_type": first[0]["args"]["outcome"].get("enum")
            if first and isinstance(first[0]["args"].get("outcome"), Mapping)
            else None,
            "agent_continued": any((r.get("turn") or 0) > 1 for r in scripted),
            # The held first answer asked for a tool; it was used when the tool ran afterwards.
            "held_answer_used": bool(_calls(result, "set_status_text"))
            or any((r.get("turn") or 0) > 1 for r in scripted),
            "answer_sent": any(
                _strip(c["args"].get("content")) == answer for c in _calls(result, "send")
            ),
            "acknowledgement_sent": any(
                c["args"].get("reply_to") == "m-2" for c in _calls(result, "send")
            ),
            "interrupt_session_activity_called": bool(_calls(result, "interrupt_session_activity")),
        }
        ways[way] = entry
        if used is None and not entry["agent_continued"] and entry["outcome"] is not None:
            used = way
    facts = {"ways": ways, "way_used": used}
    holds = used is not None and ways[used]["outcome"] == "cancelled"
    return {"facts": facts, "holds": bool(holds)}


# ---- session_switch ----------------------------------------------------------------------
def _last_texts(result: ScenarioResult) -> list[str]:
    scripted = _scripted(result)
    return list(scripted[-1].get("scenario_texts") or []) if scripted else []


def _session(results: Results, scenarios: Scenarios) -> dict[str, Any]:
    def switch(name: str, target: str) -> dict[str, Any]:
        result = results[name]
        texts = scenarios[name].texts
        sessions = result.sessions
        return {
            "switched": bool(sessions.get("after"))
            and sessions.get("after") == sessions.get(target),
            "earlier_text_in_next_request": texts.get("first", "") in _last_texts(result),
        }

    new = results["session_new"]
    confirms = [c["result"] for c in new.commands if c["command"] == "confirm"]
    facts: dict[str, Any] = {
        "session_id_read_through": SESSION_ID_SURFACE,
        "session_id_public": True,
        "new": {
            "way": "/new through handle_message",
            "confirmation_asked_with": "send_slash_confirm"
            if _calls(new, "send_slash_confirm")
            else None,
            "confirmation_resolved_with": confirms[0].get("way") if confirms else None,
            "session_id_changed": bool(new.sessions.get("after"))
            and new.sessions.get("after") != new.sessions.get("before"),
            "earlier_text_in_next_request": scenarios["session_new"].texts.get("first", "")
            in _last_texts(new),
            "reset_hook_called": bool(_calls(new, "hook:on_session_reset")),
        },
        "resume_other_thread": switch("session_resume", "thread_one"),
        "resume_other_thread_as_admin": switch("session_resume_admin", "thread_one"),
        "resume_same_thread": switch("session_resume_same_thread", "before"),
        "resume_missing": {
            "kept_current_session": results["session_resume_missing"].sessions.get("after")
            == results["session_resume_missing"].sessions.get("before"),
            "notice_sent": any(
                c["args"].get("reply_to") == "m-2"
                for c in _calls(results["session_resume_missing"], "send")
            ),
        },
    }
    new_works = (
        facts["new"]["session_id_changed"] and not facts["new"]["earlier_text_in_next_request"]
    )
    bind_works = facts["resume_same_thread"]["switched"] and (
        facts["resume_other_thread"]["switched"]
        or facts["resume_other_thread_as_admin"]["switched"]
    )
    return {"facts": facts, "holds": bool(new_works and bind_works and facts["session_id_public"])}


# ---- own_access_policy -------------------------------------------------------------------
def _access(results: Results, scenarios: Scenarios) -> dict[str, Any]:
    def entry(name: str) -> dict[str, Any]:
        result = results[name]
        reads = [c["args"].get("value") for c in _calls(result, "read:enforces_own_access_policy")]
        upstream = [
            c["args"].get("value") for c in _calls(result, "read:authorization_is_upstream")
        ]
        return {
            "message_reached_turn": bool(_scripted(result)),
            "hermes_replied": bool(_calls(result, "send")),
            "declaration_read": "BasePlatformAdapter.enforces_own_access_policy (property)"
            if reads
            else None,
            "declaration_value": reads[0] if reads else None,
            "upstream_read": "BasePlatformAdapter.authorization_is_upstream (property)"
            if upstream
            else None,
            "upstream_value": upstream[0] if upstream else None,
            "policy_value": scenarios[name].probe.get("dm_policy"),
        }

    own = entry("access_policy_own")
    default = entry("access_policy_default")
    # Evidence only (Hermes v0.22 checks again at admission and trusts an own policy there only
    # with authorization_is_upstream, 09 17.7); the verdict stays the own policy's.
    facts = {
        "own": own,
        "default": default,
        "upstream": entry("access_policy_upstream"),
        "policy_key": "PlatformConfig.extra.dm_policy",
        "chat_type": "thread",
    }
    return {
        "facts": facts,
        "holds": own["message_reached_turn"] and not default["message_reached_turn"],
    }


_POINTS: Final[Mapping[str, Callable[[Results, Scenarios], dict[str, Any]]]] = {
    "progress_hooks": _progress,
    "steer": _steer,
    "credential_type": _credential,
    "interrupt_outcome": _interrupt,
    "session_switch": _session,
    "own_access_policy": _access,
}


def observe(results: Results, scenarios: Scenarios) -> dict[str, dict[str, Any]]:
    """Facts and verdict per point slug.

    Args:
        results: Recordings by scenario name (every scenario of :data:`POINT_SCENARIOS`).
        scenarios: The scenarios by name.

    Returns:
        ``{slug: {"facts": ..., "holds": bool}}`` for every slug of :data:`SLUGS`.

    Raises:
        KeyError: A scenario of a point is missing.
    """
    return {slug: _POINTS[slug](results, scenarios) for slug in SLUGS}


def surface_facts(results: Results) -> dict[str, Any]:
    """Presence and parameter names of each surface, from the ``describe`` command."""
    for result in results.values():
        for command in result.commands:
            if command["command"] == "describe":
                found = dict(command["result"])
                connect = next((c for c in result.calls if c["method"] == "connect"), None)
                keys = (connect or {}).get("args", {}).get("extra_keys", [])
                found["config.extra:thread_sessions_per_user"] = {
                    "present": "thread_sessions_per_user" in keys,
                    "kind": SEEDED_EXTRA,
                }
                return {k: found[k] for k in sorted(found)}
    return {}


def egress_facts(results: Results) -> list[dict[str, str]]:
    """Every attempted destination with its phase, sorted and without repeats."""
    seen = {(dest, phase) for result in results.values() for dest, phase in result.egress}
    return [{"destination": d, "phase": p} for d, p in sorted(seen)]


def store_call_facts(results: Results) -> dict[str, list[str]]:
    """Credential store and service shim calls per scenario, without repeats."""
    return {
        name: sorted(set(r.store_calls)) for name, r in sorted(results.items()) if r.store_calls
    }


def discovery_facts(results: Results) -> dict[str, Any]:
    """Where the probe was installed and whether every gateway loaded it."""
    loaded = all(any(c["method"] == "connect" for c in r.calls) for r in results.values())
    return {
        "place": f"<HERMES_HOME>/{PLUGIN_PLACE.as_posix()}",
        "enabled_through": "plugins.enabled",
        "loaded_in_every_scenario": loaded,
    }


def scenario_names() -> Sequence[str]:
    """Every scenario a point needs, sorted."""
    return sorted({name for names in POINT_SCENARIOS.values() for name in names})
