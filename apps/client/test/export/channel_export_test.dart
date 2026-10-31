// SPDX-License-Identifier: Apache-2.0

// The channel export (brief M4-33 Tests and Design note 3; 07 2.4; openapi `exportChannel`):
// "Export history" only with the capability `export`; against the fake server the step-up is
// asked with its purpose and the file equals the stream; a refusal names the server's reason
// and leaves no file; cancel and a broken stream delete the partial file.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:auth/auth.dart' show StepUpCredentials, StepUpPrompt;
import 'package:client/export/channel_export.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/menus/channel_menu.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/testing.dart';

import '../support/column_host.dart';
import '../support/fake_instance.dart';
import '../support/file_fakes.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

/// Answers every step-up with yes and records its purpose.
class RecordingPrompt implements StepUpPrompt {
  /// The purposes asked.
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return true;
  }

  /// These tests confirm with the step-up key; a password prompt cancels.
  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => null;
}

/// An adapter whose one response streams what the test adds to [body].
class StreamAdapter implements HttpClientAdapter {
  final StreamController<Uint8List> body = StreamController<Uint8List>();

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    unawaited(cancelFuture?.then((_) => body.close()));
    return ResponseBody(body.stream, 200);
  }

  @override
  void close({bool force = false}) {}
}

const Duration patience = Duration(minutes: 3);

