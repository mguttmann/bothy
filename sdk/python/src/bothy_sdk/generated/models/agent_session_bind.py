from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentSessionBind")


@_attrs_define
class AgentSessionBind:
    """
    Attributes:
        external_session_ref (str):
        expected_current_session_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        take_over (bool | Unset): Required when the session is current in another thread; otherwise 409 `conflict` with
            `reason` `session_bound_elsewhere`. Default: False.
    """

    external_session_ref: str
    expected_current_session_id: UUID | Unset = UNSET
    take_over: bool | Unset = False

    def to_dict(self) -> dict[str, Any]:
        external_session_ref = self.external_session_ref

        expected_current_session_id: str | Unset = UNSET
        if not isinstance(self.expected_current_session_id, Unset):
            expected_current_session_id = str(self.expected_current_session_id)

        take_over = self.take_over

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "external_session_ref": external_session_ref,
            }
        )
        if expected_current_session_id is not UNSET:
            field_dict["expected_current_session_id"] = expected_current_session_id
        if take_over is not UNSET:
            field_dict["take_over"] = take_over

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        external_session_ref = d.pop("external_session_ref")

        _expected_current_session_id = d.pop("expected_current_session_id", UNSET)
        expected_current_session_id: UUID | Unset
        if isinstance(_expected_current_session_id, Unset):
            expected_current_session_id = UNSET
        else:
            expected_current_session_id = UUID(_expected_current_session_id)

        take_over = d.pop("take_over", UNSET)

        agent_session_bind = cls(
            external_session_ref=external_session_ref,
            expected_current_session_id=expected_current_session_id,
            take_over=take_over,
        )

        return agent_session_bind
