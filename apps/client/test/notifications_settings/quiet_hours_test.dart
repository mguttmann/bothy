// SPDX-License-Identifier: Apache-2.0

// The quiet hours of D9 (brief M4-33 Tests; 13 3.4; D-M3-17-4): the weekday mask with bit 0
// on Monday, a window across midnight, "All day" stored as 00:00 to 00:00, the time zone that
// defaults to the person's, the summary, and the writes of a new, an edited and a deleted
// rule through the outbox.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:client/notifications_settings/providers.dart';
import 'package:client/notifications_settings/quiet_hours_editor.dart';
import 'package:client/sync/notify_ops.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/widgets.dart';

/// Answers every request with [body] as JSON and status 200.
class JsonAdapter implements HttpClientAdapter {
  JsonAdapter(this.body);

  final Map<String, Object?> body;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromString(
    jsonEncode(body),
    200,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );

  @override
  void close({bool force = false}) {}
}

QuietHours rule({
  int weekdays = 0x1f,
  String start = '19:00',
  String end = '07:00',
  String? zone,
  bool enabled = true,
}) => QuietHours(
  id: QuietHoursId.parse('0192f4a0-7b1c-7c3e-88ef-5d4efd8f4f24'),
  weekdays: weekdays,
  startLocal: start,
  endLocal: end,
  timeZone: zone,
  enabled: enabled,
  version: const ResourceVersion(3),
);

