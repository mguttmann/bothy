from enum import StrEnum


class TargetKind(StrEnum):
    CHANNEL = "channel"
    HOST = "host"
    INSTANCE = "instance"
    SPACE = "space"

    def __str__(self) -> str:
        return str(self.value)
