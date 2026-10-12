from enum import StrEnum


class WebhookKeyState(StrEnum):
    ACTIVE = "active"
    NEXT = "next"
    RETIRING = "retiring"

    def __str__(self) -> str:
        return str(self.value)
