from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="InteractionCreate")


@_attrs_define
class InteractionCreate:
    """
    Attributes:
        client_interaction_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        values (JsonObject | Unset): Free-form JSON object whose schema is named in the field description.
    """

    client_interaction_id: UUID
    values: JsonObject | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        client_interaction_id = str(self.client_interaction_id)

        values: dict[str, Any] | Unset = UNSET
        if not isinstance(self.values, Unset):
            values = self.values.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "client_interaction_id": client_interaction_id,
            }
        )
        if values is not UNSET:
            field_dict["values"] = values

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        client_interaction_id = UUID(d.pop("client_interaction_id"))

        _values = d.pop("values", UNSET)
        values: JsonObject | Unset
        if isinstance(_values, Unset):
            values = UNSET
        else:
            values = JsonObject.from_dict(_values)

        interaction_create = cls(
            client_interaction_id=client_interaction_id,
            values=values,
        )

        return interaction_create
