// SPDX-License-Identifier: Apache-2.0

// The notification preferences, quiet hours and channel export of the fake (brief M4-33
// Scope "Fake server handlers"; 13 3.1 to 3.4; 04 14; openapi `exportChannel`): put, list and
// delete with their personal events and versions, the automatic following of 13 3.2, the
// rules with the complete list in `quiet_hours.updated`, the effect of a REST level on the
// hint decision, and the export with its capability, step-up and range.

import 'dart:convert';

import 'package:api_client/api_client.dart' as api;
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
  });
  tearDown(() => h.stop());

  Future<DioException?> failure(Future<Object?> call) =>
      call.then((_) => null, onError: (Object e) => e as DioException);

  List<FakeEvent> events(String type) => [
    for (final e in world.log)
      if (e.type == type) e,
  ];

  Map<String, Object?> payloadOf(FakeEvent e) =>
      e.envelope['payload']! as Map<String, Object?>;

  api.NotificationPreferencePut put(
    api.NotificationLevel level, {
    DateTime? until,
  }) => api.NotificationPreferencePut(
    (b) => b
      ..level = level
      ..mutedUntil = until,
  );

  group('preferences', () {
    test('put, an equal put, list and delete with their events', () async {
      final alex = h.device();
      await signedIn(h, alex);
      final prefs = alex.api.getPreferencesApi();
      final general = h.seed.general;

      final first = (await prefs.putNotificationPreference(
        targetKind: 'channel',
        targetId: general,
        notificationPreferencePut: put(api.NotificationLevel.mute),
      )).data!;
      expect(first.version, 1);
      final same = (await prefs.putNotificationPreference(
        targetKind: 'channel',
        targetId: general,
        notificationPreferencePut: put(api.NotificationLevel.mute),
      )).data!;
      expect(same.version, 1, reason: 'a write that changes nothing');
      expect(events('notification_pref.updated'), hasLength(1));

      final until = DateTime.utc(2030, 1, 1, 8);
      final muted = (await prefs.putNotificationPreference(
        targetKind: 'channel',
        targetId: general,
        notificationPreferencePut: put(api.NotificationLevel.all, until: until),
      )).data!;
      expect((muted.version, muted.mutedUntil), (2, until));

      final listed = (await prefs.listMyNotificationPreferences()).data!.items;
      expect(listed.single.targetId, general);

      await prefs.deleteNotificationPreference(
        targetKind: 'channel',
        targetId: general,
      );
      final deleted = events('notification_pref.updated').last;
      expect(deleted.audience, h.seed.alex.id);
      expect(payloadOf(deleted)['deleted'], isTrue);
      expect(
        (payloadOf(deleted)['preference']! as Map)['version'],
        3,
        reason: 'the last version plus one',
      );
      // Deleting again answers 204 without an event.
      await prefs.deleteNotificationPreference(
        targetKind: 'channel',
        targetId: general,
      );
      expect(events('notification_pref.updated'), hasLength(3));
      expect(
        (await prefs.listMyNotificationPreferences()).data!.items,
        isEmpty,
      );
      expect(h.server.violations, isEmpty);
    });

    test('an unreadable target is 404, an archived one 409', () async {
      final alex = h.device();
      await signedIn(h, alex);
      final prefs = alex.api.getPreferencesApi();
      final hidden = await failure(
        prefs.putNotificationPreference(
          targetKind: 'channel',
          targetId: h.seed.private,
          notificationPreferencePut: put(api.NotificationLevel.all),
        ),
      );
      expect(hidden?.response?.statusCode, 404);
      world.channels[h.seed.support]!.json['archived_at'] = world.stamp();
      final archived = await failure(
        prefs.putNotificationPreference(
          targetKind: 'thread',
          targetId: h.seed.printer,
          notificationPreferencePut: put(api.NotificationLevel.all),
        ),
      );
      expect(archived?.response?.statusCode, 409);
      expect((archived?.response?.data as Map)['reason'], 'target_archived');
    });

    test('a level set over REST decides the hints of 13 3.1', () async {
      final alex = h.device();
      await signedIn(h, alex);
      world.connectedDevices = (userId) => {
        for (final d in world.devices.values)
          if (d.userId == userId) d.id,
      };
      await alex.api.getPreferencesApi().putNotificationPreference(
        targetKind: 'channel',
        targetId: h.seed.general,
        notificationPreferencePut: put(api.NotificationLevel.mute),
      );
      final message = h.seed.post(h.seed.sam, h.seed.releaseNotes, 'One');
      expect(world.notify(message['id']! as String), isEmpty);
      await alex.api.getPreferencesApi().putNotificationPreference(
        targetKind: 'thread',
        targetId: h.seed.releaseNotes,
        notificationPreferencePut: put(api.NotificationLevel.all),
      );
      final next = h.seed.post(h.seed.sam, h.seed.releaseNotes, 'Two');
      expect(world.notify(next['id']! as String).map((n) => n.recipient), [
        h.seed.alex.id,
      ]);
    });
  });

  group('automatic following (13 3.2)', () {
    test('a person who writes in a thread follows it once', () {
      final thread = h.seed.releaseNotes;
      h.seed.post(h.seed.sam, thread, 'Hello');
      final follow = events('notification_pref.updated').single;
      expect(follow.audience, h.seed.sam.id);
      expect(payloadOf(follow)['preference'], {
        'target_kind': 'thread',
        'target_id': thread,
        'level': 'all',
        'version': 1,
      });
      h.seed.post(h.seed.sam, thread, 'Again');
      expect(events('notification_pref.updated'), hasLength(1));
    });

    test('an own row stays, agents and the main thread never follow', () {
      world.setNotificationPref(
        h.seed.sam.id,
        'thread',
        h.seed.releaseNotes,
        'mute',
      );
      h.seed.post(h.seed.sam, h.seed.releaseNotes, 'Muted');
      h.seed.post(h.seed.robin, h.seed.releaseNotes, 'Agent');
      final main = world.addMainThread(h.seed.general, createdBy: 'x');
      h.seed.post(h.seed.sam, main, 'Main');
      expect(events('notification_pref.updated'), isEmpty);
      expect(world.effectiveLevel(h.seed.sam.id, h.seed.releaseNotes), 'mute');
    });
  });

  group('quiet hours', () {
    api.QuietHoursRuleInput rule({
      int weekdays = 31,
      String start = '19:00',
      String end = '07:00',
      String? zone,
      bool enabled = true,
    }) => api.QuietHoursRuleInput(
      (b) => b
        ..weekdays = weekdays
        ..startLocal = start
        ..endLocal = end
        ..timeZone = zone
        ..enabled = enabled,
    );

    test('create, put and delete send the complete list', () async {
      final alex = h.device();
      await signedIn(h, alex);
      final prefs = alex.api.getPreferencesApi();
      final weekday = (await prefs.createQuietHoursRule(
        quietHoursRuleInput: rule(),
      )).data!;
      final weekend = (await prefs.createQuietHoursRule(
        quietHoursRuleInput: rule(weekdays: 96, start: '00:00', end: '00:00'),
      )).data!;
      expect(
        (payloadOf(events('quiet_hours.updated').last)['rules']! as List)
            .length,
        2,
      );

      final same = (await prefs.putQuietHoursRule(
        ruleId: weekday.id,
        quietHoursRuleInput: rule(),
      )).data!;
      expect(same.version, 1);
      expect(events('quiet_hours.updated'), hasLength(2));
      final zoned = (await prefs.putQuietHoursRule(
        ruleId: weekday.id,
        quietHoursRuleInput: rule(zone: 'Europe/Berlin'),
      )).data!;
      expect((zoned.version, zoned.timeZone), (2, 'Europe/Berlin'));

      await prefs.deleteQuietHoursRule(ruleId: weekend.id);
      final rules =
          payloadOf(events('quiet_hours.updated').last)['rules']! as List;
      expect([for (final r in rules) (r as Map)['id']], [weekday.id]);
      expect((await prefs.listMyQuietHours()).data!.items.map((r) => r.id), [
        weekday.id,
      ]);
      final missing = await failure(
        prefs.deleteQuietHoursRule(ruleId: weekend.id),
      );
      expect(missing?.response?.statusCode, 404);
      expect(h.server.violations, isEmpty);
    });

    test('a time that is not HH:MM is refused', () async {
      final alex = h.device();
      await signedIn(h, alex);
      final e = await failure(
        alex.api.getPreferencesApi().createQuietHoursRule(
          quietHoursRuleInput: rule(start: '7:00'),
        ),
      );
      expect(e?.response?.statusCode, 400);
    });
  });

  group('channel export', () {
    Future<Response<List<int>>> export(
      TestDevice device,
      String channel, {
      required String from,
      required String to,
    }) => device.api.dio.get<List<int>>(
      '/channels/$channel/export',
      queryParameters: {'from': from, 'to': to},
      options: Options(
        responseType: ResponseType.bytes,
        extra: {
          'secure': [
            {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          ],
        },
      ),
    );

    test('without the capability export it is 403', () async {
      final alex = h.device();
      await signedIn(h, alex);
      final e = await failure(
        export(
          alex,
          h.seed.general,
          from: '2000-01-01T00:00:00Z',
          to: '2100-01-01T00:00:00Z',
        ),
      );
      expect(e?.response?.statusCode, 403);
    });

    test('with the capability: step-up, then the messages as NDJSON', () async {
      final first = h.seed.post(h.seed.sam, h.seed.releaseNotes, 'One');
      final second = h.seed.post(h.seed.alex, h.seed.releaseNotes, 'Two');
      world.grantCapability(h.seed.alex.id, h.seed.general, 'export');
      final alex = h.device();
      await signedIn(h, alex);
      h.server.requests.clear();
      final response = await export(
        alex,
        h.seed.general,
        from: '2000-01-01T00:00:00Z',
        to: '2100-01-01T00:00:00Z',
      );
      expect(
        [for (final r in h.server.requests) '${r.operationId} ${r.status}'],
        [
          'exportChannel 401',
          'createStepUpChallenge 200',
          'completeStepUpWithDeviceKey 200',
          'exportChannel 200',
        ],
      );
      final lines = const LineSplitter().convert(utf8.decode(response.data!));
      expect(
        [for (final l in lines) (jsonDecode(l) as Map)['id']],
        [first['id'], second['id']],
      );
      final empty = await export(
        alex,
        h.seed.general,
        from: '2000-01-01T00:00:00Z',
        to: '2000-01-02T00:00:00Z',
      );
      expect(empty.data, isEmpty);
      final backwards = await failure(
        export(
          alex,
          h.seed.general,
          from: '2000-01-02T00:00:00Z',
          to: '2000-01-01T00:00:00Z',
        ),
      );
      expect(backwards?.response?.statusCode, 400);
      expect(h.server.violations, isEmpty);
    });
  });
}
