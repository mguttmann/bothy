from enum import StrEnum


class BillingEvidenceKeyType(StrEnum):
    API_KEY = "api_key"
    CLOUD = "cloud"
    NONE = "none"
    OAUTH = "oauth"
    UNKNOWN = "unknown"

    def __str__(self) -> str:
        return str(self.value)
