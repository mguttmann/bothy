# SPDX-License-Identifier: Apache-2.0
"""Atlassian Document Format built for Jira and read from Jira (brief M2-50 Design notes 6, 13
and 15; Tests bullet ``test_adf.py``).

Proves that built documents have ``type`` ``doc`` and ``version`` 1, contain no empty ``text``
node, hold one paragraph per line (an empty line is a paragraph without content), carry the
``code`` mark only on the display name of a mirrored comment, keep the content of a
``codeBlock`` unchanged (backticks, ``[~accountid:x]``, ``{code}`` and ``@alex`` stay text),
and that the ``/issue`` description ends with the paragraph "Created from chat.". The reading
direction of Design note 6 (rich text to plain text) lives in the same module and is covered
here too.
"""

from __future__ import annotations

from collections.abc import Iterator
from typing import Any

import pytest

from jira_app.jira.adf import (
    CREATED_FROM_CHAT,
    code_block,
    document,
    issue_description,
    mirror_comment,
    paragraph,
    paragraphs,
    read_text,
    text_node,
)

LITERAL = (
    "use `x` and ```py\nprint(1)\n``` [~accountid:x] {code}y{code} @alex [a](https://example.com)"
)
"""Chat text full of Jira and Markdown markup that must stay literal text."""


def walk(node: Any) -> Iterator[dict[str, Any]]:
    """Yield every node of an ADF tree, the root first."""
    stack: list[Any] = [node]
    while stack:
        current = stack.pop()
        assert isinstance(current, dict)
        yield current
        content = current.get("content")
        if content is not None:
            assert isinstance(content, list)
            stack.extend(reversed(content))


def built_documents() -> list[dict[str, Any]]:
    """Documents of every builder, with edge cases (empty lines, empty text, empty name)."""
    return [
        document(paragraphs("one\n\ntwo\n")),
        document(paragraphs("")),
        document([code_block("")]),
        document([code_block(LITERAL)]),
        issue_description(""),
        issue_description("   \n "),
        issue_description("first\r\n\r\nthird\rfourth"),
        issue_description(LITERAL),
        mirror_comment("Alex", LITERAL),
        mirror_comment("", ""),
        mirror_comment("Sam", "\n"),
    ]


@pytest.mark.parametrize("doc", built_documents())
def test_built_documents_are_doc_version_1(doc: dict[str, Any]) -> None:
    assert doc["type"] == "doc"
    assert doc["version"] == 1
    assert isinstance(doc["content"], list)
    assert doc["content"], "a built document has at least one block"
    for child in walk(doc):
        if child is not doc:
            assert child["type"] != "doc"


@pytest.mark.parametrize("doc", built_documents())
def test_no_empty_text_node_anywhere(doc: dict[str, Any]) -> None:
    texts = [n for n in walk(doc) if n["type"] == "text"]
    for node in texts:
        assert isinstance(node["text"], str)
        assert node["text"] != ""
    for node in walk(doc):
        if "content" in node:
            assert node["content"] != [], "an empty block has no content member at all"


def test_text_node_refuses_empty_text() -> None:
    with pytest.raises(ValueError, match="empty"):
        text_node("")
    with pytest.raises(ValueError, match="empty"):
        text_node("", code=True)


def test_one_paragraph_per_line() -> None:
    doc = document(paragraphs("alpha\n\nbeta\n"))
    assert doc["content"] == [
        {"type": "paragraph", "content": [{"type": "text", "text": "alpha"}]},
        {"type": "paragraph"},
        {"type": "paragraph", "content": [{"type": "text", "text": "beta"}]},
        {"type": "paragraph"},
    ]


def test_cr_lf_and_cr_count_as_line_breaks() -> None:
    assert paragraphs("a\r\nb\rc") == paragraphs("a\nb\nc")
    assert len(paragraphs("a\r\n\r\nb")) == 3


