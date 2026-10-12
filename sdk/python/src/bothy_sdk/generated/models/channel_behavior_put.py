from __future__ import annotations

from collections.abc import Mapping
from typing import Any, Self, TypeVar, cast

from attrs import define as _attrs_define

T = TypeVar("T", bound="ChannelBehaviorPut")


@_attrs_define
class ChannelBehaviorPut:
    """
    Attributes:
        require_mention (bool):
        thread_require_mention (bool):
        free_response (bool):
        auto_thread (bool):
        allow_mentioned_agents (bool):
        history_backfill_limit (int):
        prompt (str):
        skills (list[str]):
    """

    require_mention: bool
    thread_require_mention: bool
    free_response: bool
    auto_thread: bool
    allow_mentioned_agents: bool
    history_backfill_limit: int
    prompt: str
    skills: list[str]

    def to_dict(self) -> dict[str, Any]:
        require_mention = self.require_mention

        thread_require_mention = self.thread_require_mention

        free_response = self.free_response

        auto_thread = self.auto_thread

        allow_mentioned_agents = self.allow_mentioned_agents

        history_backfill_limit = self.history_backfill_limit

        prompt = self.prompt

        skills = self.skills

        field_dict: dict[str, Any] = {}

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
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        require_mention = d.pop("require_mention")

        thread_require_mention = d.pop("thread_require_mention")

        free_response = d.pop("free_response")

        auto_thread = d.pop("auto_thread")

        allow_mentioned_agents = d.pop("allow_mentioned_agents")

        history_backfill_limit = d.pop("history_backfill_limit")

        prompt = d.pop("prompt")

        skills = cast(list[str], d.pop("skills"))

        channel_behavior_put = cls(
            require_mention=require_mention,
            thread_require_mention=thread_require_mention,
            free_response=free_response,
            auto_thread=auto_thread,
            allow_mentioned_agents=allow_mentioned_agents,
            history_backfill_limit=history_backfill_limit,
            prompt=prompt,
            skills=skills,
        )

        return channel_behavior_put
