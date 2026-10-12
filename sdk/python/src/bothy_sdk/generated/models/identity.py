from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.identity_provider_kind import IdentityProviderKind
from ..types import UNSET, Unset

T = TypeVar("T", bound="Identity")


@_attrs_define
class Identity:
    """
    Attributes:
        id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        provider_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        provider_kind (IdentityProviderKind): Kind of identity provider (B9).
        subject (str):
        linked_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
        issuer (str | Unset):
        email_at_link (str | Unset):
        last_login_at (datetime.datetime | Unset): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
    """

    id: UUID
    provider_id: UUID
    provider_kind: IdentityProviderKind
    subject: str
    linked_at: datetime.datetime
    issuer: str | Unset = UNSET
    email_at_link: str | Unset = UNSET
    last_login_at: datetime.datetime | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        provider_id = str(self.provider_id)

        provider_kind = self.provider_kind.value

        subject = self.subject

        linked_at = self.linked_at.isoformat()

        issuer = self.issuer

        email_at_link = self.email_at_link

        last_login_at: str | Unset = UNSET
        if not isinstance(self.last_login_at, Unset):
            last_login_at = self.last_login_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "provider_id": provider_id,
                "provider_kind": provider_kind,
                "subject": subject,
                "linked_at": linked_at,
            }
        )
        if issuer is not UNSET:
            field_dict["issuer"] = issuer
        if email_at_link is not UNSET:
            field_dict["email_at_link"] = email_at_link
        if last_login_at is not UNSET:
            field_dict["last_login_at"] = last_login_at

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        provider_id = UUID(d.pop("provider_id"))

        provider_kind = IdentityProviderKind(d.pop("provider_kind"))

        subject = d.pop("subject")

        linked_at = datetime.datetime.fromisoformat(d.pop("linked_at"))

        issuer = d.pop("issuer", UNSET)

        email_at_link = d.pop("email_at_link", UNSET)

        _last_login_at = d.pop("last_login_at", UNSET)
        last_login_at: datetime.datetime | Unset
        if isinstance(_last_login_at, Unset):
            last_login_at = UNSET
        else:
            last_login_at = datetime.datetime.fromisoformat(_last_login_at)

        identity = cls(
            id=id,
            provider_id=provider_id,
            provider_kind=provider_kind,
            subject=subject,
            linked_at=linked_at,
            issuer=issuer,
            email_at_link=email_at_link,
            last_login_at=last_login_at,
        )

        identity.additional_properties = d
        return identity

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
