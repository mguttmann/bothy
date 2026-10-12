from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.forum_sort import ForumSort
from ..models.notification_level import NotificationLevel
from ..types import UNSET, Unset

T = TypeVar("T", bound="ChannelUpdate")


@_attrs_define
class ChannelUpdate:
    """
    Attributes:
        name (str | Unset):
        topic (str | Unset): At most `channel.topic_max_chars` characters, no control or bidi characters.
        forum_default_sort (ForumSort | Unset): Sort order of forum posts.
        default_notification_level (NotificationLevel | Unset):
        clear_default_notification_level (bool | Unset): Removes the channel default; there is no JSON null (R-API-03).
    """

    name: str | Unset = UNSET
    topic: str | Unset = UNSET
    forum_default_sort: ForumSort | Unset = UNSET
    default_notification_level: NotificationLevel | Unset = UNSET
    clear_default_notification_level: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        topic = self.topic

        forum_default_sort: str | Unset = UNSET
        if not isinstance(self.forum_default_sort, Unset):
            forum_default_sort = self.forum_default_sort.value

        default_notification_level: str | Unset = UNSET
        if not isinstance(self.default_notification_level, Unset):
            default_notification_level = self.default_notification_level.value

        clear_default_notification_level = self.clear_default_notification_level

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if name is not UNSET:
            field_dict["name"] = name
        if topic is not UNSET:
            field_dict["topic"] = topic
        if forum_default_sort is not UNSET:
            field_dict["forum_default_sort"] = forum_default_sort
        if default_notification_level is not UNSET:
            field_dict["default_notification_level"] = default_notification_level
        if clear_default_notification_level is not UNSET:
            field_dict["clear_default_notification_level"] = (
                clear_default_notification_level
            )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name", UNSET)

        topic = d.pop("topic", UNSET)

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

        clear_default_notification_level = d.pop(
            "clear_default_notification_level", UNSET
        )

        channel_update = cls(
            name=name,
            topic=topic,
            forum_default_sort=forum_default_sort,
            default_notification_level=default_notification_level,
            clear_default_notification_level=clear_default_notification_level,
        )

        return channel_update
