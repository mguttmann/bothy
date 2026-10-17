// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

void main() {
  group('Uuid7Generator', () {
    late int now;
    late int seed;
    late Uuid7Generator gen;

    setUp(() {
      now = 0x0192f4a07b1c;
      seed = 7;
      gen = Uuid7Generator(
        nowMillis: () => now,
        random: (bits) {
          seed = (seed * 1103515245 + 12345) & 0x7fffffff;
          return seed & ((1 << bits) - 1);
        },
      );
    });

    test('sets version 7 and variant 10', () {
      final id = gen.next();
      expect(
        id,
        matches(
          RegExp(
            r'^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
          ),
        ),
      );
      expect(id.substring(0, 13), '0192f4a0-7b1c');
    });

    test('1000 IDs in the same millisecond sort strictly increasing', () {
      final ids = [for (var i = 0; i < 1000; i++) gen.next()];
      for (var i = 1; i < ids.length; i++) {
        expect(ids[i].compareTo(ids[i - 1]), greaterThan(0), reason: 'at $i');
      }
    });

    test('the timestamp advances on counter overflow', () {
      final first = gen.next();
      for (var i = 0; i < 4096; i++) {
        gen.next();
      }
      final later = gen.next();
      expect(later.substring(0, 13), isNot(first.substring(0, 13)));
      expect(later.compareTo(first), greaterThan(0));
    });

    test('a clock going backwards never lowers the ID', () {
      final first = gen.next();
      now -= 1000;
      expect(gen.next().compareTo(first), greaterThan(0));
    });

    test('a new millisecond starts after the previous one', () {
      final first = gen.next();
      now += 1;
      final second = gen.next();
      expect(second.compareTo(first), greaterThan(0));
      expect(second.substring(9, 13), '7b1d');
    });

    test('rejects a clock outside 48 bits', () {
      now = -1;
      expect(gen.next, throwsStateError);
    });

    test('the output parses as a message ID', () {
      expect(MessageId.parse(gen.next()).value, hasLength(36));
    });
  });

  group('typed IDs', () {
    const valid = '0192F4A0-7B1C-78F9-8074-FA94E84BAB7D';

    test('parse accepts either case and lowercases', () {
      expect(ThreadId.parse(valid).value, valid.toLowerCase());
      expect(SpaceId.parse(valid.toLowerCase()).value, valid.toLowerCase());
    });

    test('parse rejects 35 and 37 characters and braces without echoing', () {
      for (final bad in [
        valid.substring(1),
        '${valid}0',
        '{$valid}',
        'not-a-uuid',
      ]) {
        expect(
          () => MessageId.parse(bad),
          throwsA(
            isA<FormatException>()
                .having((e) => e.message, 'message', isNot(contains(bad)))
                .having((e) => e.message, 'message', contains('${bad.length}')),
          ),
        );
      }
    });

    test('every ID kind parses', () {
      final ids = <Object>[
        SpaceId.parse(valid),
        ChannelId.parse(valid),
        ThreadId.parse(valid),
        MessageId.parse(valid),
        RunId.parse(valid),
        PrincipalId.parse(valid),
        FileId.parse(valid),
        DeviceId.parse(valid),
        HostId.parse(valid),
        TerminalSessionId.parse(valid),
        ComponentId.parse(valid),
        CommandId.parse(valid),
        ForumTagId.parse(valid),
        DraftId.parse(valid),
        QuietHoursId.parse(valid),
      ];
      expect(ids.toSet(), hasLength(1));
    });
  });

  group('sequence types', () {
    test('compare by value', () {
      expect(const GlobalSeq(2) > const GlobalSeq(1), isTrue);
      expect(const GlobalSeq(1) < const GlobalSeq(2), isTrue);
      expect(const GlobalSeq(2) >= const GlobalSeq(2), isTrue);
      expect(const ThreadSeq(3) <= const ThreadSeq(3), isTrue);
      expect(const ThreadSeq(3) >= const ThreadSeq(4), isFalse);
      expect(const ThreadSeq(3).next, const ThreadSeq(4));
      expect(const ResourceVersion(1) < const ResourceVersion(2), isTrue);
      expect(const ResourceVersion(2) <= const ResourceVersion(1), isFalse);
      expect(const ResourceVersion(2) >= const ResourceVersion(1), isTrue);
      expect(const PermissionVersion(2) > const PermissionVersion(1), isTrue);
      expect(const PermissionVersion(2) < const PermissionVersion(1), isFalse);
    });
  });
}