def test_an_empty_line_is_a_paragraph_without_content() -> None:
    assert paragraphs("") == [{"type": "paragraph"}]
    assert paragraph() == {"type": "paragraph"}


def test_lines_keep_their_spaces_and_are_not_parsed() -> None:
    doc = document(paragraphs("  *bold* _it_ [~accountid:x]  "))
    assert doc["content"] == [
        {
            "type": "paragraph",
            "content": [{"type": "text", "text": "  *bold* _it_ [~accountid:x]  "}],
        }
    ]


def test_no_builder_emits_marks_other_than_the_display_name_code() -> None:
    for doc in built_documents():
        marked = [n for n in walk(doc) if "marks" in n]
        for node in marked:
            assert node["marks"] == [{"type": "code"}]
            assert node["type"] == "text"


def test_code_mark_only_on_the_display_name() -> None:
    doc = mirror_comment("Alex", "hello @sam")
    first, block = doc["content"]
    assert first == {
        "type": "paragraph",
        "content": [
            {"type": "text", "text": "Alex", "marks": [{"type": "code"}]},
            {"type": "text", "text": " wrote in chat:"},
        ],
    }
    assert block == {"type": "codeBlock", "content": [{"type": "text", "text": "hello @sam"}]}
    marked = [n for n in walk(doc) if "marks" in n]
    assert marked == [first["content"][0]]


def test_issue_description_carries_no_code_mark() -> None:
    doc = issue_description("`Alex` wrote:\n@sam")
    assert [n for n in walk(doc) if "marks" in n] == []


def test_mirror_comment_without_display_name_names_someone() -> None:
    doc = mirror_comment("", "hi")
    name = doc["content"][0]["content"][0]
    assert name == {"type": "text", "text": "someone", "marks": [{"type": "code"}]}


@pytest.mark.parametrize(
    "content",
    [
        LITERAL,
        "```",
        "`single` and ``double``",
        "[~accountid:x]",
        "[~accountid:557058:0a1b2c3d]",
        "{code}\nraw\n{code}",
        "{noformat}x{noformat} {panel}p{panel}",
        "@alex @Sam @robin",
        "line one\n\nline three\r\n",
        "  leading and trailing  ",
        "<b>html</b> &amp; https://example.com/a?b=c",
    ],
)
def test_code_block_content_unchanged(content: str) -> None:
    for doc in (document([code_block(content)]), mirror_comment("Robin", content)):
        blocks = [n for n in walk(doc) if n["type"] == "codeBlock"]
        assert len(blocks) == 1
        assert blocks[0]["content"] == [{"type": "text", "text": content}]
        inside = list(walk(blocks[0]))
        assert {n["type"] for n in inside} == {"codeBlock", "text"}
        assert not any("marks" in n for n in inside)


def test_markup_never_becomes_mention_link_or_macro() -> None:
    docs = [
        mirror_comment("@alex [~accountid:x]", LITERAL),
        issue_description(LITERAL),
        document(paragraphs(LITERAL)),
    ]
    allowed = {"doc", "paragraph", "text", "codeBlock"}
    for doc in docs:
        for node in walk(doc):
            assert node["type"] in allowed
            for mark in node.get("marks", []):
                assert mark["type"] == "code"


def test_display_name_with_markup_stays_one_code_text_node() -> None:
    name = "@alex [~accountid:x] `x`"
    doc = mirror_comment(name, "hi")
    assert doc["content"][0]["content"][0] == {
        "type": "text",
        "text": name,
        "marks": [{"type": "code"}],
    }


def test_empty_mirror_content_gives_a_code_block_without_content() -> None:
    doc = mirror_comment("Sam", "")
    assert doc["content"][1] == {"type": "codeBlock"}


def test_issue_description_ends_with_created_from_chat() -> None:
    doc = issue_description("Login fails\n\nSince the update.")
    assert CREATED_FROM_CHAT == "Created from chat."
    assert doc["content"] == [
        {"type": "paragraph", "content": [{"type": "text", "text": "Login fails"}]},
        {"type": "paragraph"},
        {"type": "paragraph", "content": [{"type": "text", "text": "Since the update."}]},
        {"type": "paragraph", "content": [{"type": "text", "text": "Created from chat."}]},
    ]


