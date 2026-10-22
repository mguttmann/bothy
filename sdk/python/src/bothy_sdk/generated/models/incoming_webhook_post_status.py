from enum import StrEnum


class IncomingWebhookPostStatus(StrEnum):
    FIRING = "firing"
    RESOLVED = "resolved"

    def __str__(self) -> str:
        return str(self.value)
