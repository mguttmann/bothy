from enum import StrEnum


class StatsLiveTaskKind(StrEnum):
    BACKGROUND_JOB = "background_job"
    SUBAGENT = "subagent"
    WORKFLOW = "workflow"

    def __str__(self) -> str:
        return str(self.value)
