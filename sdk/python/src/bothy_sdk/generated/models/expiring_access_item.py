from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..models.expiring_access_item_kind import ExpiringAccessItemKind
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.grant import Grant


T = TypeVar("T", bound="ExpiringAccessItem")


@_attrs_define
class ExpiringAccessItem:
    """One access that expires within `authz.expiry_reminder_lead`. `grant` carries the `grant`; `membership` names
    `membership_id`, `group_id` and `user_id`; `token` names `token_id` and the holder `principal_id` (an agent or an
    app), never the token value.

        Attributes:
            kind (ExpiringAccessItemKind):
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            grant (Grant | Unset):
            membership_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            group_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            user_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            token_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            principal_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    kind: ExpiringAccessItemKind
    expires_at: datetime.datetime
    grant: Grant | Unset = UNSET
    membership_id: UUID | Unset = UNSET
    group_id: UUID | Unset = UNSET
    user_id: UUID | Unset = UNSET
    token_id: UUID | Unset = UNSET
    principal_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        kind = self.kind.value

        expires_at = self.expires_at.isoformat()

        grant: dict[str, Any] | Unset = UNSET
        if not isinstance(self.grant, Unset):
            grant = self.grant.to_dict()

        membership_id: str | Unset = UNSET
        if not isinstance(self.membership_id, Unset):
            membership_id = str(self.membership_id)

        group_id: str | Unset = UNSET
        if not isinstance(self.group_id, Unset):
            group_id = str(self.group_id)

        user_id: str | Unset = UNSET
        if not isinstance(self.user_id, Unset):
            user_id = str(self.user_id)

        token_id: str | Unset = UNSET
        if not isinstance(self.token_id, Unset):
            token_id = str(self.token_id)

        principal_id: str | Unset = UNSET
        if not isinstance(self.principal_id, Unset):
            principal_id = str(self.principal_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "kind": kind,
                "expires_at": expires_at,
            }
        )
        if grant is not UNSET:
            field_dict["grant"] = grant
        if membership_id is not UNSET:
            field_dict["membership_id"] = membership_id
        if group_id is not UNSET:
            field_dict["group_id"] = group_id
        if user_id is not UNSET:
            field_dict["user_id"] = user_id
        if token_id is not UNSET:
            field_dict["token_id"] = token_id
        if principal_id is not UNSET:
            field_dict["principal_id"] = principal_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.grant import Grant

        d = dict(src_dict)
        kind = ExpiringAccessItemKind(d.pop("kind"))

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        _grant = d.pop("grant", UNSET)
        grant: Grant | Unset
        if isinstance(_grant, Unset):
            grant = UNSET
        else:
            grant = Grant.from_dict(_grant)

        _membership_id = d.pop("membership_id", UNSET)
        membership_id: UUID | Unset
        if isinstance(_membership_id, Unset):
            membership_id = UNSET
        else:
            membership_id = UUID(_membership_id)

        _group_id = d.pop("group_id", UNSET)
        group_id: UUID | Unset
        if isinstance(_group_id, Unset):
            group_id = UNSET
        else:
            group_id = UUID(_group_id)

        _user_id = d.pop("user_id", UNSET)
        user_id: UUID | Unset
        if isinstance(_user_id, Unset):
            user_id = UNSET
        else:
            user_id = UUID(_user_id)

        _token_id = d.pop("token_id", UNSET)
        token_id: UUID | Unset
        if isinstance(_token_id, Unset):
            token_id = UNSET
        else:
            token_id = UUID(_token_id)

        _principal_id = d.pop("principal_id", UNSET)
        principal_id: UUID | Unset
        if isinstance(_principal_id, Unset):
            principal_id = UNSET
        else:
            principal_id = UUID(_principal_id)

        expiring_access_item = cls(
            kind=kind,
            expires_at=expires_at,
            grant=grant,
            membership_id=membership_id,
            group_id=group_id,
            user_id=user_id,
            token_id=token_id,
            principal_id=principal_id,
        )

        expiring_access_item.additional_properties = d
        return expiring_access_item

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
