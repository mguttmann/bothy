from enum import StrEnum


class StatsMetric(StrEnum):
    APPROVAL_CARDS = "approval_cards"
    BILLING_MODE_SWITCHES = "billing_mode_switches"
    CATCHUPS = "catchups"
    COST = "cost"
    INVOCATIONS_DENIED = "invocations_denied"
    MESSAGES_CREATED = "messages_created"
    RESYNCS = "resyncs"
    RUNS_FINISHED = "runs_finished"
    RUNS_STARTED = "runs_started"
    SLI_API_AVAILABILITY = "sli.api_availability"
    SLI_DELIVERY_LATENCY = "sli.delivery_latency"
    SLI_PUSH_ACCEPTANCE = "sli.push_acceptance"
    SLI_RECONNECT = "sli.reconnect"
    SLI_RUN_START = "sli.run_start"
    SLI_TURN_DURATION = "sli.turn_duration"
    SLOW_CONSUMER_CLOSES = "slow_consumer_closes"
    THREADS_CREATED = "threads_created"
    TOOL_CALLS = "tool_calls"

    def __str__(self) -> str:
        return str(self.value)
