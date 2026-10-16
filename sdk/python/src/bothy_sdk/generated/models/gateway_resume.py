from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="GatewayResume")


@_attrs_define
class GatewayResume:
    """
    Attributes:
        gateway_session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        cursor (int):
        access_token (str | Unset):
        api_token (str | Unset):
        dpop (str | Unset):
        sync_epoch (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    gateway_session_id: UUID
    cursor: int
    access_token: str | Unset = UNSET
    api_token: str | Unset = UNSET
    dpop: str | Unset = UNSET
    sync_epoch: UUID | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        gateway_session_id = str(self.gateway_session_id)

        cursor = self.cursor

        access_token = self.access_token

        api_token = self.api_token

        dpop = self.dpop

        sync_epoch: str | Unset = UNSET
        if not isinstance(self.sync_epoch, Unset):
            sync_epoch = str(self.sync_epoch)

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "gateway_session_id": gateway_session_id,
                "cursor": cursor,
            }
        )
        if access_token is not UNSET:
            field_dict["access_token"] = access_token
        if api_token is not UNSET:
            field_dict["api_token"] = api_token
        if dpop is not UNSET:
            field_dict["dpop"] = dpop
        if sync_epoch is not UNSET:
            field_dict["sync_epoch"] = sync_epoch

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        gateway_session_id = UUID(d.pop("gateway_session_id"))

        cursor = d.pop("cursor")

        access_token = d.pop("access_token", UNSET)

        api_token = d.pop("api_token", UNSET)

        dpop = d.pop("dpop", UNSET)

        _sync_epoch = d.pop("sync_epoch", UNSET)
        sync_epoch: UUID | Unset
        if isinstance(_sync_epoch, Unset):
            sync_epoch = UNSET
        else:
            sync_epoch = UUID(_sync_epoch)

        gateway_resume = cls(
            gateway_session_id=gateway_session_id,
            cursor=cursor,
            access_token=access_token,
            api_token=api_token,
            dpop=dpop,
            sync_epoch=sync_epoch,
        )

        return gateway_resume
