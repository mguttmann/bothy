from enum import StrEnum


class HostUpdatedEventType(StrEnum):
    HOST_UPDATED = "host.updated"

    def __str__(self) -> str:
        return str(self.value)
