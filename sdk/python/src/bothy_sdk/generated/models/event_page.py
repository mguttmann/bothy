from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.access_changed_event import AccessChangedEvent
    from ..models.access_expiring_event import AccessExpiringEvent
    from ..models.agent_config_changed_event import AgentConfigChangedEvent
    from ..models.agent_followup_event import AgentFollowupEvent
    from ..models.agent_invocation_event import AgentInvocationEvent
    from ..models.agent_session_changed_event import AgentSessionChangedEvent
    from ..models.agent_updated_event import AgentUpdatedEvent
    from ..models.channel_archived_event import ChannelArchivedEvent
    from ..models.channel_created_event import ChannelCreatedEvent
    from ..models.channel_settings_changed_event import ChannelSettingsChangedEvent
    from ..models.channel_updated_event import ChannelUpdatedEvent
    from ..models.command_changed_event import CommandChangedEvent
    from ..models.command_invoked_event import CommandInvokedEvent
    from ..models.device_pending_event import DevicePendingEvent
    from ..models.draft_deleted_event import DraftDeletedEvent
    from ..models.draft_updated_event import DraftUpdatedEvent
    from ..models.file_updated_event import FileUpdatedEvent
    from ..models.gdpr_request_due_event import GdprRequestDueEvent
    from ..models.gdpr_request_updated_event import GdprRequestUpdatedEvent
    from ..models.grant_changed_event import GrantChangedEvent
    from ..models.group_changed_event import GroupChangedEvent
    from ..models.host_updated_event import HostUpdatedEvent
    from ..models.import_progress_event import ImportProgressEvent
    from ..models.instance_updated_event import InstanceUpdatedEvent
    from ..models.interaction_created_event import InteractionCreatedEvent
    from ..models.message_created_event import MessageCreatedEvent
    from ..models.message_deleted_event import MessageDeletedEvent
    from ..models.message_updated_event import MessageUpdatedEvent
    from ..models.notification_pref_updated_event import NotificationPrefUpdatedEvent
    from ..models.pin_added_event import PinAddedEvent
    from ..models.pin_removed_event import PinRemovedEvent
    from ..models.quiet_hours_updated_event import QuietHoursUpdatedEvent
    from ..models.reaction_added_event import ReactionAddedEvent
    from ..models.reaction_removed_event import ReactionRemovedEvent
    from ..models.read_state_updated_event import ReadStateUpdatedEvent
    from ..models.run_cancel_requested_event import RunCancelRequestedEvent
    from ..models.run_state_changed_event import RunStateChangedEvent
    from ..models.session_revoked_event import SessionRevokedEvent
    from ..models.settings_updated_event import SettingsUpdatedEvent
    from ..models.sidebar_channel_updated_event import SidebarChannelUpdatedEvent
    from ..models.sidebar_updated_event import SidebarUpdatedEvent
    from ..models.space_archived_event import SpaceArchivedEvent
    from ..models.space_created_event import SpaceCreatedEvent
    from ..models.space_updated_event import SpaceUpdatedEvent
    from ..models.status_updated_event import StatusUpdatedEvent
    from ..models.terminal_session_changed_event import TerminalSessionChangedEvent
    from ..models.thread_closed_event import ThreadClosedEvent
    from ..models.thread_created_event import ThreadCreatedEvent
    from ..models.thread_moved_event import ThreadMovedEvent
    from ..models.thread_moved_out_event import ThreadMovedOutEvent
    from ..models.thread_reopened_event import ThreadReopenedEvent
    from ..models.thread_updated_event import ThreadUpdatedEvent
    from ..models.user_updated_event import UserUpdatedEvent


T = TypeVar("T", bound="EventPage")


