from enum import StrEnum


class ReactionAddedEventType(StrEnum):
    REACTION_ADDED = "reaction.added"

    def __str__(self) -> str:
        return str(self.value)
