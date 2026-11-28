// SPDX-License-Identifier: Apache-2.0

// "Play" of an audio attachment (12 9.2; AUD-PKG-Q03, #809): after "Pause" the player
// continues where it stopped (`resume`) instead of fetching a new ticket and starting again
// from the beginning.

import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  final song = fileOf(1, name: 'song.mp3', mime: 'audio/mpeg');

  testWidgets('Play after Pause continues the playback without a new ticket', (
    tester,
  ) async {
    FakePlayer.created.clear();
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    await host.run(tester, () async {
      await seedThread(host.db, messages: [msg(1, text: 'listen')]);
      await attachFiles(host.db, 1, [song]);
    });
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        ...services.overrides,
      ],
    );
    services.adapter.on(
      '/files/${song.id.value}/tickets',
      ScriptedAnswer.json(201, {
        'url': '/api/v1/files/${song.id.value}/content?ticket=abc',
        'expires_at': '2026-10-17T09:05:00.000Z',
      }),
    );
    final play = find.byKey(ValueKey('file.${song.id.value}.play'));
    await settleUntil(tester, host, play);

    await tester.tap(play);
    await settleUntil(tester, host, find.text(en.fileMediaPause));
    final player = FakePlayer.created.single;
    expect(player.played, hasLength(1));

    await tester.tap(play);
    await settleUntil(
      tester,
      host,
      find.text(en.fileMediaPlay(dsIsolate('song.mp3'))),
    );
    await tester.tap(play);
    await settleUntil(tester, host, find.text(en.fileMediaPause));

    expect(player.played, hasLength(1), reason: 'resumed, not played again');
    expect(player.resumed, 1);
    expect(
      services.adapter.requested.where(
        (p) => p == '/files/${song.id.value}/tickets',
      ),
      hasLength(1),
    );
    await host.dispose(tester);
  });
}
