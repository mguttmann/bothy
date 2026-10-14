from enum import StrEnum


class SessionClientComponent(StrEnum):
    APP = "app"
    CONSOLE = "console"
    SHARE_EXT = "share_ext"

    def __str__(self) -> str:
        return str(self.value)
