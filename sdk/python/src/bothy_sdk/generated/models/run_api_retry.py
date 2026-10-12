from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="RunApiRetry")


@_attrs_define
class RunApiRetry:
    """Last retry of a model API call, shown in the live box while set.

    Attributes:
        attempt (int):
        max_retries (int | Unset):
        retry_delay_ms (int | Unset):
        error (str | Unset):
    """

    attempt: int
    max_retries: int | Unset = UNSET
    retry_delay_ms: int | Unset = UNSET
    error: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        attempt = self.attempt

        max_retries = self.max_retries

        retry_delay_ms = self.retry_delay_ms

        error = self.error

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "attempt": attempt,
            }
        )
        if max_retries is not UNSET:
            field_dict["max_retries"] = max_retries
        if retry_delay_ms is not UNSET:
            field_dict["retry_delay_ms"] = retry_delay_ms
        if error is not UNSET:
            field_dict["error"] = error

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        attempt = d.pop("attempt")

        max_retries = d.pop("max_retries", UNSET)

        retry_delay_ms = d.pop("retry_delay_ms", UNSET)

        error = d.pop("error", UNSET)

        run_api_retry = cls(
            attempt=attempt,
            max_retries=max_retries,
            retry_delay_ms=retry_delay_ms,
            error=error,
        )

        return run_api_retry
