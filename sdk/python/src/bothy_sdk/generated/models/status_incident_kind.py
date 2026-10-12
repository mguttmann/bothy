from enum import StrEnum


class StatusIncidentKind(StrEnum):
    INCIDENT = "incident"
    MAINTENANCE = "maintenance"

    def __str__(self) -> str:
        return str(self.value)
