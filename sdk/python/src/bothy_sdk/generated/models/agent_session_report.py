from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentSessionReport")


@_attrs_define
class AgentSessionReport:
    """
    Attributes:
        external_session_ref (str):
        model (str | Unset):
    """

    external_session_ref: str
    model: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        external_session_ref = self.external_session_ref

        model = self.model

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "external_session_ref": external_session_ref,
            }
        )
        if model is not UNSET:
            field_dict["model"] = model

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        external_session_ref = d.pop("external_session_ref")

        model = d.pop("model", UNSET)

        agent_session_report = cls(
            external_session_ref=external_session_ref,
            model=model,
        )

        return agent_session_report
