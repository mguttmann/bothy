from enum import StrEnum


class AgentSessionChangedEventType(StrEnum):
    AGENT_SESSION_CHANGED = "agent_session.changed"

    def __str__(self) -> str:
        return str(self.value)
