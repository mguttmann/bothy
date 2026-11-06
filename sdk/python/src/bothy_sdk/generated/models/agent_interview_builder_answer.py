from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define

from ..models.agent_interview_builder_answer_kind import AgentInterviewBuilderAnswerKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_draft import AgentDraft
    from ..models.cost_report import CostReport


T = TypeVar("T", bound="AgentInterviewBuilderAnswer")


@_attrs_define
class AgentInterviewBuilderAnswer:
    """Answer of the builder agent to `agent.interview_requested` (action `agent.interview.respond`, 26 6.1 item 2). It has
    no member for settings files, hooks, permission rules, commands, subagents or plugin parts (26 4.3 item 1).

        Attributes:
            kind (AgentInterviewBuilderAnswerKind):
            questions (list[str] | Unset): Only with `kind = questions`; each at most `agents.interview.answer_max_chars`
                characters.
            draft (AgentDraft | Unset): A definition draft of the builder (26 6.1 item 2): the members of `AgentDefinition`
                plus the agent's name, handle, description and an optional channel suggestion. The server checks it like a
                definition for the class of `settings.tool_class_wish` and against the template catalogue; it never stores it.
            costs (list[CostReport] | Unset): Usage of the model call, as in `run.finish`.
    """

    kind: AgentInterviewBuilderAnswerKind
    questions: list[str] | Unset = UNSET
    draft: AgentDraft | Unset = UNSET
    costs: list[CostReport] | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        questions: list[str] | Unset = UNSET
        if not isinstance(self.questions, Unset):
            questions = self.questions

        draft: dict[str, Any] | Unset = UNSET
        if not isinstance(self.draft, Unset):
            draft = self.draft.to_dict()

        costs: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.costs, Unset):
            costs = []
            for costs_item_data in self.costs:
                costs_item = costs_item_data.to_dict()
                costs.append(costs_item)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "kind": kind,
            }
        )
        if questions is not UNSET:
            field_dict["questions"] = questions
        if draft is not UNSET:
            field_dict["draft"] = draft
        if costs is not UNSET:
            field_dict["costs"] = costs

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_draft import AgentDraft
        from ..models.cost_report import CostReport

        d = dict(src_dict)
        kind = AgentInterviewBuilderAnswerKind(d.pop("kind"))

        questions = cast(list[str], d.pop("questions", UNSET))

        _draft = d.pop("draft", UNSET)
        draft: AgentDraft | Unset
        if isinstance(_draft, Unset):
            draft = UNSET
        else:
            draft = AgentDraft.from_dict(_draft)

        _costs = d.pop("costs", UNSET)
        costs: list[CostReport] | Unset = UNSET
        if _costs is not UNSET:
            costs = []
            for costs_item_data in _costs:
                costs_item = CostReport.from_dict(costs_item_data)

                costs.append(costs_item)

        agent_interview_builder_answer = cls(
            kind=kind,
            questions=questions,
            draft=draft,
            costs=costs,
        )

        return agent_interview_builder_answer
