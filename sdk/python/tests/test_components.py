# SPDX-License-Identifier: Apache-2.0
"""Components of `component.v1` (docs/spec/09-agents.md 11.1, 11.5 and 11.6).

The builders return generated `ComponentCreate` objects and refuse, before anything is sent,
what the server would refuse with `400 validation_failed`: styles and field types outside the
vocabularies, the limits `component.max_per_message`, `component.custom_id_max_chars` and
`component.form_max_fields`, and the rules of the definitions. Error texts name the field,
never the text a caller passed.
"""

from __future__ import annotations

import re
from collections.abc import Callable
from datetime import UTC, datetime
from functools import cache
from pathlib import Path

import pytest

from bothy_sdk.components import (
    BUTTON_STYLES,
    FORM_FIELD_TYPES,
    FormField,
    SelectOption,
    approval_card,
    button,
    check_components,
    form,
    question_card,
    select,
)
from bothy_sdk.generated.models import ComponentCreate
from bothy_sdk.settings import SdkSettings

SMALL = SdkSettings(
    component_max_per_message=3, component_custom_id_max_chars=12, component_form_max_fields=2
)


def repo_root() -> Path:
    """The repository root: the nearest parent directory that holds docs/api."""
    for parent in Path(__file__).resolve().parents:
        if (parent / "docs" / "api").is_dir():
            return parent
    raise AssertionError("repository root with docs/api not found")


@cache
def chapter() -> str:
    """docs/spec/09-agents.md."""
    return (repo_root() / "docs" / "spec" / "09-agents.md").read_text(encoding="utf-8")


def section(number: str) -> str:
    """The text of section `number` of chapter 09 up to the next heading."""
    found = re.search(rf"^### {re.escape(number)} .*?(?=^#)", chapter(), re.MULTILINE | re.DOTALL)
    assert found is not None
    return found.group(0)


def refuses(field: str, call: Callable[[], object], value: str | None = None) -> None:
    """`call` raises ValueError naming `field` and never showing `value`."""
    with pytest.raises(ValueError, match=re.escape(field)) as caught:
        call()
    if value:
        assert value not in str(caught.value)


def test_button_select_form_build_generated_models() -> None:
    expires = datetime(2026, 10, 21, 12, 0, tzinfo=UTC)
    ok = button("ok", "Looks good", "yes", style="success", icon="check", expires_at=expires)
    assert isinstance(ok, ComponentCreate)
    assert ok.to_dict() == {
        "custom_id": "ok",
        "type": "button",
        "definition": {"label": "Looks good", "style": "success", "icon": "check", "value": "yes"},
        "expires_at": expires.isoformat(),
    }
    assert button("b", "Label", "v").to_dict()["definition"] == {
        "label": "Label",
        "style": "secondary",
        "value": "v",
    }

    pick = select(
        "room",
        [SelectOption("Room A", "a", description="Ground floor"), SelectOption("Room B", "b")],
        placeholder="Pick a room",
        min_values=1,
        max_values=2,
        single_use=True,
        restricted_to="run_starter",
    )
    assert pick.to_dict() == {
        "custom_id": "room",
        "type": "select",
        "definition": {
            "placeholder": "Pick a room",
            "options": [
                {"label": "Room A", "value": "a", "description": "Ground floor"},
                {"label": "Room B", "value": "b"},
            ],
            "min_values": 1,
            "max_values": 2,
        },
        "single_use": True,
        "restricted_to": "run_starter",
    }

    survey = form(
        "survey",
        "Trip details",
        [
            FormField("destination", "Destination", "text", required=True, max_length=80),
            FormField("class", "Class", "choice", choices=("economy", "business")),
        ],
        submit_label="Send",
    )
    assert survey.to_dict()["definition"] == {
        "title": "Trip details",
        "submit_label": "Send",
        "fields": [
            {
                "name": "destination",
                "label": "Destination",
                "type": "text",
                "required": True,
                "max_length": 80,
            },
            {
                "name": "class",
                "label": "Class",
                "type": "choice",
                "required": False,
                "choices": ["economy", "business"],
            },
        ],
    }
    assert ComponentCreate.from_dict(survey.to_dict()).to_dict() == survey.to_dict()