void main() {
  group('the menu entry follows the capability', () {
    Future<void> menuWith(
      WidgetTester tester,
      List<Capability> capabilities,
    ) async {
      final host = await ColumnHost.open(tester);
      final general = channel(1, name: 'general');
      await tester.runAsync(() async {
        await host.db.structureDao.upsertSpaceIfNewer(space(1));
        await host.db.structureDao.upsertChannelIfNewer(general);
        await host.db.structureDao.putChannelAccess(
          general.id,
          AccessView(level: AccessLevel.read, capabilities: capabilities),
        );
      });
      await host.pump(
        tester,
        ChannelMenu(channel: general, child: const Text('general')),
      );
      await tester.tap(find.text('general'), buttons: kSecondaryMouseButton);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('menu.notifications')), findsOneWidget);
      expect(
        find.byKey(const ValueKey('menu.export')),
        capabilities.contains(Capability.export)
            ? findsOneWidget
            : findsNothing,
      );
      await tester.tapAt(Offset.zero);
      await tester.pumpAndSettle();
      await host.dispose(tester);
    }

    testWidgets(
      'hidden without export',
      (tester) => menuWith(tester, const []),
    );

    testWidgets(
      'shown with export',
      (tester) => menuWith(tester, const [Capability.export]),
    );
  });

  group('against the fake server', () {
    HttpOverrides? saved;
    late FakeInstance f;
    late ProviderContainer c;
    late RecordingPrompt prompt;
    late Directory out;

    setUp(() async {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      prompt = RecordingPrompt();
      f = await FakeInstance.start(stepUpPrompt: prompt);
      c = appContainer(f);
      out = await Directory.systemTemp.createTemp('export');
    });

    tearDown(() async {
      c.dispose();
      await f.stop();
      await out.delete(recursive: true);
      HttpOverrides.global = saved;
    });

    Future<ApiClient> signedIn() async {
      final control = await connected(c, f);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      return c.read(onboardingControllerProvider).services!.auth.api;
    }

    test('step-up asked, the file equals the stream', () async {
      final world = f.server.world;
      final one = f.seed.post(f.seed.sam, f.seed.releaseNotes, 'One');
      final two = f.seed.post(f.seed.alex, f.seed.releaseNotes, 'Two');
      world.grantCapability(f.seed.alex.id, f.seed.general, 'export');
      final api = await signedIn();
      final path = '${out.path}/general.ndjson';
      final progress = <int>[];
      final bytes = await ChannelExporter(api).run(
        channel: ChannelId.parse(f.seed.general),
        from: DateTime.utc(2000),
        to: DateTime.utc(2100),
        path: path,
        purpose: 'Export the history of general',
        onProgress: progress.add,
      );
      expect(prompt.asked, ['Export the history of general']);
      final text = File(path).readAsStringSync();
      final lines = const LineSplitter().convert(text);
      expect(
        [for (final l in lines) (jsonDecode(l) as Map)['id']],
        [one['id'], two['id']],
      );
      // Byte for byte what the server sent: one JSON object per line, as stored.
      expect(text, '${jsonEncode(one)}\n${jsonEncode(two)}\n');
      expect(bytes, utf8.encode(text).length);
      expect(progress.last, bytes);
      expect(f.server.violations, isEmpty);
    });

    test('without the capability the server refuses, no file stays', () async {
      final api = await signedIn();
      final path = '${out.path}/general.ndjson';
      final refused = await ChannelExporter(api)
          .run(
            channel: ChannelId.parse(f.seed.general),
            from: DateTime.utc(2000),
            to: DateTime.utc(2100),
            path: path,
            purpose: 'Export',
            onProgress: (_) {},
          )
          .then<Object?>((_) => null, onError: (Object e) => e);
      expect(refused, isA<ChannelExportRefused>());
      final problem = (refused! as ChannelExportRefused).problem!;
      expect(
        (problem.status, problem.code, problem.reason),
        (403, 'forbidden', 'capability_missing'),
      );
      expect(File(path).existsSync(), isFalse);
    });
  });

  group('cancel and a broken stream', () {
    late Directory out;
    late StreamAdapter adapter;
    late ChannelExporter exporter;

    setUp(() async {
      out = await Directory.systemTemp.createTemp('export');
      adapter = StreamAdapter();
      exporter = ChannelExporter(
        ApiClient(
          dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com'))
            ..httpClientAdapter = adapter,
        ),
      );
    });

    tearDown(() => out.delete(recursive: true));

    Future<Object?> start(
      String path, {
      CancelToken? cancel,
      required Completer<void> firstChunk,
    }) => exporter
        .run(
          channel: ChannelId.parse('0192f4a0-7b1c-72d4-8bae-7661d2b2f203'),
          from: DateTime.utc(2026),
          to: DateTime.utc(2027),
          path: path,
          purpose: 'Export',
          onProgress: (_) {
            if (!firstChunk.isCompleted) firstChunk.complete();
          },
          cancel: cancel,
        )
        .then<Object?>((n) => n, onError: (Object e) => e);

    test('cancel aborts the stream and deletes the partial file', () async {
      final path = '${out.path}/partial.ndjson';
      final cancel = CancelToken();
      final first = Completer<void>();
      final result = start(path, cancel: cancel, firstChunk: first);
      adapter.body.add(Uint8List.fromList(utf8.encode('{"id":"a"}\n')));
      await first.future.timeout(patience);
      // `openWrite` opens the file on the IO thread after the first chunk; on a loaded
      // machine that lags behind the progress callback, so wait for the file to appear.
      final watch = Stopwatch()..start();
      while (!File(path).existsSync()) {
        if (watch.elapsed > patience) fail('the partial file never appeared');
        await Future<void>.delayed(const Duration(milliseconds: 5));
      }
      cancel.cancel();
      expect(await result.timeout(patience), isA<ChannelExportCancelled>());
      expect(File(path).existsSync(), isFalse);
    });

    test('a stream that breaks off deletes the partial file', () async {
      final path = '${out.path}/broken.ndjson';
      final first = Completer<void>();
      final result = start(path, firstChunk: first);
      adapter.body.add(Uint8List.fromList(utf8.encode('{"id":"a"}\n')));
      await first.future.timeout(patience);
      adapter.body.addError(const SocketException('reset'));
      expect(await result.timeout(patience), isA<ChannelExportBrokeOff>());
      expect(File(path).existsSync(), isFalse);
    });
  });

  group('the dialog', () {
    late HttpOverrides? saved;
    late FakeInstance f;
    late ProviderContainer c;
    late FakePicker picker;
    late Directory out;

    Future<void> open(WidgetTester tester, {required bool capability}) async {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      picker = FakePicker();
      await tester.runAsync(() async {
        f = await FakeInstance.start(
          stepUpPrompt: RecordingPrompt(),
          files: FilePorts(picker: picker),
        );
        c = appContainer(f);
        out = await Directory.systemTemp.createTemp('export');
        if (capability) {
          f.server.world.grantCapability(
            f.seed.alex.id,
            f.seed.general,
            'export',
          );
        }
        final control = await connected(c, f);
        await control.loginWithPassword(
          WorldSeed.alexLogin,
          WorldSeed.password,
          WorldSeed.totp,
        );
      });
      final template = channel(1, name: 'general');
      final general = Channel(
        id: ChannelId.parse(f.seed.general),
        spaceId: template.spaceId,
        kind: template.kind,
        name: template.name,
        topic: template.topic,
        sortKey: template.sortKey,
        operator: template.operator,
        readOnly: false,
        mainThreadId: template.mainThreadId,
        dmParticipantIds: const [],
        createdAt: template.createdAt,
        version: template.version,
        settingsVersion: template.settingsVersion,
      );
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: c,
          child: hostScreen(
            Scaffold(body: ChannelExportDialog(channel: general)),
          ),
        ),
      );
      await tester.pump();
    }

    Future<void> close(WidgetTester tester) async {
      await tester.pumpWidget(const SizedBox());
      await tester.runAsync(() async {
        c.dispose();
        await f.stop();
        await out.delete(recursive: true);
      });
      HttpOverrides.global = saved;
    }

    /// Lets the real IO of the fake run until [finder] shows, bounded by the patience.
    Future<void> until(WidgetTester tester, Finder finder) async {
      final watch = Stopwatch()..start();
      while (finder.evaluate().isEmpty) {
        if (watch.elapsed > patience) fail('timed out: $finder');
        await tester.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 10)),
        );
        // Also the fake time, so the timers of the request in the test zone fire.
        await tester.pump(const Duration(milliseconds: 10));
      }
    }

    testWidgets('with the capability the file is saved and the size shown', (
      tester,
    ) async {
      await open(tester, capability: true);
      picker.saveTo = '${out.path}/general.ndjson';
      expect(find.byKey(const ValueKey('export.from')), findsOneWidget);
      expect(find.byKey(const ValueKey('export.to')), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('export.start')));
      await until(tester, find.byKey(const ValueKey('export.close')));
      expect(picker.asked.single, startsWith('general-'));
      expect(File(picker.saveTo!).existsSync(), isTrue);
      expect(find.byKey(const ValueKey('export.notice')), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('export.close')));
      await tester.pump();
      await close(tester);
    });

    testWidgets('without the capability the server\'s reason is shown', (
      tester,
    ) async {
      await open(tester, capability: false);
      picker.saveTo = '${out.path}/general.ndjson';
      await tester.tap(find.byKey(const ValueKey('export.start')));
      await until(tester, find.byKey(const ValueKey('export.notice')));
      expect(File(picker.saveTo!).existsSync(), isFalse);
      expect(find.byKey(const ValueKey('export.close')), findsOneWidget);
      await close(tester);
    });

    testWidgets('a cancelled save dialog keeps the range', (tester) async {
      await open(tester, capability: true);
      await tester.tap(find.byKey(const ValueKey('export.start')));
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 10)),
      );
      await tester.pump();
      expect(picker.asked, hasLength(1));
      expect(find.byKey(const ValueKey('export.start')), findsOneWidget);
      expect(find.byKey(const ValueKey('export.notice')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('export.cancel')));
      await tester.pump();
      await close(tester);
    });
  });
}
