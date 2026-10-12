from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.llm_credentials import LlmCredentials
from ..types import UNSET, Unset

T = TypeVar("T", bound="HostConfirm")


@_attrs_define
class HostConfirm:
    """
    Attributes:
        fingerprint_prefix (str): First `terminal.fingerprint_confirm_chars` characters of the fingerprint without
            separators, case-insensitive.
        llm_credentials (LlmCredentials): Owner declaration of the LLM login on a host (24, 7.2). `personal` needs
            `credential_holder_id` (I6).
        declared_server_host (bool):
        reason (str):
        credential_holder_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    fingerprint_prefix: str
    llm_credentials: LlmCredentials
    declared_server_host: bool
    reason: str
    credential_holder_id: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        fingerprint_prefix = self.fingerprint_prefix

        llm_credentials = self.llm_credentials.value

        declared_server_host = self.declared_server_host

        reason = self.reason

        credential_holder_id: str | Unset = UNSET
        if not isinstance(self.credential_holder_id, Unset):
            credential_holder_id = str(self.credential_holder_id)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "fingerprint_prefix": fingerprint_prefix,
                "llm_credentials": llm_credentials,
                "declared_server_host": declared_server_host,
                "reason": reason,
            }
        )
        if credential_holder_id is not UNSET:
            field_dict["credential_holder_id"] = credential_holder_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        fingerprint_prefix = d.pop("fingerprint_prefix")

        llm_credentials = LlmCredentials(d.pop("llm_credentials"))

        declared_server_host = d.pop("declared_server_host")

        reason = d.pop("reason")

        _credential_holder_id = d.pop("credential_holder_id", UNSET)
        credential_holder_id: UUID | Unset
        if isinstance(_credential_holder_id, Unset):
            credential_holder_id = UNSET
        else:
            credential_holder_id = UUID(_credential_holder_id)

        host_confirm = cls(
            fingerprint_prefix=fingerprint_prefix,
            llm_credentials=llm_credentials,
            declared_server_host=declared_server_host,
            reason=reason,
            credential_holder_id=credential_holder_id,
        )

        return host_confirm
