from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.file_ticket_create_variant import FileTicketCreateVariant

T = TypeVar("T", bound="FileTicketCreate")


@_attrs_define
class FileTicketCreate:
    """
    Attributes:
        variant (FileTicketCreateVariant):
    """

    variant: FileTicketCreateVariant

    def to_dict(self) -> dict[str, Any]:
        variant = self.variant.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "variant": variant,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        variant = FileTicketCreateVariant(d.pop("variant"))

        file_ticket_create = cls(
            variant=variant,
        )

        return file_ticket_create
