from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="ComputerAttachIdentify")


@_attrs_define
class ComputerAttachIdentify:
    """The single-use attach token of the session (27 5.4); the connection carries no agent token.

    Attributes:
        protocol_version (int): The version of the attach protocol `bothy-computer-mcp` chose from `protocol_versions`.
        token (str): The attach token of `ComputerSessionGrant`, valid once.
    """

    protocol_version: int
    token: str

    def to_dict(self) -> dict[str, Any]:
        protocol_version = self.protocol_version

        token = self.token

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "protocol_version": protocol_version,
                "token": token,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        protocol_version = d.pop("protocol_version")

        token = d.pop("token")

        computer_attach_identify = cls(
            protocol_version=protocol_version,
            token=token,
        )

        return computer_attach_identify
