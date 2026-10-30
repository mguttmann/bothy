// SPDX-License-Identifier: Apache-2.0

// The pure rules of long texts in the composer (brief M4-47; 01 6.4): which paste is long, the
// text one change inserted, the file a long paste becomes, and the cut of a draft that is
// larger than one message may carry (one message, the rest as one attachment, a fenced code
// block kept whole).

import 'dart:convert';
import 'dart:math';

import 'package:client/composer/long_text.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('PasteRule', () {
    const rule = PasteRule(maxChars: 10, maxLines: 3);

    test('a paste over the characters or the lines is long', () {
      expect(rule.isLong('0123456789'), isFalse);
      expect(rule.isLong('0123456789a'), isTrue);
      expect(rule.isLong('a\nb\nc'), isFalse);
      expect(rule.isLong('a\nb\nc\nd'), isTrue);
    });

    test('a trailing line break counts no extra line', () {
      expect(rule.isLong('a\nb\nc\n'), isFalse);
    });
  });

  group('insertionOf', () {
    test('the text a paste inserted at the cursor', () {
      final i = insertionOf('Hello world', 'Hello big world')!;
      expect(i.start, 6);
      expect(i.inserted, 'big ');
      expect(i.removedLength, 0);
    });

    test('a paste over a selection', () {
      final i = insertionOf('Hello world', 'Hello there')!;
      expect(i.start, 6);
      expect(i.inserted, 'there');
      expect(i.removedLength, 5);
    });

    test('a deletion inserts nothing', () {
      expect(insertionOf('Hello world', 'Hello'), isNull);
    });

    test('repeated characters stay inside the old text', () {
      final i = insertionOf('aaa', 'aaaaaa')!;
      expect(i.start + i.inserted.length, lessThanOrEqualTo(6));
      expect(i.inserted.length, 3);
    });

    test('without the inserted text the old text comes back', () {
      final random = Random(47);
      for (var n = 0; n < 200; n++) {
        final before = _randomText(random, 30);
        final at = random.nextInt(before.length + 1);
        final cut = at + random.nextInt(before.length - at + 1);
        final pasted = _randomText(random, 40);
        final after = before.replaceRange(at, cut, pasted);
        final i = insertionOf(before, after);
        if (pasted.isEmpty && cut == at) {
          expect(i, isNull);
          continue;
        }
        if (i == null) continue; // a paste equal to the replaced text
        expect(
          after.replaceRange(i.start, i.start + i.inserted.length, ''),
          before.replaceRange(i.start, i.start + i.removedLength, ''),
        );
      }
    });
  });

  group('pasteFileOf', () {
    test('plain text becomes paste.txt with the text as it is', () {
      final f = pasteFileOf('Line one\nLine two');
      expect(f.name, 'paste.txt');
      expect(f.content, 'Line one\nLine two');
      expect(f.mimeType, 'text/plain');
    });

    test(
      'one fenced block with a language becomes a code file of its content',
      () {
        final f = pasteFileOf('```python\nprint("hi")\nx = 1\n```\n');
        expect(f.name, 'paste.py');
        expect(f.content, 'print("hi")\nx = 1\n');
        expect(f.mimeType, 'text/plain');
      },
    );

    test('a fence with an unknown language keeps txt but drops the fence', () {
      final f = pasteFileOf('~~~brainfuck\n+++\n~~~');
      expect(f.name, 'paste.txt');
      expect(f.content, '+++\n');
    });

    test('text around a fenced block stays one text file', () {
      final f = pasteFileOf('Look:\n```dart\nvoid main() {}\n```\nok');
      expect(f.name, 'paste.txt');
      expect(f.content, 'Look:\n```dart\nvoid main() {}\n```\nok');
    });

    test('file content is recognized', () {
      expect(pasteFileOf('{"a": [1, 2]}').name, 'paste.json');
      expect(pasteFileOf('{"a": [1, 2]}').mimeType, 'application/json');
      expect(pasteFileOf('#!/usr/bin/env python3\nprint(1)').name, 'paste.py');
      expect(pasteFileOf('#!/bin/bash\necho 1').name, 'paste.sh');
      expect(pasteFileOf('<?xml version="1.0"?>\n<a/>').name, 'paste.xml');
      expect(pasteFileOf('<!DOCTYPE html>\n<html></html>').name, 'paste.html');
      expect(
        pasteFileOf('diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@').name,
        'paste.diff',
      );
      expect(pasteFileOf('{not json').name, 'paste.txt');
    });
  });

  group('splitForLimit', () {
    int bytes(String s) => utf8.encode(s).length;

    test('a text within the limit is not cut', () {
      final s = splitForLimit('short', 10);
      expect(s.head, 'short');
      expect(s.overflow, isNull);
    });

    test(
      'the head fits, ends at a line break, and head and rest are the text',
      () {
        final text = [for (var i = 0; i < 50; i++) 'line $i'].join('\n');
        final s = splitForLimit(text, 100);
        expect(bytes(s.head), lessThanOrEqualTo(100));
        expect(s.head.endsWith('\n'), isTrue);
        expect(s.head + s.overflow!, text);
      },
    );

    test('a cut never splits a character of several bytes', () {
      final text = 'é' * 100;
      final s = splitForLimit(text, 51);
      expect(bytes(s.head), 50);
      expect(s.head + s.overflow!, text);
    });

    test('a cut never splits a surrogate pair', () {
      final text = '\u{1F600}' * 20;
      final s = splitForLimit(text, 10);
      expect(s.head, '\u{1F600}\u{1F600}');
      expect(s.head + s.overflow!, text);
    });

    test('a fenced code block is never cut: it moves whole into the rest', () {
      final code = [for (var i = 0; i < 40; i++) 'x = $i'].join('\n');
      final text = 'Intro line\n```python\n$code\n```\nAfter';
      final s = splitForLimit(text, 120);
      expect(s.head, 'Intro line\n');
      expect(s.overflow, startsWith('```python\n'));
      expect(s.head + s.overflow!, text);
    });

    test('a block that starts the text stays in the head when nothing is before it', () {
      final code = [for (var i = 0; i < 40; i++) 'x = $i'].join('\n');
      final text = '```\n$code\n```';
      final s = splitForLimit(text, 60);
      expect(s.head, isNotEmpty);
      expect(bytes(s.head), lessThanOrEqualTo(60));
      expect(s.head + s.overflow!, text);
    });

    test('a closed block before the cut stays in the head', () {
      final text = '```\na\n```\n${'word ' * 40}';
      final s = splitForLimit(text, 40);
      expect(s.head, startsWith('```\na\n```\n'));
      expect(s.head + s.overflow!, text);
    });

    test('property: every cut fits and loses nothing', () {
      final random = Random(4747);
      for (var n = 0; n < 300; n++) {
        final text = _randomText(random, 400, fences: true);
        final limit = 1 + random.nextInt(200);
        final s = splitForLimit(text, limit);
        expect(bytes(s.head), lessThanOrEqualTo(limit));
        expect(s.head + (s.overflow ?? ''), text);
        expect(s.overflow == null, bytes(text) <= limit);
      }
    });
  });

  test('exceedsBytes counts UTF-8 bytes', () {
    expect(exceedsBytes('abc', 3), isFalse);
    expect(exceedsBytes('abcd', 3), isTrue);
    expect(exceedsBytes('éé', 3), isTrue);
    expect(exceedsBytes('é', 2), isFalse);
  });
}

String _randomText(Random random, int max, {bool fences = false}) {
  const parts = ['a', 'b', ' ', '\n', 'é', '\u{1F600}', 'x'];
  final out = StringBuffer();
  final length = random.nextInt(max + 1);
  for (var i = 0; i < length; i++) {
    if (fences && random.nextInt(40) == 0) {
      out.write('\n```\n');
    } else {
      out.write(parts[random.nextInt(parts.length)]);
    }
  }
  return out.toString();
}
