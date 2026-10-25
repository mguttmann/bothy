# SPDX-License-Identifier: Apache-2.0
"""The parser of Jira events (brief M2-50 Design note 6).

Proves that each fixture of the closed event list parses into the fields the app uses, that
rich text in both forms (string, ADF document) reads as text with line breaks and mentions,
that every other form of rich text gives no text, that changelog items keep ``field``,
``fromString`` and ``toString``, that a missing or malformed ``issue.id``, ``issue.key``,
project key or comment ``comment.id`` is ``invalid_payload``, that unknown members are
ignored, and that unsupported events and updates without changelog items are ignored with
the reasons of Design note 6.
"""

from __future__ import annotations

import copy
from typing import Any

import pytest

from jira_app.jira.adf import read_text
from jira_app.jira.events import Change, Ignored, Invalid, JiraEvent, parse_event
from tests.fakes.jira_api import load_fixture, render_fixture

PROJECTS = frozenset({"SUP"})

EVENTS = ("jira:issue_created", "jira:issue_updated", "comment_created")


def _parse(payload: object) -> JiraEvent:
    parsed = parse_event(payload, PROJECTS)
    assert isinstance(parsed, JiraEvent), parsed
    return parsed


def _doc(*content: Any) -> dict[str, Any]:
    return {"type": "doc", "version": 1, "content": list(content)}


def _text(text: str, **extra: Any) -> dict[str, Any]:
    return {"type": "text", "text": text, **extra}


def _para(*content: Any) -> dict[str, Any]:
    return {"type": "paragraph", "content": list(content)}


# A document with every node the reader knows (Design note 6).
RICH = _doc(
    {"type": "heading", "attrs": {"level": 2}, "content": [_text("Steps")]},
    _para(
        _text("Ask "),
        {"type": "mention", "attrs": {"id": "example-account-alex", "text": "@Alex"}},
        _text(" to log in", marks=[{"type": "strong"}]),
        {"type": "hardBreak"},
        _text("then reload"),
    ),
    {
        "type": "bulletList",
        "content": [
            {"type": "listItem", "content": [_para(_text("first"))]},
            {"type": "listItem", "content": [_para(_text("second"))]},
        ],
    },
    {"type": "codeBlock", "attrs": {"language": "text"}, "content": [_text("error 42")]},
    {"type": "blockquote", "content": [_para(_text("quoted"))]},
)

RICH_TEXT = "Steps\nAsk @Alex to log in\nthen reload\nfirst\n\nsecond\n\nerror 42\nquoted\n\n"


# Each fixture into its fields.


def test_issue_created_fixture_parses_into_its_fields() -> None:
    raw = load_fixture("jira:issue_created")
    event = _parse(raw)
    fields = raw["issue"]["fields"]
    assert event == JiraEvent(
        event="jira:issue_created",
        issue_id=raw["issue"]["id"],
        issue_key=raw["issue"]["key"],
        project=fields["project"]["key"],
        summary=fields["summary"],
        actor=raw["user"]["displayName"],
        text=fields["description"],
    )
    assert (event.issue_id, event.issue_key, event.project) == ("10012", "SUP-12", "SUP")
    assert event.actor == "Sam"
    assert event.changes == ()
    assert event.comment_id == ""


def test_issue_updated_fixture_parses_into_its_fields() -> None:
    raw = load_fixture("jira:issue_updated")
    event = _parse(raw)
    fields = raw["issue"]["fields"]
    assert event.event == "jira:issue_updated"
    assert event.issue_id == raw["issue"]["id"]
    assert event.issue_key == raw["issue"]["key"]
    assert event.project == fields["project"]["key"]
    assert event.summary == fields["summary"]
    assert event.actor == raw["user"]["displayName"]
    assert event.text == ""
    assert event.comment_id == ""
    assert event.changes == tuple(
        Change(item["field"], item["fromString"], item["toString"])
        for item in raw["changelog"]["items"]
    )


