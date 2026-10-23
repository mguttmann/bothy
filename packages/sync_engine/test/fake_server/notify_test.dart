// SPDX-License-Identifier: Apache-2.0

// The hints of the fake (brief M4-30; 13 2, 3.1, 4.6, 6.3, 6.4): the effective level in the
// order of 13 3.1, the kinds message, mention and dm, the excluded author and the messages
// that give no hint, the delivery as `notification.created` only to live connections of the
// recipient, the preview modes and the quiet flag; and the hint at the engine's stream.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import '../scenarios/support.dart' as scenario;
import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed seed;
  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    seed = h.seed;
  });
  tearDown(() => h.stop());

  var deviceCount = 0;

  /// An active device of [person] without a gateway connection.
  String addDevice(FakePerson person) {
    final id = world.nextId();
    world.devices[id] = FakeDevice(
      id: id,
      userId: person.id,
      name: 'Device ${++deviceCount}',
      platform: 'windows',
      state: 'active',
      jkt: 'jkt-$id',
      createdAt: world.stamp(),
    );
    return id;
  }

  /// Treats every active device as connected, for the decision tests without sockets.
  void everyDeviceConnected() => world.connectedDevices = (userId) => {
    for (final d in world.devices.values)
      if (d.userId == userId && d.state == 'active') d.id,
  };

  /// A direct message channel of Alex and Sam with one thread.
  String dmThread() {
    final channel = world.addChannel(seed.space, 'alex-sam', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    world.channels[channel]!.json
      ..remove('space_id')
      ..['kind'] = 'dm';
    return world.addThread(channel, 'Sam', createdBy: seed.alex.id);
  }

  JsonMap post(
    FakePerson author,
    String thread,
    String text, {
    List<String> mentions = const [],
    bool silent = false,
    String origin = 'human',
  }) => world.createMessage(author.id, thread, {
    'client_msg_id': world.nextId(),
    'content_md': text,
    if (mentions.isNotEmpty) 'mention_ids': mentions,
    if (silent) 'silent': true,
  }, origin: origin).message;

  group('effective level (13 3.1)', () {
    test('the first existing value wins, in order', () {
      final sam = seed.sam.id;
      final thread = seed.releaseNotes;
      expect(world.effectiveLevel(sam, thread), 'mentions');
      world.userDefaultLevel = 'mute';
      expect(world.effectiveLevel(sam, thread), 'mute');
      world.putSetting(sam, 'notify.default_level', 'all');
      expect(world.effectiveLevel(sam, thread), 'all');
      world.channels[seed.general]!.json['default_notification_level'] =
          'mentions';
      expect(world.effectiveLevel(sam, thread), 'mentions');
      world.setNotificationPref(sam, 'space', seed.space, 'mute');
      expect(world.effectiveLevel(sam, thread), 'mute');
      world.setNotificationPref(sam, 'channel', seed.general, 'all');
      expect(world.effectiveLevel(sam, thread), 'all');
      world.setNotificationPref(sam, 'thread', thread, 'mentions');
      expect(world.effectiveLevel(sam, thread), 'mentions');
      world.deleteNotificationPref(sam, 'thread', thread);
      expect(world.effectiveLevel(sam, thread), 'all');
      expect(
        world.effectiveLevel(seed.alex.id, thread),
        'mentions',
        reason: "Sam's preferences are not Alex's",
      );
    });

    test('a DM channel counts as all before the personal default', () {
      final thread = dmThread();
      world.putSetting(seed.sam.id, 'notify.default_level', 'mute');
      expect(world.effectiveLevel(seed.sam.id, thread), 'all');
    });

    test('muted_until in the future mutes, in the past does not', () {
      final sam = seed.sam.id;
      final thread = seed.releaseNotes;
      world.setNotificationPref(
        sam,
        'thread',
        thread,
        'all',
        mutedUntil: world.now().add(const Duration(hours: 1)),
      );
      expect(world.effectiveLevel(sam, thread), 'mute');
      world.setNotificationPref(
        sam,
        'thread',
        thread,
        'all',
        mutedUntil: world.now().subtract(const Duration(minutes: 1)),
      );
      expect(world.effectiveLevel(sam, thread), 'all');
    });

    test('a target kind or level outside the contract is rejected', () {
      expect(
        () => world.setNotificationPref(seed.sam.id, 'user', 'x', 'all'),
        throwsArgumentError,
      );
      expect(
        () => world.setNotificationPref(
          seed.sam.id,
          'thread',
          seed.releaseNotes,
          'loud',
        ),
        throwsArgumentError,
      );
    });
  });

  group('recipients and kinds (13 2)', () {
    setUp(() {
      addDevice(seed.alex);
      addDevice(seed.sam);
      everyDeviceConnected();
    });

    test('message needs all; the author and the agent get nothing', () {
      final m = post(seed.alex, seed.releaseNotes, 'Deploy at noon');
      expect(world.notify(m['id']! as String), isEmpty);
      expect(world.notificationSkips, [(userId: seed.sam.id, reason: 'level')]);
      world.putSetting(seed.sam.id, 'notify.default_level', 'all');
      final created = world.notify(m['id']! as String);
      expect(
        [for (final n in created) (n.recipient, n.kind)],
        [(seed.sam.id, 'message')],
      );
      expect(created.single.threadId, seed.releaseNotes);
      expect(created.single.messageId, m['id']);
      expect(world.notifications, created);
    });

    test('a mention needs mentions', () {
      final m = post(
        seed.alex,
        seed.releaseNotes,
        'Sam, please check',
        mentions: [seed.sam.id],
      );
      expect(world.notify(m['id']! as String).single.kind, 'mention');
      world.setNotificationPref(seed.sam.id, 'channel', seed.general, 'mute');
      expect(world.notify(m['id']! as String), isEmpty);
    });

    test('a message in a DM channel is dm, also when it mentions', () {
      final thread = dmThread();
      final m = post(seed.alex, thread, 'Lunch?', mentions: [seed.sam.id]);
      final n = world.notify(m['id']! as String).single;
      expect((n.recipient, n.kind), (seed.sam.id, 'dm'));
      expect(n.payload.containsKey('space_name'), isFalse);
      expect(n.payload['channel_name'], 'alex-sam');
    });

    test('silent and imported messages give no hint', () {
      world.putSetting(seed.sam.id, 'notify.default_level', 'all');
      final silent = post(seed.alex, seed.releaseNotes, 'fyi', silent: true);
      final imported = post(
        seed.alex,
        seed.releaseNotes,
        'old',
        origin: 'import',
      );
      expect(world.notify(silent['id']! as String), isEmpty);
      expect(world.notify(imported['id']! as String), isEmpty);
      expect(world.notificationSkips, isEmpty);
    });

    test('notifyOnMessages makes createMessage decide', () {
      world.putSetting(seed.sam.id, 'notify.default_level', 'all');
      post(seed.alex, seed.releaseNotes, 'not yet');
      expect(world.notifications, isEmpty, reason: 'off by default');
      world.notifyOnMessages = true;
      post(seed.alex, seed.releaseNotes, 'now');
      expect(world.notifications.single.recipient, seed.sam.id);
    });

    test('the preview follows notify.preview_mode (13 6.4)', () {
      world
        ..putSetting(seed.sam.id, 'notify.default_level', 'all')
        ..previewMaxChars = 10;
      final m = post(seed.alex, seed.releaseNotes, 'Deploy at noon today');
      final full = world.notify(m['id']! as String).single.payload;
      expect(full, {
        'nid': isA<String>(),
        'kind': 'message',
        'thread_id': seed.releaseNotes,
        'message_id': m['id'],
        'quiet': false,
        'count': 1,
        'space_name': 'Team',
        'channel_name': 'general',
        'thread_title': 'Release notes',
        'sender_display_name': 'Alex',
        'excerpt': 'Deploy at ',
      });
      world.putSetting(seed.sam.id, 'notify.preview_mode', 'names');
      final names = world.notify(m['id']! as String).single.payload;
      expect(names.containsKey('excerpt'), isFalse);
      expect(names['sender_display_name'], 'Alex');
      world.putSetting(seed.sam.id, 'notify.preview_mode', 'none');
      final none = world.notify(m['id']! as String).single.payload;
      expect(none.keys.toSet(), {
        'nid',
        'kind',
        'thread_id',
        'message_id',
        'quiet',
        'count',
      });
    });

    test('quiet hours set the quiet flag', () {
      world
        ..putSetting(seed.sam.id, 'notify.default_level', 'all')
        ..quietHours.add(seed.sam.id);
      final m = post(seed.alex, seed.releaseNotes, 'late');
      final n = world.notify(m['id']! as String).single;
      expect(n.quiet, isTrue);
      expect(n.payload['quiet'], isTrue);
    });
  });

  group('delivery (13 4.6, 6.3)', () {
    final clients = <RawClient>[];
    tearDown(() {
      for (final c in clients) {
        c.close();
      }
      clients.clear();
    });

    Future<RawGateway> connect(
      TestDevice device,
      ({String token, String deviceId}) s,
    ) async {
      final raw = RawClient(h.server.baseUri, device.key);
      clients.add(raw);
      final g = await RawGateway.connect(h.server.gatewayUri, raw);
      await g.identify(s.token, s.deviceId);
      await g.waitFor('ready');
      return g;
    }

    List<Map<String, Object?>> hints(RawGateway g) => [
      for (final d in g.data('dispatch'))
        if (d['type'] == 'notification.created') d,
    ];

    Future<void> until(bool Function() ok) async {
      final end = DateTime.now().add(const Duration(seconds: 10));
      while (!ok()) {
        if (DateTime.now().isAfter(end)) throw StateError('timed out');
        await Future<void>.delayed(const Duration(milliseconds: 5));
      }
    }

    test('a connected device gets one dispatch, another device none', () async {
      final samDevice = h.device(login: WorldSeed.samLogin, keySeed: 2);
      final connected = await signedIn(h, samDevice);
      final idle = addDevice(seed.sam);
      final g = await connect(samDevice, connected);
      final m = post(
        seed.alex,
        seed.releaseNotes,
        'Sam, the build is ready',
        mentions: [seed.sam.id],
      );
      final n = world.notify(m['id']! as String).single;
      expect(n.devices, {connected.deviceId});
      expect(n.devices, isNot(contains(idle)));
      await until(() => hints(g).isNotEmpty);
      // A later message proves nothing else was on its way.
      post(seed.alex, seed.releaseNotes, 'after');
      await until(
        () =>
            g
                .data('dispatch')
                .where((d) => d['type'] == 'message.created')
                .length >=
            2,
      );
      final d = hints(g).single;
      expect(d['seq'], isNull);
      expect(d['audience'], 'self');
      expect(d['resource'], {'kind': 'thread', 'id': seed.releaseNotes});
      expect(d['scope'], {
        'channel_id': seed.general,
        'thread_id': seed.releaseNotes,
      });
      expect(d['payload'], n.payload);
      expect(d['payload'], containsPair('kind', 'mention'));
      expect(d['payload'], containsPair('excerpt', 'Sam, the build is ready'));
      expect(world.log.where((e) => e.type == 'notification.created'), isEmpty);
      expect(h.server.violations, isEmpty);
      await g.close();
    });

    test('only the audience on the target devices gets it', () async {
      final alexDevice = h.device(keySeed: 1);
      final phone = h.device(login: WorldSeed.samLogin, keySeed: 2);
      final laptop = h.device(login: WorldSeed.samLogin, keySeed: 3);
      final a = await connect(alexDevice, await signedIn(h, alexDevice));
      final p = await signedIn(h, phone);
      final l = await signedIn(h, laptop);
      final gp = await connect(phone, p);
      final gl = await connect(laptop, l);
      final m = post(
        seed.alex,
        seed.releaseNotes,
        'Sam?',
        mentions: [seed.sam.id],
      );
      expect(world.notify(m['id']! as String).single.devices, {
        p.deviceId,
        l.deviceId,
      });
      await until(() => hints(gp).isNotEmpty && hints(gl).isNotEmpty);
      // An event for the phone alone reaches neither the laptop nor Alex.
      world.publishEphemeral(
        'notification.created',
        resource: {'kind': 'thread', 'id': seed.releaseNotes},
        scope: {'channel_id': seed.general, 'thread_id': seed.releaseNotes},
        payload: {
          'nid': world.nextId(),
          'kind': 'message',
          'quiet': false,
          'count': 1,
        },
        audience: seed.sam.id,
        devices: {p.deviceId},
      );
      await until(() => hints(gp).length == 2);
      post(seed.alex, seed.releaseNotes, 'after');
      bool echoed(RawGateway g) =>
          g
              .data('dispatch')
              .where((d) => d['type'] == 'message.created')
              .length >=
          2;
      await until(() => echoed(gl) && echoed(a));
      expect(hints(gl), hasLength(1));
      expect(hints(a), isEmpty);
      expect(h.server.violations, isEmpty);
    });

    test(
      'a recipient without a connected device is skipped as no_route',
      () async {
        await signedIn(h, h.device(login: WorldSeed.samLogin, keySeed: 2));
        final m = post(
          seed.alex,
          seed.releaseNotes,
          'Sam?',
          mentions: [seed.sam.id],
        );
        expect(world.notify(m['id']! as String), isEmpty);
        expect(world.notificationSkips, [
          (userId: seed.sam.id, reason: 'no_route'),
        ]);
      },
    );
  });

  test('the engine streams a hint of the fake (13 6.3)', () async {
    await scenario.withScenario((server, w, client) async {
      server.world.notifyOnMessages = true;
      await client.start();
      final received = <NotificationCreated>[];
      final sub = client.engine.notifications.listen(received.add);
      expect(client.engine.focus.idle, isFalse);
      client.engine.focus.setVisible({}, idle: true);
      expect(client.engine.focus.idle, isTrue);
      w.post(w.sam, w.releaseNotes, 'Alex, ready for review');
      server.world.createMessage(w.sam.id, w.releaseNotes, {
        'client_msg_id': server.world.nextId(),
        'content_md': 'Alex, see above',
        'mention_ids': [w.alex.id],
      });
      await waitFor('the hint', () => received.isNotEmpty);
      final n = received.single;
      expect(n.kind, HintKind.mention);
      expect(n.threadId, ThreadId.parse(w.releaseNotes));
      expect(n.senderDisplayName, 'Sam');
      expect(n.excerpt, 'Alex, see above');
      expect(server.violations, isEmpty);
      await sub.cancel();
    });
  });
}
