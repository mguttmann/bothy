from enum import StrEnum


class AgentInterviewAnswerFallbackReason(StrEnum):
    BUILDER_INVALID = "builder_invalid"
    BUILDER_NOT_CONFIGURED = "builder_not_configured"
    BUILDER_OFFLINE = "builder_offline"
    BUILDER_TIMEOUT = "builder_timeout"

    def __str__(self) -> str:
        return str(self.value)
