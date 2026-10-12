from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="AgentSessionBind")


@_attrs_define
class AgentSessionBind:
    """
    Attributes:
        external_session_ref (str):
    """

    external_session_ref: str

    def to_dict(self) -> dict[str, Any]:
        external_session_ref = self.external_session_ref

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "external_session_ref": external_session_ref,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        external_session_ref = d.pop("external_session_ref")

        agent_session_bind = cls(
            external_session_ref=external_session_ref,
        )

        return agent_session_bind
