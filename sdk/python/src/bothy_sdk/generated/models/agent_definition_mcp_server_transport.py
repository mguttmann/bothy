from enum import StrEnum


class AgentDefinitionMcpServerTransport(StrEnum):
    HTTP = "http"
    STDIO = "stdio"

    def __str__(self) -> str:
        return str(self.value)
