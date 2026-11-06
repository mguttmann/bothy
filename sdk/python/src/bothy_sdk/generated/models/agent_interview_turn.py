from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.agent_interview_turn_role import AgentInterviewTurnRole

T = TypeVar("T", bound="AgentInterviewTurn")


@_attrs_define
class AgentInterviewTurn:
    """
    Attributes:
        role (AgentInterviewTurnRole): `builder` for a question of the builder, `owner` for an answer of the person.
        text (str): At most `agents.interview.answer_max_chars` characters; line breaks allowed, no other control or
            bidi character.
    """

    role: AgentInterviewTurnRole
    text: str

    def to_dict(self) -> dict[str, Any]:
        role = self.role.value

        text = self.text

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "role": role,
                "text": text,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        role = AgentInterviewTurnRole(d.pop("role"))

        text = d.pop("text")

        agent_interview_turn = cls(
            role=role,
            text=text,
        )

        return agent_interview_turn
