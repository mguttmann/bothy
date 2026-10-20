// SPDX-License-Identifier: Apache-2.0

// TM-T14: the escape sequence locks of docs/spec/24-terminal.md 11.2 (SEC-TERM-6) hold in
// terminal_ui in front of the renderer (docs/spec/15-client-architektur.md 23.4). Every case runs
// in one call, split at every byte position across two calls and byte by byte, because the
// host output arrives in frames that may cut a sequence anywhere (allow list rule 7).

import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/src/render/escape_filter.dart';
import 'package:xterm/core.dart';

/// The bytes of [s]; `\x1b` and the other controls are single bytes, the rest is UTF-8.
List<int> b(String s) => utf8.encode(s);

/// A readable form of [chunks]: adjacent text is merged (also across calls), text is `T:` plus
/// one code unit per byte, a link start is `S:` plus the target, a link end is `E`.
List<String> describe(List<FilteredChunk> chunks) {
  final out = <String>[];
  final text = <int>[];
  void flush() {
    if (text.isNotEmpty) {
      out.add('T:${String.fromCharCodes(text)}');
      text.clear();
    }
  }

  for (final chunk in chunks) {
    switch (chunk) {
      case FilteredText(:final bytes):
        text.addAll(bytes);
      case LinkStart(:final target):
        flush();
        out.add('S:$target');
      case LinkEnd():
        flush();
        out.add('E');
    }
  }
  flush();
  return out;
}

/// The text form of [s] as [describe] writes it (one code unit per byte).
String t(String s) => 'T:${String.fromCharCodes(b(s))}';

/// Feeds [parts] to one fresh filter, one process() call per part.
List<FilteredChunk> feedRaw(List<List<int>> parts) {
  final filter = EscapeFilter();
  return [for (final part in parts) ...filter.process(part)];
}

/// [describe] of [feedRaw].
List<String> feed(List<List<int>> parts) => describe(feedRaw(parts));

/// Checks that [input] gives [expected] in one call, split at every byte position across two
/// calls, and fed byte by byte.
void expectAllSplits(List<int> input, List<String> expected) {
  expect(feed([input]), expected, reason: 'one call');
  for (var i = 0; i <= input.length; i++) {
    expect(
      feed([input.sublist(0, i), input.sublist(i)]),
      expected,
      reason: 'split at $i',
    );
  }
  expect(
    feed([
      for (final byte in input) [byte],
    ]),
    expected,
    reason: 'byte by byte',
  );
}

/// The renderer side of TM-T14: what xterm 4.0.0 does with the filtered bytes.
final class XtermProbe {
  /// Creates a fresh 80x24 xterm terminal that records every side effect.
  XtermProbe() {
    terminal = Terminal(
      onOutput: replies.add,
      onTitleChange: (title) => sideEffects.add('title'),
      onIconChange: (icon) => sideEffects.add('icon'),
      onResize: (w, h, pw, ph) => sideEffects.add('resize'),
      onPrivateOSC: (code, args) => sideEffects.add('osc $code'),
    );
  }

  /// The terminal under test.
  late final Terminal terminal;

  /// Every answer xterm sends back to the host.
  final List<String> replies = [];

  /// Title, icon, resize and private OSC calls; must stay empty.
  final List<String> sideEffects = [];

  /// Writes the text bytes of [chunks] as the view would: decoded UTF-8.
  void write(List<FilteredChunk> chunks) {
    for (final chunk in chunks) {
      if (chunk is FilteredText) {
        terminal.write(utf8.decode(chunk.bytes, allowMalformed: true));
      }
    }
  }
}

/// The only answers allowed through: CPR, DA1 and DA2 as xterm 4.0.0 writes them.
final allowedReply = RegExp(r'^\x1b\[(\?1;2c|>0;0;0c|\d+;\d+R)$');

/// Sequences that must reach the renderer byte-identical.
const passing = <String>[
  '\x1b[1;31m',
  '\x1b[38;5;196m',
  '\x1b[38;2;1;2;3m',
  '\x1b[m',
  '\x1b[0m',
  '\x1b[10;20H',
  '\x1b[A',
  '\x1b[5B',
  '\x1b[2C',
  '\x1b[D',
  '\x1b[2J',
  '\x1b[K',
  '\x1b[?1049h',
  '\x1b[?1049l',
  '\x1b[?2004h',
  '\x1b[?25l',
  '\x1b[2 q',
  '\x1b[1;24r',
  '\x1b[q',
  '\x1b[!p',
  '\x1b[6n',
  '\x1b[c',
  '\x1b[0c',
  '\x1b[>c',
  '\x1b[>0c',
  '\x1b7',
  '\x1b8',
  '\x1bD',
  '\x1bM',
  '\x1b(B',
  '\x1b=',
  '\x1b>',
];

