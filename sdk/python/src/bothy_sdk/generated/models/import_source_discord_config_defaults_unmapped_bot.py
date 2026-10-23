from enum import StrEnum


class ImportSourceDiscordConfigDefaultsUnmappedBot(StrEnum):
    PLACEHOLDER = "placeholder"
    SKIP = "skip"

    def __str__(self) -> str:
        return str(self.value)
