from enum import StrEnum


class AgentInvocationEventType(StrEnum):
    AGENT_INVOCATION = "agent.invocation"

    def __str__(self) -> str:
        return str(self.value)
