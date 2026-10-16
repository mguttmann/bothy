from enum import StrEnum


class LoginClient(StrEnum):
    APP = "app"
    CONSOLE = "console"

    def __str__(self) -> str:
        return str(self.value)