def test_comment_created_fixture_parses_into_its_fields() -> None:
    raw = load_fixture("comment_created")
    event = _parse(raw)
    assert event == JiraEvent(
        event="comment_created",
        issue_id=raw["issue"]["id"],
        issue_key=raw["issue"]["key"],
        project=raw["issue"]["fields"]["project"]["key"],
        summary=raw["issue"]["fields"]["summary"],
        actor=raw["comment"]["author"]["displayName"],
        text=raw["comment"]["body"],
        comment_id=raw["comment"]["id"],
    )
    assert event.actor == "Robin"
    assert event.comment_id == "10050"


@pytest.mark.parametrize("name", EVENTS)
def test_parsed_event_survives_the_job_store_round_trip(name: str) -> None:
    event = _parse(load_fixture(name))
    assert JiraEvent.from_json(event.to_json()) == event


# Rich text: ADF documents.


def test_adf_description_reads_as_text_with_line_breaks_and_mentions() -> None:
    event = _parse(render_fixture("jira:issue_created", text=RICH))
    assert event.text == RICH_TEXT


def test_adf_comment_reads_as_text_with_line_breaks_and_mentions() -> None:
    event = _parse(render_fixture("comment_created", text=RICH))
    assert event.text == RICH_TEXT


@pytest.mark.parametrize(
    ("node", "expected"),
    [
        (_para(_text("a")), "a\n"),
        ({"type": "heading", "content": [_text("h")]}, "h\n"),
        ({"type": "codeBlock", "content": [_text("x = 1")]}, "x = 1\n"),
        ({"type": "blockquote", "content": [_text("q")]}, "q\n"),
        ({"type": "listItem", "content": [_text("i")]}, "i\n"),
        (_para(_text("a"), {"type": "hardBreak"}, _text("b")), "a\nb\n"),
        (_para({"type": "mention", "attrs": {"text": "@Sam"}}), "@Sam\n"),
        (_para({"type": "paragraph"}), "\n\n"),
    ],
    ids=[
        "paragraph",
        "heading",
        "codeBlock",
        "blockquote",
        "listItem",
        "hardBreak",
        "mention",
        "empty",
    ],
)
def test_each_block_node_ends_with_a_line_break(node: dict[str, Any], expected: str) -> None:
    assert read_text(_doc(node)) == expected


def test_text_keeps_document_order_through_other_containers() -> None:
    panel = {"type": "panel", "content": [_para(_text("one"), _text(" two"))]}
    table = {
        "type": "table",
        "content": [
            {"type": "tableRow", "content": [{"type": "tableCell", "content": [_text("t")]}]}
        ],
    }
    assert read_text(_doc(panel, table, _para(_text("end")))) == "one two\ntend\n"


def test_mention_without_text_and_malformed_nodes_give_nothing() -> None:
    doc = _doc(
        _para(
            {"type": "mention", "attrs": {"id": "example-account-alex"}},
            {"type": "mention"},
            {"type": "mention", "attrs": {"text": 7}},
            {"type": "text", "text": None},
            {"type": "text"},
            "loose string",
            42,
            _text("kept"),
        )
    )
    assert read_text(doc) == "kept\n"


def test_deeply_nested_document_does_not_exhaust_the_stack() -> None:
    node: dict[str, Any] = _text("deep")
    for _ in range(5000):
        node = {"type": "blockquote", "content": [node]}
    assert read_text(_doc(node)) == "deep" + "\n" * 5000


def test_string_rich_text_is_taken_as_it_is() -> None:
    text = "line one\n\n*not markup* @Alex"
    assert _parse(render_fixture("jira:issue_created", text=text)).text == text
    assert _parse(render_fixture("comment_created", text=text)).text == text


@pytest.mark.parametrize(
    "value",
    [
        None,
        42,
        4.5,
        True,
        ["a", "b"],
        [_doc(_para(_text("in a list")))],
        {"type": "paragraph", "content": [_text("no doc")]},
        {"content": [_para(_text("no type"))]},
        {"type": "DOC", "content": [_para(_text("wrong case"))]},
        {},
    ],
    ids=[
        "null",
        "int",
        "float",
        "bool",
        "list",
        "list-of-doc",
        "paragraph",
        "no-type",
        "type-case",
        "empty-object",
    ],
)
@pytest.mark.parametrize("name", ["jira:issue_created", "comment_created"])
def test_other_forms_of_rich_text_give_no_text(name: str, value: object) -> None:
    event = _parse(render_fixture(name, text=value))
    assert event.text == ""


