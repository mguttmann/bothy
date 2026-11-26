from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast
from uuid import UUID

from attrs import define as _attrs_define

from ..models.computer_close_detail import ComputerCloseDetail
from ..models.host_computer_ended_close_reason import HostComputerEndedCloseReason
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.host_computer_ended_action_counts import HostComputerEndedActionCounts


T = TypeVar("T", bound="HostComputerEnded")


@_attrs_define
class HostComputerEnded:
    """
    Attributes:
        session_id (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        close_reason (HostComputerEndedCloseReason): The host service sends only these reasons (27 12.3).
        close_detail (ComputerCloseDetail | Unset): Closed list of 27 8.3: only with `stopped_by_user` (`local_escape`,
            `indicator_stop`, `client_stop`), `open_failed` (`token_invalid`, `token_expired`, `token_replayed`,
            `handshake_failed`, `helper_absent`, `helper_untrusted`, `not_enabled`, `busy`, `open_timeout`) and `error`
            (`stream_fault`, `helper_crashed`).
        action_counts (HostComputerEndedActionCounts | Unset): A count per action kind of 27 6.4.
        apps (list[str] | Unset): Identities of the frontmost apps (bundle ID or executable name with the signer's team
            ID or subject); the server keeps at most `computer.audit_apps_max`.
        paused_ms (int | Unset):
    """

    session_id: UUID
    close_reason: HostComputerEndedCloseReason
    close_detail: ComputerCloseDetail | Unset = UNSET
    action_counts: HostComputerEndedActionCounts | Unset = UNSET
    apps: list[str] | Unset = UNSET
    paused_ms: int | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        session_id = str(self.session_id)

        close_reason = self.close_reason.value

        close_detail: str | Unset = UNSET
        if not isinstance(self.close_detail, Unset):
            close_detail = self.close_detail.value

        action_counts: dict[str, Any] | Unset = UNSET
        if not isinstance(self.action_counts, Unset):
            action_counts = self.action_counts.to_dict()

        apps: list[str] | Unset = UNSET
        if not isinstance(self.apps, Unset):
            apps = self.apps

        paused_ms = self.paused_ms

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "session_id": session_id,
                "close_reason": close_reason,
            }
        )
        if close_detail is not UNSET:
            field_dict["close_detail"] = close_detail
        if action_counts is not UNSET:
            field_dict["action_counts"] = action_counts
        if apps is not UNSET:
            field_dict["apps"] = apps
        if paused_ms is not UNSET:
            field_dict["paused_ms"] = paused_ms

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.host_computer_ended_action_counts import (
            HostComputerEndedActionCounts,
        )

        d = dict(src_dict)
        session_id = UUID(d.pop("session_id"))

        close_reason = HostComputerEndedCloseReason(d.pop("close_reason"))

        _close_detail = d.pop("close_detail", UNSET)
        close_detail: ComputerCloseDetail | Unset
        if isinstance(_close_detail, Unset):
            close_detail = UNSET
        else:
            close_detail = ComputerCloseDetail(_close_detail)

        _action_counts = d.pop("action_counts", UNSET)
        action_counts: HostComputerEndedActionCounts | Unset
        if isinstance(_action_counts, Unset):
            action_counts = UNSET
        else:
            action_counts = HostComputerEndedActionCounts.from_dict(_action_counts)

        apps = cast(list[str], d.pop("apps", UNSET))

        paused_ms = d.pop("paused_ms", UNSET)

        host_computer_ended = cls(
            session_id=session_id,
            close_reason=close_reason,
            close_detail=close_detail,
            action_counts=action_counts,
            apps=apps,
            paused_ms=paused_ms,
        )

        return host_computer_ended
