# SPDX-License-Identifier: Apache-2.0
"""Gate G5 for Python (docs/spec/06-api.md 15, ADR 0053): the generated core, made tolerant by
`bothy_sdk.tolerance`, decodes the shared fixtures of `sdk/testdata/tolerance/`.

Each fixture goes through the generated operation functions with a fake transport, so the test
covers the path an application takes, not only `from_dict`.
"""

from __future__ import annotations

import datetime
import enum
import importlib
import inspect
import pkgutil
import re
from collections.abc import Callable, Iterator, Mapping
from typing import Any
from uuid import UUID

import httpx
import pytest
from conftest import fixture, schema, spec

import bothy_sdk
from bothy_sdk import tolerance
from bothy_sdk.generated import models
from bothy_sdk.generated.api.agent_runtime import poll_agent_events
from bothy_sdk.generated.api.sync import list_events
from bothy_sdk.generated.client import Client
from bothy_sdk.generated.models import (
    ActorKind,
    EventPage,
    ThreadKind,
    ThreadMovedOutEvent,
    ThreadState,
    ThreadUpdatedEvent,
)
from bothy_sdk.generated.types import UNSET

Answering = Callable[[Mapping[str, Any]], tuple[Client, list[httpx.Request]]]

UNKNOWN_EVENT_TYPE = "future.example"
UNKNOWN_ENUM_VALUES = {
    "thread.state": "frozen_example",
    "thread.kind": "sidebar_example",
    "actor.kind": "robot_example",
}
UNKNOWN_MEMBERS = {
    "page": "future_page_member",
    "envelope": "future_envelope_member",
    "payload": "future_payload_member",
    "thread": "future_thread_member",
}


def _list(answering: Answering, name: str) -> tuple[EventPage, list[httpx.Request]]:
    client, seen = answering(fixture(name))
    page = list_events.sync(client=client, after=0)
    assert isinstance(page, EventPage)
    return page, seen


def _event(page: EventPage, index: int = 0) -> ThreadUpdatedEvent:
    event = page.events[index]
    assert isinstance(event, ThreadUpdatedEvent)
    return event


def _utc(text: str) -> datetime.datetime:
    return datetime.datetime.fromisoformat(text)


def test_g5_unknown_field_is_ignored(answering: Answering) -> None:
    raw = fixture("unknown_field")
    page, seen = _list(answering, "unknown_field")

    assert [str(r.url) for r in seen] == ["https://example.com/api/v1/events?after=0"]
    assert len(page.events) == 1
    event = _event(page)
    raw_event = raw["events"][0]
    raw_thread = raw_event["payload"]["thread"]
    thread = event.payload.thread

    # Every known field equals the fixture.
    assert (page.next_cursor, page.head_seq, str(page.sync_epoch)) == (
        raw["next_cursor"],
        raw["head_seq"],
        raw["sync_epoch"],
    )
    assert event.type_.value == raw_event["type"]
    assert (str(event.id), event.seq, event.schema_version) == (
        raw_event["id"],
        raw_event["seq"],
        raw_event["schema_version"],
    )
    assert event.occurred_at == _utc(raw_event["occurred_at"])
    assert event.permission_version == raw_event["permission_version"]
    assert event.actor is not UNSET
    assert event.actor.to_dict() == raw_event["actor"]
    assert event.scope.to_dict() == raw_event["scope"]
    assert event.resource.to_dict() == raw_event["resource"]
    expected_thread = {k: v for k, v in raw_thread.items() if k != UNKNOWN_MEMBERS["thread"]}
    decoded_thread = thread.to_dict()
    decoded_thread.pop(UNKNOWN_MEMBERS["thread"])
    for key in ("last_activity_at", "created_at"):
        assert _utc(decoded_thread.pop(key)) == _utc(expected_thread.pop(key))
    assert decoded_thread == expected_thread

    # The unknown members appear only in additional_properties.
    assert page.additional_properties == {"future_page_member": "page extra"}
    assert event.additional_properties == {"future_envelope_member": "envelope extra"}
    assert event.payload.additional_properties == {"future_payload_member": 1}
    assert thread.additional_properties == {"future_thread_member": {"nested": True, "count": 2}}
    assert event.scope.additional_properties == {}
    assert event.resource.additional_properties == {}


