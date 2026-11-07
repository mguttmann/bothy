from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="RunAutonomyLast")


@_attrs_define
class RunAutonomyLast:
    """The idle flag of the last autonomous input or notification turn (09 5.8.6); the server counts each distinct `input`
    once in the idle count of the run's thread and person.

        Attributes:
            input_ (str): The wake-up id of a wake run or wake input, or the runner's id of a notification turn.
            idle (bool): True when the input made no tool call other than `bothy-wake` and started no background task.
    """

    input_: str
    idle: bool

    def to_dict(self) -> dict[str, Any]:
        input_ = self.input_

        idle = self.idle

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "input": input_,
                "idle": idle,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        input_ = d.pop("input")

        idle = d.pop("idle")

        run_autonomy_last = cls(
            input_=input_,
            idle=idle,
        )

        return run_autonomy_last
