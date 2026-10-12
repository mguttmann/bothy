from enum import StrEnum


class AgentPresenceChangedEventType(StrEnum):
    AGENT_PRESENCE_CHANGED = "agent.presence_changed"

    def __str__(self) -> str:
        return str(self.value)
