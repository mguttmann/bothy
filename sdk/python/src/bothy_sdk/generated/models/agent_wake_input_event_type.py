from enum import StrEnum


class AgentWakeInputEventType(StrEnum):
    AGENT_WAKE_INPUT = "agent.wake_input"

    def __str__(self) -> str:
        return str(self.value)
