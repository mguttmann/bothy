// SPDX-License-Identifier: Apache-2.0

/// Gateway, catch-up, outbox, reconciliation and run documents of the apps, without Flutter.
///
/// Skeleton created by M4-01; filled by M4-13 to M4-16. The run documents of the live box
/// (M4-16): [RunStore] keeps the document of every focused run in memory and applies
/// `run.updated` by the client rule of 04 11.3 item 4, [applyRunPatch] applies RFC 6902
/// patches restricted to `add`, `replace` and `remove`, [RunDocument] reads the fields of
/// 09 5.4, and [RunDocumentFetcher] and [FrameTicker] are the ports M4-17 wires.
library;

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
