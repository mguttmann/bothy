from enum import StrEnum


class AgentDefinitionChangedEventType(StrEnum):
    AGENT_DEFINITION_CHANGED = "agent.definition_changed"

    def __str__(self) -> str:
        return str(self.value)
