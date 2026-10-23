from enum import StrEnum


class ImportUserDataDiscordKind(StrEnum):
    BOT = "bot"
    USER = "user"
    WEBHOOK = "webhook"

    def __str__(self) -> str:
        return str(self.value)
