// SPDX-License-Identifier: Apache-2.0

/// Gateway, catch-up, outbox, reconciliation and run documents of the apps, without Flutter.
///
/// Skeleton created by M4-01; filled by M4-13 to M4-16. The gateway connection (M4-13):
/// [GatewayConnection] holds the one WebSocket of the device, answers `hello` through
/// [GatewaySession], sends heartbeats ([Heartbeat]), waits by [ReconnectBackoff] and reacts to
/// every close code by [reactTo]; its [ConnectionState] follows 15 7.2, [FocusManager] and
/// [TypingThrottle] send the ephemeral ops, and catch-up plugs in through [SessionListener]
/// (M4-14). The outside world comes in through [GatewayTransport], [NetworkStatus],
/// [AppLifecycle] and [RandomSource]. The outbox (M4-15): [Outbox] is the
/// one write path of the apps (04 13.2, 15 7.5), [OutboxExecutors] the registry of the
/// operations ([MessageOps], [PersonalOps]), [classify] the outcome classes. The run documents of the live box
/// (M4-16): [RunStore] keeps the document of every focused run in memory and applies
/// `run.updated` by the client rule of 04 11.3 item 4, [applyRunPatch] applies RFC 6902
/// patches restricted to `add`, `replace` and `remove`, [RunDocument] reads the fields of
/// 09 5.4, and [RunDocumentFetcher] and [FrameTicker] are the ports M4-17 wires.
library;

export 'src/backoff.dart';
export 'src/close_codes.dart';
export 'src/connection.dart';
export 'src/connection_state.dart';
export 'src/focus_manager.dart';
export 'src/frames.dart';
export 'src/heartbeat.dart';
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
export 'src/ports.dart';
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
export 'src/session.dart';
export 'src/session_listener.dart';
export 'src/transport_io.dart';
export 'src/typing_throttle.dart';
