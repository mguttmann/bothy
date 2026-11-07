// SPDX-License-Identifier: Apache-2.0

/// The notes of the thread (01 6.21 item 3; brief AR-08): the server writes the end of a
/// wake-up other than `fired` (09 5.8.4) and an ended chain of autonomous inputs (5.8.6) as a
/// message with origin `system` and kind `text`, authored by the agent, whose English content
/// names the label and the reason (`messaging.AppendSystemNote`, D-AR-04-6). The client draws
/// every such message as a note of the thread, never as a person's message, and translates the
/// known lines by reading them back into their parts (ruling D-AR-08-2); a line it does not
/// know shows as the server wrote it.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';

/// Whether [message] is a note of the thread: written by the server (`origin = system`) with
/// a text instead of a system code. Only the server writes `origin = system` (SEC-OUT-3), so
/// no person and no agent can make a message look like a note.
bool isThreadNote(Message message) =>
    message.origin == MessageOrigin.system &&
    message.kind != MessageKind.system &&
    message.kind != MessageKind.run;

/// What a note of the thread says.
enum WakeNoteKind {
  /// A wake-up expired (09 5.8.4).
  expired,

  /// A wake-up was cancelled.
  cancelled,

  /// The check of a wake-up kept failing.
  failed,

  /// A check refused a wake-up; [WakeNote.reason] names the check.
  refused,

  /// A wake-up was refused because the channel does not let agents run on alone (5.8.6).
  notAllowed,

  /// The chain of autonomous inputs reached `run.autonomous_chain_max` (5.8.6).
  chainCap,

  /// The idle count reached `run.autonomous_idle_max` (5.8.6).
  idleCap,

  /// Another end of a newer server.
  ended,
}

/// A note read back into its parts: [kind], the wake-up's [label] (agent content; null when
/// the note names none) and the [reason] code.
final class WakeNote {
  /// Creates the note.
  const WakeNote(this.kind, {this.label, this.reason});

  /// What it says.
  final WakeNoteKind kind;

  /// The label of the wake-up, or null for "Wake-up" without one.
  final String? label;

  /// The reason code, when the note names one.
  final String? reason;
}

final RegExp _wake = RegExp(
  r"^Wake-up(?: '(.*)')? (expired|cancelled|failed|refused|ended)(.*)$",
  dotAll: true,
);
final RegExp _reason = RegExp(r'^\s*\(([a-z0-9_.:-]+)\)');
final RegExp _stopped = RegExp(
  r'^Autonomous work stopped \(([a-z0-9_.:-]+)\): a reply continues it$',
);

const String _chainMax = 'autonomous_chain_max';
const String _idleMax = 'autonomous_idle_max';

/// Reads the server line [content] of a note (the texts of `domain.WakeNoteText` and
/// `domain.AutonomyNoteText` of AR-04), or null for a line this version does not know.
///
/// Example: `decodeWakeNote("Wake-up 'CI of #123' cancelled (cancelled_by_person)")` is a
/// [WakeNoteKind.cancelled] note with the label `CI of #123`.
WakeNote? decodeWakeNote(String content) {
  final stopped = _stopped.firstMatch(content);
  if (stopped != null) {
    final reason = stopped[1]!;
    return WakeNote(
      reason == _chainMax ? WakeNoteKind.chainCap : WakeNoteKind.idleCap,
      reason: reason,
    );
  }
  final m = _wake.firstMatch(content);
  if (m == null) return null;
  final label = m[1];
  final rest = m[3]!;
  final reason = _reason.firstMatch(rest)?[1];
  final kind = switch (m[2]) {
    'expired' => WakeNoteKind.expired,
    'cancelled' => WakeNoteKind.cancelled,
    'failed' => WakeNoteKind.failed,
    'refused' when reason == _chainMax => WakeNoteKind.chainCap,
    'refused' when reason == _idleMax => WakeNoteKind.idleCap,
    'refused' when reason != null => WakeNoteKind.refused,
    'refused' when rest.contains('may not run on alone') =>
      WakeNoteKind.notAllowed,
    _ => WakeNoteKind.ended,
  };
  return WakeNote(kind, label: label, reason: reason);
}

/// The translated text of [note]; the label stays as the agent wrote it, isolated.
String wakeNoteText(AppLocalizations l, WakeNote note) {
  final label = note.label;
  final name = label == null || label.isEmpty
      ? l.wakesUnnamed
      : l.wakesNoteName(dsIsolate(label));
  final reason = note.reason;
  return switch (note.kind) {
    WakeNoteKind.expired => l.noteWakeExpired(name),
    WakeNoteKind.cancelled => l.noteWakeCancelled(name),
    WakeNoteKind.failed => l.noteWakeFailed(name),
    WakeNoteKind.notAllowed => l.noteWakeRefusedNotAllowed(name),
    WakeNoteKind.chainCap => l.noteChainCap,
    WakeNoteKind.idleCap => l.noteIdleCap,
    WakeNoteKind.refused when reason != null => l.noteWakeRefused(
      name,
      dsReasonText(l, reason) ?? problemText(l, 'conflict', reason: reason),
    ),
    WakeNoteKind.refused => l.noteWakeEnded(name),
    WakeNoteKind.ended => l.noteWakeEnded(name),
  };
}

/// The text of the note [message]: translated when it is a known line, else the server's own.
String threadNoteText(AppLocalizations l, Message message) {
  final note = decodeWakeNote(message.contentMd);
  return note == null ? message.contentMd : wakeNoteText(l, note);
}
