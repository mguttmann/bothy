from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.grant_changed_payload_change import GrantChangedPayloadChange

if TYPE_CHECKING:
    from ..models.grant import Grant


T = TypeVar("T", bound="GrantChangedPayload")


@_attrs_define
class GrantChangedPayload:
    """
    Attributes:
        grant (Grant):
        change (GrantChangedPayloadChange):
    """

    grant: Grant
    change: GrantChangedPayloadChange
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        grant = self.grant.to_dict()

        change = self.change.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "grant": grant,
                "change": change,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.grant import Grant

        d = dict(src_dict)
        grant = Grant.from_dict(d.pop("grant"))

        change = GrantChangedPayloadChange(d.pop("change"))

        grant_changed_payload = cls(
            grant=grant,
            change=change,
        )

        grant_changed_payload.additional_properties = d
        return grant_changed_payload

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
