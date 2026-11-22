// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  final window = AppConfig.defaults().messageGroupWindow;

  test('same author shortly after continues the group', () {
    expect(
      isContinuation(
        message(1),
        message(2, createdAt: t0.add(const Duration(minutes: 1))),
        window,
      ),
      isTrue,
    );
  });

  test('text and voice are one kind class', () {
    expect(
      isContinuation(message(1), message(2, kind: MessageKind.voice), window),
      isTrue,
    );
  });

  test('another author, another kind class, a system line or the window break the group', () {
    expect(
      isContinuation(message(1), message(2, author: robin), window),
      isFalse,
    );
    expect(
      isContinuation(message(1), message(2, kind: MessageKind.card), window),
      isFalse,
    );
    expect(
      isContinuation(
        message(1, kind: MessageKind.run),
        message(2, kind: MessageKind.card),
        window,
      ),
      isFalse,
    );
    expect(
      isContinuation(message(1), message(2, kind: MessageKind.system), window),
      isFalse,
    );
    expect(
      isContinuation(message(1, kind: MessageKind.system), message(2), window),
      isFalse,
    );
    expect(
      isContinuation(message(1), message(2, createdAt: t0.add(window)), window),
      isFalse,
    );
    expect(
      isContinuation(
        message(1),
        message(2, createdAt: t0.subtract(const Duration(seconds: 1))),
        window,
      ),
      isFalse,
    );
  });

  test('two team members of one agent never share a group', () {
    // FIX-TEAM-MEMBER-AUTHOR-DISPLAY: the mirrors of two members are messages of the same
    // agent; each member keeps its own header.
    final coding = message(1).copyWith(teamHandle: 'codingagent');
    expect(
      isContinuation(
        coding,
        message(2).copyWith(teamHandle: 'reviewagent'),
        window,
      ),
      isFalse,
    );
    expect(isContinuation(message(1), coding, window), isFalse);
    expect(isContinuation(coding, message(2), window), isFalse);
    expect(
      isContinuation(
        coding,
        message(2).copyWith(teamHandle: 'codingagent'),
        window,
      ),
      isTrue,
    );
  });

  test('unknown kinds group only with unknown kinds', () {
    expect(
      isContinuation(
        message(1, kind: MessageKind.unknown),
        message(2, kind: MessageKind.unknown),
        window,
      ),
      isTrue,
    );
  });
}
