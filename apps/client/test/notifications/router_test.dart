// SPDX-License-Identifier: Apache-2.0

// From `notification.created` to a toast or nothing (brief M4-30 Tests and Design note 1): the
// thread that is open in the focused window gets no toast, a quiet notification is silent, the
// same thread replaces its slot, the kinds without a thread use a fixed slot per kind, a click
// navigates to the thread at the message, and a thread the person read loses its notification,
// while counts that are reported without a read remove none (01 10.3;
// docs/decisions.d/AUD-client-notifications-01.md AUD-CN1/3).

import 'package:client/notifications/in_app_banner.dart';
import 'package:client/notifications/notification_router.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/desktop_fakes.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

const String t1 = '01890a5d-ac96-774b-bcce-b302099a8057';
const String t2 = '01890a5d-ac96-774b-bcce-b302099a8157';
const String m1 = '01890a5d-ac96-774b-bcce-b302099a8058';

NotificationCreated hint({
  String? thread = t1,
  String? message = m1,
  HintKind kind = HintKind.message,
  bool quiet = false,
  int count = 1,
}) => NotificationCreated(
  nid: '01890a5d-ac96-774b-bcce-b302099a8059',
  kind: kind,
  threadId: thread == null ? null : ThreadId.parse(thread),
  messageId: message == null ? null : MessageId.parse(message),
  quiet: quiet,
  count: count,
  channelName: 'general',
  threadTitle: 'Release notes',
  senderDisplayName: 'Sam',
  excerpt: 'Build is green',
);

