from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_option_value import CommandOptionValue


T = TypeVar("T", bound="CommandAutocompleteRequestedPayload")


@_attrs_define
class CommandAutocompleteRequestedPayload:
    """Sent to one gateway connection of the command owner (04, 15.7); agents get an invocation token with `trigger_kind =
    autocomplete` in the delivery fields.

        Attributes:
            request_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            user_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
            option (str):
            value (str):
            expires_at (datetime.datetime): RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
            thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
            options (list[CommandOptionValue] | Unset):
    """

    request_id: UUID
    command_id: UUID
    channel_id: UUID
    user_id: UUID
    option: str
    value: str
    expires_at: datetime.datetime
    thread_id: UUID | Unset = UNSET
    options: list[CommandOptionValue] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        request_id = str(self.request_id)

        command_id = str(self.command_id)

        channel_id = str(self.channel_id)

        user_id = str(self.user_id)

        option = self.option

        value = self.value

        expires_at = self.expires_at.isoformat()

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        options: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.options, Unset):
            options = []
            for options_item_data in self.options:
                options_item = options_item_data.to_dict()
                options.append(options_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "request_id": request_id,
                "command_id": command_id,
                "channel_id": channel_id,
                "user_id": user_id,
                "option": option,
                "value": value,
                "expires_at": expires_at,
            }
        )
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if options is not UNSET:
            field_dict["options"] = options

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_option_value import CommandOptionValue

        d = dict(src_dict)
        request_id = UUID(d.pop("request_id"))

        command_id = UUID(d.pop("command_id"))

        channel_id = UUID(d.pop("channel_id"))

        user_id = UUID(d.pop("user_id"))

        option = d.pop("option")

        value = d.pop("value")

        expires_at = datetime.datetime.fromisoformat(d.pop("expires_at"))

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _options = d.pop("options", UNSET)
        options: list[CommandOptionValue] | Unset = UNSET
        if _options is not UNSET:
            options = []
            for options_item_data in _options:
                options_item = CommandOptionValue.from_dict(options_item_data)

                options.append(options_item)

        command_autocomplete_requested_payload = cls(
            request_id=request_id,
            command_id=command_id,
            channel_id=channel_id,
            user_id=user_id,
            option=option,
            value=value,
            expires_at=expires_at,
            thread_id=thread_id,
            options=options,
        )

        command_autocomplete_requested_payload.additional_properties = d
        return command_autocomplete_requested_payload

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
