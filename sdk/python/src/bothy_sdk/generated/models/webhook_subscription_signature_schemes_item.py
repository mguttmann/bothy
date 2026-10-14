from enum import StrEnum


class WebhookSubscriptionSignatureSchemesItem(StrEnum):
    V1 = "v1"
    V1A = "v1a"

    def __str__(self) -> str:
        return str(self.value)
