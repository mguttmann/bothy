from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

T = TypeVar("T", bound="InvariantViolation")


@_attrs_define
class InvariantViolation:
    """`i1_violation` names `agent_id`; `i6_violation` names `host_id`.

    Attributes:
        code (str):
        principal_ids (list[UUID]): With `i1_violation` the principals that would gain `trigger` next to the credential
            holder; with `i6_violation` the people other than the holder that a valid grant on the host names, directly or
            as members in effect of a group grant (empty for a group without members).
        agent_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        host_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        organization_ready (bool | Unset): True when the agent reports `organization` among its
            `billing_modes_supported`.
        hint (str | Unset):
    """

    code: str
    principal_ids: list[UUID]
    agent_id: UUID | Unset = UNSET
    host_id: UUID | Unset = UNSET
    credential_holder_id: UUID | Unset = UNSET
    organization_ready: bool | Unset = UNSET
    hint: str | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        code = self.code

        principal_ids = []
        for principal_ids_item_data in self.principal_ids:
            principal_ids_item = str(principal_ids_item_data)
            principal_ids.append(principal_ids_item)

        agent_id: str | Unset = UNSET
        if not isinstance(self.agent_id, Unset):
            agent_id = str(self.agent_id)

        host_id: str | Unset = UNSET
        if not isinstance(self.host_id, Unset):
            host_id = str(self.host_id)

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        organization_ready = self.organization_ready

        hint = self.hint

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "code": code,
                "principal_ids": principal_ids,
            }
        )
        if agent_id is not UNSET:
            field_dict["agent_id"] = agent_id
        if host_id is not UNSET:
            field_dict["host_id"] = host_id
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id
        if organization_ready is not UNSET:
            field_dict["organization_ready"] = organization_ready
        if hint is not UNSET:
            field_dict["hint"] = hint

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        code = d.pop("code")

        principal_ids = []
        _principal_ids = d.pop("principal_ids")
        for principal_ids_item_data in _principal_ids:
            principal_ids_item = UUID(principal_ids_item_data)

            principal_ids.append(principal_ids_item)

        _agent_id = d.pop("agent_id", UNSET)
        agent_id: UUID | Unset
        if isinstance(_agent_id, Unset):
            agent_id = UNSET
        else:
            agent_id = UUID(_agent_id)

        _host_id = d.pop("host_id", UNSET)
        host_id: UUID | Unset
        if isinstance(_host_id, Unset):
            host_id = UNSET
        else:
            host_id = UUID(_host_id)

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        organization_ready = d.pop("organization_ready", UNSET)

        hint = d.pop("hint", UNSET)

        invariant_violation = cls(
            code=code,
            principal_ids=principal_ids,
            agent_id=agent_id,
            host_id=host_id,
            credential_holder_id=credential_holder_id,
            organization_ready=organization_ready,
            hint=hint,
        )

        invariant_violation.additional_properties = d
        return invariant_violation

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
