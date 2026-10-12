from enum import StrEnum


class ReactionRemovedEventType(StrEnum):
    REACTION_REMOVED = "reaction.removed"

    def __str__(self) -> str:
        return str(self.value)
