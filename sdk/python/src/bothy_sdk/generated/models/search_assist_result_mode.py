from enum import StrEnum


class SearchAssistResultMode(StrEnum):
    ASSISTED = "assisted"
    FULLTEXT = "fulltext"

    def __str__(self) -> str:
        return str(self.value)
