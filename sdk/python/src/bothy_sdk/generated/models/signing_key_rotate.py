from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.signing_key_purpose import SigningKeyPurpose
from ..models.signing_key_rotate_mode import SigningKeyRotateMode

T = TypeVar("T", bound="SigningKeyRotate")


@_attrs_define
class SigningKeyRotate:
    """
    Attributes:
        purpose (SigningKeyPurpose): Purpose of a server signing key (03 `server_signing_keys.purpose`). `terminal`
            signs terminal session tokens (24 section 5.3); `audit_checkpoint` signs audit checkpoints (19 SEC-AUD-7).
        mode (SigningKeyRotateMode): `scheduled` announces the next key (state `next`); the rotation job makes it
            `active`
            after `agent.jwks_max_age`. `emergency` retires every key of the purpose at once and
            creates a new `active` key. Required, because the two modes have opposite effects.
        reason (str):
    """

    purpose: SigningKeyPurpose
    mode: SigningKeyRotateMode
    reason: str

    def to_dict(self) -> dict[str, Any]:
        purpose = self.purpose.value

        mode = self.mode.value

        reason = self.reason

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "purpose": purpose,
                "mode": mode,
                "reason": reason,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        purpose = SigningKeyPurpose(d.pop("purpose"))

        mode = SigningKeyRotateMode(d.pop("mode"))

        reason = d.pop("reason")

        signing_key_rotate = cls(
            purpose=purpose,
            mode=mode,
            reason=reason,
        )

        return signing_key_rotate
