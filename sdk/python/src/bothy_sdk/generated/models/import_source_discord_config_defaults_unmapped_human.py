from enum import StrEnum


class ImportSourceDiscordConfigDefaultsUnmappedHuman(StrEnum):
    PLACEHOLDER = "placeholder"
    SKIP = "skip"

    def __str__(self) -> str:
        return str(self.value)
