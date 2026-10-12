from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.agent_session_end_reason import AgentSessionEndReason
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.money import Money


T = TypeVar("T", bound="AgentSession")


@_attrs_define
class AgentSession:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        thread_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        is_current (bool):
        started (bool):
        cost (Money): Amount in micro units of the currency. Never a float.
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        version (int):
        external_session_ref (str | Unset):
        model (str | Unset):
        ended_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        end_reason (AgentSessionEndReason | Unset):
    """

    id: UUID
    thread_id: UUID
    agent_id: UUID
    is_current: bool
    started: bool
    cost: Money
    created_at: datetime.datetime
    version: int
    external_session_ref: str | Unset = UNSET
    model: str | Unset = UNSET
    ended_at: datetime.datetime | Unset = UNSET
    end_reason: AgentSessionEndReason | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        thread_id = str(self.thread_id)

        agent_id = str(self.agent_id)

        is_current = self.is_current

        started = self.started

        cost = self.cost.to_dict()

        created_at = self.created_at.isoformat()

        version = self.version

        external_session_ref = self.external_session_ref

        model = self.model

        ended_at: str | Unset = UNSET
        if not isinstance(self.ended_at, Unset):
            ended_at = self.ended_at.isoformat()

        end_reason: str | Unset = UNSET
        if not isinstance(self.end_reason, Unset):
            end_reason = self.end_reason.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "thread_id": thread_id,
                "agent_id": agent_id,
                "is_current": is_current,
                "started": started,
                "cost": cost,
                "created_at": created_at,
                "version": version,
            }
        )
        if external_session_ref is not UNSET:
            field_dict["external_session_ref"] = external_session_ref
        if model is not UNSET:
            field_dict["model"] = model
        if ended_at is not UNSET:
            field_dict["ended_at"] = ended_at
        if end_reason is not UNSET:
            field_dict["end_reason"] = end_reason

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.money import Money

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        thread_id = UUID(d.pop("thread_id"))

        agent_id = UUID(d.pop("agent_id"))

        is_current = d.pop("is_current")

        started = d.pop("started")

        cost = Money.from_dict(d.pop("cost"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        version = d.pop("version")

        external_session_ref = d.pop("external_session_ref", UNSET)

        model = d.pop("model", UNSET)

        _ended_at = d.pop("ended_at", UNSET)
        ended_at: datetime.datetime | Unset
        if isinstance(_ended_at, Unset):
            ended_at = UNSET
        else:
            ended_at = datetime.datetime.fromisoformat(_ended_at)

        _end_reason = d.pop("end_reason", UNSET)
        end_reason: AgentSessionEndReason | Unset
        if isinstance(_end_reason, Unset):
            end_reason = UNSET
        else:
            end_reason = AgentSessionEndReason(_end_reason)

        agent_session = cls(
            id=id,
            thread_id=thread_id,
            agent_id=agent_id,
            is_current=is_current,
            started=started,
            cost=cost,
            created_at=created_at,
            version=version,
            external_session_ref=external_session_ref,
            model=model,
            ended_at=ended_at,
            end_reason=end_reason,
        )

        agent_session.additional_properties = d
        return agent_session

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