@pytest.mark.parametrize("name", ["jira:issue_created", "comment_created"])
def test_absent_rich_text_gives_no_text(name: str) -> None:
    raw = load_fixture(name)
    if name == "comment_created":
        del raw["comment"]["body"]
    else:
        del raw["issue"]["fields"]["description"]
    assert _parse(raw).text == ""


# Changelog.


def test_changelog_items_of_status_summary_and_description() -> None:
    event = _parse(load_fixture("jira:issue_updated"))
    assert event.changes == (
        Change("status", "To Do", "In Progress"),
        Change("summary", "Login fails", "Login fails after the update"),
        Change(
            "description",
            "Users cannot log in since the update.",
            "Users cannot log in since the update. Robin can reproduce it.",
        ),
    )


def test_changelog_values_absent_or_not_strings_read_as_empty() -> None:
    items = [
        {"field": "assignee", "fromString": None, "toString": "Alex"},
        {"field": "labels"},
        {"field": "priority", "fromString": 3, "toString": {"name": "High"}},
    ]
    event = _parse(render_fixture("jira:issue_updated", changelog=items))
    assert event.changes == (
        Change("assignee", "", "Alex"),
        Change("labels", "", ""),
        Change("priority", "", ""),
    )


def test_changelog_items_without_a_field_are_skipped() -> None:
    items = [
        {"fromString": "a", "toString": "b"},
        {"field": "", "toString": "b"},
        {"field": 5, "toString": "b"},
        "status",
        None,
        {"field": "status", "fromString": "To Do", "toString": "Done"},
    ]
    event = _parse(render_fixture("jira:issue_updated", changelog=items))
    assert event.changes == (Change("status", "To Do", "Done"),)


# Ignored deliveries (Design note 6).


@pytest.mark.parametrize(
    "changelog",
    [None, [], [{"toString": "x"}]],
    ids=["no-changelog", "empty-items", "no-usable-item"],
)
def test_update_without_changelog_items_is_no_change(changelog: object) -> None:
    raw = render_fixture("jira:issue_updated", changelog=changelog)
    assert parse_event(raw, PROJECTS) == Ignored("no_change", "jira:issue_updated")


@pytest.mark.parametrize("changelog", [{"id": "1"}, {"id": "1", "items": "status"}, "x", 3])
def test_update_with_malformed_changelog_is_no_change(changelog: object) -> None:
    raw = load_fixture("jira:issue_updated")
    raw["changelog"] = changelog
    assert parse_event(raw, PROJECTS) == Ignored("no_change", "jira:issue_updated")


@pytest.mark.parametrize(
    "event",
    ["jira:issue_deleted", "comment_updated", "comment_deleted", "JIRA:ISSUE_CREATED", ""],
)
def test_unsupported_event_is_ignored_with_its_name(event: str) -> None:
    raw = load_fixture("jira:issue_created")
    raw["webhookEvent"] = event
    assert parse_event(raw, PROJECTS) == Ignored("unsupported", event)


@pytest.mark.parametrize("event", [None, 1, ["jira:issue_created"], {"x": 1}])
def test_webhook_event_that_is_not_a_string_is_unsupported(event: object) -> None:
    raw = load_fixture("jira:issue_created")
    raw["webhookEvent"] = event
    assert parse_event(raw, PROJECTS) == Ignored("unsupported", "")


def test_missing_webhook_event_is_unsupported() -> None:
    raw = load_fixture("jira:issue_created")
    del raw["webhookEvent"]
    assert parse_event(raw, PROJECTS) == Ignored("unsupported", "")


@pytest.mark.parametrize("payload", [[], "jira:issue_created", 3, None])
def test_payload_that_is_not_an_object_is_unsupported(payload: object) -> None:
    assert parse_event(payload, PROJECTS) == Ignored("unsupported", "")


def test_unsupported_event_wins_over_a_broken_issue() -> None:
    assert parse_event({"webhookEvent": "worklog_created"}, PROJECTS) == Ignored(
        "unsupported", "worklog_created"
    )


