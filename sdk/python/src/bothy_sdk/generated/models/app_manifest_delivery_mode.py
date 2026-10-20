from enum import StrEnum


class AppManifestDeliveryMode(StrEnum):
    GATEWAY = "gateway"
    WEBHOOK = "webhook"

    def __str__(self) -> str:
        return str(self.value)
