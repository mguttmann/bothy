from enum import StrEnum


class AgentInterviewAnswerOutcome(StrEnum):
    DRAFT = "draft"
    FALLBACK = "fallback"
    QUESTIONS = "questions"

    def __str__(self) -> str:
        return str(self.value)
