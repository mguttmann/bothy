from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="HostComputerApp")


@_attrs_define
class HostComputerApp:
    """Identity of a frontmost app (27 12.3 and 13.1); empty strings before the helper named one. Never a window title or a
    path.

        Attributes:
            id (str): Bundle ID or executable name.
            name (str): Display name.
            signer (str): Team ID or subject of the signer.
    """

    id: str
    name: str
    signer: str

    def to_dict(self) -> dict[str, Any]:
        id = self.id

        name = self.name

        signer = self.signer

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "id": id,
                "name": name,
                "signer": signer,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = d.pop("id")

        name = d.pop("name")

        signer = d.pop("signer")

        host_computer_app = cls(
            id=id,
            name=name,
            signer=signer,
        )

        return host_computer_app