@pytest.mark.parametrize("text", ["", " ", "\n\n", "x", "a\nb\n", LITERAL])
def test_issue_description_last_paragraph_is_fixed_line(text: str) -> None:
    doc = issue_description(text)
    assert doc["content"][-1] == {
        "type": "paragraph",
        "content": [{"type": "text", "text": "Created from chat."}],
    }


def test_blank_issue_description_is_only_the_fixed_line() -> None:
    for text in ("", "  ", "\n", " \r\n "):
        assert issue_description(text)["content"] == [
            {"type": "paragraph", "content": [{"type": "text", "text": "Created from chat."}]}
        ]


def test_issue_description_keeps_the_text_literal() -> None:
    doc = issue_description(LITERAL)
    lines = LITERAL.split("\n")
    body = doc["content"][:-1]
    assert len(body) == len(lines)
    for block, line in zip(body, lines, strict=True):
        assert block == {"type": "paragraph", "content": [{"type": "text", "text": line}]}


# Reading direction (Design note 6).


def test_read_text_takes_a_string_as_it_is() -> None:
    assert read_text("plain *text* [~accountid:x]") == "plain *text* [~accountid:x]"
    assert read_text("") == ""


@pytest.mark.parametrize(
    "value",
    [None, 1, 1.5, True, [], ["a"], {}, {"type": "paragraph", "content": []}, {"text": "x"}],
)
def test_read_text_of_any_other_form_is_empty(value: object) -> None:
    assert read_text(value) == ""


def test_read_text_of_document_with_line_breaks_and_mentions() -> None:
    value = {
        "type": "doc",
        "version": 1,
        "content": [
            {"type": "heading", "attrs": {"level": 1}, "content": [{"type": "text", "text": "H"}]},
            {
                "type": "paragraph",
                "content": [
                    {"type": "text", "text": "Hi "},
                    {"type": "mention", "attrs": {"id": "x", "text": "@Sam"}},
                    {"type": "hardBreak"},
                    {"type": "text", "text": "next", "marks": [{"type": "strong"}]},
                ],
            },
            {"type": "codeBlock", "content": [{"type": "text", "text": "code"}]},
            {
                "type": "blockquote",
                "content": [{"type": "paragraph", "content": [{"type": "text", "text": "q"}]}],
            },
            {
                "type": "bulletList",
                "content": [
                    {
                        "type": "listItem",
                        "content": [
                            {"type": "paragraph", "content": [{"type": "text", "text": "i"}]}
                        ],
                    }
                ],
            },
            {"type": "rule"},
            {"type": "mention", "attrs": {"id": "y"}},
            {"type": "text", "text": 5},
        ],
    }
    assert read_text(value) == "H\nHi @Sam\nnext\ncode\nq\n\ni\n\n"


def test_read_text_round_trips_built_documents() -> None:
    assert read_text(issue_description("a\n\nb")) == "a\n\nb\nCreated from chat.\n"
    assert read_text(mirror_comment("Alex", "x\ny")) == "Alex wrote in chat:\nx\ny\n"


def test_read_text_survives_deep_nesting() -> None:
    node: dict[str, Any] = {"type": "text", "text": "deep"}
    for _ in range(20000):
        node = {"type": "blockquote", "content": [node]}
    value = {"type": "doc", "content": [node]}
    assert read_text(value) == "deep" + "\n" * 20000


def test_read_text_ignores_malformed_members() -> None:
    value = {
        "type": "doc",
        "content": [
            "not a node",
            {"type": "paragraph", "content": "not a list"},
            {"type": "mention", "attrs": "not a dict"},
            {"type": "paragraph", "content": [{"type": "text", "text": "ok"}]},
        ],
    }
    assert read_text(value) == "\nok\n"
