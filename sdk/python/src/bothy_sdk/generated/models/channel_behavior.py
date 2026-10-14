from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.project_binding import ProjectBinding


T = TypeVar("T", bound="ChannelBehavior")


@_attrs_define
class ChannelBehavior:
    """Behavior of the operator in a channel. Readable only with `manage`; never in events.

    Attributes:
        require_mention (bool):
        thread_require_mention (bool):
        free_response (bool):
        auto_thread (bool):
        allow_mentioned_agents (bool):
        history_backfill_limit (int):
        prompt (str):
        skills (list[str]): Ordered skill references the adapter understands; the server does not interpret them.
        settings_version (int):
        project_binding (ProjectBinding | Unset):
    """

    require_mention: bool
    thread_require_mention: bool
    free_response: bool
    auto_thread: bool
    allow_mentioned_agents: bool
    history_backfill_limit: int
    prompt: str
    skills: list[str]
    settings_version: int
    project_binding: ProjectBinding | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        require_mention = self.require_mention

        thread_require_mention = self.thread_require_mention

        free_response = self.free_response

        auto_thread = self.auto_thread

        allow_mentioned_agents = self.allow_mentioned_agents

        history_backfill_limit = self.history_backfill_limit

        prompt = self.prompt

        skills = self.skills

        settings_version = self.settings_version

        project_binding: dict[str, Any] | Unset = UNSET
        if not isinstance(self.project_binding, Unset):
            project_binding = self.project_binding.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "require_mention": require_mention,
                "thread_require_mention": thread_require_mention,
                "free_response": free_response,
                "auto_thread": auto_thread,
                "allow_mentioned_agents": allow_mentioned_agents,
                "history_backfill_limit": history_backfill_limit,
                "prompt": prompt,
                "skills": skills,
                "settings_version": settings_version,
            }
        )
        if project_binding is not UNSET:
            field_dict["project_binding"] = project_binding

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.project_binding import ProjectBinding

        d = dict(src_dict)
        require_mention = d.pop("require_mention")

        thread_require_mention = d.pop("thread_require_mention")

        free_response = d.pop("free_response")

        auto_thread = d.pop("auto_thread")

        allow_mentioned_agents = d.pop("allow_mentioned_agents")

        history_backfill_limit = d.pop("history_backfill_limit")

        prompt = d.pop("prompt")

        skills = cast(list[str], d.pop("skills"))

        settings_version = d.pop("settings_version")

        _project_binding = d.pop("project_binding", UNSET)
        project_binding: ProjectBinding | Unset
        if isinstance(_project_binding, Unset):
            project_binding = UNSET
        else:
            project_binding = ProjectBinding.from_dict(_project_binding)

        channel_behavior = cls(
            require_mention=require_mention,
            thread_require_mention=thread_require_mention,
            free_response=free_response,
            auto_thread=auto_thread,
            allow_mentioned_agents=allow_mentioned_agents,
            history_backfill_limit=history_backfill_limit,
            prompt=prompt,
            skills=skills,
            settings_version=settings_version,
            project_binding=project_binding,
        )

        channel_behavior.additional_properties = d
        return channel_behavior

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
