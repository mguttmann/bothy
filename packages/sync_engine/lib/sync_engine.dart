// SPDX-License-Identifier: Apache-2.0

/// Gateway, catch-up, outbox, reconciliation and run documents of the apps, without Flutter.
///
/// Skeleton created by M4-01; filled by M4-13 to M4-16. The outbox (M4-15): [Outbox] is the
/// one write path of the apps (04 13.2, 15 7.5), [OutboxExecutors] the registry of the
/// operations ([MessageOps], [PersonalOps]), [classify] the outcome classes. The run documents of the live box
/// (M4-16): [RunStore] keeps the document of every focused run in memory and applies
/// `run.updated` by the client rule of 04 11.3 item 4, [applyRunPatch] applies RFC 6902
/// patches restricted to `add`, `replace` and `remove`, [RunDocument] reads the fields of
/// 09 5.4, and [RunDocumentFetcher] and [FrameTicker] are the ports M4-17 wires.
library;

export 'src/outbox/backoff.dart' show fullJitterBackoff;
export 'src/outbox/classify.dart' show Classification, OutcomeClass, classify;
export 'src/outbox/entry.dart' show OutboxEntryView, OutboxStateLabel;
export 'src/outbox/executors.dart'
    show
        ApiCallContext,
        OutboxExecutor,
        OutboxExecutors,
        OutboxFollowUpEdit,
        OutboxFollowUpRemove,
        OutboxLaneOf,
        OutboxOperation,
        OutboxRequest,
        OutboxResponse,
        decodePayload;
export 'src/outbox/operations/message_ops.dart' show MessageOps;
export 'src/outbox/operations/personal_ops.dart' show PersonalOps;
export 'src/outbox/outbox.dart' show Outbox;
export 'src/outbox/sender.dart' show OutboxConfirmationHook, OutboxTimerFactory;
export 'src/runs/json_patch.dart' show RunPatchError, applyRunPatch;
export 'src/runs/ports.dart' show FrameTicker, RunDocumentFetcher;
export 'src/runs/run_document.dart'
    show
        FollowupState,
        PlanItemStatus,
        RunApiRetryView,
        RunDocument,
        RunFollowupView,
        RunMetricsView,
        RunPlanItemView,
        RunStepNode,
        RunStepView;
export 'src/runs/run_store.dart' show RunStore;