@pytest.mark.parametrize("name", EVENTS)
@pytest.mark.parametrize("project", ["OPS", "sup", "SUP ", "SU"])
def test_other_project_is_not_configured(name: str, project: str) -> None:
    raw = render_fixture(name, project=project)
    assert parse_event(raw, PROJECTS) == Ignored("project_not_configured", name)


# invalid_payload.


@pytest.mark.parametrize("name", EVENTS)
def test_payload_without_project_key_is_invalid(name: str) -> None:
    raw = load_fixture(name)
    del raw["issue"]["fields"]["project"]["key"]
    assert parse_event(raw, PROJECTS) == Invalid(name)


@pytest.mark.parametrize("name", EVENTS)
@pytest.mark.parametrize(
    "mutate",
    [
        lambda raw: raw["issue"]["fields"].pop("project"),
        lambda raw: raw["issue"]["fields"].__setitem__("project", "SUP"),
        lambda raw: raw["issue"]["fields"]["project"].__setitem__("key", ""),
        lambda raw: raw["issue"]["fields"]["project"].__setitem__("key", 7),
        lambda raw: raw["issue"]["fields"]["project"].__setitem__("key", None),
        lambda raw: raw["issue"].pop("fields"),
        lambda raw: raw["issue"].__setitem__("fields", []),
    ],
    ids=[
        "no-project",
        "project-string",
        "empty-key",
        "key-number",
        "key-null",
        "no-fields",
        "fields-list",
    ],
)
def test_malformed_project_is_invalid(name: str, mutate: Any) -> None:
    raw = load_fixture(name)
    mutate(raw)
    assert parse_event(raw, PROJECTS) == Invalid(name)


@pytest.mark.parametrize("name", EVENTS)
@pytest.mark.parametrize(
    "issue_id",
    [None, "", "abc", "-12", "12a", " 12", "12\n", "1.5", -1, 1.5, True, [], {"id": "1"}, "١٢"],
)
def test_malformed_issue_id_is_invalid(name: str, issue_id: object) -> None:
    raw = load_fixture(name)
    raw["issue"]["id"] = issue_id
    assert parse_event(raw, PROJECTS) == Invalid(name)


@pytest.mark.parametrize("name", EVENTS)
def test_missing_issue_id_is_invalid(name: str) -> None:
    raw = load_fixture(name)
    del raw["issue"]["id"]
    assert parse_event(raw, PROJECTS) == Invalid(name)


@pytest.mark.parametrize("name", EVENTS)
def test_numeric_issue_id_is_taken_as_its_decimal_form(name: str) -> None:
    raw = load_fixture(name)
    raw["issue"]["id"] = 10012
    assert _parse(raw).issue_id == "10012"


@pytest.mark.parametrize("name", EVENTS)
@pytest.mark.parametrize(
    "key",
    [
        None,
        "",
        "sup-12",
        "SUP12",
        "SUP-0",
        "SUP-012",
        "1SUP-1",
        "SUP-12\n",
        " SUP-12",
        "SUP-",
        12,
        ["SUP-12"],
    ],
)
def test_malformed_issue_key_is_invalid(name: str, key: object) -> None:
    raw = load_fixture(name)
    raw["issue"]["key"] = key
    assert parse_event(raw, PROJECTS) == Invalid(name)


@pytest.mark.parametrize("name", EVENTS)
def test_missing_issue_key_is_invalid(name: str) -> None:
    raw = load_fixture(name)
    del raw["issue"]["key"]
    assert parse_event(raw, PROJECTS) == Invalid(name)


@pytest.mark.parametrize("name", EVENTS)
@pytest.mark.parametrize("issue", [None, "SUP-12", [], {}])
def test_missing_or_malformed_issue_is_invalid(name: str, issue: object) -> None:
    raw = load_fixture(name)
    raw["issue"] = issue
    assert parse_event(raw, PROJECTS) == Invalid(name)


def test_invalid_issue_wins_over_an_unconfigured_project() -> None:
    raw = render_fixture("jira:issue_created", project="OPS")
    raw["issue"]["id"] = "x"
    assert parse_event(raw, PROJECTS) == Invalid("jira:issue_created")


