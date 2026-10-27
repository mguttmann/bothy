from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field

if TYPE_CHECKING:
    from ..models.agent_presence import AgentPresence
    from ..models.channel_view import ChannelView
    from ..models.draft_ref import DraftRef
    from ..models.evaluated_flag import EvaluatedFlag
    from ..models.principal_ref import PrincipalRef
    from ..models.run_summary import RunSummary
    from ..models.sidebar_state import SidebarState
    from ..models.space_view import SpaceView
    from ..models.thread import Thread
    from ..models.unread_thread import UnreadThread
    from ..models.user_setting import UserSetting


T = TypeVar("T", bound="Snapshot")


@_attrs_define
class Snapshot:
    """State without thread contents, delivered by `ready` and `GET /sync/snapshot`.

    Attributes:
        head_seq (int):
        permission_version (int):
        sync_epoch (UUID): UUIDv7 (RFC 9562), lowercase canonical form.
        principal (PrincipalRef): Minimal public view of a principal, embedded in other objects.
        is_owner (bool):
        spaces (list[SpaceView]):
        channels (list[ChannelView]):
        sidebar (list[SidebarState]):
        unread_threads (list[UnreadThread]): Only threads with unread messages or mentions.
        active_runs (list[RunSummary]):
        active_threads (list[Thread]): Per visible channel the open threads (`kind = thread`, as `getThread`, without
            messages) with an active run, the open threads in `unread_threads`, plus the open threads whose
            `last_activity_at` lies within `sync.snapshot_thread_window`, newest activity first, until the channel has
            `sync.snapshot_threads_per_channel` entries; threads with an active run and unread threads count toward this
            bound but are never left out (04 6.3 item 4, R-SY-28). Ordered by channel, then by descending
            `last_activity_at`. The client keeps them current through `thread.*` and `message.created`.
        agents (list[AgentPresence]):
        settings (list[UserSetting]):
        drafts (list[DraftRef]):
        features (list[EvaluatedFlag]): Feature flags evaluated for the principal, as in `InstanceInfo.features` (04
            section 6.3, step 4).
    """

    head_seq: int
    permission_version: int
    sync_epoch: UUID
    principal: PrincipalRef
    is_owner: bool
    spaces: list[SpaceView]
    channels: list[ChannelView]
    sidebar: list[SidebarState]
    unread_threads: list[UnreadThread]
    active_runs: list[RunSummary]
    active_threads: list[Thread]
    agents: list[AgentPresence]
    settings: list[UserSetting]
    drafts: list[DraftRef]
    features: list[EvaluatedFlag]
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        head_seq = self.head_seq

        permission_version = self.permission_version

        sync_epoch = str(self.sync_epoch)

        principal = self.principal.to_dict()

        is_owner = self.is_owner

        spaces = []
        for spaces_item_data in self.spaces:
            spaces_item = spaces_item_data.to_dict()
            spaces.append(spaces_item)

        channels = []
        for channels_item_data in self.channels:
            channels_item = channels_item_data.to_dict()
            channels.append(channels_item)

        sidebar = []
        for sidebar_item_data in self.sidebar:
            sidebar_item = sidebar_item_data.to_dict()
            sidebar.append(sidebar_item)

        unread_threads = []
        for unread_threads_item_data in self.unread_threads:
            unread_threads_item = unread_threads_item_data.to_dict()
            unread_threads.append(unread_threads_item)

        active_runs = []
        for active_runs_item_data in self.active_runs:
            active_runs_item = active_runs_item_data.to_dict()
            active_runs.append(active_runs_item)

        active_threads = []
        for active_threads_item_data in self.active_threads:
            active_threads_item = active_threads_item_data.to_dict()
            active_threads.append(active_threads_item)

        agents = []
        for agents_item_data in self.agents:
            agents_item = agents_item_data.to_dict()
            agents.append(agents_item)

        settings = []
        for settings_item_data in self.settings:
            settings_item = settings_item_data.to_dict()
            settings.append(settings_item)

        drafts = []
        for drafts_item_data in self.drafts:
            drafts_item = drafts_item_data.to_dict()
            drafts.append(drafts_item)

        features = []
        for features_item_data in self.features:
            features_item = features_item_data.to_dict()
            features.append(features_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "head_seq": head_seq,
                "permission_version": permission_version,
                "sync_epoch": sync_epoch,
                "principal": principal,
                "is_owner": is_owner,
                "spaces": spaces,
                "channels": channels,
                "sidebar": sidebar,
                "unread_threads": unread_threads,
                "active_runs": active_runs,
                "active_threads": active_threads,
                "agents": agents,
                "settings": settings,
                "drafts": drafts,
                "features": features,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.agent_presence import AgentPresence
        from ..models.channel_view import ChannelView
        from ..models.draft_ref import DraftRef
        from ..models.evaluated_flag import EvaluatedFlag
        from ..models.principal_ref import PrincipalRef
        from ..models.run_summary import RunSummary
        from ..models.sidebar_state import SidebarState
        from ..models.space_view import SpaceView
        from ..models.thread import Thread
        from ..models.unread_thread import UnreadThread
        from ..models.user_setting import UserSetting

        d = dict(src_dict)
        head_seq = d.pop("head_seq")

        permission_version = d.pop("permission_version")

        sync_epoch = UUID(d.pop("sync_epoch"))

        principal = PrincipalRef.from_dict(d.pop("principal"))

        is_owner = d.pop("is_owner")

        spaces = []
        _spaces = d.pop("spaces")
        for spaces_item_data in _spaces:
            spaces_item = SpaceView.from_dict(spaces_item_data)

            spaces.append(spaces_item)

        channels = []
        _channels = d.pop("channels")
        for channels_item_data in _channels:
            channels_item = ChannelView.from_dict(channels_item_data)

            channels.append(channels_item)

        sidebar = []
        _sidebar = d.pop("sidebar")
        for sidebar_item_data in _sidebar:
            sidebar_item = SidebarState.from_dict(sidebar_item_data)

            sidebar.append(sidebar_item)

        unread_threads = []
        _unread_threads = d.pop("unread_threads")
        for unread_threads_item_data in _unread_threads:
            unread_threads_item = UnreadThread.from_dict(unread_threads_item_data)

            unread_threads.append(unread_threads_item)

        active_runs = []
        _active_runs = d.pop("active_runs")
        for active_runs_item_data in _active_runs:
            active_runs_item = RunSummary.from_dict(active_runs_item_data)

            active_runs.append(active_runs_item)

        active_threads = []
        _active_threads = d.pop("active_threads")
        for active_threads_item_data in _active_threads:
            active_threads_item = Thread.from_dict(active_threads_item_data)

            active_threads.append(active_threads_item)

        agents = []
        _agents = d.pop("agents")
        for agents_item_data in _agents:
            agents_item = AgentPresence.from_dict(agents_item_data)

            agents.append(agents_item)

        settings = []
        _settings = d.pop("settings")
        for settings_item_data in _settings:
            settings_item = UserSetting.from_dict(settings_item_data)

            settings.append(settings_item)

        drafts = []
        _drafts = d.pop("drafts")
        for drafts_item_data in _drafts:
            drafts_item = DraftRef.from_dict(drafts_item_data)

            drafts.append(drafts_item)

        features = []
        _features = d.pop("features")
        for features_item_data in _features:
            features_item = EvaluatedFlag.from_dict(features_item_data)

            features.append(features_item)

        snapshot = cls(
            head_seq=head_seq,
            permission_version=permission_version,
            sync_epoch=sync_epoch,
            principal=principal,
            is_owner=is_owner,
            spaces=spaces,
            channels=channels,
            sidebar=sidebar,
            unread_threads=unread_threads,
            active_runs=active_runs,
            active_threads=active_threads,
            agents=agents,
            settings=settings,
            drafts=drafts,
            features=features,
        )

        snapshot.additional_properties = d
        return snapshot

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