def test_g5_unknown_enum_value_keeps_raw_value(answering: Answering) -> None:
    page, _ = _list(answering, "unknown_enum_value")
    event = _event(page)
    thread = event.payload.thread
    assert event.actor is not UNSET
    decoded: dict[str, enum.Enum] = {
        "thread.state": thread.state,
        "thread.kind": thread.kind,
        "actor.kind": event.actor.kind,
    }
    for place, value in decoded.items():
        raw = UNKNOWN_ENUM_VALUES[place]
        assert value.value == raw, place
        assert tolerance.is_unknown(value), place
        assert all(value is not member for member in type(value)), place
        assert value not in list(type(value)), place
        assert str(value) == raw

    thread_dict = thread.to_dict()
    assert (thread_dict["state"], thread_dict["kind"]) == ("frozen_example", "sidebar_example")
    assert event.actor.to_dict()["kind"] == "robot_example"
    assert event.to_dict()["payload"]["thread"]["state"] == "frozen_example"

    # A known value in the same run decodes to the generated member itself.
    assert thread.title_source is models.ThreadTitleSource.MANUAL
    assert ThreadState("open") is ThreadState.OPEN
    assert not tolerance.is_unknown(ThreadState.OPEN)
    assert ActorKind("user") is ActorKind.USER
    assert ThreadKind("main") is ThreadKind.MAIN


def test_g5_unknown_event_type_is_skipped(answering: Answering) -> None:
    raw = fixture("unknown_event_type")
    page, _ = _list(answering, "unknown_event_type")

    assert [type(e) for e in page.events] == [ThreadUpdatedEvent, ThreadUpdatedEvent]
    assert [e.seq for e in page.events] == [201, 203]
    unknown = raw["events"][1]
    assert page.next_cursor >= unknown["seq"]

    decoded = tolerance.decode_event(unknown)
    assert isinstance(decoded, tolerance.UnknownEvent)
    assert not isinstance(decoded, ThreadMovedOutEvent)
    assert decoded == tolerance.UnknownEvent(type=UNKNOWN_EVENT_TYPE, seq=202, raw=unknown)

    # The unknown payload fits ThreadMovedOutEvent: only the strict `type` enum keeps the
    # generated model from accepting it.
    moved_out = {**unknown, "type": "thread.moved_out"}
    assert isinstance(tolerance.decode_event(moved_out), ThreadMovedOutEvent)
    with pytest.raises(ValueError, match=UNKNOWN_EVENT_TYPE):
        ThreadMovedOutEvent.from_dict(unknown)

    known = tolerance.decode_event(raw["events"][0])
    assert isinstance(known, ThreadUpdatedEvent)
    assert known.seq == 201


@pytest.mark.parametrize(
    ("data", "expected"),
    [
        ({"seq": 9}, tolerance.UnknownEvent(type="", seq=9, raw={"seq": 9})),
        (
            {"type": 7, "seq": True},
            tolerance.UnknownEvent(type="", seq=None, raw={"type": 7, "seq": True}),
        ),
        (
            {"type": "x.y", "seq": "9"},
            tolerance.UnknownEvent(type="x.y", seq=None, raw={"type": "x.y", "seq": "9"}),
        ),
    ],
)
def test_decode_event_without_a_known_type(data: dict[str, Any], expected: Any) -> None:
    assert tolerance.decode_event(data) == expected


def test_decode_event_rejects_a_non_object() -> None:
    with pytest.raises(TypeError, match="object"):
        tolerance.decode_event(["not", "an", "object"])  # type: ignore[arg-type]


def test_g5_null_member_is_treated_as_absent(answering: Answering) -> None:
    raw = fixture("null_member")
    assert raw["events"][0]["correlation_id"] is None
    assert raw["events"][0]["payload"]["thread"]["closed_at"] is None

    page, _ = _list(answering, "null_member")
    event = _event(page)
    assert event.correlation_id is UNSET
    assert event.payload.thread.closed_at is UNSET
    assert "correlation_id" not in event.additional_properties
    assert "closed_at" not in event.payload.thread.additional_properties

    without = fixture("null_member")
    del without["events"][0]["correlation_id"]
    del without["events"][0]["payload"]["thread"]["closed_at"]
    assert EventPage.from_dict(without).to_dict() == page.to_dict()


