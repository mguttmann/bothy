// SPDX-License-Identifier: Apache-2.0

// "Copy as text" of a text block (01 6.12 item 7, R-PR-35; ticket F-KOPIER-FLIESSTEXT): an agent
// hands a text to forward in a fenced block without a language and breaks its lines by hand after
// about 60 to 70 characters. Pasted into a mail or another chat, those breaks arrived as a packet
// of short lines. [dsProseCopy] joins the lines of a paragraph with a space and keeps blank lines,
// list items and Markdown hard breaks; [dsIsProseBlock] decides which blocks offer it.

import 'package:design_system/design_system.dart';
import 'package:flutter_test/flutter_test.dart';

/// The live case, rebuilt without personal data: two paragraphs broken by hand, an emoji, an
/// umlaut, an en dash, typographic and straight quotes and a list whose second item is broken too.
const _live =
    'Hallo Sam, danke für die schnelle Rückmeldung \u{1F64F} Ich habe mir den Stand\n'
    'heute Vormittag angesehen und die „Freigabe“ für das neue Release ist\n'
    'aus meiner Sicht in Ordnung \u2013 mit zwei kleinen Punkten:\n'
    '\n'
    '- Die Übersetzung im Einstellungsdialog fehlt noch.\n'
    '- Der Export nach CSV braucht einen Test mit\n'
    '  Umlauten wie ä, ö, ü und ß.\n'
    '\n'
    'Wenn das erledigt ist, können wir am Montag live gehen. Sag gern\n'
    'Bescheid, falls du "noch etwas" brauchst. \u{1F44D}';

const _liveJoined =
    'Hallo Sam, danke für die schnelle Rückmeldung \u{1F64F} Ich habe mir den Stand '
    'heute Vormittag angesehen und die „Freigabe“ für das neue Release ist '
    'aus meiner Sicht in Ordnung \u2013 mit zwei kleinen Punkten:\n'
    '\n'
    '- Die Übersetzung im Einstellungsdialog fehlt noch.\n'
    '- Der Export nach CSV braucht einen Test mit Umlauten wie ä, ö, ü und ß.\n'
    '\n'
    'Wenn das erledigt ist, können wir am Montag live gehen. Sag gern '
    'Bescheid, falls du "noch etwas" brauchst. \u{1F44D}';

void main() {
  group('dsProseCopy', () {
    test('the live text becomes running text with its paragraphs and list', () {
      expect(dsProseCopy(_live), _liveJoined);
    });

    test('Unicode stays as it is', () {
      final out = dsProseCopy(_live);
      for (final s in [
        '\u{1F64F}',
        '\u{1F44D}',
        '\u2013',
        'ä',
        'ö',
        'ü',
        'ß',
        '„',
        '“',
        '"noch etwas"',
      ]) {
        expect(out, contains(s));
      }
    });

    test('CRLF and a lone CR are line breaks', () {
      expect(
        dsProseCopy('one\r\ntwo\r\n\r\nthree\rfour'),
        'one two\n\nthree four',
      );
    });

    test('a trailing line break and trailing blank lines are dropped', () {
      expect(dsProseCopy('one\ntwo\n'), 'one two');
      expect(dsProseCopy('one\ntwo\n\n\n'), 'one two');
    });

    test('leading blank lines are dropped, several blank lines are one '
        'paragraph break', () {
      expect(dsProseCopy('\n\none\n\n\n\ntwo'), 'one\n\ntwo');
    });

    test('a line of only spaces counts as a blank line', () {
      expect(dsProseCopy('one\n   \ntwo'), 'one\n\ntwo');
    });

    test('list items stay lines: dash, star, plus and numbers', () {
      const list = 'Steps:\n- first\n* second\n+ third\n1. fourth\n12) fifth';
      expect(dsProseCopy(list), list);
    });

    test('a broken list item joins into its item, nested items keep their '
        'indentation', () {
      expect(
        dsProseCopy('- top item\n  goes on\n  - nested\n    goes on'),
        '- top item goes on\n  - nested goes on',
      );
    });

    test('a dash or a number inside a sentence is no list item', () {
      expect(
        dsProseCopy('we ship\n-3 bugs and\n2.5 times faster'),
        'we ship -3 bugs and 2.5 times faster',
      );
    });

    test('a Markdown hard break keeps its line break and drops its marker', () {
      expect(
        dsProseCopy('Regards,  \nSam\nand team'),
        'Regards,\nSam and team',
      );
      expect(dsProseCopy('Regards,\\\nSam'), 'Regards,\nSam');
    });

    test('a heading stays its own line', () {
      expect(
        dsProseCopy('# Title\nfirst line\nsecond line'),
        '# Title\nfirst line second line',
      );
    });

    test('spaces at the joins collapse to one', () {
      expect(dsProseCopy('one \n   two'), 'one two');
    });

    test('a single line and the empty text stay as they are', () {
      expect(dsProseCopy('just one line'), 'just one line');
      expect(dsProseCopy(''), '');
    });
  });

  group('dsIsProseBlock', () {
    test('a block without a language or with a text language is prose', () {
      for (final language in [
        null,
        '',
        'text',
        'TEXT',
        'plaintext',
        'txt',
        'markdown',
        'md',
      ]) {
        expect(dsIsProseBlock(language), isTrue, reason: '$language');
      }
    });

    test('a block in a programming language is never prose', () {
      for (final language in ['dart', 'python', 'sh', 'json', 'diff', 'yaml']) {
        expect(dsIsProseBlock(language), isFalse, reason: language);
      }
    });
  });
}
