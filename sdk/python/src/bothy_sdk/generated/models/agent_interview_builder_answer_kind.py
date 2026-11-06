from enum import StrEnum


class AgentInterviewBuilderAnswerKind(StrEnum):
    DRAFT = "draft"
    QUESTIONS = "questions"

    def __str__(self) -> str:
        return str(self.value)
