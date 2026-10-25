# SPDX-License-Identifier: Apache-2.0
"""Atlassian Document Format (ADF): reading rich text of events, building text for Jira.

Reading (brief M2-50 Design note 6): a string is taken as it is; an object of ``type`` ``doc``
contributes the ``text`` of its ``text`` nodes in document order, the ``attrs.text`` of
``mention`` nodes, and a line break after each ``paragraph``, ``heading``, ``codeBlock``,
``blockquote`` and ``listItem`` node and for each ``hardBreak``; every other form gives no text.
The page cited in docs/spec/11-extensions.md 10.2 does not fix the form of rich text in
payloads, and the app must not drop events if it changes (RK-83).

Building (Design notes 13 and 15): one function, `document`, wraps nodes into
``{"type": "doc", "version": 1, "content": [...]}``; `paragraphs` makes one ``paragraph`` per
line (an empty line is a paragraph without content), `code_block` a ``codeBlock``, and `text`
nodes are never empty. Text is never parsed as markup, so chat text cannot create a Jira
mention, link or macro.

Examples:
    >>> read_text({"type": "doc", "content": [{"type": "paragraph",
    ...     "content": [{"type": "text", "text": "Hi"}]}]}) == "Hi" + chr(10)
    True
    >>> document(paragraphs(chr(10).join(["a", "", "b"])))["content"][1]
    {'type': 'paragraph'}
"""

from __future__ import annotations

from collections.abc import Sequence
from typing import Any

__all__ = [
    "BLOCK_TYPES",
    "code_block",
    "document",
    "issue_description",
    "mirror_comment",
    "paragraph",
    "paragraphs",
    "read_text",
    "text_node",
]

BLOCK_TYPES = frozenset({"paragraph", "heading", "codeBlock", "blockquote", "listItem"})
"""Nodes after which reading adds a line break."""

AdfNode = dict[str, Any]

_END = object()


def read_text(value: object) -> str:
    """Return the text of a rich-text member of a Jira payload.

    The walk is iterative, so a deeply nested document cannot exhaust the stack.

    Args:
        value: The decoded member (``issue.fields.description`` or ``comment.body``).

    Returns:
        The text; empty for any form other than a string or an ADF document.
    """
    if isinstance(value, str):
        return value
    if not isinstance(value, dict) or value.get("type") != "doc":
        return ""
    out: list[str] = []
    stack: list[object] = [value]
    while stack:
        node = stack.pop()
        if node is _END:
            out.append("\n")
            continue
        if not isinstance(node, dict):
            continue
        kind = node.get("type")
        if kind == "text":
            text = node.get("text")
            if isinstance(text, str):
                out.append(text)
            continue
        if kind == "mention":
            attrs = node.get("attrs")
            text = attrs.get("text") if isinstance(attrs, dict) else None
            if isinstance(text, str):
                out.append(text)
            continue
        if kind == "hardBreak":
            out.append("\n")
            continue
        if kind in BLOCK_TYPES:
            stack.append(_END)
        content = node.get("content")
        if isinstance(content, list):
            stack.extend(reversed(content))
    return "".join(out)


def text_node(text: str, *, code: bool = False) -> AdfNode:
    """Return a ``text`` node.

    Args:
        text: The text; must not be empty.
        code: Whether the node carries the ``code`` mark.

    Returns:
        The node.

    Raises:
        ValueError: ``text`` is empty (ADF forbids empty text nodes).
    """
    if text == "":
        raise ValueError("an ADF text node must not be empty")
    node: AdfNode = {"type": "text", "text": text}
    if code:
        node["marks"] = [{"type": "code"}]
    return node


def paragraph(nodes: Sequence[AdfNode] = ()) -> AdfNode:
    """Return a ``paragraph`` node; without nodes it has no ``content``.

    Args:
        nodes: Inline nodes.

    Returns:
        The node.
    """
    node: AdfNode = {"type": "paragraph"}
    if nodes:
        node["content"] = list(nodes)
    return node


def paragraphs(text: str) -> list[AdfNode]:
    """Return one ``paragraph`` per line of ``text``.

    Args:
        text: Plain text; CR LF and CR count as line breaks.

    Returns:
        The paragraphs; an empty line gives a paragraph without content.
    """
    lines = text.replace("\r\n", "\n").replace("\r", "\n").split("\n")
    return [paragraph([text_node(line)] if line else []) for line in lines]


def code_block(text: str) -> AdfNode:
    """Return a ``codeBlock`` with ``text`` unchanged.

    Args:
        text: The literal content.

    Returns:
        The node; an empty text gives a block without content.
    """
    node: AdfNode = {"type": "codeBlock"}
    if text:
        node["content"] = [text_node(text)]
    return node


def document(content: Sequence[AdfNode]) -> AdfNode:
    """Return an ADF document of version 1.

    Args:
        content: Block nodes.

    Returns:
        ``{"type": "doc", "version": 1, "content": [...]}``.
    """
    return {"type": "doc", "version": 1, "content": list(content)}


CREATED_FROM_CHAT = "Created from chat."
"""The last paragraph of the description of an issue created through ``/issue``."""


def issue_description(text: str) -> AdfNode:
    """Return the description of an issue of ``/issue``: the form text, then a fixed line.

    Args:
        text: The submitted description; empty gives only the fixed line.

    Returns:
        The ADF document whose last paragraph is `CREATED_FROM_CHAT`.
    """
    body = paragraphs(text) if text.strip() else []
    return document([*body, paragraph([text_node(CREATED_FROM_CHAT)])])


def mirror_comment(display_name: str, content: str) -> AdfNode:
    """Return the body of a mirrored chat reply (Design note 13).

    Args:
        display_name: The author's display name in the chat.
        content: The message content, kept literal.

    Returns:
        A paragraph with the name as a ``code`` text node and " wrote in chat:", then one
        ``codeBlock`` with the content.
    """
    name = display_name if display_name else "someone"
    return document(
        [
            paragraph([text_node(name, code=True), text_node(" wrote in chat:")]),
            code_block(content),
        ]
    )
