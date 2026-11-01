from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.billing_mode import BillingMode
from ..models.thread_kind import ThreadKind
from ..models.thread_state import ThreadState
from ..models.thread_title_source import ThreadTitleSource
from ..types import UNSET, Unset

T = TypeVar("T", bound="Thread")


@_attrs_define
class Thread:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (ThreadKind):
        title (str):
        title_source (ThreadTitleSource):
        state (ThreadState):
        last_message_seq (int):
        last_activity_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        message_count (int):
        created_by (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        version (int):
        tag_ids (list[UUID]):
        parent_message_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        closed_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        closed_by (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        operator_agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        billing_mode (BillingMode | Unset): Backend policy of an agent. `personal` runs on the subscription of exactly
            one person.
        unread_floor_seq (int | Unset): Messages up to this `thread_seq` never count as unread (import); absent means 0.
        moved_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        deleted_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    channel_id: UUID
    kind: ThreadKind
    title: str
    title_source: ThreadTitleSource
    state: ThreadState
    last_message_seq: int
    last_activity_at: datetime.datetime
    message_count: int
    created_by: UUID
    created_at: datetime.datetime
    version: int
    tag_ids: list[UUID]
    parent_message_id: UUID | Unset = UNSET
    closed_at: datetime.datetime | Unset = UNSET
    closed_by: UUID | Unset = UNSET
    operator_agent_id: UUID | Unset = UNSET
    billing_mode: BillingMode | Unset = UNSET
    unread_floor_seq: int | Unset = UNSET
    moved_at: datetime.datetime | Unset = UNSET
    deleted_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        channel_id = str(self.channel_id)

        kind = self.kind.value

        title = self.title

        title_source = self.title_source.value

        state = self.state.value

        last_message_seq = self.last_message_seq

        last_activity_at = self.last_activity_at.isoformat()

        message_count = self.message_count

        created_by = str(self.created_by)

        created_at = self.created_at.isoformat()

        version = self.version

        tag_ids = []
        for tag_ids_item_data in self.tag_ids:
            tag_ids_item = str(tag_ids_item_data)
            tag_ids.append(tag_ids_item)

        parent_message_id: str | Unset = UNSET
        if not isinstance(self.parent_message_id, Unset):
            parent_message_id = str(self.parent_message_id)

        closed_at: str | Unset = UNSET
        if not isinstance(self.closed_at, Unset):
            closed_at = self.closed_at.isoformat()

        closed_by: str | Unset = UNSET
        if not isinstance(self.closed_by, Unset):
            closed_by = str(self.closed_by)

        operator_agent_id: str | Unset = UNSET
        if not isinstance(self.operator_agent_id, Unset):
            operator_agent_id = str(self.operator_agent_id)

        billing_mode: str | Unset = UNSET
        if not isinstance(self.billing_mode, Unset):
            billing_mode = self.billing_mode.value

        unread_floor_seq = self.unread_floor_seq

        moved_at: str | Unset = UNSET
        if not isinstance(self.moved_at, Unset):
            moved_at = self.moved_at.isoformat()

        deleted_at: str | Unset = UNSET
        if not isinstance(self.deleted_at, Unset):
            deleted_at = self.deleted_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "channel_id": channel_id,
                "kind": kind,
                "title": title,
                "title_source": title_source,
                "state": state,
                "last_message_seq": last_message_seq,
                "last_activity_at": last_activity_at,
                "message_count": message_count,
                "created_by": created_by,
                "created_at": created_at,
                "version": version,
                "tag_ids": tag_ids,
            }
        )
        if parent_message_id is not UNSET:
            field_dict["parent_message_id"] = parent_message_id
        if closed_at is not UNSET:
            field_dict["closed_at"] = closed_at
        if closed_by is not UNSET:
            field_dict["closed_by"] = closed_by
        if operator_agent_id is not UNSET:
            field_dict["operator_agent_id"] = operator_agent_id
        if billing_mode is not UNSET:
            field_dict["billing_mode"] = billing_mode
        if unread_floor_seq is not UNSET:
            field_dict["unread_floor_seq"] = unread_floor_seq
        if moved_at is not UNSET:
            field_dict["moved_at"] = moved_at
        if deleted_at is not UNSET:
            field_dict["deleted_at"] = deleted_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        channel_id = UUID(d.pop("channel_id"))

        kind = ThreadKind(d.pop("kind"))

        title = d.pop("title")

        title_source = ThreadTitleSource(d.pop("title_source"))

        state = ThreadState(d.pop("state"))

        last_message_seq = d.pop("last_message_seq")

        last_activity_at = datetime.datetime.fromisoformat(d.pop("last_activity_at"))

        message_count = d.pop("message_count")

        created_by = UUID(d.pop("created_by"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        version = d.pop("version")

        tag_ids = []
        _tag_ids = d.pop("tag_ids")
        for tag_ids_item_data in _tag_ids:
            tag_ids_item = UUID(tag_ids_item_data)

            tag_ids.append(tag_ids_item)

        _parent_message_id = d.pop("parent_message_id", UNSET)
        parent_message_id: UUID | Unset
        if isinstance(_parent_message_id, Unset):
            parent_message_id = UNSET
        else:
            parent_message_id = UUID(_parent_message_id)

        _closed_at = d.pop("closed_at", UNSET)
        closed_at: datetime.datetime | Unset
        if isinstance(_closed_at, Unset):
            closed_at = UNSET
        else:
            closed_at = datetime.datetime.fromisoformat(_closed_at)

        _closed_by = d.pop("closed_by", UNSET)
        closed_by: UUID | Unset
        if isinstance(_closed_by, Unset):
            closed_by = UNSET
        else:
            closed_by = UUID(_closed_by)

        _operator_agent_id = d.pop("operator_agent_id", UNSET)
        operator_agent_id: UUID | Unset
        if isinstance(_operator_agent_id, Unset):
            operator_agent_id = UNSET
        else:
            operator_agent_id = UUID(_operator_agent_id)

        _billing_mode = d.pop("billing_mode", UNSET)
        billing_mode: BillingMode | Unset
        if isinstance(_billing_mode, Unset):
            billing_mode = UNSET
        else:
            billing_mode = BillingMode(_billing_mode)

        unread_floor_seq = d.pop("unread_floor_seq", UNSET)

        _moved_at = d.pop("moved_at", UNSET)
        moved_at: datetime.datetime | Unset
        if isinstance(_moved_at, Unset):
            moved_at = UNSET
        else:
            moved_at = datetime.datetime.fromisoformat(_moved_at)

        _deleted_at = d.pop("deleted_at", UNSET)
        deleted_at: datetime.datetime | Unset
        if isinstance(_deleted_at, Unset):
            deleted_at = UNSET
        else:
            deleted_at = datetime.datetime.fromisoformat(_deleted_at)

        thread = cls(
            id=id,
            channel_id=channel_id,
            kind=kind,
            title=title,
            title_source=title_source,
            state=state,
            last_message_seq=last_message_seq,
            last_activity_at=last_activity_at,
            message_count=message_count,
            created_by=created_by,
            created_at=created_at,
            version=version,
            tag_ids=tag_ids,
            parent_message_id=parent_message_id,
            closed_at=closed_at,
            closed_by=closed_by,
            operator_agent_id=operator_agent_id,
            billing_mode=billing_mode,
            unread_floor_seq=unread_floor_seq,
            moved_at=moved_at,
            deleted_at=deleted_at,
        )

        thread.additional_properties = d
        return thread

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
