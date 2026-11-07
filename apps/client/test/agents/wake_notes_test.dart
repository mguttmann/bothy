// SPDX-License-Identifier: Apache-2.0

// Reading the server's notes back (D-AR-08-2): every line of `domain.WakeNoteText` and
// `domain.AutonomyNoteText` of AR-04 maps to its kind, label and reason, a label with quotes
// and words of the line stays whole, a line this version does not know is null (shown as
// written), and only server messages with a text are notes.

import 'package:client/agents/wakes/wake_notes.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/widgets.dart';

void main() {
  test('every line of the server maps to its kind, label and reason', () {
    final cases = <String, (WakeNoteKind, String?, String?)>{
      "Wake-up 'CI of #123' expired: its deadline passed before it could fire":
          (WakeNoteKind.expired, 'CI of #123', null),
      'Wake-up expired: its deadline passed before it could fire': (
        WakeNoteKind.expired,
        null,
        null,
      ),
      "Wake-up 'x' cancelled (cancelled_by_person)": (
        WakeNoteKind.cancelled,
        'x',
        'cancelled_by_person',
      ),
      "Wake-up 'x' failed: its check could not run (wake_check_failed)": (
        WakeNoteKind.failed,
        'x',
        null,
      ),
      "Wake-up 'x' refused (autonomous_chain_max): a reply of the person continues the work":
          (WakeNoteKind.chainCap, 'x', 'autonomous_chain_max'),
      "Wake-up 'x' refused (autonomous_idle_max): a reply of the person continues the work":
          (WakeNoteKind.idleCap, 'x', 'autonomous_idle_max'),
      "Wake-up 'x' refused: agents may not run on alone in this channel": (
        WakeNoteKind.notAllowed,
        'x',
        null,
      ),
      "Wake-up 'x' refused (channel_read_only)": (
        WakeNoteKind.refused,
        'x',
        'channel_read_only',
      ),
      "Wake-up 'x' ended (wake_vanished)": (
        WakeNoteKind.ended,
        'x',
        'wake_vanished',
      ),
      'Autonomous work stopped (autonomous_idle_max): a reply continues it': (
        WakeNoteKind.idleCap,
        null,
        'autonomous_idle_max',
      ),
      'Autonomous work stopped (autonomous_chain_max): a reply continues it': (
        WakeNoteKind.chainCap,
        null,
        'autonomous_chain_max',
      ),
    };
    for (final MapEntry(key: line, value: (kind, label, reason))
        in cases.entries) {
      final note = decodeWakeNote(line);
      expect(note, isNotNull, reason: line);
      expect(note!.kind, kind, reason: line);
      expect(note.label, label, reason: line);
      expect(note.reason, reason, reason: line);
    }
  });

  test("a label with quotes and the words of a line stays whole", () {
    final note = decodeWakeNote(
      "Wake-up 'it's done' expired soon' expired: its deadline passed before it could fire",
    )!;
    expect(note.kind, WakeNoteKind.expired);
    expect(note.label, "it's done' expired soon");
  });

  test('an unknown line is null and shows as written', () {
    expect(decodeWakeNote('Something else'), isNull);
    final m = msg(
      1,
      author: robin,
      text: 'Something else',
      origin: MessageOrigin.system,
    );
    expect(threadNoteText(en, m), 'Something else');
  });

  test('a refusal names its reason in words', () {
    final text = wakeNoteText(
      en,
      decodeWakeNote("Wake-up 'x' refused (channel_read_only)")!,
    );
    expect(
      text,
      startsWith(en.noteWakeRefused(en.wakesNoteName(dsIsolate('x')), '')),
    );
    expect(text, isNot(endsWith(': ')));
  });

  test('only server messages with a text are notes', () {
    expect(
      isThreadNote(msg(1, author: robin, origin: MessageOrigin.system)),
      isTrue,
    );
    expect(
      isThreadNote(msg(1, author: robin, origin: MessageOrigin.agent)),
      isFalse,
    );
    expect(isThreadNote(msg(1, author: sam)), isFalse);
    expect(
      isThreadNote(
        msg(
          1,
          author: sam,
          origin: MessageOrigin.system,
          kind: MessageKind.system,
          systemCode: SystemCode.threadReopened,
        ),
      ),
      isFalse,
    );
    expect(
      isThreadNote(
        msg(
          1,
          author: robin,
          origin: MessageOrigin.system,
          kind: MessageKind.run,
        ),
      ),
      isFalse,
    );
  });
}
