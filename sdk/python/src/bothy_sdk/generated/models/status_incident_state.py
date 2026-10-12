from enum import StrEnum


class StatusIncidentState(StrEnum):
    IDENTIFIED = "identified"
    INVESTIGATING = "investigating"
    MONITORING = "monitoring"
    RESOLVED = "resolved"
    SCHEDULED = "scheduled"

    def __str__(self) -> str:
        return str(self.value)
