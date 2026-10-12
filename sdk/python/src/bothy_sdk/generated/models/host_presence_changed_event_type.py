from enum import StrEnum


class HostPresenceChangedEventType(StrEnum):
    HOST_PRESENCE_CHANGED = "host.presence_changed"

    def __str__(self) -> str:
        return str(self.value)
