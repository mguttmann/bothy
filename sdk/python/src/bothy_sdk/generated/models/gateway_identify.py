from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.gateway_client_info import GatewayClientInfo


T = TypeVar("T", bound="GatewayIdentify")


@_attrs_define
class GatewayIdentify:
    """People send `access_token`, `dpop`, `device_id`; agents and apps send `api_token`, `consumer`
    and `dpop` when the token is DPoP-bound.

        Attributes:
            protocol_version (int):
            access_token (str | Unset):
            api_token (str | Unset):
            dpop (str | Unset): DPoP proof with `htm = GET`, `htu` = HTTPS form of the gateway URL, `nonce` from `hello`,
                `ath`.
            device_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            consumer (str | Unset):
            cursor (int | Unset): Last processed `seq`; absent on first connect.
            sync_epoch (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            client (GatewayClientInfo | Unset):
    """

    protocol_version: int
    access_token: str | Unset = UNSET
    api_token: str | Unset = UNSET
    dpop: str | Unset = UNSET
    device_id: UUID | Unset = UNSET
    consumer: str | Unset = UNSET
    cursor: int | Unset = UNSET
    sync_epoch: UUID | Unset = UNSET
    client: GatewayClientInfo | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        protocol_version = self.protocol_version

        access_token = self.access_token

        api_token = self.api_token

        dpop = self.dpop

        device_id: str | Unset = UNSET
        if not isinstance(self.device_id, Unset):
            device_id = str(self.device_id)

        consumer = self.consumer

        cursor = self.cursor

        sync_epoch: str | Unset = UNSET
        if not isinstance(self.sync_epoch, Unset):
            sync_epoch = str(self.sync_epoch)

        client: dict[str, Any] | Unset = UNSET
        if not isinstance(self.client, Unset):
            client = self.client.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "protocol_version": protocol_version,
            }
        )
        if access_token is not UNSET:
            field_dict["access_token"] = access_token
        if api_token is not UNSET:
            field_dict["api_token"] = api_token
        if dpop is not UNSET:
            field_dict["dpop"] = dpop
        if device_id is not UNSET:
            field_dict["device_id"] = device_id
        if consumer is not UNSET:
            field_dict["consumer"] = consumer
        if cursor is not UNSET:
            field_dict["cursor"] = cursor
        if sync_epoch is not UNSET:
            field_dict["sync_epoch"] = sync_epoch
        if client is not UNSET:
            field_dict["client"] = client

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.gateway_client_info import GatewayClientInfo

        d = dict(src_dict)
        protocol_version = d.pop("protocol_version")

        access_token = d.pop("access_token", UNSET)

        api_token = d.pop("api_token", UNSET)

        dpop = d.pop("dpop", UNSET)

        _device_id = d.pop("device_id", UNSET)
        device_id: UUID | Unset
        if isinstance(_device_id, Unset):
            device_id = UNSET
        else:
            device_id = UUID(_device_id)

        consumer = d.pop("consumer", UNSET)

        cursor = d.pop("cursor", UNSET)

        _sync_epoch = d.pop("sync_epoch", UNSET)
        sync_epoch: UUID | Unset
        if isinstance(_sync_epoch, Unset):
            sync_epoch = UNSET
        else:
            sync_epoch = UUID(_sync_epoch)

        _client = d.pop("client", UNSET)
        client: GatewayClientInfo | Unset
        if isinstance(_client, Unset):
            client = UNSET
        else:
            client = GatewayClientInfo.from_dict(_client)

        gateway_identify = cls(
            protocol_version=protocol_version,
            access_token=access_token,
            api_token=api_token,
            dpop=dpop,
            device_id=device_id,
            consumer=consumer,
            cursor=cursor,
            sync_epoch=sync_epoch,
            client=client,
        )

        return gateway_identify
