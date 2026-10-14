from enum import StrEnum


class TranscriptSource(StrEnum):
    AGENT = "agent"
    SERVICE = "service"

    def __str__(self) -> str:
        return str(self.value)
