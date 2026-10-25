// SPDX-License-Identifier: Apache-2.0

// Focus (04 11.3 item 2) and typing (04 12 item 1) with their limits from AppConfig (brief
// M4-13, Tests).

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

void main() {
  group('FocusManager', () {
    late List<GatewayFrame> sent;
    late bool open;
    late FocusManager focus;

    setUp(() {
      sent = [];
      open = true;
      focus = FocusManager(
        limit: 3,
        trySend: (f) {
          if (!open) return false;
          sent.add(f);
          return true;
        },
      );
    });

    List<String> idsOf(GatewayFrame f) =>
        (f as FocusFrame).model.threadIds.toList();

    test('more IDs than the limit are cut, the most recent win', () async {
      var dropped = focus.setVisible({thread(1), thread(2)}, idle: false);
      expect(dropped, isEmpty);
      dropped = focus.setVisible({
        thread(1),
        thread(2),
        thread(3),
        thread(4),
        thread(5),
      }, idle: false);
      expect(dropped, {thread(1), thread(2)});
      await settle();
      expect(sent, hasLength(1), reason: 'one frame per microtask');
      expect(idsOf(sent.single).toSet(), {
        thread(3).value,
        thread(4).value,
        thread(5).value,
      });
      expect((sent.single as FocusFrame).model.idle, isFalse);
    });

    test('one frame per change, none without a change', () async {
      focus.setVisible({thread(1)}, idle: false);
      await settle();
      focus.setVisible({thread(1)}, idle: false);
      await settle();
      expect(sent, hasLength(1));
      focus.setVisible({thread(1)}, idle: true);
      await settle();
      expect(sent, hasLength(2));
      expect((sent.last as FocusFrame).model.idle, isTrue);
    });

    test('a rejected focus keeps the previous set', () async {
      focus.setVisible({thread(1)}, idle: false);
      await settle();
      focus.setVisible({thread(1), thread(2)}, idle: false);
      await settle();
      focus.onRejected();
      expect(focus.current, {thread(1)});
      focus.setVisible({thread(1), thread(2)}, idle: false);
      await settle();
      expect(sent, hasLength(3), reason: 'the change is sent again');
    });

    List<String>? backgroundOf(GatewayFrame f) =>
        (f as FocusFrame).model.backgroundThreadIds?.toList();

    test('background threads share the limit after the visible ones (04 6.2, '
        'R-SY-27)', () async {
      focus.setVisible(
        {thread(1)},
        idle: false,
        background: [thread(1), thread(2), thread(3), thread(4)],
      );
      await settle();
      final frame = sent.single as FocusFrame;
      expect(idsOf(frame), [thread(1).value]);
      // A thread in both lists stays in thread_ids only; the limit of 3 leaves two places.
      expect(backgroundOf(frame), [thread(2).value, thread(3).value]);
      expect(frame.model.idle, isFalse, reason: 'cards never change idle');
      expect(focus.grantedBackground, {thread(1), thread(2), thread(3)});
    });

    test('without background threads the member is left out; a change of the '
        'background list alone sends a frame', () async {
      focus.setVisible({thread(1)}, idle: true);
      await settle();
      expect(backgroundOf(sent.single), isNull);
      focus.setVisible({thread(1)}, idle: true, background: [thread(2)]);
      await settle();
      expect(sent, hasLength(2));
      expect(backgroundOf(sent.last), [thread(2).value]);
      expect((sent.last as FocusFrame).model.idle, isTrue);
      focus.setVisible({thread(1)}, idle: true, background: [thread(2)]);
      await settle();
      expect(sent, hasLength(2), reason: 'no change, no frame');
    });

    test('while not identified nothing is sent; ready sends the set', () async {
      open = false;
      focus.setVisible({thread(1)}, idle: false);
      await settle();
      expect(sent, isEmpty);
      open = true;
      focus.onReady();
      expect(idsOf(sent.single), [thread(1).value]);
      focus.onResumed();
      expect(sent, hasLength(1), reason: 'the server keeps the set on resume');
    });
  });

  test(
    'the connection re-sends focus after ready and cuts at the limit',
    () async {
      final h = Harness(overrides: {'sync.max_focused_threads': '2'});
      addTearDown(h.dispose);
      final dropped = h.connection.focus.setVisible({
        thread(1),
        thread(2),
        thread(3),
      }, idle: false);
      expect(dropped, {thread(1)});
      final ch = await h.goLive();
      expect(ch.sentOps, ['identify', 'focus']);
      expect(ch.lastData('focus'), {
        'thread_ids': [thread(2).value, thread(3).value],
        'idle': false,
      });
      ch.push('error', {'op': 'focus', 'code': 'validation_failed'});
      await settle();
      expect(h.connection.focus.current, isEmpty);
    },
  );

  group('TypingThrottle', () {
    late Harness h;
    late List<GatewayFrame> sent;
    late TypingThrottle typing;

    setUp(() {
      h = Harness();
      sent = [];
      typing = TypingThrottle(
        clock: h.clock,
        interval: h.config.typingMinInterval,
        trySend: (f) {
          sent.add(f);
          return true;
        },
      );
    });
    tearDown(() => h.dispose());

    test('typing.start at most once per interval', () async {
      typing.onInput(thread(1));
      typing.onInput(thread(1));
      await h.clock.elapse(const Duration(seconds: 4));
      typing.onInput(thread(1));
      expect(sent.map((f) => f.op), ['typing.start']);
      await h.clock.elapse(const Duration(seconds: 1));
      typing.onInput(thread(1));
      expect(sent.map((f) => f.op), ['typing.start', 'typing.start']);
    });

    test('typing.stop resets the throttle', () async {
      typing.onInput(thread(1));
      typing.onSent(thread(1));
      typing.onInput(thread(1));
      typing.onCleared(thread(1));
      expect(sent.map((f) => f.op), [
        'typing.start',
        'typing.stop',
        'typing.start',
        'typing.stop',
      ]);
      expect((sent.first as TypingStartFrame).model.threadId, thread(1).value);
    });

    test('no typing.stop without a start', () {
      typing.onCleared(thread(1));
      expect(sent, isEmpty);
    });
  });

  test('the connection sends typing only while identified', () async {
    final h = Harness();
    addTearDown(h.dispose);
    h.connection.typing.onInput(thread(1));
    final ch = await h.goLive();
    h.connection.typing.onInput(thread(1));
    expect(ch.sentOps, ['identify', 'typing.start']);
    expect(ch.lastData('typing.start'), {'thread_id': thread(1).value});
  });
}