def test_vocabularies_match_spec() -> None:
    kinds = section("11.1")
    button_row = re.search(r"^\| `button` \|.*?`style` \((.*?)\)", kinds, re.MULTILINE)
    form_row = re.search(r"^\| `form` \|.*?`type` \S+ (.*?);", kinds, re.MULTILINE)
    assert button_row is not None
    assert form_row is not None
    assert tuple(re.findall(r"`([a-z]+)`", button_row.group(1))) == BUTTON_STYLES
    assert tuple(re.findall(r"`([a-z]+)`", form_row.group(1))) == FORM_FIELD_TYPES

    for style in BUTTON_STYLES:
        assert button("b", "Label", "v", style=style).to_dict()["definition"]["style"] == style
    refuses("style", lambda: button("b", "Label", "v", style="red"), "red")
    refuses("style", lambda: button("b", "Label", "v", style="#ff0000"), "#ff0000")
    refuses("fields[0].type", lambda: form("f", "T", [FormField("a", "A", "date")]))
    refuses(
        "required_capability", lambda: button("b", "Label", "v", required_capability="terminal")
    )
    refuses(
        "required_capability", lambda: button("b", "Label", "v", required_capability="superuser")
    )
    refuses("restricted_to", lambda: button("b", "Label", "v", restricted_to="admins"))
    refuses(
        "expires_at", lambda: button("b", "Label", "v", expires_at=datetime(2026, 10, 21, 12, 0))
    )


def test_limits_from_settings() -> None:
    assert button("x" * 12, "Label", "v", settings=SMALL).custom_id == "x" * 12
    refuses("custom_id", lambda: button("x" * 13, "Label", "v", settings=SMALL))
    refuses("custom_id", lambda: button("", "Label", "v"))
    default_limit = SdkSettings().component_custom_id_max_chars
    assert button("y" * default_limit, "Label", "v").custom_id == "y" * default_limit
    refuses("custom_id", lambda: button("y" * (default_limit + 1), "Label", "v"))

    two = [FormField("a", "A", "text"), FormField("b", "B", "number")]
    assert len(form("f", "T", two, settings=SMALL).to_dict()["definition"]["fields"]) == 2
    refuses(
        "fields", lambda: form("f", "T", [*two, FormField("c", "C", "boolean")], settings=SMALL)
    )
    refuses("fields", lambda: form("f", "T", []))

    three = [button(f"b{i}", "Label", "v") for i in range(3)]
    check_components(three, SMALL)
    refuses("components", lambda: check_components([*three, button("b3", "Label", "v")], SMALL))
    refuses("components", lambda: check_components([three[0], three[0]], SMALL))
    refuses(
        "components[1].custom_id",
        lambda: check_components(
            [three[0], ComponentCreate.from_dict({**three[1].to_dict(), "custom_id": "z" * 13})],
            SMALL,
        ),
    )
    check_components(
        [button(f"c{i}", "L", "v") for i in range(SdkSettings().component_max_per_message)]
    )


def test_definition_rules() -> None:
    # select: options, unique values, bounds.
    one = [SelectOption("A", "a")]
    refuses("options", lambda: select("s", []))
    refuses("options[1].value", lambda: select("s", [*one, SelectOption("B", "a")]))
    refuses("max_values", lambda: select("s", one, max_values=2))
    refuses("max_values", lambda: select("s", [*one, SelectOption("B", "b")], max_values=0))
    refuses("min_values", lambda: select("s", one, min_values=-1))
    refuses(
        "min_values",
        lambda: select("s", [*one, SelectOption("B", "b")], min_values=2, max_values=1),
    )
    refuses("min_values", lambda: select("s", one, min_values=True))
    assert select("s", one, min_values=0).to_dict()["definition"]["min_values"] == 0

    # form: names, max_length only for text and textarea, choices only for choice.
    refuses("fields[0].name", lambda: form("f", "T", [FormField("Bad Name", "A", "text")]))
    refuses("fields[0].name", lambda: form("f", "T", [FormField("1st", "A", "text")]))
    refuses(
        "fields[1].name",
        lambda: form("f", "T", [FormField("a", "A", "text"), FormField("a", "B", "number")]),
    )
    refuses(
        "fields[0].max_length",
        lambda: form("f", "T", [FormField("a", "A", "boolean", max_length=4)]),
    )
    refuses(
        "fields[0].max_length",
        lambda: form("f", "T", [FormField("a", "A", "textarea", max_length=0)]),
    )
    refuses("fields[0].choices", lambda: form("f", "T", [FormField("a", "A", "choice")]))
    refuses(
        "fields[0].choices", lambda: form("f", "T", [FormField("a", "A", "text", choices=("x",))])
    )
    refuses(
        "fields[0].choices",
        lambda: form("f", "T", [FormField("a", "A", "choice", choices=("x", "x"))]),
    )
    refuses("title", lambda: form("f", "", [FormField("a", "A", "text")]))
    refuses("label", lambda: button("b", "", "v"))
    refuses("value", lambda: button("b", "Label", ""))


