from enum import StrEnum


class AgentConfigChangedEventType(StrEnum):
    AGENT_CONFIG_CHANGED = "agent.config_changed"

    def __str__(self) -> str:
        return str(self.value)
