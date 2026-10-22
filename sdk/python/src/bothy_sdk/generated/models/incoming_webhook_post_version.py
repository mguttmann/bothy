from enum import StrEnum


class IncomingWebhookPostVersion(StrEnum):
    VALUE_0 = "4"

    def __str__(self) -> str:
        return str(self.value)
