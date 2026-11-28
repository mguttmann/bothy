// SPDX-License-Identifier: Apache-2.0

// Typing in the composer (D-M4-20-2; AUD-PKG-Q03, #1252): every key press rewrites the draft
// entry of the outbox, but the thread's outbox view drops the draft writes and gives no new
// list for them, so the history and the composer do not rebuild per keystroke.

import 'dart:async';

import 'package:client/thread/providers.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

OutboxEntryView _view(String id, String kind, String payload) {
  final at = DateTime.utc(2026, 11, 28, 9);
  return OutboxEntryView(
    entry: OutboxEntry(
      id: id,
      kind: kind,
      payload: payload,
      state: OutboxState.pending,
      attempts: 0,
      createdAt: at,
      stateChangedAt: at,
    ),
    stateLabel: OutboxStateLabel.pending,
  );
}

void main() {
  test('a draft write alone gives no new list; a message does', () async {
    final source = StreamController<List<OutboxEntryView>>();
    final seen = <List<OutboxEntryView>>[];
    final sub = withoutDraftWrites(source.stream).listen(seen.add);
    final message = _view(
      'm1',
      MessageOps.createMessage,
      '{"content_md":"hi"}',
    );

    source
      ..add([message, _view('d1', PersonalOps.putThreadDraft, '{"c":"h"}')])
      ..add([message, _view('d1', PersonalOps.putThreadDraft, '{"c":"he"}')])
      ..add([message, _view('d1', PersonalOps.putThreadDraft, '{"c":"hel"}')]);
    await pumpEventQueue();
    expect(seen, [
      [message],
    ]);

    final second = _view('m2', MessageOps.createMessage, '{"content_md":"x"}');
    source.add([message, second]);
    await pumpEventQueue();
    expect(seen.last, [message, second]);
    expect(seen, hasLength(2));
    await sub.cancel();
    await source.close();
  });
}
