// SPDX-License-Identifier: Apache-2.0

// The flow of integration_test/notifications_windows_test.dart headless (brief M4-30 Tests):
// the app's providers against the fake server with a Windows window that hides to the
// notification area. With the window hidden, a message from Sam makes the fake send
// `notification.created` and the presenter shows one toast; a click opens the thread at the
// message; a read on another device removes the toast of that thread (01 10.3), while an edit
// by another person and a new identify, which report the counts without a read, remove none
// (docs/decisions.d/AUD-client-notifications-01.md AUD-CN1/3); after Quit the device has no
// connection and the fake creates nothing; after a restart the counters show the messages.

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/desktop/providers.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/notifications/providers.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/sidebar/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/desktop_fakes.dart';
import '../support/fake_instance.dart';

/// Another device of Alex, the one a read comes from.
const String otherDevice = '01890a5d-ac96-774b-bcce-b302099a8d01';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late FakeWindow window;
  late RecordingPresenter presenter;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    window = FakeWindow();
    presenter = RecordingPresenter();
    fake = await FakeInstance.start(
      platform: 'windows',
      window: window,
      notifications: presenter,
    );
    c = appContainer(fake);
    final control = await connected(c, fake);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
    await c.read(desktopShellProvider.future);
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  test(
    'hidden: one toast with title and text; a click opens the thread',
    () async {
      final targets = <String>[];
      final sub = c
          .read(desktopNotificationsProvider)
          .targets
          .listen(targets.add);
      final world = fake.server.world;
      final seed = fake.seed;
      world
        ..notifyOnMessages = true
        ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all');

      window.send(DesktopWindowEvent.close);
      await eventually('hidden', () => window.calls.contains('hide'));
      final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');
      await eventually('a toast', () => presenter.shown.isNotEmpty);
      final toast = presenter.shown.single;
      expect(toast.content.title, 'general · Release notes');
      expect(toast.content.body, 'Sam: Build is green');
      expect(toast.silent, isFalse);
      expect(toast.payload, '/t/${seed.releaseNotes}?m=${message['id']}');

      presenter.taps.add(toast.payload);
      await eventually('the click', () => targets.isNotEmpty);
      expect(targets, [toast.payload]);
      await sub.cancel();
    },
  );

  test('read on another device: the notification of the thread is removed, no '
      'other', () async {
    c.read(desktopNotificationsProvider);
    final world = fake.server.world;
    final seed = fake.seed;
    world
      ..notifyOnMessages = true
      ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all');

    window.send(DesktopWindowEvent.close);
    await eventually('hidden', () => window.calls.contains('hide'));
    final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');
    await eventually('a toast', () => presenter.shown.isNotEmpty);
    final tag = presenter.shown.single.tag;
    expect(tag, 'thread:${seed.releaseNotes}');
    expect(presenter.cancelled, isEmpty, reason: 'unread: it stays');

    // Alex reads another thread on the phone: the notification of this one stays.
    world.putReadState(seed.alex.id, seed.printer, 0, deviceId: otherDevice);
    // Then the thread of the notification, up to the message: `read_state.updated` reaches
    // this app through the gateway, and the notification goes (01 10.3).
    world.putReadState(
      seed.alex.id,
      seed.releaseNotes,
      message['thread_seq']! as int,
      deviceId: otherDevice,
    );
    await eventually('the removal', () => presenter.cancelled.isNotEmpty);
    expect(presenter.cancelled, [tag]);
  });

  /// Alex has read everything in the thread and hides the window; then the hint of Alex's run
  /// that waits for an answer arrives without a message (13 2.1: the agent wrote nothing yet),
  /// so the toast shows while nothing is unread. The fake has no runs, so the hint is sent as
  /// the contract names it. Returns the tag of the toast and Sam's older message.
  Future<({String tag, JsonMap old, List<ThreadId> counted})>
  waitingRun() async {
    c.read(desktopNotificationsProvider);
    final world = fake.server.world;
    final seed = fake.seed;
    final engine = c.read(syncEngineProvider)!;
    final thread = ThreadId.parse(seed.releaseNotes);
    final device = c
        .read(signedInServicesProvider)!
        .auth
        .session!
        .deviceId
        .value;

    final old = seed.post(seed.sam, seed.releaseNotes, 'The build is gren');
    await eventually(
      'the message counted',
      () => engine.catchUp.countsOf(thread).unread == 1,
    );
    world.putReadState(
      seed.alex.id,
      seed.releaseNotes,
      old['thread_seq']! as int,
      deviceId: device,
    );
    await eventually(
      'the read',
      () => engine.catchUp.countsOf(thread).unread == 0,
    );
    seed.post(seed.alex, seed.releaseNotes, 'Please deploy the build');

    window.send(DesktopWindowEvent.close);
    await eventually('hidden', () => window.calls.contains('hide'));
    world.publishEphemeral(
      'notification.created',
      resource: {'kind': 'thread', 'id': seed.releaseNotes},
      scope: {
        'channel_id': world.threads[seed.releaseNotes]!['channel_id'],
        'thread_id': seed.releaseNotes,
      },
      payload: {
        'nid': world.nextId(),
        'kind': 'input_needed',
        'thread_id': seed.releaseNotes,
        'quiet': false,
        'count': 1,
      },
      audience: seed.alex.id,
      devices: {device},
    );
    await eventually('a toast', () => presenter.shown.isNotEmpty);
    final tag = presenter.shown.single.tag;
    expect(tag, 'thread:${seed.releaseNotes}');
    expect(engine.catchUp.countsOf(thread).unread, 0);
    expect(presenter.cancelled, isEmpty);
    // The count changes of the thread from here on, as the provider receives them before.
    final counted = <ThreadId>[];
    final changes = engine.catchUp.countChanges
        .where((t) => t == thread)
        .listen(counted.add);
    addTearDown(changes.cancel);
    return (tag: tag, old: old, counted: counted);
  }

  test('a waiting run without anything unread keeps its notification when '
      'another person edits a message of the thread; a later read removes '
      'it', () async {
    final world = fake.server.world;
    final seed = fake.seed;
    final run = await waitingRun();
    final read = Map.of(
      world.readStates['${seed.alex.id}|${seed.releaseNotes}']!,
    );

    // Sam fixes a typo in the old message: the counts of the thread are reported again, and
    // Alex read nothing.
    world.updateMessage(
      seed.sam.id,
      run.old['id']! as String,
      run.old['version']! as int,
      {'content_md': 'The build is green'},
    );
    await eventually('the edit counted', () => run.counted.isNotEmpty);
    await Future<void>.delayed(Duration.zero);
    expect(world.readStates['${seed.alex.id}|${seed.releaseNotes}'], read);
    expect(
      presenter.cancelled,
      isEmpty,
      reason: 'nobody read anything: the question of the run stays',
    );

    // Sam writes, and Alex reads the thread on the phone: now it is a read.
    final next = seed.post(seed.sam, seed.releaseNotes, 'Deploy it at noon');
    world.putReadState(
      seed.alex.id,
      seed.releaseNotes,
      next['thread_seq']! as int,
      deviceId: otherDevice,
    );
    await eventually('the removal', () => presenter.cancelled.isNotEmpty);
    expect(presenter.cancelled, [run.tag]);
  });

  test('a waiting run without anything unread keeps its notification through '
      'a new identify', () async {
    final world = fake.server.world;
    final seed = fake.seed;
    final run = await waitingRun();
    final engine = c.read(syncEngineProvider)!;
    final read = Map.of(
      world.readStates['${seed.alex.id}|${seed.releaseNotes}']!,
    );

    // The gateway session is gone, as after a sleep of the computer: the engine identifies
    // again, and the snapshot of `ready` reports the counts of every thread again (04 7.3).
    fake.server.gateway.closeAll(4006, 'session invalid');
    await eventually('the snapshot counted', () => run.counted.isNotEmpty);
    await eventually(
      'live again',
      () => engine.state.phase == ConnectionPhase.live,
    );
    await Future<void>.delayed(Duration.zero);
    expect(world.readStates['${seed.alex.id}|${seed.releaseNotes}'], read);
    expect(
      presenter.cancelled,
      isEmpty,
      reason: 'nobody read anything: the question of the run stays',
    );
  });

  test('an unread message keeps its notification through a new identify; the '
      'read after it removes it', () async {
    c.read(desktopNotificationsProvider);
    final world = fake.server.world;
    final seed = fake.seed;
    final engine = c.read(syncEngineProvider)!;
    final thread = ThreadId.parse(seed.releaseNotes);
    world
      ..notifyOnMessages = true
      ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all');

    window.send(DesktopWindowEvent.close);
    await eventually('hidden', () => window.calls.contains('hide'));
    final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');
    await eventually('a toast', () => presenter.shown.isNotEmpty);
    final tag = presenter.shown.single.tag;
    final counted = <ThreadId>[];
    final changes = engine.catchUp.countChanges
        .where((t) => t == thread)
        .listen(counted.add);
    addTearDown(changes.cancel);

    fake.server.gateway.closeAll(4006, 'session invalid');
    await eventually('the snapshot counted', () => counted.isNotEmpty);
    await eventually(
      'live again',
      () => engine.state.phase == ConnectionPhase.live,
    );
    expect(engine.catchUp.countsOf(thread).unread, 1);
    expect(presenter.cancelled, isEmpty, reason: 'unread: it stays');

    world.putReadState(
      seed.alex.id,
      seed.releaseNotes,
      message['thread_seq']! as int,
      deviceId: otherDevice,
    );
    await eventually('the removal', () => presenter.cancelled.isNotEmpty);
    expect(presenter.cancelled, [tag]);
  });

  test('after Quit: no connection of the device and no notification', () async {
    final world = fake.server.world;
    final seed = fake.seed;
    world
      ..notifyOnMessages = true
      ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all');
    c.read(desktopNotificationsProvider);
    final device = c
        .read(signedInServicesProvider)!
        .auth
        .session!
        .deviceId
        .value;

    await c.read(windowBehaviorProvider).quit();
    expect(window.calls.last, 'destroy');
    await eventually(
      'no connection of the device',
      () => !fake.server.gateway.connections.any(
        (conn) => conn.open && conn.deviceId == device,
      ),
    );
    seed.post(seed.sam, seed.releaseNotes, 'Deploy is done');
    expect(
      world.notifications.where((n) => n.recipient == seed.alex.id),
      isEmpty,
    );
    expect(world.notificationSkips.where((s) => s.userId == seed.alex.id), [
      (userId: seed.alex.id, reason: 'no_route'),
    ]);
    await Future<void>.delayed(const Duration(milliseconds: 100));
    expect(presenter.shown, isEmpty);
  });

  test('a restart after Quit shows both messages in the counters', () async {
    final seed = fake.seed;
    seed.post(seed.sam, seed.releaseNotes, 'Build is green');
    final general = ChannelId.parse(seed.general);
    await eventually(
      'the live message counted',
      () =>
          c
              .read(syncEngineProvider)!
              .catchUp
              .countsOfChannel(general)
              .values
              .firstOrNull
              ?.unread ==
          1,
    );
    await c.read(windowBehaviorProvider).quit();
    seed.post(seed.sam, seed.releaseNotes, 'Deploy is done');

    // A new start of the app on the same data within the resume window (04 6.3 item 7): the
    // new engine identifies, and the snapshot of `ready` seeds the counters (04 14).
    final restarted = appContainer(fake);
    addTearDown(restarted.dispose);
    restarted.read(onboardingControllerProvider);
    final rows = restarted.listen(
      channelsOfSpaceProvider(seed.space),
      (_, _) {},
    );
    addTearDown(rows.close);
    await eventually(
      'two unread in general',
      () => restarted
          .read(channelsOfSpaceProvider(seed.space))
          .any((row) => row.channel.id == general && row.counts.unread == 2),
    );
  });
}