void main() {
  late RecordingPresenter presenter;
  late Set<ThreadId> openFocused;
  late Map<ThreadId, int> unread;
  late List<String> paths;
  late List<String> logs;
  late NotificationRouter router;

  setUp(() {
    presenter = RecordingPresenter();
    openFocused = {};
    unread = {};
    paths = [];
    logs = [];
    router = NotificationRouter(
      presenter: presenter,
      texts: () => en,
      appName: () => 'ExampleChat',
      isOpenAndFocused: openFocused.contains,
      unreadOf: (thread) => unread[thread] ?? 0,
      navigate: paths.add,
      localChannelName: (_) async => 'general',
      log: logs.add,
    );
  });
  tearDown(() => presenter.dispose());

  test('the thread open in the focused window gets no toast', () async {
    openFocused.add(ThreadId.parse(t1));
    await router.onEvent(hint());
    expect(presenter.shown, isEmpty);
    await router.onEvent(hint(thread: t2));
    expect(presenter.shown, hasLength(1), reason: 'another thread still shows');
  });

  test('a quiet notification is silent, a normal one is not', () async {
    await router.onEvent(hint(quiet: true));
    await router.onEvent(hint(thread: t2));
    expect(presenter.shown.map((s) => s.silent), [true, false]);
  });

  test('the same thread replaces its slot; threads have their own', () async {
    await router.onEvent(hint());
    await router.onEvent(hint(count: 2));
    await router.onEvent(hint(thread: t2));
    final tags = presenter.shown.map((s) => s.tag).toList();
    expect(tags[0], tags[1]);
    expect(tags[2], isNot(tags[0]));
    expect(presenter.shown[1].content.body, startsWith('2 new messages'));
  });

  test('a read thread loses its notification, once; a thread without one asks '
      'nothing', () async {
    final thread = ThreadId.parse(t1);
    final other = ThreadId.parse(t2);
    unread[thread] = 2;
    await router.onEvent(hint());
    await router.onCountChange(other);
    expect(presenter.cancelled, isEmpty, reason: 'nothing shown for it');
    // The counts are reported while a message is unread: an edit, a snapshot, a part read.
    await router.onCountChange(thread);
    unread[thread] = 1;
    await router.onCountChange(thread);
    expect(presenter.cancelled, isEmpty, reason: 'unread: it stays');
    unread[thread] = 0;
    await router.onCountChange(thread);
    expect(presenter.cancelled, ['thread:$t1']);
    await router.onCountChange(thread);
    expect(presenter.cancelled, hasLength(1), reason: 'the slot is empty');

    unread[thread] = 1;
    await router.onEvent(hint());
    unread[thread] = 0;
    await router.onCountChange(thread);
    expect(presenter.cancelled, hasLength(2), reason: 'a new notification');
  });

  test('a notification shown while nothing is unread stays when the counts are '
      'reported; the read of a later message removes it', () async {
    final thread = ThreadId.parse(t1);
    // A run that waits for the person before its agent wrote a message (13 2.1).
    await router.onEvent(hint(message: null, kind: HintKind.inputNeeded));
    expect(presenter.shown.single.tag, 'thread:$t1');
    // An edit by another person, a snapshot of `ready`: counts without a read.
    await router.onCountChange(thread);
    await router.onCountChange(thread);
    expect(presenter.cancelled, isEmpty, reason: 'nobody read anything');

    unread[thread] = 1;
    await router.onCountChange(thread);
    expect(presenter.cancelled, isEmpty, reason: 'a new message is no read');
    unread[thread] = 0;
    await router.onCountChange(thread);
    expect(presenter.cancelled, ['thread:$t1']);
  });

  test('a notification that replaces the one of a read thread starts '
      'again', () async {
    final thread = ThreadId.parse(t1);
    unread[thread] = 1;
    await router.onEvent(hint());
    unread[thread] = 0;
    await router.onCountChange(thread);
    expect(presenter.cancelled, hasLength(1));

    // The next hint of the thread shows while nothing is unread: it is not the read one.
    await router.onEvent(hint(message: null, kind: HintKind.inputNeeded));
    await router.onCountChange(thread);
    expect(presenter.cancelled, hasLength(1), reason: 'nobody read anything');
  });

  test('on a phone the banner of a read thread goes; the banner of another '
      'thread stays', () async {
    final banners = InAppBanners();
    final thread = ThreadId.parse(t1);
    final phone = NotificationRouter(
      presenter: banners,
      texts: () => en,
      appName: () => 'ExampleChat',
      isOpenAndFocused: openFocused.contains,
      unreadOf: (thread) => unread[thread] ?? 0,
      navigate: paths.add,
    );
    unread[thread] = 1;
    await phone.onEvent(hint());
    expect(banners.current.value?.tag, 'thread:$t1');
    await phone.onCountChange(thread);
    expect(banners.current.value?.tag, 'thread:$t1', reason: 'unread');
    unread[thread] = 0;
    await phone.onCountChange(thread);
    expect(banners.current.value, isNull);

    unread[thread] = 1;
    await phone.onEvent(hint());
    await phone.onEvent(hint(thread: t2));
    unread[thread] = 0;
    await phone.onCountChange(thread);
    expect(
      banners.current.value?.tag,
      'thread:$t2',
      reason: 'the banner on screen is of another thread',
    );
    await banners.dispose();
  });

  test(
    'the thread open in the focused window has no notification to remove',
    () async {
      final thread = ThreadId.parse(t1);
      openFocused.add(thread);
      unread[thread] = 1;
      await router.onEvent(hint());
      unread[thread] = 0;
      await router.onCountChange(thread);
      expect(presenter.cancelled, isEmpty);
    },
  );

  test(
    'kinds without a thread use a fixed slot and open their screen',
    () async {
      await router.onEvent(
        hint(thread: null, message: null, kind: HintKind.security),
      );
      await router.onEvent(
        hint(thread: null, message: null, kind: HintKind.security),
      );
      await router.onEvent(
        hint(thread: null, message: null, kind: HintKind.admin),
      );
      expect(presenter.shown[0].tag, presenter.shown[1].tag);
      expect(presenter.shown[2].tag, isNot(presenter.shown[0].tag));
      expect(presenter.shown[0].payload, '/settings/devices');
      expect(presenter.shown[2].payload, '/admin');
    },
  );

  test('an unknown kind without a thread shows nothing', () async {
    await router.onEvent(
      hint(thread: null, message: null, kind: HintKind.unknown),
    );
    expect(presenter.shown, isEmpty);
  });

  test('a click navigates to the thread at the message', () async {
    await router.onEvent(hint());
    expect(presenter.shown.single.payload, '/t/$t1?m=$m1');
    final sub = router.listen();
    presenter.taps.add(presenter.shown.single.payload);
    await Future<void>.delayed(Duration.zero);
    expect(paths, ['/t/$t1?m=$m1']);
    await sub.cancel();
  });

  test('a click that started the app navigates once at the start', () async {
    presenter.launch = '/t/$t1?m=$m1';
    final sub = router.listen();
    await Future<void>.delayed(Duration.zero);
    expect(paths, ['/t/$t1?m=$m1']);
    await sub.cancel();
  });

  test('a payload that is not an app path is ignored', () async {
    router.onClick('https://example.com/t/$t1');
    router.onClick('');
    expect(paths, isEmpty);
  });

  test('logs carry no content', () async {
    openFocused.add(ThreadId.parse(t1));
    await router.onEvent(hint());
    await router.onEvent(hint(thread: t2));
    for (final line in logs) {
      expect(line, isNot(contains('Sam')));
      expect(line, isNot(contains('Build')));
      expect(line, isNot(contains('general')));
    }
  });

  test('without a sink a logged step does not fail', () async {
    // The lines then go to the developer log of the app, which a test cannot read.
    final plain = NotificationRouter(
      presenter: presenter,
      texts: () => en,
      appName: () => 'ExampleChat',
      isOpenAndFocused: (_) => true,
      unreadOf: (_) => 0,
      navigate: paths.add,
    );
    await plain.onEvent(hint());
    plain.onClick('https://example.com/');
    expect(presenter.shown, isEmpty);
    expect(paths, isEmpty);
  });

  test('a failed lookup of the local channel name still shows the placeholder toast', () async {
    final failing = NotificationRouter(
      presenter: presenter,
      texts: () => en,
      appName: () => 'ExampleChat',
      isOpenAndFocused: openFocused.contains,
      unreadOf: (_) => 0,
      navigate: paths.add,
      localChannelName: (_) async => throw StateError('database closed'),
      log: logs.add,
    );
    await failing.onEvent(
      NotificationCreated(
        nid: '01890a5d-ac96-774b-bcce-b302099a8059',
        kind: HintKind.message,
        threadId: ThreadId.parse(t1),
        messageId: MessageId.parse(m1),
        quiet: false,
        count: 1,
      ),
    );
    expect(presenter.shown, hasLength(1));
    expect(logs.last, contains('no local channel name'));
  });
}
