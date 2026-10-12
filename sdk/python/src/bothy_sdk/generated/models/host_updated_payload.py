from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.host import Host


T = TypeVar("T", bound="HostUpdatedPayload")


@_attrs_define
class HostUpdatedPayload:
    """
    Attributes:
        host (Host): A connected host. Keys are public; the client pins `noise_public_key` per device (24, 5.5).
        key_changed (bool | Unset): True after a re-pairing (24, 3.3).
    """

    host: Host
    key_changed: bool | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        host = self.host.to_dict()

        key_changed = self.key_changed

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "host": host,
            }
        )
        if key_changed is not UNSET:
            field_dict["key_changed"] = key_changed

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host import Host

        d = dict(src_dict)
        host = Host.from_dict(d.pop("host"))

        key_changed = d.pop("key_changed", UNSET)

        host_updated_payload = cls(
            host=host,
            key_changed=key_changed,
        )

        host_updated_payload.additional_properties = d
        return host_updated_payload

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
