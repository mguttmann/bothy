from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.run_state import RunState
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.stats_run_chat import StatsRunChat


T = TypeVar("T", bound="StatsTimelineRun")


@_attrs_define
class StatsTimelineRun:
    """
    Attributes:
        run_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        agent_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        queued_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        state (RunState):
        started_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        finished_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        chat (StatsRunChat | Unset): The chat a run belongs to, as the sidebar names it: the name of its channel (the
            project) and the title of its thread. Present only when the caller may read the thread (`thread.read`, the same
            rule as the sidebar); absent otherwise, and then a client shows neither a name nor a link
            (docs/spec/25-visualization.md, section 4, rule 6).
    """

    run_id: UUID
    channel_id: UUID
    agent_id: UUID
    queued_at: datetime.datetime
    state: RunState
    started_at: datetime.datetime | Unset = UNSET
    finished_at: datetime.datetime | Unset = UNSET
    chat: StatsRunChat | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        run_id = str(self.run_id)

        channel_id = str(self.channel_id)

        agent_id = str(self.agent_id)

        queued_at = self.queued_at.isoformat()

        state = self.state.value

        started_at: str | Unset = UNSET
        if not isinstance(self.started_at, Unset):
            started_at = self.started_at.isoformat()

        finished_at: str | Unset = UNSET
        if not isinstance(self.finished_at, Unset):
            finished_at = self.finished_at.isoformat()

        chat: dict[str, Any] | Unset = UNSET
        if not isinstance(self.chat, Unset):
            chat = self.chat.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "run_id": run_id,
                "channel_id": channel_id,
                "agent_id": agent_id,
                "queued_at": queued_at,
                "state": state,
            }
        )
        if started_at is not UNSET:
            field_dict["started_at"] = started_at
        if finished_at is not UNSET:
            field_dict["finished_at"] = finished_at
        if chat is not UNSET:
            field_dict["chat"] = chat

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.stats_run_chat import StatsRunChat

        d = dict(src_dict)
        run_id = UUID(d.pop("run_id"))

        channel_id = UUID(d.pop("channel_id"))

        agent_id = UUID(d.pop("agent_id"))

        queued_at = datetime.datetime.fromisoformat(d.pop("queued_at"))

        state = RunState(d.pop("state"))

        _started_at = d.pop("started_at", UNSET)
        started_at: datetime.datetime | Unset
        if isinstance(_started_at, Unset):
            started_at = UNSET
        else:
            started_at = datetime.datetime.fromisoformat(_started_at)

        _finished_at = d.pop("finished_at", UNSET)
        finished_at: datetime.datetime | Unset
        if isinstance(_finished_at, Unset):
            finished_at = UNSET
        else:
            finished_at = datetime.datetime.fromisoformat(_finished_at)

        _chat = d.pop("chat", UNSET)
        chat: StatsRunChat | Unset
        if isinstance(_chat, Unset):
            chat = UNSET
        else:
            chat = StatsRunChat.from_dict(_chat)

        stats_timeline_run = cls(
            run_id=run_id,
            channel_id=channel_id,
            agent_id=agent_id,
            queued_at=queued_at,
            state=state,
            started_at=started_at,
            finished_at=finished_at,
            chat=chat,
        )

        stats_timeline_run.additional_properties = d
        return stats_timeline_run

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
