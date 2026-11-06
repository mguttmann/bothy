from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_interview_answer_fallback_reason import (
    AgentInterviewAnswerFallbackReason,
)
from ..models.agent_interview_answer_outcome import AgentInterviewAnswerOutcome
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.agent_draft import AgentDraft
    from ..models.agent_interview_cost import AgentInterviewCost
    from ..models.field_error import FieldError


T = TypeVar("T", bound="AgentInterviewAnswer")


@_attrs_define
class AgentInterviewAnswer:
    """The answer of `interviewAgent` (26 6.1). Nothing of it is stored.

    Attributes:
        outcome (AgentInterviewAnswerOutcome):
        questions (list[str] | Unset): One or two questions of the builder; only with `outcome = questions`.
        draft (AgentDraft | Unset): A definition draft of the builder (26 6.1 item 2): the members of `AgentDefinition`
            plus the agent's name, handle, description and an optional channel suggestion. The server checks it like a
            definition for the class of `settings.tool_class_wish` and against the template catalogue; it never stores it.
        errors (list[FieldError] | Unset): Only with `outcome = draft`; the findings of 26 4.3 and of the catalogue
            check (`mcp_not_in_catalogue`, 26 7.4) as JSON Pointers into the draft, empty when the draft passes for the
            class of its `settings.tool_class_wish`.
        fallback_reason (AgentInterviewAnswerFallbackReason | Unset): Only with `outcome = fallback`; the apps then
            offer the templates and the manual form.
        cost_estimate (AgentInterviewCost | Unset): Estimate of the builder's model call, for information only; not
            booked as a cost entry.
    """

    outcome: AgentInterviewAnswerOutcome
    questions: list[str] | Unset = UNSET
    draft: AgentDraft | Unset = UNSET
    errors: list[FieldError] | Unset = UNSET
    fallback_reason: AgentInterviewAnswerFallbackReason | Unset = UNSET
    cost_estimate: AgentInterviewCost | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        outcome = self.outcome.value

        questions: list[str] | Unset = UNSET
        if not isinstance(self.questions, Unset):
            questions = self.questions

        draft: dict[str, Any] | Unset = UNSET
        if not isinstance(self.draft, Unset):
            draft = self.draft.to_dict()

        errors: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.errors, Unset):
            errors = []
            for errors_item_data in self.errors:
                errors_item = errors_item_data.to_dict()
                errors.append(errors_item)

        fallback_reason: str | Unset = UNSET
        if not isinstance(self.fallback_reason, Unset):
            fallback_reason = self.fallback_reason.value

        cost_estimate: dict[str, Any] | Unset = UNSET
        if not isinstance(self.cost_estimate, Unset):
            cost_estimate = self.cost_estimate.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "outcome": outcome,
            }
        )
        if questions is not UNSET:
            field_dict["questions"] = questions
        if draft is not UNSET:
            field_dict["draft"] = draft
        if errors is not UNSET:
            field_dict["errors"] = errors
        if fallback_reason is not UNSET:
            field_dict["fallback_reason"] = fallback_reason
        if cost_estimate is not UNSET:
            field_dict["cost_estimate"] = cost_estimate

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_draft import AgentDraft
        from ..models.agent_interview_cost import AgentInterviewCost
        from ..models.field_error import FieldError

        d = dict(src_dict)
        outcome = AgentInterviewAnswerOutcome(d.pop("outcome"))

        questions = cast(list[str], d.pop("questions", UNSET))

        _draft = d.pop("draft", UNSET)
        draft: AgentDraft | Unset
        if isinstance(_draft, Unset):
            draft = UNSET
        else:
            draft = AgentDraft.from_dict(_draft)

        _errors = d.pop("errors", UNSET)
        errors: list[FieldError] | Unset = UNSET
        if _errors is not UNSET:
            errors = []
            for errors_item_data in _errors:
                errors_item = FieldError.from_dict(errors_item_data)

                errors.append(errors_item)

        _fallback_reason = d.pop("fallback_reason", UNSET)
        fallback_reason: AgentInterviewAnswerFallbackReason | Unset
        if isinstance(_fallback_reason, Unset):
            fallback_reason = UNSET
        else:
            fallback_reason = AgentInterviewAnswerFallbackReason(_fallback_reason)

        _cost_estimate = d.pop("cost_estimate", UNSET)
        cost_estimate: AgentInterviewCost | Unset
        if isinstance(_cost_estimate, Unset):
            cost_estimate = UNSET
        else:
            cost_estimate = AgentInterviewCost.from_dict(_cost_estimate)

        agent_interview_answer = cls(
            outcome=outcome,
            questions=questions,
            draft=draft,
            errors=errors,
            fallback_reason=fallback_reason,
            cost_estimate=cost_estimate,
        )

        agent_interview_answer.additional_properties = d
        return agent_interview_answer

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
