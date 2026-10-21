from enum import StrEnum


class AgentSessionReportBindFailed(StrEnum):
    SESSION_MISSING = "session_missing"

    def __str__(self) -> str:
        return str(self.value)
