from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="HostPairResult")


@_attrs_define
class HostPairResult:
    """
    Attributes:
        host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        key_fingerprint (str):
        instance_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    host_id: UUID
    key_fingerprint: str
    instance_id: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        host_id = str(self.host_id)

        key_fingerprint = self.key_fingerprint

        instance_id = str(self.instance_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "host_id": host_id,
                "key_fingerprint": key_fingerprint,
                "instance_id": instance_id,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        host_id = UUID(d.pop("host_id"))

        key_fingerprint = d.pop("key_fingerprint")

        instance_id = UUID(d.pop("instance_id"))

        host_pair_result = cls(
            host_id=host_id,
            key_fingerprint=key_fingerprint,
            instance_id=instance_id,
        )

        host_pair_result.additional_properties = d
        return host_pair_result

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
