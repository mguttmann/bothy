from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define

if TYPE_CHECKING:
    from ..models.json_object import JsonObject


T = TypeVar("T", bound="CspReportLegacy")


@_attrs_define
class CspReportLegacy:
    """Legacy report of the `report-uri` directive (CSP Level 2). The server reads only `effective-directive` (else
    `violated-directive`), `blocked-uri` and `disposition` of `csp-report`.

        Attributes:
            csp_report (JsonObject): Free-form JSON object whose schema is named in the field description.
    """

    csp_report: JsonObject

    def to_dict(self) -> dict[str, Any]:
        csp_report = self.csp_report.to_dict()

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "csp-report": csp_report,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.json_object import JsonObject

        d = dict(src_dict)
        csp_report = JsonObject.from_dict(d.pop("csp-report"))

        csp_report_legacy = cls(
            csp_report=csp_report,
        )

        return csp_report_legacy
