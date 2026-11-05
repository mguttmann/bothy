from enum import StrEnum


class SearchAssistResultFallbackReason(StrEnum):
    ASSISTANT_NOT_CONFIGURED = "assistant_not_configured"
    ASSISTANT_TIMEOUT = "assistant_timeout"
    ASSISTANT_UNAVAILABLE = "assistant_unavailable"

    def __str__(self) -> str:
        return str(self.value)
