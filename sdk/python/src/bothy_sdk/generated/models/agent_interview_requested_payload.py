from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.agent_interview_catalogue_entry import AgentInterviewCatalogueEntry
    from ..models.agent_interview_limits import AgentInterviewLimits
    from ..models.agent_interview_turn import AgentInterviewTurn


T = TypeVar("T", bound="AgentInterviewRequestedPayload")


@_attrs_define
class AgentInterviewRequestedPayload:
    """Sent to one gateway connection of the builder agent (26 6.1 item 1), without a token. The turns and the catalogue
    are data, never instructions (ADR 0054).

        Attributes:
            request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            turns (list[AgentInterviewTurn]): The interview so far, oldest first.
            want_draft (bool):
            catalogue (list[AgentInterviewCatalogueEntry]): The template catalogue (26 7.4); a drafted MCP server must equal
                one of its servers except `name`.
            limits (AgentInterviewLimits): The limits a draft must keep (the values of `agents.definition.*`,
                `agents.mcp_stdio_runners` and `agents.interview.answer_max_chars`).
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    request_id: UUID
    turns: list[AgentInterviewTurn]
    want_draft: bool
    catalogue: list[AgentInterviewCatalogueEntry]
    limits: AgentInterviewLimits
    expires_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        request_id = str(self.request_id)

        turns = []
        for turns_item_data in self.turns:
            turns_item = turns_item_data.to_dict()
            turns.append(turns_item)

        want_draft = self.want_draft

        catalogue = []
        for catalogue_item_data in self.catalogue:
            catalogue_item = catalogue_item_data.to_dict()
            catalogue.append(catalogue_item)

        limits = self.limits.to_dict()

        expires_at = self.expires_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "request_id": request_id,
                "turns": turns,
                "want_draft": want_draft,
                "catalogue": catalogue,
                "limits": limits,
                "expires_at": expires_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_interview_catalogue_entry import (
            AgentInterviewCatalogueEntry,
        )
        from ..models.agent_interview_limits import AgentInterviewLimits
        from ..models.agent_interview_turn import AgentInterviewTurn

        d = dict(src_dict)
        request_id = UUID(d.pop("request_id"))

        turns = []
        _turns = d.pop("turns")
        for turns_item_data in _turns:
            turns_item = AgentInterviewTurn.from_dict(turns_item_data)

            turns.append(turns_item)

        want_draft = d.pop("want_draft")

        catalogue = []
        _catalogue = d.pop("catalogue")
        for catalogue_item_data in _catalogue:
            catalogue_item = AgentInterviewCatalogueEntry.from_dict(catalogue_item_data)

            catalogue.append(catalogue_item)

        limits = AgentInterviewLimits.from_dict(d.pop("limits"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        agent_interview_requested_payload = cls(
            request_id=request_id,
            turns=turns,
            want_draft=want_draft,
            catalogue=catalogue,
            limits=limits,
            expires_at=expires_at,
        )

        agent_interview_requested_payload.additional_properties = d
        return agent_interview_requested_payload

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