@_attrs_define
class EventPage:
    """
    Attributes:
        events (list[AccessChangedEvent | AccessExpiringEvent | AgentConfigChangedEvent | AgentFollowupEvent |
            AgentInvocationEvent | AgentSessionChangedEvent | AgentUpdatedEvent | ChannelArchivedEvent | ChannelCreatedEvent
            | ChannelSettingsChangedEvent | ChannelUpdatedEvent | CommandChangedEvent | CommandInvokedEvent |
            DevicePendingEvent | DraftDeletedEvent | DraftUpdatedEvent | FileUpdatedEvent | GdprRequestDueEvent |
            GdprRequestUpdatedEvent | GrantChangedEvent | GroupChangedEvent | HostUpdatedEvent | ImportProgressEvent |
            InstanceUpdatedEvent | InteractionCreatedEvent | MessageCreatedEvent | MessageDeletedEvent | MessageUpdatedEvent
            | NotificationPrefUpdatedEvent | PinAddedEvent | PinRemovedEvent | QuietHoursUpdatedEvent | ReactionAddedEvent |
            ReactionRemovedEvent | ReadStateUpdatedEvent | RunCancelRequestedEvent | RunStateChangedEvent |
            SessionRevokedEvent | SettingsUpdatedEvent | SidebarChannelUpdatedEvent | SidebarUpdatedEvent |
            SpaceArchivedEvent | SpaceCreatedEvent | SpaceUpdatedEvent | StatusUpdatedEvent | TerminalSessionChangedEvent |
            ThreadClosedEvent | ThreadCreatedEvent | ThreadMovedEvent | ThreadMovedOutEvent | ThreadReopenedEvent |
            ThreadUpdatedEvent | UserUpdatedEvent]):
        next_cursor (int): Highest checked `seq`; use as `after` for the next call even if `events` is empty.
        head_seq (int):
        sync_epoch (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
    """

    events: list[
        AccessChangedEvent
        | AccessExpiringEvent
        | AgentConfigChangedEvent
        | AgentFollowupEvent
        | AgentInvocationEvent
        | AgentSessionChangedEvent
        | AgentUpdatedEvent
        | ChannelArchivedEvent
        | ChannelCreatedEvent
        | ChannelSettingsChangedEvent
        | ChannelUpdatedEvent
        | CommandChangedEvent
        | CommandInvokedEvent
        | DevicePendingEvent
        | DraftDeletedEvent
        | DraftUpdatedEvent
        | FileUpdatedEvent
        | GdprRequestDueEvent
        | GdprRequestUpdatedEvent
        | GrantChangedEvent
        | GroupChangedEvent
        | HostUpdatedEvent
        | ImportProgressEvent
        | InstanceUpdatedEvent
        | InteractionCreatedEvent
        | MessageCreatedEvent
        | MessageDeletedEvent
        | MessageUpdatedEvent
        | NotificationPrefUpdatedEvent
        | PinAddedEvent
        | PinRemovedEvent
        | QuietHoursUpdatedEvent
        | ReactionAddedEvent
        | ReactionRemovedEvent
        | ReadStateUpdatedEvent
        | RunCancelRequestedEvent
        | RunStateChangedEvent
        | SessionRevokedEvent
        | SettingsUpdatedEvent
        | SidebarChannelUpdatedEvent
        | SidebarUpdatedEvent
        | SpaceArchivedEvent
        | SpaceCreatedEvent
        | SpaceUpdatedEvent
        | StatusUpdatedEvent
        | TerminalSessionChangedEvent
        | ThreadClosedEvent
        | ThreadCreatedEvent
        | ThreadMovedEvent
        | ThreadMovedOutEvent
        | ThreadReopenedEvent
        | ThreadUpdatedEvent
        | UserUpdatedEvent
    ]
    next_cursor: int
    head_seq: int
    sync_epoch: UUID
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        from ..models.access_changed_event import AccessChangedEvent
        from ..models.access_expiring_event import AccessExpiringEvent
        from ..models.agent_config_changed_event import AgentConfigChangedEvent
        from ..models.agent_followup_event import AgentFollowupEvent
        from ..models.agent_invocation_event import AgentInvocationEvent
        from ..models.agent_session_changed_event import AgentSessionChangedEvent
        from ..models.agent_updated_event import AgentUpdatedEvent
        from ..models.channel_archived_event import ChannelArchivedEvent
        from ..models.channel_created_event import ChannelCreatedEvent
        from ..models.channel_settings_changed_event import ChannelSettingsChangedEvent
        from ..models.channel_updated_event import ChannelUpdatedEvent
        from ..models.command_changed_event import CommandChangedEvent
        from ..models.command_invoked_event import CommandInvokedEvent
        from ..models.device_pending_event import DevicePendingEvent
        from ..models.draft_deleted_event import DraftDeletedEvent
        from ..models.draft_updated_event import DraftUpdatedEvent
        from ..models.file_updated_event import FileUpdatedEvent
        from ..models.gdpr_request_due_event import GdprRequestDueEvent
        from ..models.gdpr_request_updated_event import GdprRequestUpdatedEvent
        from ..models.grant_changed_event import GrantChangedEvent
        from ..models.group_changed_event import GroupChangedEvent
        from ..models.host_updated_event import HostUpdatedEvent
        from ..models.import_progress_event import ImportProgressEvent
        from ..models.instance_updated_event import InstanceUpdatedEvent
        from ..models.interaction_created_event import InteractionCreatedEvent
        from ..models.message_created_event import MessageCreatedEvent
        from ..models.message_deleted_event import MessageDeletedEvent
        from ..models.message_updated_event import MessageUpdatedEvent
        from ..models.notification_pref_updated_event import (
            NotificationPrefUpdatedEvent,
        )
        from ..models.pin_added_event import PinAddedEvent
        from ..models.pin_removed_event import PinRemovedEvent
        from ..models.quiet_hours_updated_event import QuietHoursUpdatedEvent
        from ..models.reaction_added_event import ReactionAddedEvent
        from ..models.reaction_removed_event import ReactionRemovedEvent
        from ..models.read_state_updated_event import ReadStateUpdatedEvent
        from ..models.run_cancel_requested_event import RunCancelRequestedEvent
        from ..models.run_state_changed_event import RunStateChangedEvent
        from ..models.session_revoked_event import SessionRevokedEvent
        from ..models.settings_updated_event import SettingsUpdatedEvent
        from ..models.sidebar_channel_updated_event import SidebarChannelUpdatedEvent
        from ..models.sidebar_updated_event import SidebarUpdatedEvent
        from ..models.space_archived_event import SpaceArchivedEvent
        from ..models.space_created_event import SpaceCreatedEvent
        from ..models.space_updated_event import SpaceUpdatedEvent
        from ..models.status_updated_event import StatusUpdatedEvent
        from ..models.thread_closed_event import ThreadClosedEvent
        from ..models.thread_created_event import ThreadCreatedEvent
        from ..models.thread_moved_event import ThreadMovedEvent
        from ..models.thread_moved_out_event import ThreadMovedOutEvent
        from ..models.thread_reopened_event import ThreadReopenedEvent
        from ..models.thread_updated_event import ThreadUpdatedEvent
        from ..models.user_updated_event import UserUpdatedEvent

        events = []
        for events_item_data in self.events:
            events_item: dict[str, Any]
            if (
                isinstance(events_item_data, MessageCreatedEvent)
                or isinstance(events_item_data, MessageUpdatedEvent)
                or isinstance(events_item_data, MessageDeletedEvent)
                or isinstance(events_item_data, ReactionAddedEvent)
                or isinstance(events_item_data, ReactionRemovedEvent)
                or isinstance(events_item_data, PinAddedEvent)
                or isinstance(events_item_data, PinRemovedEvent)
                or isinstance(events_item_data, ThreadCreatedEvent)
                or isinstance(events_item_data, ThreadUpdatedEvent)
                or isinstance(events_item_data, ThreadClosedEvent)
                or isinstance(events_item_data, ThreadReopenedEvent)
                or isinstance(events_item_data, ThreadMovedEvent)
                or isinstance(events_item_data, ThreadMovedOutEvent)
                or isinstance(events_item_data, AgentSessionChangedEvent)
                or isinstance(events_item_data, RunStateChangedEvent)
                or isinstance(events_item_data, SpaceCreatedEvent)
                or isinstance(events_item_data, SpaceUpdatedEvent)
                or isinstance(events_item_data, SpaceArchivedEvent)
                or isinstance(events_item_data, ChannelCreatedEvent)
                or isinstance(events_item_data, ChannelUpdatedEvent)
                or isinstance(events_item_data, ChannelSettingsChangedEvent)
                or isinstance(events_item_data, ChannelArchivedEvent)
                or isinstance(events_item_data, CommandChangedEvent)
                or isinstance(events_item_data, FileUpdatedEvent)
                or isinstance(events_item_data, UserUpdatedEvent)
                or isinstance(events_item_data, AgentUpdatedEvent)
                or isinstance(events_item_data, InstanceUpdatedEvent)
                or isinstance(events_item_data, StatusUpdatedEvent)
                or isinstance(events_item_data, GrantChangedEvent)
                or isinstance(events_item_data, GroupChangedEvent)
                or isinstance(events_item_data, AccessChangedEvent)
                or isinstance(events_item_data, AccessExpiringEvent)
                or isinstance(events_item_data, DevicePendingEvent)
                or isinstance(events_item_data, SessionRevokedEvent)
                or isinstance(events_item_data, ReadStateUpdatedEvent)
                or isinstance(events_item_data, DraftUpdatedEvent)
                or isinstance(events_item_data, DraftDeletedEvent)
                or isinstance(events_item_data, SettingsUpdatedEvent)
                or isinstance(events_item_data, SidebarUpdatedEvent)
                or isinstance(events_item_data, SidebarChannelUpdatedEvent)
                or isinstance(events_item_data, NotificationPrefUpdatedEvent)
                or isinstance(events_item_data, QuietHoursUpdatedEvent)
                or isinstance(events_item_data, GdprRequestUpdatedEvent)
                or isinstance(events_item_data, GdprRequestDueEvent)
                or isinstance(events_item_data, ImportProgressEvent)
                or isinstance(events_item_data, AgentInvocationEvent)
                or isinstance(events_item_data, AgentFollowupEvent)
                or isinstance(events_item_data, RunCancelRequestedEvent)
                or isinstance(events_item_data, InteractionCreatedEvent)
                or isinstance(events_item_data, CommandInvokedEvent)
                or isinstance(events_item_data, AgentConfigChangedEvent)
                or isinstance(events_item_data, HostUpdatedEvent)
            ):
                events_item = events_item_data.to_dict()
            else:
                events_item = events_item_data.to_dict()

            events.append(events_item)

        next_cursor = self.next_cursor

        head_seq = self.head_seq

        sync_epoch = str(self.sync_epoch)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "events": events,
                "next_cursor": next_cursor,
                "head_seq": head_seq,
                "sync_epoch": sync_epoch,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.access_changed_event import AccessChangedEvent
        from ..models.access_expiring_event import AccessExpiringEvent
        from ..models.agent_config_changed_event import AgentConfigChangedEvent
        from ..models.agent_followup_event import AgentFollowupEvent
        from ..models.agent_invocation_event import AgentInvocationEvent
        from ..models.agent_session_changed_event import AgentSessionChangedEvent
        from ..models.agent_updated_event import AgentUpdatedEvent
        from ..models.channel_archived_event import ChannelArchivedEvent
        from ..models.channel_created_event import ChannelCreatedEvent
        from ..models.channel_settings_changed_event import ChannelSettingsChangedEvent
        from ..models.channel_updated_event import ChannelUpdatedEvent
        from ..models.command_changed_event import CommandChangedEvent
        from ..models.command_invoked_event import CommandInvokedEvent
        from ..models.device_pending_event import DevicePendingEvent
        from ..models.draft_deleted_event import DraftDeletedEvent
        from ..models.draft_updated_event import DraftUpdatedEvent
        from ..models.file_updated_event import FileUpdatedEvent
        from ..models.gdpr_request_due_event import GdprRequestDueEvent
        from ..models.gdpr_request_updated_event import GdprRequestUpdatedEvent
        from ..models.grant_changed_event import GrantChangedEvent
        from ..models.group_changed_event import GroupChangedEvent
        from ..models.host_updated_event import HostUpdatedEvent
        from ..models.import_progress_event import ImportProgressEvent
        from ..models.instance_updated_event import InstanceUpdatedEvent
        from ..models.interaction_created_event import InteractionCreatedEvent
        from ..models.message_created_event import MessageCreatedEvent
        from ..models.message_deleted_event import MessageDeletedEvent
        from ..models.message_updated_event import MessageUpdatedEvent
        from ..models.notification_pref_updated_event import (
            NotificationPrefUpdatedEvent,
        )
        from ..models.pin_added_event import PinAddedEvent
        from ..models.pin_removed_event import PinRemovedEvent
        from ..models.quiet_hours_updated_event import QuietHoursUpdatedEvent
        from ..models.reaction_added_event import ReactionAddedEvent
        from ..models.reaction_removed_event import ReactionRemovedEvent
        from ..models.read_state_updated_event import ReadStateUpdatedEvent
        from ..models.run_cancel_requested_event import RunCancelRequestedEvent
        from ..models.run_state_changed_event import RunStateChangedEvent
        from ..models.session_revoked_event import SessionRevokedEvent
        from ..models.settings_updated_event import SettingsUpdatedEvent
        from ..models.sidebar_channel_updated_event import SidebarChannelUpdatedEvent
        from ..models.sidebar_updated_event import SidebarUpdatedEvent
        from ..models.space_archived_event import SpaceArchivedEvent
        from ..models.space_created_event import SpaceCreatedEvent
        from ..models.space_updated_event import SpaceUpdatedEvent
        from ..models.status_updated_event import StatusUpdatedEvent
        from ..models.terminal_session_changed_event import TerminalSessionChangedEvent
        from ..models.thread_closed_event import ThreadClosedEvent
        from ..models.thread_created_event import ThreadCreatedEvent
        from ..models.thread_moved_event import ThreadMovedEvent
        from ..models.thread_moved_out_event import ThreadMovedOutEvent
        from ..models.thread_reopened_event import ThreadReopenedEvent
        from ..models.thread_updated_event import ThreadUpdatedEvent
        from ..models.user_updated_event import UserUpdatedEvent

        d = dict(src_dict)
        events = []
        _events = d.pop("events")
        for events_item_data in _events:

            def _parse_events_item(
                data: object,
            ) -> (
                AccessChangedEvent
                | AccessExpiringEvent
                | AgentConfigChangedEvent
                | AgentFollowupEvent
                | AgentInvocationEvent
                | AgentSessionChangedEvent
                | AgentUpdatedEvent
                | ChannelArchivedEvent
                | ChannelCreatedEvent
                | ChannelSettingsChangedEvent
                | ChannelUpdatedEvent
                | CommandChangedEvent
                | CommandInvokedEvent
                | DevicePendingEvent
                | DraftDeletedEvent
                | DraftUpdatedEvent
                | FileUpdatedEvent
                | GdprRequestDueEvent
                | GdprRequestUpdatedEvent
                | GrantChangedEvent
                | GroupChangedEvent
                | HostUpdatedEvent
                | ImportProgressEvent
                | InstanceUpdatedEvent
                | InteractionCreatedEvent
                | MessageCreatedEvent
                | MessageDeletedEvent
                | MessageUpdatedEvent
                | NotificationPrefUpdatedEvent
                | PinAddedEvent
                | PinRemovedEvent
                | QuietHoursUpdatedEvent
                | ReactionAddedEvent
                | ReactionRemovedEvent
                | ReadStateUpdatedEvent
                | RunCancelRequestedEvent
                | RunStateChangedEvent
                | SessionRevokedEvent
                | SettingsUpdatedEvent
                | SidebarChannelUpdatedEvent
                | SidebarUpdatedEvent
                | SpaceArchivedEvent
                | SpaceCreatedEvent
                | SpaceUpdatedEvent
                | StatusUpdatedEvent
                | TerminalSessionChangedEvent
                | ThreadClosedEvent
                | ThreadCreatedEvent
                | ThreadMovedEvent
                | ThreadMovedOutEvent
                | ThreadReopenedEvent
                | ThreadUpdatedEvent
                | UserUpdatedEvent
            ):
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_0 = MessageCreatedEvent.from_dict(data)

                    return componentsschemas_event_type_0
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_1 = MessageUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_1
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_2 = MessageDeletedEvent.from_dict(data)

                    return componentsschemas_event_type_2
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_3 = ReactionAddedEvent.from_dict(data)

                    return componentsschemas_event_type_3
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_4 = ReactionRemovedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_4
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_5 = PinAddedEvent.from_dict(data)

                    return componentsschemas_event_type_5
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_6 = PinRemovedEvent.from_dict(data)

                    return componentsschemas_event_type_6
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_7 = ThreadCreatedEvent.from_dict(data)

                    return componentsschemas_event_type_7
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_8 = ThreadUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_8
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_9 = ThreadClosedEvent.from_dict(data)

                    return componentsschemas_event_type_9
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_10 = ThreadReopenedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_10
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_11 = ThreadMovedEvent.from_dict(data)

                    return componentsschemas_event_type_11
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_12 = ThreadMovedOutEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_12
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_13 = (
                        AgentSessionChangedEvent.from_dict(data)
                    )

                    return componentsschemas_event_type_13
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_14 = RunStateChangedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_14
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_15 = SpaceCreatedEvent.from_dict(data)

                    return componentsschemas_event_type_15
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_16 = SpaceUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_16
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_17 = SpaceArchivedEvent.from_dict(data)

                    return componentsschemas_event_type_17
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_18 = ChannelCreatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_18
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_19 = ChannelUpdatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_19
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_20 = (
                        ChannelSettingsChangedEvent.from_dict(data)
                    )

                    return componentsschemas_event_type_20
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_21 = ChannelArchivedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_21
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_22 = CommandChangedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_22
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_23 = FileUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_23
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_24 = UserUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_24
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_25 = AgentUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_25
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_26 = InstanceUpdatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_26
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_27 = StatusUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_27
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_28 = GrantChangedEvent.from_dict(data)

                    return componentsschemas_event_type_28
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_29 = GroupChangedEvent.from_dict(data)

                    return componentsschemas_event_type_29
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_30 = AccessChangedEvent.from_dict(data)

                    return componentsschemas_event_type_30
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_31 = AccessExpiringEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_31
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_32 = DevicePendingEvent.from_dict(data)

                    return componentsschemas_event_type_32
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_33 = SessionRevokedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_33
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_34 = ReadStateUpdatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_34
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_35 = DraftUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_35
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_36 = DraftDeletedEvent.from_dict(data)

                    return componentsschemas_event_type_36
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_37 = SettingsUpdatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_37
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_38 = SidebarUpdatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_38
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_39 = (
                        SidebarChannelUpdatedEvent.from_dict(data)
                    )

                    return componentsschemas_event_type_39
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_40 = (
                        NotificationPrefUpdatedEvent.from_dict(data)
                    )

                    return componentsschemas_event_type_40
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_41 = QuietHoursUpdatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_41
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_42 = GdprRequestUpdatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_42
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_43 = GdprRequestDueEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_43
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_44 = ImportProgressEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_44
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_45 = AgentInvocationEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_45
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_46 = AgentFollowupEvent.from_dict(data)

                    return componentsschemas_event_type_46
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_47 = RunCancelRequestedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_47
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_48 = InteractionCreatedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_48
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_49 = CommandInvokedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_49
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_50 = AgentConfigChangedEvent.from_dict(
                        data
                    )

                    return componentsschemas_event_type_50
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                try:
                    if not isinstance(data, dict):
                        raise TypeError()
                    componentsschemas_event_type_51 = HostUpdatedEvent.from_dict(data)

                    return componentsschemas_event_type_51
                except (TypeError, ValueError, AttributeError, KeyError):
                    pass
                if not isinstance(data, dict):
                    raise TypeError()
                componentsschemas_event_type_52 = TerminalSessionChangedEvent.from_dict(
                    data
                )

                return componentsschemas_event_type_52

            events_item = _parse_events_item(events_item_data)

            events.append(events_item)

        next_cursor = d.pop("next_cursor")

        head_seq = d.pop("head_seq")

        sync_epoch = UUID(d.pop("sync_epoch"))

        event_page = cls(
            events=events,
            next_cursor=next_cursor,
            head_seq=head_seq,
            sync_epoch=sync_epoch,
        )

        event_page.additional_properties = d
        return event_page

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
