// SPDX-License-Identifier: Apache-2.0

import 'package:client/platform/guarded.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  late List<String> lines;

  setUp(() => lines = []);

  test('a step that passes runs and leaves no line', () async {
    var ran = false;
    await guarded(lines.add, 'push', 'settings', () async => ran = true);
    expect(ran, isTrue);
    expect(lines, isEmpty);
  });

  test('a failing step is swallowed and leaves one line with the area, the '
      'step and the type of the error, never its text', () async {
    await guarded(
      lines.add,
      'share',
      'session',
      () async => throw StateError('bothy_pvw_EXAMPLE'),
    );
    expect(lines, ['share: session failed (StateError)']);
  });

  test('a step that throws before its first await is swallowed too', () async {
    await guarded(lines.add, 'push', 'counters', () {
      throw const FormatException('keychain says no');
    });
    expect(lines, ['push: counters failed (FormatException)']);
  });

  test('the value of a step that passes, without a line', () async {
    expect(
      await guardedValue(lines.add, 'push', 'availability', () async => true),
      isTrue,
    );
    expect(lines, isEmpty);
  });

  test('null and the same line after a failing step with a value', () async {
    final value = await guardedValue<bool>(
      lines.add,
      'share',
      'availability',
      () async => throw StateError('bothy_pvw_EXAMPLE'),
    );
    expect(value, isNull);
    expect(lines, ['share: availability failed (StateError)']);
  });
}
