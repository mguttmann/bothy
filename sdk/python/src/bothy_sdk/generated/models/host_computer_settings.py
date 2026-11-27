from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field

T = TypeVar("T", bound="HostComputerSettings")


@_attrs_define
class HostComputerSettings:
    """The server values of a computer session (27 18), sent with `computer.open`.

    Attributes:
        image_max_edge (int): Value of `computer.image_max_edge`.
        jpeg_quality (int): Value of `computer.jpeg_quality`.
        settle_ms (int): Value of `computer.settle` in milliseconds.
        auto_screenshot (bool): Value of `computer.auto_screenshot`.
        wait_max_ms (int): Value of `computer.wait_max` in milliseconds.
        key_repeat_max (int): Value of `computer.key_repeat_max`.
        scroll_amount_max (int): Value of `computer.scroll_amount_max`.
        audit_apps_max (int): Value of `computer.audit_apps_max`.
        status_interval_ms (int): Value of `computer.status_interval` in milliseconds.
    """

    image_max_edge: int
    jpeg_quality: int
    settle_ms: int
    auto_screenshot: bool
    wait_max_ms: int
    key_repeat_max: int
    scroll_amount_max: int
    audit_apps_max: int
    status_interval_ms: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        image_max_edge = self.image_max_edge

        jpeg_quality = self.jpeg_quality

        settle_ms = self.settle_ms

        auto_screenshot = self.auto_screenshot

        wait_max_ms = self.wait_max_ms

        key_repeat_max = self.key_repeat_max

        scroll_amount_max = self.scroll_amount_max

        audit_apps_max = self.audit_apps_max

        status_interval_ms = self.status_interval_ms

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "image_max_edge": image_max_edge,
                "jpeg_quality": jpeg_quality,
                "settle_ms": settle_ms,
                "auto_screenshot": auto_screenshot,
                "wait_max_ms": wait_max_ms,
                "key_repeat_max": key_repeat_max,
                "scroll_amount_max": scroll_amount_max,
                "audit_apps_max": audit_apps_max,
                "status_interval_ms": status_interval_ms,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        image_max_edge = d.pop("image_max_edge")

        jpeg_quality = d.pop("jpeg_quality")

        settle_ms = d.pop("settle_ms")

        auto_screenshot = d.pop("auto_screenshot")

        wait_max_ms = d.pop("wait_max_ms")

        key_repeat_max = d.pop("key_repeat_max")

        scroll_amount_max = d.pop("scroll_amount_max")

        audit_apps_max = d.pop("audit_apps_max")

        status_interval_ms = d.pop("status_interval_ms")

        host_computer_settings = cls(
            image_max_edge=image_max_edge,
            jpeg_quality=jpeg_quality,
            settle_ms=settle_ms,
            auto_screenshot=auto_screenshot,
            wait_max_ms=wait_max_ms,
            key_repeat_max=key_repeat_max,
            scroll_amount_max=scroll_amount_max,
            audit_apps_max=audit_apps_max,
            status_interval_ms=status_interval_ms,
        )

        host_computer_settings.additional_properties = d
        return host_computer_settings

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
