from enum import StrEnum


class AccessTargetKind(StrEnum):
    CHANNEL = "channel"
    HOST = "host"
    SPACE = "space"

    def __str__(self) -> str:
        return str(self.value)
