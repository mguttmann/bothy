from enum import StrEnum


class AgentTemplateSource(StrEnum):
    PRIVATE = "private"
    SHIPPED = "shipped"

    def __str__(self) -> str:
        return str(self.value)
