from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="RunAwaitInput")


@_attrs_define
class RunAwaitInput:
    """
    Attributes:
        status_line (str):
    """

    status_line: str

    def to_dict(self) -> dict[str, Any]:
        status_line = self.status_line

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "status_line": status_line,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        status_line = d.pop("status_line")

        run_await_input = cls(
            status_line=status_line,
        )

        return run_await_input
