from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

from ..models.billing_mode import BillingMode
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.billing_evidence import BillingEvidence


T = TypeVar("T", bound="RunStart")


@_attrs_define
class RunStart:
    """
    Attributes:
        invocation_token (str): JWS from the delivered invocation (I3).
        billing_mode_used (BillingMode): Backend policy of an agent. `personal` runs on the subscription of exactly one
            person.
        external_session_ref (str | Unset): Absent when the runtime creates the reference only during the run; then the
            first `reportAgentSession` creates the session row.
        model (str | Unset):
        billing_evidence (BillingEvidence | Unset): How the runtime proves its actual billing mode. Never a key value or
            prefix (schema `billing_evidence.v1`).
    """

    invocation_token: str
    billing_mode_used: BillingMode
    external_session_ref: str | Unset = UNSET
    model: str | Unset = UNSET
    billing_evidence: BillingEvidence | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        invocation_token = self.invocation_token

        billing_mode_used = self.billing_mode_used.value

        external_session_ref = self.external_session_ref

        model = self.model

        billing_evidence: dict[str, Any] | Unset = UNSET
        if not isinstance(self.billing_evidence, Unset):
            billing_evidence = self.billing_evidence.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "invocation_token": invocation_token,
                "billing_mode_used": billing_mode_used,
            }
        )
        if external_session_ref is not UNSET:
            field_dict["external_session_ref"] = external_session_ref
        if model is not UNSET:
            field_dict["model"] = model
        if billing_evidence is not UNSET:
            field_dict["billing_evidence"] = billing_evidence

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.billing_evidence import BillingEvidence

        d = dict(src_dict)
        invocation_token = d.pop("invocation_token")

        billing_mode_used = BillingMode(d.pop("billing_mode_used"))

        external_session_ref = d.pop("external_session_ref", UNSET)

        model = d.pop("model", UNSET)

        _billing_evidence = d.pop("billing_evidence", UNSET)
        billing_evidence: BillingEvidence | Unset
        if isinstance(_billing_evidence, Unset):
            billing_evidence = UNSET
        else:
            billing_evidence = BillingEvidence.from_dict(_billing_evidence)

        run_start = cls(
            invocation_token=invocation_token,
            billing_mode_used=billing_mode_used,
            external_session_ref=external_session_ref,
            model=model,
            billing_evidence=billing_evidence,
        )

        return run_start
