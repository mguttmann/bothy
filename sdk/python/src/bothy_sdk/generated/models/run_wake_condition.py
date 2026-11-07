from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="RunWakeCondition")


@_attrs_define
class RunWakeCondition:
    """Schema `run_wake_condition.v1` (03 `run_wakes`); `repository`, `number` and `head_commit` for `pr_checks`,
    `command_line` for `command`, nothing for `at`.

        Attributes:
            repository (str | Unset):
            number (int | Unset):
            head_commit (str | Unset): The head commit of the pull request at the creation.
            command_line (str | Unset): Run with `/bin/sh -c` in the turn's working folder; fires on exit code 0. At most
                `message.max_content_bytes` bytes, no control or bidi characters except line feed and tab.
    """

    repository: str | Unset = UNSET
    number: int | Unset = UNSET
    head_commit: str | Unset = UNSET
    command_line: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        repository = self.repository

        number = self.number

        head_commit = self.head_commit

        command_line = self.command_line

        field_dict: dict[str, Any] = {}

        field_dict.update({})
        if repository is not UNSET:
            field_dict["repository"] = repository
        if number is not UNSET:
            field_dict["number"] = number
        if head_commit is not UNSET:
            field_dict["head_commit"] = head_commit
        if command_line is not UNSET:
            field_dict["command_line"] = command_line

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        repository = d.pop("repository", UNSET)

        number = d.pop("number", UNSET)

        head_commit = d.pop("head_commit", UNSET)

        command_line = d.pop("command_line", UNSET)

        run_wake_condition = cls(
            repository=repository,
            number=number,
            head_commit=head_commit,
            command_line=command_line,
        )

        return run_wake_condition