@pytest.mark.parametrize("key", ["SUP_1-9", "A-1", "S2-100"])
def test_well_formed_issue_keys_are_accepted(key: str) -> None:
    raw = render_fixture("jira:issue_created", key=key, project="SUP")
    assert _parse(raw).issue_key == key


@pytest.mark.parametrize("comment_id", [None, "", "c1", "-1", "10050 ", 1.5, False, ["10050"]])
def test_malformed_comment_id_is_invalid(comment_id: object) -> None:
    raw = load_fixture("comment_created")
    raw["comment"]["id"] = comment_id
    assert parse_event(raw, PROJECTS) == Invalid("comment_created")


def test_missing_comment_id_is_invalid() -> None:
    raw = load_fixture("comment_created")
    del raw["comment"]["id"]
    assert parse_event(raw, PROJECTS) == Invalid("comment_created")


@pytest.mark.parametrize("comment", [None, "text", []])
def test_missing_or_malformed_comment_is_invalid(comment: object) -> None:
    raw = load_fixture("comment_created")
    raw["comment"] = comment
    assert parse_event(raw, PROJECTS) == Invalid("comment_created")


def test_missing_comment_member_is_invalid() -> None:
    raw = load_fixture("comment_created")
    del raw["comment"]
    assert parse_event(raw, PROJECTS) == Invalid("comment_created")


@pytest.mark.parametrize("name", ["jira:issue_created", "jira:issue_updated"])
def test_issue_events_do_not_need_a_comment(name: str) -> None:
    raw = load_fixture(name)
    raw["comment"] = {"id": "not digits"}
    assert _parse(raw).comment_id == ""


# Actor and summary.


@pytest.mark.parametrize("name", ["jira:issue_created", "jira:issue_updated"])
@pytest.mark.parametrize("user", [None, {}, {"displayName": ""}, {"displayName": 3}, "Sam"])
def test_issue_event_without_actor_name_is_someone(name: str, user: object) -> None:
    raw = load_fixture(name)
    raw["user"] = user
    assert _parse(raw).actor == "someone"


def test_comment_actor_is_the_comment_author_not_the_user() -> None:
    raw = load_fixture("comment_created")
    raw["user"] = {"displayName": "Sam"}
    assert _parse(raw).actor == "Robin"
    del raw["comment"]["author"]
    assert _parse(raw).actor == "someone"


@pytest.mark.parametrize("name", EVENTS)
@pytest.mark.parametrize("summary", [None, 5, ["x"]])
def test_summary_that_is_not_a_string_is_empty(name: str, summary: object) -> None:
    raw = load_fixture(name)
    raw["issue"]["fields"]["summary"] = summary
    assert _parse(raw).summary == ""


# Unknown members.


@pytest.mark.parametrize("name", EVENTS)
def test_unknown_members_are_ignored(name: str) -> None:
    plain = load_fixture(name)
    raw = copy.deepcopy(plain)
    raw["unknownTop"] = {"nested": [1, 2, 3]}
    raw["issue"]["unknownIssue"] = "x"
    raw["issue"]["fields"]["customfield_10099"] = {"value": "y"}
    raw["issue"]["fields"]["project"]["unknownProject"] = True
    if "comment" in raw:
        raw["comment"]["unknownComment"] = None
        raw["comment"]["author"]["unknownAuthor"] = 1
    else:
        raw["user"]["unknownUser"] = []
    if "changelog" in raw:
        raw["changelog"]["unknownChangelog"] = 0
        for item in raw["changelog"]["items"]:
            item["unknownItem"] = "z"
    assert _parse(raw) == _parse(plain)


@pytest.mark.parametrize("name", EVENTS)
def test_parsed_event_keeps_only_the_used_fields(name: str) -> None:
    stored = _parse(load_fixture(name)).to_json()
    assert set(stored) == {
        "event",
        "issue_id",
        "issue_key",
        "project",
        "summary",
        "actor",
        "text",
        "changes",
        "comment_id",
    }
    assert "jira.example.com" not in repr(stored)
    assert "example-account" not in repr(stored)