void main() {
  final l = lookupAppLocalizations(const Locale('en'));

  group('the pure rules', () {
    test('bit 0 is Monday, bit 6 Sunday', () {
      expect(weekdayBit(DateTime.monday), 1);
      expect(weekdayBit(DateTime.sunday), 64);
      expect(weekdayBit(DateTime.saturday) | weekdayBit(DateTime.sunday), 0x60);
    });

    test('the summary names runs of days, the range and "All day"', () {
      expect(quietHoursSummary(l, rule()), 'Mon-Fri 19:00 to 07:00');
      expect(
        quietHoursSummary(
          l,
          rule(weekdays: 0x60, start: '00:00', end: '00:00'),
        ),
        'Sat, Sun All day',
      );
      expect(
        quietHoursSummary(l, rule(weekdays: 0x15, zone: 'Asia/Tokyo')),
        'Mon, Wed, Fri 19:00 to 07:00 (Asia/Tokyo)',
      );
      expect(
        quietHoursSummary(l, rule(enabled: false)),
        'Mon-Fri 19:00 to 07:00 (off)',
      );
    });

    test('only HH:MM from 00:00 to 23:59 is a time', () {
      expect(isLocalTime('07:30'), isTrue);
      expect(isLocalTime('23:59'), isTrue);
      expect(isLocalTime('7:30'), isFalse);
      expect(isLocalTime('24:00'), isFalse);
      expect(isAllDay(rule(start: '00:00', end: '00:00')), isTrue);
      expect(isAllDay(rule(start: '08:00', end: '08:00')), isFalse);
    });
  });

  group('the editor', () {
    Future<QuietHoursEdit?> edit(
      WidgetTester tester,
      Future<void> Function() steps, {
      QuietHours? existing,
    }) async {
      QuietHoursEdit? result;
      desktopWindow(tester);
      await tester.pumpWidget(
        hostScreen(
          Builder(
            builder: (context) => TextButton(
              onPressed: () async => result = await showQuietHoursEditor(
                context,
                rule: existing,
                personZone: 'Europe/Berlin',
                timeZones: () => const ['Asia/Tokyo', 'Europe/Berlin'],
              ),
              child: const Text('open'),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await steps();
      await tester.tap(find.byKey(const ValueKey('quiet.save')));
      await tester.pumpAndSettle();
      return result;
    }

    testWidgets('a new rule: Mon-Fri across midnight in the person\'s zone', (
      tester,
    ) async {
      final result = await edit(tester, () async {
        expect(find.text('Mine (Europe/Berlin)'), findsWidgets);
      });
      final saved = (result! as QuietHoursSave).rule;
      expect(saved.id, newRulePlaceholder);
      expect(saved.weekdays, 0x1f);
      expect((saved.startLocal, saved.endLocal), ('19:00', '07:00'));
      expect(saved.timeZone, isNull, reason: 'NULL follows users.time_zone');
      expect(saved.enabled, isTrue);
    });

    testWidgets('weekend all day stores 00:00 to 00:00 on bits 5 and 6', (
      tester,
    ) async {
      final result = await edit(tester, () async {
        for (var i = 0; i < 5; i++) {
          await tester.tap(find.byKey(ValueKey('quiet.day.$i')));
        }
        await tester.tap(find.byKey(const ValueKey('quiet.day.5')));
        await tester.tap(find.byKey(const ValueKey('quiet.day.6')));
        await tester.tap(find.byKey(const ValueKey('quiet.allDay')));
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('quiet.start')), findsNothing);
      });
      final saved = (result! as QuietHoursSave).rule;
      expect(saved.weekdays, 0x60);
      expect((saved.startLocal, saved.endLocal), ('00:00', '00:00'));
    });

    testWidgets('a wrong time and no day are refused', (tester) async {
      QuietHoursEdit? result;
      result = await edit(tester, () async {
        await tester.enterText(find.byKey(const ValueKey('quiet.start')), '7');
        for (var i = 0; i < 5; i++) {
          await tester.tap(find.byKey(ValueKey('quiet.day.$i')));
        }
        await tester.pumpAndSettle();
      });
      expect(result, isNull, reason: 'the editor stays open');
      expect(find.text(l.quietHoursTimeInvalid), findsOneWidget);
      expect(find.byKey(const ValueKey('quiet.noDays')), findsOneWidget);
    });

    testWidgets('an existing rule keeps its ID and its own zone', (
      tester,
    ) async {
      final existing = rule(zone: 'Asia/Tokyo');
      final result = await edit(existing: existing, tester, () async {
        await tester.enterText(
          find.byKey(const ValueKey('quiet.end')),
          '06:30',
        );
      });
      final saved = (result! as QuietHoursSave).rule;
      expect(saved.id, existing.id);
      expect(saved.timeZone, 'Asia/Tokyo');
      expect(saved.endLocal, '06:30');
    });
  });

  group('the writes', () {
    late ColumnHost host;
    var n = 0;

    NotifyWriter writer() => NotifyWriter(
      host.engine,
      host.db,
      () =>
          '0192f4a0-7b1c-7000-8000-${(++n).toRadixString(16).padLeft(12, '0')}',
    );

    testWidgets('add, edit before it is sent, then delete', (tester) async {
      host = await ColumnHost.open(tester);
      final w = writer();
      final id = (await tester.runAsync(() => w.addQuietHours(rule())))!;
      var stored = (await tester.runAsync(host.db.personalDao.quietHours))!;
      expect(stored.single.id, id, reason: 'shown at once under its local ID');

      await tester.runAsync(
        () => w.putQuietHours(
          QuietHours(
            id: id,
            weekdays: 0x1f,
            startLocal: '20:00',
            endLocal: '07:00',
            enabled: true,
            version: const ResourceVersion(1),
          ),
        ),
      );
      var entries = (await tester.runAsync(host.db.outboxDao.allEntries))!;
      expect(entries.map((e) => e.kind), [
        NotifyOps.createQuietHoursRule,
        NotifyOps.putQuietHoursRule,
      ]);
      expect(entries.first.id, id.value);
      // The edit waits for the create and gets the server's ID from it.
      expect(entries.last.dependsOn, id.value);
      expect(jsonDecode(entries.last.payload)['rule_id'], isNull);
      stored = (await tester.runAsync(host.db.personalDao.quietHours))!;
      expect(stored.single.startLocal, '20:00');

      // The create was never sent: deleting drops it and its edit from the outbox.
      await tester.runAsync(() => w.deleteQuietHours(id));
      entries = (await tester.runAsync(host.db.outboxDao.allEntries))!;
      expect([
        for (final e in entries)
          if (e.state != OutboxState.failed && e.state != OutboxState.confirmed)
            e.kind,
      ], isEmpty);
      expect(await tester.runAsync(host.db.personalDao.quietHours), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('a stored rule is put and deleted by its ID', (tester) async {
      host = await ColumnHost.open(tester);
      final w = writer();
      final existing = rule();
      await tester.runAsync(
        () => host.db.personalDao.upsertQuietHoursIfNewer(existing),
      );
      await tester.runAsync(() => w.putQuietHours(rule(enabled: false)));
      await tester.runAsync(() => w.deleteQuietHours(existing.id));
      final entries = (await tester.runAsync(host.db.outboxDao.allEntries))!;
      expect(
        [for (final e in entries) jsonDecode(e.payload)['rule_id']],
        [existing.id.value, existing.id.value],
      );
      expect(entries.map((e) => e.kind), [
        NotifyOps.putQuietHoursRule,
        NotifyOps.deleteQuietHoursRule,
      ]);
      await host.dispose(tester);
    });

    testWidgets(
      'a level write runs while a quiet hours write of the same person waits',
      (tester) async {
        host = await ColumnHost.open(tester);
        final w = writer();
        const channel = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
        final existing = rule();
        await tester.runAsync(
          () => host.db.personalDao.upsertQuietHoursIfNewer(existing),
        );
        await tester.runAsync(
          () => w.setLevel(
            NotificationTargetKind.channel,
            channel,
            NotificationLevel.mute,
          ),
        );
        await tester.runAsync(() => w.putQuietHours(rule(enabled: false)));
        final entries = (await tester.runAsync(host.db.outboxDao.allEntries))!;
        expect(entries.map((e) => e.kind), [
          NotifyOps.putNotificationPreference,
          NotifyOps.putQuietHoursRule,
        ]);
        final executors = OutboxExecutors();
        NotifyOps.register(
          executors,
          ApiClient(
            dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com'))
              ..httpClientAdapter = JsonAdapter({
                'target_kind': 'channel',
                'target_id': channel,
                'level': 'mute',
                'version': 7,
              }),
          ),
          host.db,
        );
        final response = await tester.runAsync(
          () => executors
              .lookup(NotifyOps.putNotificationPreference)!
              .send(
                entries.first,
                ApiCallContext(idempotencyKey: entries.first.id),
              ),
        );
        expect(response!.status, 200);
        final stored = (await tester.runAsync(
          host.db.personalDao.notificationPreferences,
        ))!;
        expect(stored.single.version, const ResourceVersion(7));
        await host.dispose(tester);
      },
    );
  });

  test('the defaults of 13 17 apply where a key is absent', () {
    final none = NotifySettings.resolve(const {});
    expect(none.personalDefault, isNull);
    expect(none.previewMode, 'full');
    expect(none.inputBreaksQuietHours, isFalse);
    expect(none.adminEnabled, isTrue);
    final set = NotifySettings.resolve(const {
      'notify.default_level': 'all',
      'notify.preview_mode': 'none',
      'notify.input_needed_breaks_quiet_hours': true,
      'notify.admin_enabled': false,
    });
    expect(set.personalDefault, NotificationLevel.all);
    expect(set.previewMode, 'none');
    expect(set.inputBreaksQuietHours, isTrue);
    expect(set.adminEnabled, isFalse);
  });
}
