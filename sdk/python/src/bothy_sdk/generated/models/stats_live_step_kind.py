from enum import StrEnum


class StatsLiveStepKind(StrEnum):
    BACKGROUND_JOB = "background_job"
    STATUS = "status"
    SUBAGENT = "subagent"
    TOOL_CALL = "tool_call"
    WORKFLOW = "workflow"

    def __str__(self) -> str:
        return str(self.value)
