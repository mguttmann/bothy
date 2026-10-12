from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.llm_credentials import LlmCredentials
from ..types import UNSET, Unset

T = TypeVar("T", bound="HostLlmCredentialsSet")


@_attrs_define
class HostLlmCredentialsSet:
    """
    Attributes:
        llm_credentials (LlmCredentials): Owner declaration of the LLM login on a host (24, 7.2). `personal` needs
            `credential_holder_id` (I6).
        reason (str):
        credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    llm_credentials: LlmCredentials
    reason: str
    credential_holder_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        llm_credentials = self.llm_credentials.value

        reason = self.reason

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "llm_credentials": llm_credentials,
                "reason": reason,
            }
        )
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        llm_credentials = LlmCredentials(d.pop("llm_credentials"))

        reason = d.pop("reason")

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        host_llm_credentials_set = cls(
            llm_credentials=llm_credentials,
            reason=reason,
            credential_holder_id=credential_holder_id,
        )

        return host_llm_credentials_set
