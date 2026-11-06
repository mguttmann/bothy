from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_interview_turn import AgentInterviewTurn


T = TypeVar("T", bound="AgentInterviewRequest")


@_attrs_define
class AgentInterviewRequest:
    """The interview so far, kept by the client (26 6.1). Nothing of it is stored.

    Attributes:
        turns (list[AgentInterviewTurn]): The builder's earlier questions and the person's answers, oldest first; at
            most `agents.interview.max_turns`. Empty for the start of an interview.
        want_draft (bool | Unset): The person asks for a draft now instead of further questions.
    """

    turns: list[AgentInterviewTurn]
    want_draft: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        turns = []
        for turns_item_data in self.turns:
            turns_item = turns_item_data.to_dict()
            turns.append(turns_item)

        want_draft = self.want_draft

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "turns": turns,
            }
        )
        if want_draft is not UNSET:
            field_dict["want_draft"] = want_draft

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_interview_turn import AgentInterviewTurn

        d = dict(src_dict)
        turns = []
        _turns = d.pop("turns")
        for turns_item_data in _turns:
            turns_item = AgentInterviewTurn.from_dict(turns_item_data)

            turns.append(turns_item)

        want_draft = d.pop("want_draft", UNSET)

        agent_interview_request = cls(
            turns=turns,
            want_draft=want_draft,
        )

        return agent_interview_request
