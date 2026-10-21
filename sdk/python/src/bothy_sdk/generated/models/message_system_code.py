from enum import StrEnum


class MessageSystemCode(StrEnum):
    AGENT_SESSION_RESUME_FAILED = "agent_session.resume_failed"
    THREAD_REOPENED = "thread.reopened"

    def __str__(self) -> str:
        return str(self.value)
