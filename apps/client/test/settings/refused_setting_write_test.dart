// SPDX-License-Identifier: Apache-2.0

// A personal setting the instance refuses for good (docs/decisions.d/AUD-client-settings-01.md;
// 04 13.2 item 5): its outbox entry has no thread, so no history shows it. The settings show
// it with the reason, "Send again" sends it under a new ID, and "Discard" removes the entry
// and puts the local row back to what the instance stores. The real auth session, local
// database, engine and outbox run against the fake server; the notice is a widget test over
// the same repository.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/settings/refused_writes_notice.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:client/settings/settings_shell.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart' show problemText;
import 'package:local_store/local_store.dart' show OutboxState;
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/fakes.dart';
import '../support/widgets.dart';

void main() {
  group('against the fake server', () {
    // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
    HttpOverrides? saved;
    late FakeInstance f;
    late ProviderContainer c;

    setUp(() async {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      f = await FakeInstance.start();
      c = appContainer(f);
      c.listen(userSettingsProvider, (_, _) {});
      final control = await connected(c, f);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      await eventually(
        'live',
        () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
      );
      c.listen(refusedSettingWritesProvider, (_, _) {});
    });

    tearDown(() async {
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    });

    SettingsRepository writer() => c.read(settingsRepositoryProvider)!;
    PersonalSettings shown() => c.read(personalSettingsProvider);
    List<RefusedSettingWrite> refused() =>
        c.read(refusedSettingWritesProvider).value ?? const [];
    Object? onServer(String key) =>
        f.server.world.settings[f.seed.alex.id]?[key]?['value'];
    Future<int> outbox() async =>
        (await c.read(localDatabaseProvider)!.outboxDao.allEntries()).length;

    /// The instance refuses the next write of [operation] for good.
    void refuseNext(String operation) => f.server.faults.onRequest(
      operation,
      status: 422,
      code: 'validation_failed',
      reason: 'unknown_key',
    );

    Future<void> stored(String key, Object? value) =>
        eventually('$key stored', () async {
          return onServer(key) == value && await outbox() == 0;
        });

    test('a refused setting stays visible with its reason, and discarding it '
        'puts the stored value back', () async {
      await writer().set(SettingKeys.theme, 'dark');
      await stored(SettingKeys.theme, 'dark');

      refuseNext('putMySetting');
      await writer().set(SettingKeys.theme, 'light');
      await eventually('the refused write', () => refused().length == 1);

      final write = refused().single;
      expect(write.key, SettingKeys.theme);
      expect(write.code, 'validation_failed');
      expect(write.reason, 'unknown_key');
      // What the person chose shows until the person decides about the entry.
      expect(shown().theme, 'light');
      expect(onServer(SettingKeys.theme), 'dark');

      await writer().discard(write);

      await eventually('no refused write', () => refused().isEmpty);
      await eventually('the stored theme', () => shown().theme == 'dark');
      expect(await outbox(), 0);
      expect(f.server.violations, isEmpty);
    });

    test('sending a refused setting again stores it', () async {
      refuseNext('putMySetting');
      await writer().set(SettingKeys.theme, 'dark');
      await eventually('the refused write', () => refused().length == 1);
      final first = refused().single;

      await writer().sendAgain(first);

      await stored(SettingKeys.theme, 'dark');
      await eventually('no refused write', () => refused().isEmpty);
      expect(shown().theme, 'dark');
      // A new ID: the instance saw two requests (04 13.2 item 5).
      final keys = [
        for (final r in f.server.requestsOf('putMySetting'))
          r.headers['idempotency-key'],
      ];
      expect(keys, hasLength(2));
      expect(keys.toSet(), hasLength(2));
    });

    test('discarding a refused first value removes the local row', () async {
      refuseNext('putMySetting');
      await writer().set(SettingKeys.enterSends, false);
      await eventually('the refused write', () => refused().length == 1);
      expect(shown().enterSends, isFalse);

      await writer().discard(refused().single);

      await eventually('the default', () => shown().enterSends);
      expect(
        await c.read(localDatabaseProvider)!.personalDao.watchSettings().first,
        isEmpty,
      );
    });

    test('discarding a refused reset puts the stored value back', () async {
      await writer().set(SettingKeys.theme, 'dark');
      await stored(SettingKeys.theme, 'dark');

      refuseNext('deleteMySetting');
      await writer().reset(SettingKeys.theme);
      await eventually('the refused write', () => refused().length == 1);
      expect(shown().theme, 'system');

      await writer().discard(refused().single);

      await eventually('the stored theme', () => shown().theme == 'dark');
      await eventually('no refused write', () => refused().isEmpty);
    });

    test(
      'a discard that cannot read the stored value changes nothing',
      () async {
        refuseNext('putMySetting');
        await writer().set(SettingKeys.theme, 'dark');
        await eventually('the refused write', () => refused().length == 1);
        final write = refused().single;

        f.server.faults.onRequest(
          'listMySettings',
          status: 503,
          code: 'unavailable',
          times: 100000,
        );
        await expectLater(
          writer().discard(write),
          throwsA(isA<DioException>()),
        );

        expect(refused(), [write]);
        expect(shown().theme, 'dark');
        expect(await outbox(), 1);
      },
    );

    test(
      'a later write of the same setting that still waits keeps its value',
      () async {
        refuseNext('putMySetting');
        await writer().set(SettingKeys.theme, 'dark');
        await eventually('the refused write', () => refused().length == 1);
        final write = refused().single;

        // The next write is not answered for now: it waits in the outbox.
        f.server.faults.onRequest(
          'putMySetting',
          status: 503,
          code: 'unavailable',
          times: 100000,
        );
        await writer().set(SettingKeys.theme, 'light');
        await eventually(
          'the second write was tried',
          () => f.server.requestsOf('putMySetting').length >= 2,
        );

        await writer().discard(write);

        await eventually('no refused write', () => refused().isEmpty);
        expect(shown().theme, 'light');
        while (f.server.faults.pending.isNotEmpty) {
          f.server.faults.take('putMySetting');
        }
        await stored(SettingKeys.theme, 'light');
        expect(shown().theme, 'light');
      },
    );

    test('a refused write of another kind is none of the settings', () async {
      final engine = c.read(syncEngineProvider)!;
      refuseNext('putMySidebarSpace');
      await engine.outbox.enqueue(
        OutboxRequest(
          kind: 'putMySidebarSpace',
          payload: {'space_id': f.seed.space, 'collapsed': false},
        ),
      );
      await eventually('the failed entry', () async {
        final entries = await c
            .read(localDatabaseProvider)!
            .outboxDao
            .allEntries();
        return entries.any((e) => e.state == OutboxState.failed);
      });
      // The list had its time to follow the outbox.
      await Future<void>.delayed(const Duration(milliseconds: 100));
      expect(refused(), isEmpty);
    });
  });

  group('the notice in the settings', () {
    const theme = RefusedSettingWrite(
      entryId: 'e1',
      key: SettingKeys.theme,
      code: 'validation_failed',
    );

    Widget host(
      List<RefusedSettingWrite> writes,
      _Actions actions, {
      Widget child = const RefusedSettingWritesNotice(),
    }) => ProviderScope(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(dataRoot: Directory.systemTemp),
        ),
        personalSettingsProvider.overrideWithValue(
          PersonalSettings.resolve(
            const {},
            desktop: true,
            hasCrashTarget: false,
          ),
        ),
        settingsRepositoryProvider.overrideWithValue(null),
        refusedSettingWritesProvider.overrideWith(
          (ref) => Stream.value(writes),
        ),
        refusedSettingActionsProvider.overrideWithValue(actions),
      ],
      child: hostScreen(Scaffold(body: SingleChildScrollView(child: child))),
    );

    testWidgets('nothing shows while no setting was refused', (tester) async {
      await tester.pumpWidget(host(const [], _Actions()));
      await tester.pump();
      expect(find.byKey(const ValueKey('settings.refusedWrite')), findsNothing);
    });

    testWidgets('a refused setting is named with the reason and both actions', (
      tester,
    ) async {
      await tester.pumpWidget(host(const [theme], _Actions()));
      await tester.pump();

      expect(
        find.text(
          en.settingsWriteRefused(
            en.settingsTheme,
            problemText(en, 'validation_failed'),
          ),
        ),
        findsOneWidget,
      );
      expect(find.text(en.settingsWriteSendAgain), findsOneWidget);
      expect(find.text(en.settingsWriteDiscard), findsOneWidget);
    });

    testWidgets('a setting without a row of its own is named by its key', (
      tester,
    ) async {
      const write = RefusedSettingWrite(
        entryId: 'e2',
        key: 'notify.preview_mode',
        code: 'forbidden',
        reason: 'owner_only',
      );
      await tester.pumpWidget(host(const [write], _Actions()));
      await tester.pump();
      expect(
        find.text(
          en.settingsWriteRefused(
            'notify.preview_mode',
            problemText(en, 'forbidden', reason: 'owner_only'),
          ),
        ),
        findsOneWidget,
      );
    });

    testWidgets('every section of the settings shows it above its rows', (
      tester,
    ) async {
      desktopWindow(tester);
      await tester.pumpWidget(
        host(
          const [theme],
          _Actions(),
          child: sectionBody(SettingsSection.privacy),
        ),
      );
      await tester.pump();
      final notice = tester.getTopLeft(
        find.byKey(const ValueKey('settings.refusedWrite')),
      );
      final section = tester.getTopLeft(find.text(en.settingsSectionPrivacy));
      expect(notice.dy, lessThan(section.dy));
    });

    testWidgets('"Send again" and "Discard" act on the entry', (tester) async {
      final actions = _Actions();
      await tester.pumpWidget(host(const [theme], actions));
      await tester.pump();

      await tester.tap(find.text(en.settingsWriteSendAgain));
      await tester.pump();
      await tester.tap(find.text(en.settingsWriteDiscard));
      await tester.pump();

      expect(actions.calls, ['sendAgain e1', 'discard e1']);
    });

    testWidgets('a discard without an answer says so and keeps both actions', (
      tester,
    ) async {
      final actions = _Actions()
        ..discardFails = DioException.connectionError(
          requestOptions: RequestOptions(path: '/me/settings'),
          reason: 'no route',
        );
      await tester.pumpWidget(host(const [theme], actions));
      await tester.pump();

      await tester.tap(find.text(en.settingsWriteDiscard));
      await tester.pump();

      expect(
        find.text(en.settingsWriteDiscardFailed(en.selfOffline)),
        findsOneWidget,
      );
      expect(
        tester
            .widget<TextButton>(
              find.widgetWithText(TextButton, en.settingsWriteDiscard),
            )
            .onPressed,
        isNotNull,
      );
    });
  });
}

/// Records what the notice asks for.
class _Actions implements RefusedSettingActions {
  final List<String> calls = [];
  Object? discardFails;

  @override
  Future<void> sendAgain(RefusedSettingWrite write) async =>
      calls.add('sendAgain ${write.entryId}');

  @override
  Future<void> discard(RefusedSettingWrite write) async {
    calls.add('discard ${write.entryId}');
    if (discardFails case final Object error) throw error;
  }
}
