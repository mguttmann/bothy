// SPDX-License-Identifier: Apache-2.0

// From `notification.created` to a toast or nothing (brief M4-30 Tests and Design note 1): the
// thread that is open in the focused window gets no toast, a quiet notification is silent, the
// same thread replaces its slot, the kinds without a thread use a fixed slot per kind, and a
// click navigates to the thread at the message.

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
  late List<String> paths;
  late List<String> logs;
  late NotificationRouter router;

  setUp(() {
    presenter = RecordingPresenter();
    openFocused = {};
    paths = [];
    logs = [];
    router = NotificationRouter(
      presenter: presenter,
      texts: () => en,
      appName: () => 'ExampleChat',
      isOpenAndFocused: openFocused.contains,
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

  test('a failed lookup of the local channel name still shows the placeholder toast', () async {
    final failing = NotificationRouter(
      presenter: presenter,
      texts: () => en,
      appName: () => 'ExampleChat',
      isOpenAndFocused: openFocused.contains,
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
