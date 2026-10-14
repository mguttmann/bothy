from enum import StrEnum


class AgentUpdatedEventType(StrEnum):
    AGENT_UPDATED = "agent.updated"

    def __str__(self) -> str:
        return str(self.value)
