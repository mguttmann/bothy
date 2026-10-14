from enum import StrEnum


class EventEnvelopeAudience(StrEnum):
    SELF = "self"

    def __str__(self) -> str:
        return str(self.value)
