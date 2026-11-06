from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar

from attrs import define as _attrs_define

from ..types import UNSET, Unset

T = TypeVar("T", bound="AgentDefinitionEntry")


@_attrs_define
class AgentDefinitionEntry:
    """A header or an environment entry with exactly one of `value` and `secret_ref`. A header named `Authorization`,
    `Cookie` or `Proxy-Authorization`, and every entry whose name contains `KEY`, `TOKEN`, `SECRET`, `PASSWORD`,
    `PASSWD`, `CREDENTIAL`, `AUTH` or `COOKIE` (any case), needs `secret_ref` (`secret_value_not_allowed`). The server
    never resolves a `secret_ref`.

        Attributes:
            name (str):
            value (str | Unset): A plain value, shown in the preview.
            secret_ref (str | Unset): A key of the secret provider of the hosting runner, `env:<NAME>` or `file:<absolute
                path>`.
    """

    name: str
    value: str | Unset = UNSET
    secret_ref: str | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        name = self.name

        value = self.value

        secret_ref = self.secret_ref

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "name": name,
            }
        )
        if value is not UNSET:
            field_dict["value"] = value
        if secret_ref is not UNSET:
            field_dict["secret_ref"] = secret_ref

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        name = d.pop("name")

        value = d.pop("value", UNSET)

        secret_ref = d.pop("secret_ref", UNSET)

        agent_definition_entry = cls(
            name=name,
            value=value,
            secret_ref=secret_ref,
        )

        return agent_definition_entry
