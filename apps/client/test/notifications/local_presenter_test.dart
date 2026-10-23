// SPDX-License-Identifier: Apache-2.0

// The `flutter_local_notifications` presenter of M4-30 over a recording plugin (brief M4-30
// Design note 2, D-M4-30-2): one initialization with the app name, the AUMID and the GUID; a
// slot per tag; quiet without sound; a click delivers the payload; without package identity no
// cancel; a failing plugin logs one line without content and shows nothing.

import 'package:client/notifications/notification_presenter.dart';
import 'package:client/notifications/toast_content.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:flutter_test/flutter_test.dart';

/// A plugin that records the calls the presenter makes.
class RecordingPlugin implements FlutterLocalNotificationsPlugin {
  final List<String> calls = [];
  InitializationSettings? settings;
  DidReceiveNotificationResponseCallback? onResponse;
  final List<
    ({
      int id,
      String? title,
      String? body,
      NotificationDetails? details,
      String? payload,
    })
  >
  shown = [];
  NotificationAppLaunchDetails? launch;
  bool failInit = false;
  bool failCalls = false;

  @override
  Future<bool?> initialize({
    required InitializationSettings settings,
    DidReceiveNotificationResponseCallback? onDidReceiveNotificationResponse,
    DidReceiveBackgroundNotificationResponseCallback?
    onDidReceiveBackgroundNotificationResponse,
  }) async {
    calls.add('initialize');
    if (failInit) throw StateError('no plugin');
    this.settings = settings;
    onResponse = onDidReceiveNotificationResponse;
    return true;
  }

  @override
  Future<void> show({
    required int id,
    String? title,
    String? body,
    NotificationDetails? notificationDetails,
    String? payload,
  }) async {
    calls.add('show');
    if (failCalls) throw StateError('show failed');
    shown.add((
      id: id,
      title: title,
      body: body,
      details: notificationDetails,
      payload: payload,
    ));
  }

  @override
  Future<void> cancel({required int id, String? tag}) async {
    calls.add('cancel $id');
    if (failCalls) throw StateError('cancel failed');
  }

  @override
  Future<NotificationAppLaunchDetails?>
  getNotificationAppLaunchDetails() async {
    calls.add('launch');
    if (failCalls) throw StateError('launch failed');
    return launch;
  }

  @override
  T? resolvePlatformSpecificImplementation<
    T extends FlutterLocalNotificationsPlatform
  >() => null;

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnimplementedError(invocation.memberName.toString());
}

const ToastContent toast = ToastContent(
  title: 'general · Release notes',
  body: 'Sam: Build is green',
);

