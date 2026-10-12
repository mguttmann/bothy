from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.draft import Draft


T = TypeVar("T", bound="DraftPutResult")


@_attrs_define
class DraftPutResult:
    """
    Attributes:
        draft (Draft):
        overwrote_newer (bool):
    """

    draft: Draft
    overwrote_newer: bool
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        draft = self.draft.to_dict()

        overwrote_newer = self.overwrote_newer

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "draft": draft,
                "overwrote_newer": overwrote_newer,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.draft import Draft

        d = dict(src_dict)
        draft = Draft.from_dict(d.pop("draft"))

        overwrote_newer = d.pop("overwrote_newer")

        draft_put_result = cls(
            draft=draft,
            overwrote_newer=overwrote_newer,
        )

        draft_put_result.additional_properties = d
        return draft_put_result

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
