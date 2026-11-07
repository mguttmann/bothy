// SPDX-License-Identifier: Apache-2.0

/// Wake-ups as the client sees them (03 `run_wakes`, 09 5.8.1, API `RunWake`; stage AR) and the
/// end state of a run (03 `runs.metrics.end_state`, 09 18.19.11). A thread shows its pending
/// wake-ups above the composer and every wake run with its label (01 6.21).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';

/// Kind of a wake-up (API `RunWakeKind`, 03 `run_wakes.kind`).
enum RunWakeKind implements Coded {
  /// At a time.
  at('at'),

  /// When the checks of a pull request finish.
  prChecks('pr_checks'),

  /// When a command succeeds.
  command('command'),

  /// A value of a newer server.
  unknown('');

  const RunWakeKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static RunWakeKind fromCode(String code) => decodeCode(values, code, unknown);
}

/// Why a wake-up exists (API `RunWakeReason`, 03 `run_wakes.reason`).
enum RunWakeReason implements Coded {
  /// The agent asked for it (`bothy-wake`).
  agent('agent'),

  /// The runner continues after lost background work (09 18.19.9).
  recovery('recovery'),

  /// The usage window of the account resets (09 18.19.10).
  usageLimit('usage_limit'),

  /// A value of a newer server.
  unknown('');

  const RunWakeReason(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static RunWakeReason fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// State of a wake-up (API `RunWakeState`, 03 `run_wakes.state`).
enum RunWakeState implements Coded {
  /// Waiting for its time or condition.
  pending('pending'),

  /// Started a wake run or reached the active run as a wake input.
  fired('fired'),

  /// Cancelled by the agent or a person.
  cancelled('cancelled'),

  /// Its deadline passed.
  expired('expired'),

  /// A check refused it.
  refused('refused'),

  /// Its check could not run.
  failed('failed'),

  /// A value of a newer server.
  unknown('');

  const RunWakeState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static RunWakeState fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// The end state of a finished run (API `RunMetrics.end_state`, 09 18.19.11): the last line
/// `STATE:` of its last answer.
enum RunEndState implements Coded {
  /// Nothing more to do.
  done('done'),

  /// More work after something happens.
  waiting('waiting'),

  /// Needs a person.
  blocked('blocked'),

  /// A value of a newer server.
  unknown('');

  const RunEndState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static RunEndState fromCode(String code) => decodeCode(values, code, unknown);
}

/// A wake-up of a thread (API `RunWake`, 03 `run_wakes`): it continues the thread later without
/// a new message, on behalf of the person of its source run ([triggeredBy]).
///
/// Example: the pending list above the composer shows [label], the [condition], [notBefore] and
/// [deadline] of every wake-up with [isPending].
final class RunWake {
  /// Creates a [RunWake].
  const RunWake({
    required this.id,
    required this.agentId,
    required this.threadId,
    required this.sourceRunId,
    required this.triggeredBy,
    required this.kind,
    required this.reason,
    required this.label,
    required this.condition,
    required this.notBefore,
    required this.deadline,
    required this.state,
    required this.resultText,
    required this.createdAt,
    required this.version,
    this.outage,
    this.stateReason,
    this.firedRunId,
    this.firedIntoRunId,
    this.endedAt,
  });

  /// ID.
  final WakeId id;

  /// The agent of the source run.
  final PrincipalId agentId;

  /// The thread it continues.
  final ThreadId threadId;

  /// The run that asked for it, or the ended run of a recovery or usage-limit wake-up.
  final RunId sourceRunId;

  /// The person of the source run; every check of the server uses this person.
  final PrincipalId triggeredBy;

  /// Kind.
  final RunWakeKind kind;

  /// Why it exists.
  final RunWakeReason reason;

  /// Short text of the agent; may be empty. Agent content: never translated.
  final String label;

  /// Schema `run_wake_condition.v1`: `repository`, `number` and `head_commit` for `pr_checks`,
  /// `command_line` for `command`, nothing for `at`.
  final Map<String, Object?> condition;

  /// The run's outage number of a `usage_limit` wake-up of `createRunWake`.
  final int? outage;

  /// The time it fires at the earliest.
  final DateTime notBefore;

  /// The time it expires at.
  final DateTime deadline;

  /// State.
  final RunWakeState state;

  /// Reason code of the last state change (09 5.8.3, 5.8.6).
  final String? stateReason;

  /// The result of a fired check, as the runner cut it.
  final String resultText;

  /// The wake run it started.
  final RunId? firedRunId;

  /// The active run it reached as a wake input.
  final RunId? firedIntoRunId;

  /// Creation time.
  final DateTime createdAt;

  /// Time of every state other than [RunWakeState.pending].
  final DateTime? endedAt;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Whether it still waits ([RunWakeState.pending]).
  bool get isPending => state == RunWakeState.pending;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  RunWake copyWith({
    RunWakeState? state,
    String? stateReason,
    String? label,
    Map<String, Object?>? condition,
    DateTime? notBefore,
    DateTime? deadline,
    RunId? firedRunId,
    RunId? firedIntoRunId,
    DateTime? endedAt,
    ResourceVersion? version,
  }) => RunWake(
    id: id,
    agentId: agentId,
    threadId: threadId,
    sourceRunId: sourceRunId,
    triggeredBy: triggeredBy,
    kind: kind,
    reason: reason,
    label: label ?? this.label,
    condition: condition ?? this.condition,
    outage: outage,
    notBefore: notBefore ?? this.notBefore,
    deadline: deadline ?? this.deadline,
    state: state ?? this.state,
    stateReason: stateReason ?? this.stateReason,
    resultText: resultText,
    firedRunId: firedRunId ?? this.firedRunId,
    firedIntoRunId: firedIntoRunId ?? this.firedIntoRunId,
    createdAt: createdAt,
    endedAt: endedAt ?? this.endedAt,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is RunWake &&
      other.id == id &&
      other.agentId == agentId &&
      other.threadId == threadId &&
      other.sourceRunId == sourceRunId &&
      other.triggeredBy == triggeredBy &&
      other.kind == kind &&
      other.reason == reason &&
      other.label == label &&
      jsonEquals(other.condition, condition) &&
      other.outage == outage &&
      other.notBefore == notBefore &&
      other.deadline == deadline &&
      other.state == state &&
      other.stateReason == stateReason &&
      other.resultText == resultText &&
      other.firedRunId == firedRunId &&
      other.firedIntoRunId == firedIntoRunId &&
      other.createdAt == createdAt &&
      other.endedAt == endedAt &&
      other.version == version;

  @override
  int get hashCode => Object.hash(
    id,
    agentId,
    threadId,
    sourceRunId,
    triggeredBy,
    kind,
    reason,
    label,
    jsonHash(condition),
    outage,
    notBefore,
    deadline,
    state,
    stateReason,
    resultText,
    firedRunId,
    firedIntoRunId,
    createdAt,
    endedAt,
    version,
  );
}