/// Sequences that must vanish completely, content included.
const dropped = <String>[
  // OSC 52 clipboard set and query, BEL and ST.
  '\x1b]52;c;aGVsbG8=\x07',
  '\x1b]52;c;aGVsbG8=\x1b\\',
  '\x1b]52;c;?\x07',
  '\x1b]52;c;?\x1b\\',
  // Titles, icon, colours and other OSC.
  '\x1b]0;title\x07',
  '\x1b]1;icon\x07',
  '\x1b]2;title\x1b\\',
  '\x1b]4;1;?\x07',
  '\x1b]10;?\x07',
  '\x1b]11;?\x07',
  '\x1b]11;?\x1b\\',
  '\x1b]12;rgb:1/2/3\x07',
  '\x1b]7;file://example.com/home/alex\x07',
  '\x1b]1337;x\x07',
  '\x1b]\x07',
  // Window operations and reports.
  '\x1b[21t',
  '\x1b[20t',
  '\x1b[18t',
  '\x1b[14t',
  '\x1b[8;100;200t',
  '\x1b[t',
  // Status reports other than CPR.
  '\x1b[5n',
  '\x1b[?6n',
  '\x1b[n',
  '\x1b[15n',
  '\x1b[6 n',
  // Device attributes other than DA1 and DA2.
  '\x1b[=c',
  '\x1b[=0c',
  '\x1b[?1;2c',
  '\x1b[ c',
  // XTVERSION, DECRQM and the $ family, DECREQTPARM.
  '\x1b[>q',
  '\x1b[>0q',
  '\x1b[?2004\$p',
  '\x1b[4\$p',
  '\x1b[1;1;2;2\$x',
  '\x1b[x',
  '\x1b[1x',
  // DCS (DECRQSS, XTGETTCAP), SOS, PM, APC; BEL does not end them.
  '\x1bP\$qm\x1b\\',
  '\x1bP+q544e\x1b\\',
  '\x1bP\$q\x07m\x1b\\',
  '\x1bXsos\x1b\\',
  '\x1b^pm\x1b\\',
  '\x1b_apc\x1b\\',
];

