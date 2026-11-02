from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="ThreadTitlePut")


@_attrs_define
class ThreadTitlePut:
    """
    Attributes:
        title (str):
        auto (bool | Unset): An automatic title (09 section 12 point 5) that never replaces a meaningful title, such as
            one an agent or a person set on purpose. Default: False.
    """

    title: str
    auto: bool | Unset = False

    def to_dict(self) -> dict[str, Any]:
        title = self.title

        auto = self.auto

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "title": title,
            }
        )
        if auto is not UNSET:
            field_dict["auto"] = auto

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        title = d.pop("title")

        auto = d.pop("auto", UNSET)

        thread_title_put = cls(
            title=title,
            auto=auto,
        )

        return thread_title_put
