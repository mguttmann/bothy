from enum import StrEnum


class AgentInterviewCatalogueEntryToolClassWish(StrEnum):
    CHAT = "chat"
    HOST = "host"
    SCOPED = "scoped"

    def __str__(self) -> str:
        return str(self.value)
