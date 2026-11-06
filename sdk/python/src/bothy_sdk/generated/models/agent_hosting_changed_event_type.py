from enum import StrEnum


class AgentHostingChangedEventType(StrEnum):
    AGENT_HOSTING_CHANGED = "agent.hosting_changed"

    def __str__(self) -> str:
        return str(self.value)
