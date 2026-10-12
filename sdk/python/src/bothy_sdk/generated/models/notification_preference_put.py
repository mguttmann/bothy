from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.notification_level import NotificationLevel
from ..types import UNSET, Unset

T = TypeVar("T", bound="NotificationPreferencePut")


@_attrs_define
class NotificationPreferencePut:
    """
    Attributes:
        level (NotificationLevel):
        muted_until (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    level: NotificationLevel
    muted_until: datetime.datetime | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        level = self.level.value

        muted_until: str | Unset = UNSET
        if not isinstance(self.muted_until, Unset):
            muted_until = self.muted_until.isoformat()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "level": level,
            }
        )
        if muted_until is not UNSET:
            field_dict["muted_until"] = muted_until

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        level = NotificationLevel(d.pop("level"))

        _muted_until = d.pop("muted_until", UNSET)
        muted_until: datetime.datetime | Unset
        if isinstance(_muted_until, Unset):
            muted_until = UNSET
        else:
            muted_until = datetime.datetime.fromisoformat(_muted_until)

        notification_preference_put = cls(
            level=level,
            muted_until=muted_until,
        )

        return notification_preference_put
