// SPDX-License-Identifier: Apache-2.0

// Sharing on Android (brief M5-06 Design note 6; 01 5.4): the app takes the pending share from
// the runner at start and on `onShareReceived`, opens the route `/share` with the target sheet,
// and sends through its outbox (uploads, then the message with a new `client_msg_id`); the
// copies in the app's cache go once the outbox holds its own, or when the person cancels.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/share/share_android.dart';
import 'package:client/share/share_target.dart';
import 'package:design_system/design_system.dart' show dsIsolate;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:sync_engine/sync_engine.dart';

import '../phone/support/phone_app.dart' show ScriptedOnboarding, phoneWindow;
import '../support/fake_instance.dart' show eventually;
import '../support/fakes.dart';
import 'share_fakes.dart';
import 'share_harness.dart';

const String _shareId = '0192f4a0-7b1c-7000-8000-000000000301';

Future<SharedFileRecord> _cached(
  Directory dir,
  String name,
  String text,
) async {
  final file = File('${dir.path}/$name');
  await file.create(recursive: true);
  await file.writeAsString(text, flush: true);
  return SharedFileRecord(
    path: file.path,
    name: name,
    mimeType: 'text/plain',
    sizeBytes: await file.length(),
  );
}

void main() {
  group('the glue', () {
    late Directory cache;
    late FakeAndroidShareHost host;

    setUp(() async {
      cache = await Directory.systemTemp.createTemp('share-cache');
      host = FakeAndroidShareHost();
    });

    tearDown(() async {
      if (cache.existsSync()) await cache.delete(recursive: true);
    });

    test(
      'takes the share that started the app and every later one once',
      () async {
        host.pending = SharedContentRecord(
          id: _shareId,
          text: 'https://example.com/report',
          files: [await _cached(cache, '1-report.txt', 'printer log')],
        );
        final share = AndroidShare(host: host);
        final got = <SharedContent>[];
        share.shares.listen(got.add);
        await share.listen();
        await pumpEventQueue();
        expect(got.single.id, _shareId);
        expect(got.single.text, 'https://example.com/report');
        expect(got.single.files.single.name, '1-report.txt');
        expect(got.single.files.single.sizeBytes, 11);

        share.onShareReceived();
        await pumpEventQueue();
        expect(got, hasLength(1), reason: 'nothing was pending');
        host.pending = SharedContentRecord(id: 'second', files: []);
        share.onShareReceived();
        await pumpEventQueue();
        expect([for (final s in got) s.id], [_shareId, 'second']);
      },
    );

    test('a runner without the channel gives nothing', () async {
      final share = AndroidShare(host: _BrokenHost());
      final got = <SharedContent>[];
      share.shares.listen(got.add);
      await share.listen();
      share.onShareReceived();
      await pumpEventQueue();
      expect(got, isEmpty);
    });

    test('discarding a share deletes its cache copies', () async {
      final file = await _cached(cache, '1-a.txt', 'a');
      await AndroidShare.discard(
        SharedContent(
          id: _shareId,
          files: [
            ShareFile(
              path: file.path,
              name: file.name,
              mimeType: file.mimeType,
              sizeBytes: file.sizeBytes,
            ),
          ],
        ),
      );
      expect(File(file.path).existsSync(), isFalse);
    });
  });

  group('the send through the outbox', () {
    late ShareHarness h;

    setUp(() async => h = await ShareHarness.start());
    tearDown(() => h.stop());

    test('uploads, then the message with a new client_msg_id; the cache copies '
        'go once the outbox holds its own', () async {
      final cache = Directory('${h.root.path}/cache');
      final record = await _cached(cache, '1-log.txt', 'error 42');
      final content = SharedContent.fromRecord(
        SharedContentRecord(id: _shareId, text: 'link', files: [record]),
      );
      const clientMsgId = '0192f4a0-7b1c-7000-8000-000000000302';
      final sender = SharedContentSender(
        outbox: h.shares,
        newId: () => clientMsgId,
      );
      final id = await sender.send(
        content,
        target: ShareTarget(
          kind: ShareTargetKind.thread,
          channelId: ChannelId.parse(h.seed.general),
          threadId: ThreadId.parse(h.seed.releaseNotes),
          path: 'general > Release notes',
        ),
        comment: 'See the log',
      );
      expect(id, clientMsgId);
      expect(File(record.path).existsSync(), isFalse);
      final entries = await h.db.outboxDao.allEntries();
      expect(entries.map((e) => e.kind), ['upload', MessageOps.createMessage]);
      expect(entries.last.id, clientMsgId);

      await h.outbox.start();
      await eventually(
        'sent',
        () => h.sent(h.seed.releaseNotes, clientMsgId).length == 1,
      );
      final sent = h.sent(h.seed.releaseNotes, clientMsgId).single;
      expect(sent['content_md'], 'See the log');
      expect(sent['attachments'], hasLength(1));
    });
  });

  group('the route', () {
    late Directory root;

    setUp(
      () async => root = await Directory.systemTemp.createTemp('share-app'),
    );
    tearDown(() async {
      if (root.existsSync()) await root.delete(recursive: true);
    });

    testWidgets(
      'a share opens /share with the sheet; Cancel deletes the copies '
      'and leaves',
      (tester) async {
        final host = FakeAndroidShareHost();
        final file = await tester.runAsync(
          () =>
              _cached(Directory('${root.path}/cache'), '1-photo.txt', 'pixels'),
        );
        host.pending = SharedContentRecord(id: _shareId, files: [file!]);
        phoneWindow(tester);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              appPortsProvider.overrideWithValue(
                testPorts(
                  dataRoot: root,
                  platform: 'android',
                  androidShare: AndroidShare(host: host),
                ),
              ),
              onboardingControllerProvider.overrideWith(
                () => ScriptedOnboarding(
                  const OnboardingState(step: OnboardingStep.signedIn),
                ),
              ),
            ],
            child: const ClientApp(),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('share.sheet')), findsOneWidget);
        expect(find.text(dsIsolate('1-photo.txt')), findsOneWidget);
        expect(_location(tester), '/share');

        await tester.tap(find.byKey(const ValueKey('share.cancel')));
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('share.sheet')), findsNothing);
        await tester.runAsync(() async {
          await eventually('copy deleted', () => !File(file.path).existsSync());
        });
      },
    );
  });
}

String _location(WidgetTester tester) =>
    GoRouter.of(tester.element(find.byKey(const ValueKey('share.sheet'))))
        .routeInformationProvider
        .value
        .uri
        .toString();

class _BrokenHost extends FakeAndroidShareHost {
  @override
  Future<SharedContentRecord?> takePendingShare() =>
      Future.error(StateError('no channel'));
}