void main() {
  group('passes unchanged', () {
    test('text, UTF-8 and C0 controls', () {
      expectAllSplits(b('Grüße, Alex ✓ 日本'), [t('Grüße, Alex ✓ 日本')]);
      expectAllSplits(b('a\x07\x08\t\n\x0b\x0c\r\x0e\x0fz'), [
        t('a\x07\x08\t\n\x0b\x0c\r\x0e\x0fz'),
      ]);
    });

    test('8-bit C1 bytes are not interpreted', () {
      final input = [0x41, 0x9b, 0x35, 0x6e, 0x90, 0x9d, 0x42];
      expectAllSplits(input, ['T:${String.fromCharCodes(input)}']);
    });

    for (final seq in passing) {
      test('${jsonEncode(seq)} byte-identical', () {
        expectAllSplits(b('Sam${seq}Robin'), [t('Sam${seq}Robin')]);
      });
    }

    test('the allowed queries still reach xterm: CPR, DA1 and DA2', () {
      final probe = XtermProbe()
        ..write(feedRaw([b('\x1b[6n\x1b[c\x1b[0c\x1b[>c\x1b[>0c')]));
      // xterm 4.0.0 writes CPR zero-based (its emitter uses the raw cursor), not 1;1.
      expect(probe.replies, [
        '\x1b[0;0R',
        '\x1b[?1;2c',
        '\x1b[?1;2c',
        '\x1b[>0;0;0c',
        '\x1b[>0;0;0c',
      ]);
    });
  });

  group('drops completely', () {
    for (final seq in dropped) {
      test(jsonEncode(seq), () {
        final input = b('Sam${seq}Robin');
        expectAllSplits(input, [t('SamRobin')]);
        final probe = XtermProbe()..write(feedRaw([input]));
        expect(probe.replies, isEmpty);
        expect(probe.sideEffects, isEmpty);
      });
    }

    test('xterm itself would answer or act on the unfiltered sequences', () {
      // Guards the meaning of the test: without the filter the renderer leaks.
      final probe = XtermProbe()
        ..terminal.write(
          '\x1b[5n\x1b[18t\x1b[=c\x1b]2;x\x07\x1b]52;c;?\x07\x1b[8;10;10t',
        );
      expect(probe.replies, isNotEmpty);
      expect(probe.sideEffects, containsAll(['title', 'osc 52', 'resize']));
    });
  });

  group('OSC 8 hyperlinks become inert text plus a record', () {
    test('open, text, close with ST', () {
      expectAllSplits(
        b('\x1b]8;;https://example.com/a\x1b\\link\x1b]8;;\x1b\\'),
        ['S:https://example.com/a', 'T:link', 'E'],
      );
    });

    test('BEL terminators, params ignored, text around kept', () {
      expectAllSplits(
        b('see \x1b]8;id=alex;https://example.com/b\x07here\x1b]8;;\x07 now'),
        ['T:see ', 'S:https://example.com/b', 'T:here', 'E', 'T: now'],
      );
    });

    test('a new link while one is open closes the first', () {
      expectAllSplits(
        b(
          '\x1b]8;;https://example.com/1\x07x\x1b]8;;https://example.com/2\x07y\x1b]8;;\x07',
        ),
        [
          'S:https://example.com/1',
          'T:x',
          'E',
          'S:https://example.com/2',
          'T:y',
          'E',
        ],
      );
    });

    test('a close without an open link emits nothing', () {
      expectAllSplits(b('a\x1b]8;;\x07b'), ['T:ab']);
    });

    test(
      'a percent-encoded target is kept; a raw non-ASCII one is dropped',
      () {
        expectAllSplits(
          b('\x1b]8;;https://example.com/gr%C3%BC%C3%9Fe\x1b\\'),
          ['S:https://example.com/gr%C3%BC%C3%9Fe'],
        );
        expectAllSplits(b('a\x1b]8;;https://example.com/grüße\x1b\\b'), [
          'T:ab',
        ]);
      },
    );

    test(
      'a line separator or a space in the target drops it (red team M4-35)',
      () {
        expectAllSplits(
          b('a\x1b]8;;https://example.com\u2028\u2028@evil.example/\x07b'),
          ['T:ab'],
        );
        expectAllSplits(b('a\x1b]8;;https://example.com @evil.example/\x07b'), [
          'T:ab',
        ]);
      },
    );

    test('malformed UTF-8 or control characters in the target drop it', () {
      expectAllSplits(
        [...b('a\x1b]8;;https://example.com/'), 0xc3, 0x28, ...b('\x07b')],
        ['T:ab'],
      );
      expectAllSplits(b('a\x1b]8;;https://example.com/\x01x\x07b'), ['T:ab']);
      expectAllSplits(b('a\x1b]8;;https://example.com/\tx\x07b'), ['T:ab']);
      expectAllSplits(b('a\x1b]8;;https://example.com/\u0085x\x07b'), ['T:ab']);
      expectAllSplits(b('a\x1b]8;;https://example.com/\u202egpj.exe\x07b'), [
        'T:ab',
      ]);
      expectAllSplits(b('a\x1b]8;;https://example.com/\u2066x\x07b'), ['T:ab']);
    });

    test('malformed OSC 8 forms emit nothing', () {
      expectAllSplits(b('a\x1b]8;id=1\x07b'), ['T:ab']);
      expectAllSplits(b('a\x1b]8\x07b'), ['T:ab']);
      expectAllSplits(b('a\x1b]88;;https://example.com\x07b'), ['T:ab']);
      expectAllSplits(b('a\x1b]8x;;https://example.com\x07b'), ['T:ab']);
    });

    test('a target at the cap passes, one byte more is dropped', () {
      const prefix = 'https://example.com/';
      final atCap =
          prefix + 'a' * (EscapeFilter.maxLinkTargetBytes - prefix.length);
      expect(feed([b('\x1b]8;;$atCap\x07')]), ['S:$atCap']);
      expect(feed([b('x\x1b]8;;${atCap}a\x07y')]), ['T:xy']);
    });

    test('a dropped oversized target leaves an open link open', () {
      final long = 'a' * (EscapeFilter.maxLinkTargetBytes + 1);
      expect(
        feed([
          b('\x1b]8;;https://example.com\x07x\x1b]8;;$long\x07y\x1b]8;;\x07'),
        ]),
        ['S:https://example.com', 'T:xy', 'E'],
      );
    });
  });

  group('aborts', () {
    test('CAN and SUB abort a sequence and are dropped themselves', () {
      expectAllSplits(b('a\x1b[21\x18tb'), [t('atb')]);
      expectAllSplits(b('a\x1b]52;c;x\x1ab'), [t('ab')]);
      expectAllSplits(b('a\x1bP\$q\x18m'), [t('am')]);
      expectAllSplits(b('a\x1b(\x1ab'), [t('ab')]);
      expectAllSplits(b('a\x1b\x18b'), [t('ab')]);
      expectAllSplits(b('a\x1b]8;;https://example.com\x18b'), [t('ab')]);
    });

    test('CAN and SUB outside a sequence pass', () {
      expectAllSplits(b('a\x18\x1ab'), [t('a\x18\x1ab')]);
    });

    test('an ESC inside a sequence starts a new one', () {
      expectAllSplits(b('\x1b[1\x1b[31mY'), [t('\x1b[31mY')]);
      expectAllSplits(b('\x1b]52;c;x\x1b[1mY'), [t('\x1b[1mY')]);
      expectAllSplits(b('\x1bPq\x1b]52;c;?\x07Z'), [t('Z')]);
      expectAllSplits(b('\x1b\x1b[5nZ'), [t('Z')]);
      expectAllSplits(b('\x1bPtmux;\x1b\x1b]52;c;aa\x07Z'), [t('Z')]);
    });

    test('C0 controls inside a CSI are executed before it', () {
      expectAllSplits(b('x\x1b[1\n;31my'), [t('x\n\x1b[1;31my')]);
      expectAllSplits(b('x\x1b[2\r1ty'), [t('x\ry')]);
    });

    test('a non-ASCII byte inside a CSI aborts it', () {
      expectAllSplits(b('\x1b[5én'), [t('én')]);
    });

    test('a parameter byte after an intermediate makes the CSI invalid', () {
      expectAllSplits(b('\x1b[1 2qX'), [t('X')]);
    });
  });

  group('renderer safety', () {
    // xterm 4.0.0 reads SGR 38/48 sub-parameters without a bounds check and repeats REP
    // once per count; host output must neither crash nor freeze the view (rule 3).
    for (final seq in [
      '\x1b[38m',
      '\x1b[48m',
      '\x1b[38;5m',
      '\x1b[48;2m',
      '\x1b[38;2;1;2m',
      '\x1b[1;48;5m',
      '\x1b[?38m',
      '\x1b[999999999b',
      '\x1b[10000b',
      '\x1b[513b',
      '\x1b[38;5;256m',
      '\x1b[48;5;9999m',
      '\x1b[0X',
      '\x1b[1;99999999999999999999999H',
    ]) {
      test('${jsonEncode(seq)} is dropped and xterm stays sound', () {
        final input = b('a${seq}Sam\x1b[6n');
        expectAllSplits(input, [t('aSam\x1b[6n')]);
        final probe = XtermProbe()..write(feedRaw([input]));
        expect(probe.replies, ['\x1b[0;4R']);
      });
    }

    for (final seq in [
      '\x1b[38;5;1m',
      '\x1b[48;2;1;2;3m',
      '\x1b[38;2;1;2;3;48;5;9m',
      '\x1b[38;;1m',
      '\x1b[38;;m',
      '\x1b[512b',
      '\x1b[38;5;255m',
      '\x1b[X',
      '\x1b[1X',
      '\x1b[9999;9999H',
    ]) {
      test('${jsonEncode(seq)} still passes', () {
        expectAllSplits(b('a${seq}b'), [t('a${seq}b')]);
        XtermProbe().write(feedRaw([b('a${seq}b')]));
      });
    }
  });

  group('memory caps', () {
    test('a CSI longer than the cap is dropped up to its final byte', () {
      final atCap =
          '\x1b[${('1;' * 200).substring(0, EscapeFilter.maxControlSequenceBytes - 3)}m';
      expect(b(atCap), hasLength(EscapeFilter.maxControlSequenceBytes));
      expect(feed([b('a${atCap}b')]), [t('a${atCap}b')]);
      final over =
          '\x1b[${('1;' * 200).substring(0, EscapeFilter.maxControlSequenceBytes - 2)}m';
      expect(feed([b('a${over}b')]), [t('ab')]);
      expect(feed([b('a\x1b[${'9' * 100000}tb')]), [t('ab')]);
    });

    test('a long dropped string sequence is consumed in many frames', () {
      final filter = EscapeFilter();
      final out = <FilteredChunk>[...filter.process(b('Alex\x1b]52;c;'))];
      final frame = Uint8List.fromList(List.filled(65536, 0x41));
      for (var i = 0; i < 32; i++) {
        out.addAll(filter.process(frame));
      }
      out.addAll(filter.process(b('\x07Sam')));
      expect(describe(out), [t('AlexSam')]);
    });
  });

  group('reset', () {
    test('drops a partial sequence', () {
      final filter = EscapeFilter();
      expect(filter.process(b('\x1b]52;c;')), isEmpty);
      filter.reset();
      expect(describe(filter.process(b('abc\x07'))), [t('abc\x07')]);
      filter.process(b('\x1b[2'));
      filter.reset();
      expect(describe(filter.process(b('1t'))), [t('1t')]);
      filter.process(b('\x1b'));
      filter.reset();
      expect(describe(filter.process(b('[5n'))), [t('[5n')]);
    });

    test('drops the link state', () {
      final filter = EscapeFilter();
      expect(describe(filter.process(b('\x1b]8;;https://example.com\x07'))), [
        'S:https://example.com',
      ]);
      filter.reset();
      expect(filter.process(b('\x1b]8;;\x07')), isEmpty);
    });
  });

  test('values outside a byte are taken modulo 256 and never throw', () {
    expect(
      feed([
        [0x141, 0x11b, 0x5b, 0x35, 0x6e, 0x142],
      ]),
      [t('AB')],
    );
  });

  test(
    'fuzz: never throws, never leaks a dropped sequence, splits do not matter',
    () {
      final random = Random(20261020);
      const alphabet = [
        0x1b,
        0x1b,
        0x1b,
        0x5b,
        0x5b,
        0x5d,
        0x5d,
        0x5c,
        0x07,
        0x18,
        0x1a,
        0x50,
        0x58,
        0x5e,
        0x5f,
        0x33,
        0x34,
        0x38,
        0x38,
        0x35,
        0x32,
        0x36,
        0x31,
        0x3b,
        0x3b,
        0x3f,
        0x3e,
        0x3d,
        0x24,
        0x20,
        0x6e,
        0x63,
        0x74,
        0x71,
        0x78,
        0x70,
        0x6d,
        0x6d,
        0x62,
        0x0a,
        0xc3,
        0xa9,
        0x9b,
        0x7f,
        0x01,
      ];
      final forbidden = {0x5d, 0x50, 0x58, 0x5e, 0x5f};
      for (var round = 0; round < 3000; round++) {
        final length = random.nextInt(120);
        final input = [
          for (var i = 0; i < length; i++)
            random.nextInt(4) == 0
                ? random.nextInt(256)
                : alphabet[random.nextInt(alphabet.length)],
        ];
        final whole = feedRaw([input]);
        final parts = <List<int>>[];
        var at = 0;
        while (at < input.length) {
          final end = min(input.length, at + 1 + random.nextInt(8));
          parts.add(input.sublist(at, end));
          at = end;
        }
        final reason = 'round $round: $input';
        expect(describe(feedRaw(parts)), describe(whole), reason: reason);

        final text = [
          for (final chunk in whole)
            if (chunk is FilteredText) ...chunk.bytes,
        ];
        for (var i = 0; i + 1 < text.length; i++) {
          if (text[i] == 0x1b) {
            expect(forbidden.contains(text[i + 1]), isFalse, reason: reason);
          }
        }
        // Idempotent: what passed once passes again unchanged and carries no link.
        expect(
          feed([text]),
          text.isEmpty ? <String>[] : ['T:${String.fromCharCodes(text)}'],
          reason: reason,
        );

        for (final chunk in whole) {
          if (chunk is LinkStart) {
            expect(chunk.target, isNotEmpty, reason: reason);
            expect(
              chunk.target.runes.any(
                (r) => r < 0x20 || (r >= 0x7f && r <= 0x9f),
              ),
              isFalse,
              reason: reason,
            );
          }
        }

        final probe = XtermProbe()..write(whole);
        for (final reply in probe.replies) {
          expect(
            allowedReply.hasMatch(reply),
            isTrue,
            reason: '$reason reply ${jsonEncode(reply)}',
          );
        }
        expect(probe.sideEffects, isEmpty, reason: reason);
        expect(probe.terminal.viewWidth, 80, reason: reason);
        expect(probe.terminal.viewHeight, 24, reason: reason);
      }
    },
  );
}