@pytest.mark.parametrize("text", ["line\nbreak", "tab\there", "bidi\u202eevil", "nul\x00"])
def test_display_texts_refuse_control_and_bidi(text: str) -> None:
    refuses("label", lambda: button("b", text, "v"), text)
    refuses("value", lambda: button("b", "Label", text), text)
    refuses("custom_id", lambda: button(text, "Label", "v"), text)
    refuses(
        "options[0].description",
        lambda: select("s", [SelectOption("A", "a", description=text)]),
        text,
    )
    refuses(
        "fields[0].choices",
        lambda: form("f", "T", [FormField("a", "A", "choice", choices=(text,))]),
        text,
    )
    # Emoji and accented letters are display text like any other.
    assert button("b", "Caf\u00e9 \U0001f44d\u200d", "v").to_dict()["definition"]["label"]


def test_approval_card_matches_spec_table() -> None:
    rows = re.findall(
        r"^\| [^|`]+ \| `([a-z]+)` \| (`[a-z_]+`|none) \| `([a-z_]+)` \|$",
        section("11.5"),
        re.MULTILINE,
    )
    rows = [r for r in rows if r[0] != "value"]  # the header row names the columns
    assert [r[0] for r in rows] == ["once", "session", "always", "deny"]

    card = approval_card()
    assert [c.custom_id for c in card] == [f"approval.{r[0]}" for r in rows]
    for component, (value, capability, restricted) in zip(card, rows, strict=True):
        body = component.to_dict()
        assert body["type"] == "button"
        assert body["definition"]["value"] == value
        assert body["definition"]["label"]
        assert body.get("required_capability") == (
            None if capability == "none" else capability.strip("`")
        )
        assert body["restricted_to"] == restricted
        # One decision per card (09 11.3).
        assert body["single_use"] is True
    assert card[3].to_dict()["definition"]["style"] == "danger"

    without = approval_card(with_always=False)
    assert [c.custom_id for c in without] == ["approval.once", "approval.session", "approval.deny"]
    check_components(card)


def test_question_card() -> None:
    answers = [SelectOption("Friday", "fri"), SelectOption("Monday", "mon")]

    buttons = question_card("when", answers)
    assert [c.custom_id for c in buttons] == ["when.0", "when.1"]
    assert [c.to_dict()["definition"]["value"] for c in buttons] == ["fri", "mon"]
    assert [c.to_dict()["definition"]["label"] for c in buttons] == ["Friday", "Monday"]
    # The first answer wins and disables the rest (09 11.6); only the run's starter answers.
    assert all(c.single_use is True for c in buttons)
    assert all(c.to_dict()["restricted_to"] == "run_starter" for c in buttons)

    [pick] = question_card("when", answers, as_select=True, placeholder="Pick a day")
    assert pick.to_dict()["type"] == "select"
    assert pick.to_dict()["definition"]["options"] == [
        {"label": "Friday", "value": "fri"},
        {"label": "Monday", "value": "mon"},
    ]
    assert (
        pick.to_dict()["definition"]["min_values"],
        pick.to_dict()["definition"]["max_values"],
    ) == (1, 1)
    assert pick.single_use is True

    refuses("answers", lambda: question_card("when", []))
    refuses(
        "components",
        lambda: question_card(
            "q", [SelectOption(str(i), str(i)) for i in range(4)], settings=SMALL
        ),
    )
    refuses("custom_id", lambda: question_card("q" * 11, answers, settings=SMALL))
    assert (
        len(
            question_card(
                "q",
                [SelectOption(str(i), str(i)) for i in range(4)],
                as_select=True,
                settings=SMALL,
            )
        )
        == 1
    )
