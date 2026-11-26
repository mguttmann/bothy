from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define

from ..models.computer_close_detail import ComputerCloseDetail
from ..models.host_computer_ended_close_reason import HostComputerEndedCloseReason
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.host_computer_app import HostComputerApp
    from ..models.host_computer_ended_action_counts import HostComputerEndedActionCounts


T = TypeVar("T", bound="HostComputerEnded")


@_attrs_define
class HostComputerEnded:
    """
    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        close_reason (HostComputerEndedCloseReason): The host service sends only these reasons (27 12.3).
        action_counts (HostComputerEndedActionCounts): A count per action kind of 27 6.4.
        apps (list[HostComputerApp]): Identities of the frontmost apps; the server keeps at most
            `computer.audit_apps_max` in its audit entry.
        paused_ms (int):
        close_detail (ComputerCloseDetail | Unset): Closed list of 27 8.3: only with `stopped_by_user` (`local_escape`,
            `indicator_stop`, `client_stop`), `open_failed` (`token_invalid`, `token_expired`, `token_replayed`,
            `handshake_failed`, `helper_absent`, `helper_untrusted`, `not_enabled`, `busy`, `open_timeout`) and `error`
            (`stream_fault`, `helper_crashed`).
    """

    session_id: UUID
    close_reason: HostComputerEndedCloseReason
    action_counts: HostComputerEndedActionCounts
    apps: list[HostComputerApp]
    paused_ms: int
    close_detail: ComputerCloseDetail | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        close_reason = self.close_reason.value

        action_counts = self.action_counts.to_dict()

        apps = []
        for apps_item_data in self.apps:
            apps_item = apps_item_data.to_dict()
            apps.append(apps_item)

        paused_ms = self.paused_ms

        close_detail: str | Unset = UNSET
        if not isinstance(self.close_detail, Unset):
            close_detail = self.close_detail.value

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "session_id": session_id,
                "close_reason": close_reason,
                "action_counts": action_counts,
                "apps": apps,
                "paused_ms": paused_ms,
            }
        )
        if close_detail is not UNSET:
            field_dict["close_detail"] = close_detail

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_computer_app import HostComputerApp
        from ..models.host_computer_ended_action_counts import (
            HostComputerEndedActionCounts,
        )

        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        close_reason = HostComputerEndedCloseReason(d.pop("close_reason"))

        action_counts = HostComputerEndedActionCounts.from_dict(d.pop("action_counts"))

        apps = []
        _apps = d.pop("apps")
        for apps_item_data in _apps:
            apps_item = HostComputerApp.from_dict(apps_item_data)

            apps.append(apps_item)

        paused_ms = d.pop("paused_ms")

        _close_detail = d.pop("close_detail", UNSET)
        close_detail: ComputerCloseDetail | Unset
        if isinstance(_close_detail, Unset):
            close_detail = UNSET
        else:
            close_detail = ComputerCloseDetail(_close_detail)

        host_computer_ended = cls(
            session_id=session_id,
            close_reason=close_reason,
            action_counts=action_counts,
            apps=apps,
            paused_ms=paused_ms,
            close_detail=close_detail,
        )

        return host_computer_ended
