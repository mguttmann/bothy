from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="SidebarStatePut")


@_attrs_define
class SidebarStatePut:
    """
    Attributes:
        collapsed (bool):
        sort_key (str | Unset):
        use_global_order (bool | Unset): True removes the personal order. Replaces sending null.
    """

    collapsed: bool
    sort_key: str | Unset = UNSET
    use_global_order: bool | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        collapsed = self.collapsed

        sort_key = self.sort_key

        use_global_order = self.use_global_order

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "collapsed": collapsed,
            }
        )
        if sort_key is not UNSET:
            field_dict["sort_key"] = sort_key
        if use_global_order is not UNSET:
            field_dict["use_global_order"] = use_global_order

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        collapsed = d.pop("collapsed")

        sort_key = d.pop("sort_key", UNSET)

        use_global_order = d.pop("use_global_order", UNSET)

        sidebar_state_put = cls(
            collapsed=collapsed,
            sort_key=sort_key,
            use_global_order=use_global_order,
        )

        return sidebar_state_put