def test_g5_poll_agent_events_is_tolerant(answering: Answering) -> None:
    for name in ("unknown_field", "unknown_enum_value", "unknown_event_type", "null_member"):
        client, seen = answering(fixture(name))
        polled = poll_agent_events.sync(consumer="primary", client=client)
        listed, _ = _list(answering, name)
        assert isinstance(polled, EventPage), name
        assert polled.to_dict() == listed.to_dict(), name
        assert seen[0].url.path == "/api/v1/agents/me/consumers/primary/events"


def test_known_event_types_match_spec() -> None:
    mapping = schema("Event")["discriminator"]["mapping"]
    assert frozenset(mapping) == tolerance.KNOWN_EVENT_TYPES
    assert len(tolerance.KNOWN_EVENT_TYPES) == len(schema("Event")["oneOf"])


def _generated_enums() -> Iterator[type[enum.Enum]]:
    for info in pkgutil.iter_modules(models.__path__, models.__name__ + "."):
        module = importlib.import_module(info.name)
        for _, value in inspect.getmembers(module, inspect.isclass):
            if issubclass(value, enum.Enum) and value.__module__ == info.name:
                yield value


def _event_type_enum_names() -> set[str]:
    """Names of the `type` enums of every event schema composed from EventEnvelope."""
    names = set()
    for name, body in spec()["components"]["schemas"].items():
        parts = body.get("allOf", [])
        if {"$ref": "#/components/schemas/EventEnvelope"} in parts:
            names.add(f"{name}Type")
    return names


def test_event_type_enums_stay_strict() -> None:
    strict = _event_type_enum_names()
    found = {cls.__name__: cls for cls in _generated_enums()}
    assert strict <= set(found), sorted(strict - set(found))
    assert len(strict) > len(tolerance.KNOWN_EVENT_TYPES)  # durable and ephemeral events
    for name, cls in found.items():
        if name in strict:
            with pytest.raises(ValueError, match="is not a valid"):
                cls("unknown_example")
        else:
            value = cls("unknown_example")
            assert tolerance.is_unknown(value), name
            assert value.value == "unknown_example", name


def _properties(name: str) -> dict[str, Any]:
    body = schema(name)
    props: dict[str, Any] = dict(body.get("properties", {}))
    for part in body.get("allOf", []):
        if "$ref" in part:
            props.update(_properties(part["$ref"].rsplit("/", 1)[1]))
        props.update(part.get("properties", {}))
    return props


def _enum_values(prop: dict[str, Any]) -> list[str]:
    if "$ref" in prop:
        prop = schema(prop["$ref"].rsplit("/", 1)[1])
    values: list[str] = prop["enum"]
    return values


def test_fixture_unknowns_are_unknown_to_the_spec() -> None:
    assert UNKNOWN_EVENT_TYPE not in schema("Event")["discriminator"]["mapping"]
    assert re.search(rf"\b{re.escape(UNKNOWN_EVENT_TYPE)}\b", str(spec())) is None

    thread = _properties("Thread")
    actor = _properties("EventActor")
    assert UNKNOWN_ENUM_VALUES["thread.state"] not in _enum_values(thread["state"])
    assert UNKNOWN_ENUM_VALUES["thread.kind"] not in _enum_values(thread["kind"])
    assert UNKNOWN_ENUM_VALUES["actor.kind"] not in _enum_values(actor["kind"])
    assert thread["state"] == {"$ref": "#/components/schemas/ThreadState"}
    assert actor["kind"] == {"$ref": "#/components/schemas/ActorKind"}
    assert "enum" in thread["kind"]

    assert UNKNOWN_MEMBERS["page"] not in _properties("EventPage")
    assert UNKNOWN_MEMBERS["envelope"] not in _properties("ThreadUpdatedEvent")
    assert UNKNOWN_MEMBERS["payload"] not in _properties("ThreadPayload")
    assert UNKNOWN_MEMBERS["thread"] not in thread

    raw = fixture("unknown_field")
    raw_event = raw["events"][0]
    assert UNKNOWN_MEMBERS["page"] in raw
    assert UNKNOWN_MEMBERS["envelope"] in raw_event
    assert UNKNOWN_MEMBERS["payload"] in raw_event["payload"]
    assert UNKNOWN_MEMBERS["thread"] in raw_event["payload"]["thread"]
    unknown_type = fixture("unknown_event_type")["events"][1]
    assert unknown_type["type"] == UNKNOWN_EVENT_TYPE
    assert set(unknown_type["payload"]) == set(_properties("ThreadMovedOutPayload"))


