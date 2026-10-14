from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.command_option_value import CommandOptionValue
    from ..models.principal_ref import PrincipalRef


T = TypeVar("T", bound="CommandInvokedPayload")


@_attrs_define
class CommandInvokedPayload:
    """
    Attributes:
        invocation_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        command_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        command_name (str):
        control (bool):
        channel_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        invoked_by (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        options (list[CommandOptionValue]):
        thread_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
        run_id (UUID | Unset): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    invocation_id: UUID
    command_id: UUID
    command_name: str
    control: bool
    channel_id: UUID
    invoked_by: PrincipalRef
    options: list[CommandOptionValue]
    thread_id: UUID | Unset = UNSET
    run_id: UUID | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        invocation_id = str(self.invocation_id)

        command_id = str(self.command_id)

        command_name = self.command_name

        control = self.control

        channel_id = str(self.channel_id)

        invoked_by = self.invoked_by.to_dict()

        options = []
        for options_item_data in self.options:
            options_item = options_item_data.to_dict()
            options.append(options_item)

        thread_id: str | Unset = UNSET
        if not isinstance(self.thread_id, Unset):
            thread_id = str(self.thread_id)

        run_id: str | Unset = UNSET
        if not isinstance(self.run_id, Unset):
            run_id = str(self.run_id)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "invocation_id": invocation_id,
                "command_id": command_id,
                "command_name": command_name,
                "control": control,
                "channel_id": channel_id,
                "invoked_by": invoked_by,
                "options": options,
            }
        )
        if thread_id is not UNSET:
            field_dict["thread_id"] = thread_id
        if run_id is not UNSET:
            field_dict["run_id"] = run_id

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.command_option_value import CommandOptionValue
        from ..models.principal_ref import PrincipalRef

        d = dict(src_dict)
        invocation_id = UUID(d.pop("invocation_id"))

        command_id = UUID(d.pop("command_id"))

        command_name = d.pop("command_name")

        control = d.pop("control")

        channel_id = UUID(d.pop("channel_id"))

        invoked_by = PrincipalRef.from_dict(d.pop("invoked_by"))

        options = []
        _options = d.pop("options")
        for options_item_data in _options:
            options_item = CommandOptionValue.from_dict(options_item_data)

            options.append(options_item)

        _thread_id = d.pop("thread_id", UNSET)
        thread_id: UUID | Unset
        if isinstance(_thread_id, Unset):
            thread_id = UNSET
        else:
            thread_id = UUID(_thread_id)

        _run_id = d.pop("run_id", UNSET)
        run_id: UUID | Unset
        if isinstance(_run_id, Unset):
            run_id = UNSET
        else:
            run_id = UUID(_run_id)

        command_invoked_payload = cls(
            invocation_id=invocation_id,
            command_id=command_id,
            command_name=command_name,
            control=control,
            channel_id=channel_id,
            invoked_by=invoked_by,
            options=options,
            thread_id=thread_id,
            run_id=run_id,
        )

        command_invoked_payload.additional_properties = d
        return command_invoked_payload

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
