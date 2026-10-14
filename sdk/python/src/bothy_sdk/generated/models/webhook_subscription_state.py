from enum import StrEnum


class WebhookSubscriptionState(StrEnum):
    ACTIVE = "active"
    DISABLED = "disabled"
    PAUSED = "paused"

    def __str__(self) -> str:
        return str(self.value)