def test_install_is_idempotent() -> None:
    before = {cls: cls.__dict__.get("_missing_") for cls in _generated_enums()}
    page_from_dict = EventPage.__dict__["from_dict"]
    thread_from_dict = models.Thread.__dict__["from_dict"]
    tolerance.install()
    tolerance.install()
    importlib.reload(bothy_sdk)
    assert {cls: cls.__dict__.get("_missing_") for cls in _generated_enums()} == before
    assert EventPage.__dict__["from_dict"] is page_from_dict
    assert models.Thread.__dict__["from_dict"] is thread_from_dict
    page = EventPage.from_dict(fixture("unknown_event_type"))
    assert [e.seq for e in page.events] == [201, 203]


def test_known_page_is_unchanged_by_the_hook() -> None:
    raw = fixture("unknown_event_type")
    raw["events"].pop(1)
    hooked = EventPage.from_dict(raw)
    plain = tolerance.generated_from_dict(EventPage)(raw)
    assert isinstance(plain, EventPage)
    assert hooked == plain
    assert hooked.to_dict() == plain.to_dict()
    assert [type(e) for e in hooked.events] == [ThreadUpdatedEvent, ThreadUpdatedEvent]
    first = _event(hooked)
    assert first.payload.thread.state is ThreadState.OPEN
    assert first.payload.thread.kind is ThreadKind.THREAD
    assert first.actor is not UNSET
    assert first.actor.kind is ActorKind.USER
    assert first.id == UUID(raw["events"][0]["id"])


def test_hook_keeps_errors_for_invalid_data() -> None:
    # A known event that does not fit its schema still fails: the union parser of the core
    # tries every variant and reports the last one, the hook adds no fallback.
    raw = fixture("unknown_field")
    del raw["events"][0]["payload"]["thread"]["title"]
    with pytest.raises((KeyError, ValueError)):
        EventPage.from_dict(raw)
    with pytest.raises(KeyError, match="title"):
        models.Thread.from_dict(raw["events"][0]["payload"]["thread"])
    with pytest.raises(ValueError, match="5 is not a valid ThreadState"):
        ThreadState(5)  # type: ignore[arg-type]
    with pytest.raises(KeyError, match="kind"):
        models.EventActor.from_dict({"kind": None})
    with pytest.raises(TypeError):
        models.EventScope.from_dict(3)  # type: ignore[arg-type]


def test_page_skips_items_that_are_not_events() -> None:
    raw = fixture("unknown_field")
    raw["events"] = ["text", 3, {"seq": 4}, None, *raw["events"]]
    page = EventPage.from_dict(raw)
    assert [e.seq for e in page.events] == [101]


def test_page_skips_events_whose_type_is_not_a_string() -> None:
    # An unhashable `type` (a list, an object) must not break the membership test.
    raw = fixture("unknown_field")
    raw["events"] = [{"type": ["a"], "seq": 1}, {"type": {"k": 1}, "seq": 2}, *raw["events"]]
    page = EventPage.from_dict(raw)
    assert [e.seq for e in page.events] == [101]


def _unions(node: object, path: str = "") -> Iterator[str]:
    if isinstance(node, dict):
        if "oneOf" in node or "anyOf" in node:
            yield path
        for key, value in node.items():
            yield from _unions(value, f"{path}/{key}")
    elif isinstance(node, list):
        for index, value in enumerate(node):
            yield from _unions(value, f"{path}/{index}")


def test_event_union_is_the_only_union_of_the_spec() -> None:
    # Tolerant enums are safe only where no union parser tries models in order: the strict
    # `type` enums cover the Event union. A new oneOf needs its discriminator enums kept
    # strict too (tolerance.py), so this test fails until that is decided.
    assert list(_unions(spec())) == ["/components/schemas/Event"]


@pytest.mark.parametrize("module", [bothy_sdk, tolerance])
def test_doc_examples_run(module: Any) -> None:
    import doctest

    result = doctest.testmod(module, optionflags=doctest.ELLIPSIS)
    assert result.attempted > 0
    assert result.failed == 0
