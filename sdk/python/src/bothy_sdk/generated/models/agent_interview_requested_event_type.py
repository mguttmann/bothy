from enum import StrEnum


class AgentInterviewRequestedEventType(StrEnum):
    AGENT_INTERVIEW_REQUESTED = "agent.interview_requested"

    def __str__(self) -> str:
        return str(self.value)
