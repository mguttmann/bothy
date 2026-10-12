from enum import StrEnum


class AuditVerifyResultFailureReason(StrEnum):
    CHECKPOINT_INVALID = "checkpoint_invalid"
    CHECKPOINT_MISMATCH = "checkpoint_mismatch"
    HASH_MISMATCH = "hash_mismatch"
    LINK_MISMATCH = "link_mismatch"
    SEQ_GAP = "seq_gap"

    def __str__(self) -> str:
        return str(self.value)
