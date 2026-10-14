from enum import StrEnum


class ExtensionSessionCreateClientComponent(StrEnum):
    SHARE_EXT = "share_ext"

    def __str__(self) -> str:
        return str(self.value)
