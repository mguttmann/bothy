from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.host_state import HostState
from ..models.llm_credentials import LlmCredentials

T = TypeVar("T", bound="HostReady")


@_attrs_define
class HostReady:
    """Sent after login and again on every change of `llm_credentials`; the host service treats each `ready` as the current
    declaration (24, 2.2 and 7.4).

        Attributes:
            host_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            state (HostState): Life cycle of a host (24, 3.1). Only `active` allows sessions.
            llm_credentials (LlmCredentials): Owner declaration of the LLM login on a host (24, 7.2). `personal` needs
                `credential_holder_id` (I6).
    """

    host_id: UUID
    state: HostState
    llm_credentials: LlmCredentials
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        host_id = str(self.host_id)

        state = self.state.value

        llm_credentials = self.llm_credentials.value

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "host_id": host_id,
                "state": state,
                "llm_credentials": llm_credentials,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        host_id = UUID(d.pop("host_id"))

        state = HostState(d.pop("state"))

        llm_credentials = LlmCredentials(d.pop("llm_credentials"))

        host_ready = cls(
            host_id=host_id,
            state=state,
            llm_credentials=llm_credentials,
        )

        host_ready.additional_properties = d
        return host_ready

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
