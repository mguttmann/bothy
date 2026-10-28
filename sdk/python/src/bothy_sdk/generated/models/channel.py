from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.channel_kind import ChannelKind
from ..models.forum_sort import ForumSort
from ..models.notification_level import NotificationLevel
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.channel_operator import ChannelOperator


T = TypeVar("T", bound="Channel")


@_attrs_define
class Channel:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        kind (ChannelKind):
        name (str):
        topic (str):
        sort_key (str):
        operator (ChannelOperator):
        read_only (bool): Write-protected for every principal including owners; only a running import writes (05, rule
            1.8).
        project_bound (bool): True exactly when a project binding exists for the forum (03 `project_bindings`, R-DM-54);
            derived, never stored. Creating or deleting a binding raises `version` and writes `channel.updated`; changing a
            binding and a readiness report do not.
        version (int):
        settings_version (int):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        space_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        forum_default_sort (ForumSort | Unset): Sort order of forum posts.
        default_notification_level (NotificationLevel | Unset):
        main_thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        dm_participant_ids (list[UUID] | Unset):
        archived_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    kind: ChannelKind
    name: str
    topic: str
    sort_key: str
    operator: ChannelOperator
    read_only: bool
    project_bound: bool
    version: int
    settings_version: int
    created_at: datetime.datetime
    space_id: UUID | Unset = UNSET
    forum_default_sort: ForumSort | Unset = UNSET
    default_notification_level: NotificationLevel | Unset = UNSET
    main_thread_id: UUID | Unset = UNSET
    dm_participant_ids: list[UUID] | Unset = UNSET
    archived_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        kind = self.kind.value

        name = self.name

        topic = self.topic

        sort_key = self.sort_key

        operator = self.operator.to_dict()

        read_only = self.read_only

        project_bound = self.project_bound

        version = self.version

        settings_version = self.settings_version

        created_at = self.created_at.isoformat()

        space_id: str | Unset = UNSET
        if not isinstance(self.space_id, Unset):
            space_id = str(self.space_id)

        forum_default_sort: str | Unset = UNSET
        if not isinstance(self.forum_default_sort, Unset):
            forum_default_sort = self.forum_default_sort.value

        default_notification_level: str | Unset = UNSET
        if not isinstance(self.default_notification_level, Unset):
            default_notification_level = self.default_notification_level.value

        main_thread_id: str | Unset = UNSET
        if not isinstance(self.main_thread_id, Unset):
            main_thread_id = str(self.main_thread_id)

        dm_participant_ids: list[str] | Unset = UNSET
        if not isinstance(self.dm_participant_ids, Unset):
            dm_participant_ids = []
            for dm_participant_ids_item_data in self.dm_participant_ids:
                dm_participant_ids_item = str(dm_participant_ids_item_data)
                dm_participant_ids.append(dm_participant_ids_item)

        archived_at: str | Unset = UNSET
        if not isinstance(self.archived_at, Unset):
            archived_at = self.archived_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "kind": kind,
                "name": name,
                "topic": topic,
                "sort_key": sort_key,
                "operator": operator,
                "read_only": read_only,
                "project_bound": project_bound,
                "version": version,
                "settings_version": settings_version,
                "created_at": created_at,
            }
        )
        if space_id is not UNSET:
            field_dict["space_id"] = space_id
        if forum_default_sort is not UNSET:
            field_dict["forum_default_sort"] = forum_default_sort
        if default_notification_level is not UNSET:
            field_dict["default_notification_level"] = default_notification_level
        if main_thread_id is not UNSET:
            field_dict["main_thread_id"] = main_thread_id
        if dm_participant_ids is not UNSET:
            field_dict["dm_participant_ids"] = dm_participant_ids
        if archived_at is not UNSET:
            field_dict["archived_at"] = archived_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.channel_operator import ChannelOperator

        d = dict(src_dict)
        id = UUID(d.pop("id"))

        kind = ChannelKind(d.pop("kind"))

        name = d.pop("name")

        topic = d.pop("topic")

        sort_key = d.pop("sort_key")

        operator = ChannelOperator.from_dict(d.pop("operator"))

        read_only = d.pop("read_only")

        project_bound = d.pop("project_bound")

        version = d.pop("version")

        settings_version = d.pop("settings_version")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        _space_id = d.pop("space_id", UNSET)
        space_id: UUID | Unset
        if isinstance(_space_id, Unset):
            space_id = UNSET
        else:
            space_id = UUID(_space_id)

        _forum_default_sort = d.pop("forum_default_sort", UNSET)
        forum_default_sort: ForumSort | Unset
        if isinstance(_forum_default_sort, Unset):
            forum_default_sort = UNSET
        else:
            forum_default_sort = ForumSort(_forum_default_sort)

        _default_notification_level = d.pop("default_notification_level", UNSET)
        default_notification_level: NotificationLevel | Unset
        if isinstance(_default_notification_level, Unset):
            default_notification_level = UNSET
        else:
            default_notification_level = NotificationLevel(_default_notification_level)

        _main_thread_id = d.pop("main_thread_id", UNSET)
        main_thread_id: UUID | Unset
        if isinstance(_main_thread_id, Unset):
            main_thread_id = UNSET
        else:
            main_thread_id = UUID(_main_thread_id)

        _dm_participant_ids = d.pop("dm_participant_ids", UNSET)
        dm_participant_ids: list[UUID] | Unset = UNSET
        if _dm_participant_ids is not UNSET:
            dm_participant_ids = []
            for dm_participant_ids_item_data in _dm_participant_ids:
                dm_participant_ids_item = UUID(dm_participant_ids_item_data)

                dm_participant_ids.append(dm_participant_ids_item)

        _archived_at = d.pop("archived_at", UNSET)
        archived_at: datetime.datetime | Unset
        if isinstance(_archived_at, Unset):
            archived_at = UNSET
        else:
            archived_at = datetime.datetime.fromisoformat(_archived_at)

        channel = cls(
            id=id,
            kind=kind,
            name=name,
            topic=topic,
            sort_key=sort_key,
            operator=operator,
            read_only=read_only,
            project_bound=project_bound,
            version=version,
            settings_version=settings_version,
            created_at=created_at,
            space_id=space_id,
            forum_default_sort=forum_default_sort,
            default_notification_level=default_notification_level,
            main_thread_id=main_thread_id,
            dm_participant_ids=dm_participant_ids,
            archived_at=archived_at,
        )

        channel.additional_properties = d
        return channel

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
