from enum import StrEnum


class SearchAssistHitSource(StrEnum):
    ASSISTANT = "assistant"
    FULLTEXT = "fulltext"

    def __str__(self) -> str:
        return str(self.value)