void main() {
  late RecordingPlugin plugin;
  late List<String> logs;

  LocalNotificationPresenter presenter({bool identity = true}) =>
      LocalNotificationPresenter(
        appName: 'ExampleChat',
        appUserModelId: 'com.example.chat',
        guid: LocalNotificationPresenter.guidFor('com.example.chat'),
        log: logs.add,
        plugin: plugin,
        hasPackageIdentity: identity,
      );

  setUp(() {
    plugin = RecordingPlugin();
    logs = [];
  });

  test('the GUID is stable per bundle and shaped like a name-based UUID', () {
    final a = LocalNotificationPresenter.guidFor('com.example.chat');
    expect(a, LocalNotificationPresenter.guidFor('com.example.chat'));
    expect(a, isNot(LocalNotificationPresenter.guidFor('com.example.other')));
    expect(
      a,
      matches(
        RegExp(
          r'^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
        ),
      ),
    );
  });

  test('the slot ID is the same per tag and fits 31 bits', () {
    final id = LocalNotificationPresenter.idOf('thread:t1');
    expect(id, LocalNotificationPresenter.idOf('thread:t1'));
    expect(id, isNot(LocalNotificationPresenter.idOf('thread:t2')));
    expect(id, inInclusiveRange(0, 0x7fffffff));
  });

  test('initializes once with the branding and shows per slot', () async {
    final p = presenter();
    await p.show(toast, tag: 'thread:t1', silent: false, payload: '/t/t1');
    await p.show(toast, tag: 'thread:t1', silent: true, payload: '/t/t1');
    expect(plugin.calls.where((c) => c == 'initialize'), hasLength(1));
    final windows = plugin.settings!.windows!;
    expect(windows.appName, 'ExampleChat');
    expect(windows.appUserModelId, 'com.example.chat');
    expect(plugin.shown, hasLength(2));
    expect(plugin.shown.first.id, plugin.shown.last.id, reason: 'one slot');
    expect(plugin.shown.first.title, toast.title);
    expect(plugin.shown.first.body, toast.body);
    expect(plugin.shown.first.payload, '/t/t1');
    expect(plugin.shown.first.details!.macOS!.presentSound, isTrue);
    expect(plugin.shown.last.details!.macOS!.presentSound, isFalse);
    expect(plugin.shown.last.details!.windows!.audio, isNotNull);
    await p.dispose();
  });

  test(
    'a click delivers the payload; the launch payload is read once',
    () async {
      final p = presenter();
      final clicks = <String>[];
      final sub = p.clicks.listen(clicks.add);
      plugin.launch = const NotificationAppLaunchDetails(
        true,
        notificationResponse: NotificationResponse(
          notificationResponseType:
              NotificationResponseType.selectedNotification,
          payload: '/t/t1?m=m1',
        ),
      );
      expect(await p.launchPayload(), '/t/t1?m=m1');
      plugin.onResponse!(
        const NotificationResponse(
          notificationResponseType:
              NotificationResponseType.selectedNotification,
          payload: '/t/t2',
        ),
      );
      await pumpEventQueue();
      expect(clicks, ['/t/t2']);
      plugin.launch = const NotificationAppLaunchDetails(false);
      expect(await p.launchPayload(), isNull);
      await sub.cancel();
      await p.dispose();
    },
  );

  test('cancel only with package identity', () async {
    final with_ = presenter();
    await with_.cancelThread('thread:t1');
    expect(
      plugin.calls,
      contains('cancel ${LocalNotificationPresenter.idOf('thread:t1')}'),
    );
    await with_.dispose();

    plugin = RecordingPlugin();
    final without = presenter(identity: false);
    await without.cancelThread('thread:t1');
    expect(plugin.calls.where((c) => c.startsWith('cancel')), isEmpty);
    expect(
      logs,
      contains('notifications: cancel skipped without package identity'),
    );
    await without.dispose();
  });

  test('a failing plugin logs without content and shows nothing', () async {
    plugin.failInit = true;
    final broken = presenter();
    await broken.show(toast, tag: 'thread:t1', silent: false, payload: '/t/t1');
    await broken.cancelThread('thread:t1');
    expect(await broken.launchPayload(), isNull);
    expect(plugin.shown, isEmpty);
    expect(logs, ['notifications: initialize failed (StateError)']);
    await broken.dispose();

    plugin = RecordingPlugin()..failCalls = true;
    logs.clear();
    final failing = presenter();
    await failing.show(
      toast,
      tag: 'thread:t1',
      silent: false,
      payload: '/t/t1',
    );
    await failing.cancelThread('thread:t1');
    expect(await failing.launchPayload(), isNull);
    expect(logs, [
      'notifications: show failed (StateError)',
      'notifications: cancel failed (StateError)',
      'notifications: launch details failed (StateError)',
    ]);
    for (final line in logs) {
      expect(line, isNot(contains('Build is green')));
    }
    await failing.dispose();
  });

  test('the no-op presenter does nothing', () async {
    const p = NoNotificationPresenter();
    await p.show(toast, tag: 't', silent: false, payload: '/');
    await p.cancelThread('t');
    expect(await p.launchPayload(), isNull);
    expect(await p.clicks.isEmpty, isTrue);
    await p.dispose();
  });
}
