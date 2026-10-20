// SPDX-License-Identifier: Apache-2.0

// Previews only from server files (SEC-OUT-2, T-AI-06; brief M4-22 Tests): a message whose
// Markdown holds an image URL and a reference to its attachment shows the thumbnail of the
// attachment from `downloadThumbnail` and loads nothing else. An `HttpOverrides` fails every
// socket of `dart:io`, so an image from a URL could not load without failing the test.

import 'dart:io';

import 'package:client/thread/history_column.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

/// Records and refuses every `HttpClient` the code under test creates.
class _NoSockets extends HttpOverrides {
  final List<String> created = [];

  @override
  HttpClient createHttpClient(SecurityContext? context) {
    created.add(StackTrace.current.toString().split('\n').take(3).join(' | '));
    throw StateError('no socket may be opened in this test');
  }
}

void main() {
  HttpOverrides? saved;
  late _NoSockets sockets;

  setUp(() {
    saved = HttpOverrides.current;
    sockets = _NoSockets();
    HttpOverrides.global = sockets;
  });
  tearDown(() => HttpOverrides.global = saved);

  testWidgets('only the thumbnail of the attachment loads, from the server', (
    tester,
  ) async {
    final image = fileOf(1, name: 'shot.png', mime: 'image/png');
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    services.adapter.on(
      '/files/${image.id.value}/thumbnails/medium',
      ScriptedAnswer.file(onePixelPng),
    );
    await host.run(tester, () async {
      await seedThread(
        host.db,
        messages: [
          msg(
            1,
            text:
                'An outside image ![logo](https://example.com/logo.png) and '
                'the attached one ![shot](attachment:${image.id.value})',
          ),
        ],
      );
      await attachFiles(host.db, 1, [image]);
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
    await settleUntil(
      tester,
      host,
      find.byKey(ValueKey('file.${image.id.value}.thumbnail')),
    );

    // The attachment: its thumbnail inline and in its tile, both from the cache file the
    // server's answer filled.
    final shown = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(shown, isNotEmpty);
    for (final i in shown) {
      expect(i.image, isA<FileImage>());
      expect(
        (i.image as FileImage).file.path,
        startsWith(services.cache.root.path),
      );
    }
    expect(
      find.byKey(ValueKey('file.${image.id.value}.inline')),
      findsOneWidget,
    );
    // The outside URL stays text with its domain (SEC-OUT-1, SEC-OUT-2).
    expect(
      find.textContaining('example.com', findRichText: true),
      findsWidgets,
    );
    expect(services.adapter.requested, [
      '/files/${image.id.value}/thumbnails/medium',
    ]);
    expect(sockets.created, isEmpty);
    await host.dispose(tester);
  });
}
