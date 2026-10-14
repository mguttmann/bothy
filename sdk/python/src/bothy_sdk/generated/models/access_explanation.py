from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.access_target_kind import AccessTargetKind

if TYPE_CHECKING:
    from ..models.access_entry import AccessEntry


T = TypeVar("T", bound="AccessExplanation")


@_attrs_define
class AccessExplanation:
    """
    Attributes:
        target_kind (AccessTargetKind): `host` appears only in `access.changed` and `access.expiring` (terminal
            capability, no level).
        target_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        entries (list[AccessEntry]):
    """

    target_kind: AccessTargetKind
    target_id: UUID
    entries: list[AccessEntry]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        target_kind = self.target_kind.value

        target_id = str(self.target_id)

        entries = []
        for entries_item_data in self.entries:
            entries_item = entries_item_data.to_dict()
            entries.append(entries_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "target_kind": target_kind,
                "target_id": target_id,
                "entries": entries,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_entry import AccessEntry

        d = dict(src_dict)
        target_kind = AccessTargetKind(d.pop("target_kind"))

        target_id = UUID(d.pop("target_id"))

        entries = []
        _entries = d.pop("entries")
        for entries_item_data in _entries:
            entries_item = AccessEntry.from_dict(entries_item_data)

            entries.append(entries_item)

        access_explanation = cls(
            target_kind=target_kind,
            target_id=target_id,
            entries=entries,
        )

        access_explanation.additional_properties = d
        return access_explanation

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
