from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

T = TypeVar("T", bound="InvocationTokenRenew")


@_attrs_define
class InvocationTokenRenew:
    """
    Attributes:
        invocation_token (str): The current invocation token of the run, or on a retry after a lost answer the token the
            last renewal replaced; it may have expired (09 section 4.5.1).
    """

    invocation_token: str

    def to_dict(self) -> dict[str, Any]:
        invocation_token = self.invocation_token

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "invocation_token": invocation_token,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        invocation_token = d.pop("invocation_token")

        invocation_token_renew = cls(
            invocation_token=invocation_token,
        )

        return invocation_token_renew
