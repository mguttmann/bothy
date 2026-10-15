from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.webhook_key_state import WebhookKeyState
from ..types import UNSET, Unset

T = TypeVar("T", bound="WebhookKey")


@_attrs_define
class WebhookKey:
    """
    Attributes:
        kid (str):
        public_key (str): Ed25519 public key with prefix `whpk_`, followed by the standard base64 (with padding) of its
            DER-encoded SubjectPublicKeyInfo (RFC 8410).
        state (WebhookKeyState):
        not_after (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    kid: str
    public_key: str
    state: WebhookKeyState
    not_after: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kid = self.kid

        public_key = self.public_key

        state = self.state.value

        not_after: str | Unset = UNSET
        if not isinstance(self.not_after, Unset):
            not_after = self.not_after.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kid": kid,
                "public_key": public_key,
                "state": state,
            }
        )
        if not_after is not UNSET:
            field_dict["not_after"] = not_after

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        kid = d.pop("kid")

        public_key = d.pop("public_key")

        state = WebhookKeyState(d.pop("state"))

        _not_after = d.pop("not_after", UNSET)
        not_after: datetime.datetime | Unset
        if isinstance(_not_after, Unset):
            not_after = UNSET
        else:
            not_after = datetime.datetime.fromisoformat(_not_after)

        webhook_key = cls(
            kid=kid,
            public_key=public_key,
            state=state,
            not_after=not_after,
        )

        webhook_key.additional_properties = d
        return webhook_key

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
