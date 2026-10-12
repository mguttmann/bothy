from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.import_item_item_kind import ImportItemItemKind
from ..models.import_item_state import ImportItemState
from ..types import UNSET, Unset

T = TypeVar("T", bound="ImportItem")


@_attrs_define
class ImportItem:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        origin_key (str):
        item_kind (ImportItemItemKind):
        state (ImportItemState):
        created_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        target_kind (str | Unset):
        target_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        error (str | Unset):
    """

    id: UUID
    origin_key: str
    item_kind: ImportItemItemKind
    state: ImportItemState
    created_at: datetime.datetime
    target_kind: str | Unset = UNSET
    target_id: UUID | Unset = UNSET
    error: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        origin_key = self.origin_key

        item_kind = self.item_kind.value

        state = self.state.value

        created_at = self.created_at.isoformat()

        target_kind = self.target_kind

        target_id: str | Unset = UNSET
        if not isinstance(self.target_id, Unset):
            target_id = str(self.target_id)

        error = self.error

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "origin_key": origin_key,
                "item_kind": item_kind,
                "state": state,
                "created_at": created_at,
            }
        )
        if target_kind is not UNSET:
            field_dict["target_kind"] = target_kind
        if target_id is not UNSET:
            field_dict["target_id"] = target_id
        if error is not UNSET:
            field_dict["error"] = error

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        origin_key = d.pop("origin_key")

        item_kind = ImportItemItemKind(d.pop("item_kind"))

        state = ImportItemState(d.pop("state"))

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        target_kind = d.pop("target_kind", UNSET)

        _target_id = d.pop("target_id", UNSET)
        target_id: UUID | Unset
        if isinstance(_target_id, Unset):
            target_id = UNSET
        else:
            target_id = UUID(_target_id)

        error = d.pop("error", UNSET)

        import_item = cls(
            id=id,
            origin_key=origin_key,
            item_kind=item_kind,
            state=state,
            created_at=created_at,
            target_kind=target_kind,
            target_id=target_id,
            error=error,
        )

        import_item.additional_properties = d
        return import_item

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
