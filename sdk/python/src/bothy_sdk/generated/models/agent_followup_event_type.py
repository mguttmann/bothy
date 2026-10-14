from enum import StrEnum


class AgentFollowupEventType(StrEnum):
    AGENT_FOLLOWUP = "agent.followup"

    def __str__(self) -> str:
        return str(self.value)
