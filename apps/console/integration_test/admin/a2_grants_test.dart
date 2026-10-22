// SPDX-License-Identifier: Apache-2.0

// A2 in the browser (brief M6-04 Design note 4): a grant with the preview text of the server,
// compared with the golden example of 07 9.3 (the seed has the same names), created after the
// confirmation and the step-up; and an I1 rejection explained with "switch to organization
// first", which cannot be confirmed.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

/// The golden example of 07 9.3 in both languages of the server's catalogue
/// (services/server/internal/authz/domain/previewtext/testdata/example_07_*.golden).
const _golden = {
  'en':
      'Sam gets the level "Trigger" in channel Marketing/Kampagnen until 2026-12-31.\n'
      'Sam can then: read and write messages, create threads, trigger agent Texter.\n'
      'Upload files: no. Confirm approval cards: no.\n'
      'Note: Texter also reads Marketing/Intern. Sam may not read Marketing/Intern but can '
      'learn its contents through Texter.',
  'de':
      'Sam erhält bis 31.12.2026 die Stufe "Auslösen" im Kanal Marketing/Kampagnen.\n'
      'Sam kann damit: Nachrichten lesen und schreiben, Threads anlegen, Agent Texter auslösen.\n'
      'Dateien hochladen: nein. Freigabe-Karten bestätigen: nein.\n'
      'Hinweis: Texter liest auch Marketing/Intern. Sam darf Marketing/Intern nicht lesen, '
      'kann aber über Texter Inhalte daraus erfahren.',
};

String _previewText(PatrolIntegrationTester $) {
  // The keyed widget is the Text itself (preview_panel.dart), so the root counts.
  final texts = find.descendant(
    of: byKey('preview.text'),
    matching: find.byType(Text),
    matchRoot: true,
  );
  return [for (final e in texts.evaluate()) (e.widget as Text).data ?? '']
      .where((t) => t.isNotEmpty)
      .join('\n');
}

void main() {
  patrolTest('A2 a grant with the preview text and an I1 rejection explained', (
    $,
  ) async {
    final seed = E2eSeed.read();
    final campaigns = seed.channel('marketingCampaigns');
    await grantsAt($, 'channel.${campaigns.id}');
    await wizardFor($, seed.sam);
    await tapKey($, 'level.trigger');
    // The golden names 2026-12-31; a later run takes a day 30 days ahead, the text stays.
    final today = DateTime.now();
    final last = today.isBefore(DateTime(2026, 12, 1))
        ? DateTime(2026, 12, 31)
        : DateTime(today.year, today.month, today.day + 30);
    String two(int v) => v.toString().padLeft(2, '0');
    await typeKey(
      $,
      'wizard.until',
      '${last.year}-${two(last.month)}-${two(last.day)}',
    );
    await typeKey($, 'wizard.reason', 'Campaign work');
    // The preview of the final request: the server's text names the agent of the hint.
    await waitFor(
      $,
      find.descendant(
        of: byKey('preview.text'),
        matching: find.textContaining('Texter'),
        matchRoot: true,
      ),
    );
    final text = _previewText($);
    // The wizard sends the typed last day as an end at the start of the next day (M4-27), and
    // the server's text names the day of that end (docs/decisions.d/M6-04.md D-M6-04-7).
    final end = DateTime(last.year, last.month, last.day + 1);
    final expected = {
      _golden['en']!.replaceFirst(
        '2026-12-31',
        '${end.year}-${two(end.month)}-${two(end.day)}',
      ),
      _golden['de']!.replaceFirst(
        '31.12.2026',
        '${two(end.day)}.${two(end.month)}.${end.year}',
      ),
    };
    expect(expected.contains(text), isTrue, reason: 'preview text was:\n$text');
    await tapKey($, 'wizard.confirm');
    await waitGone($, byKey('wizard'));
    await waitFor($, find.textContaining('Sam'));
    expectStepUp();

    // I1: support-agent runs on the personal subscription of Alex; Sam may not trigger it.
    final support = seed.space('support');
    await grantsAt($, 'space.${support.id}');
    await wizardFor($, seed.sam);
    await tapKey($, 'level.trigger');
    await typeKey($, 'wizard.reason', 'Support work');
    await waitFor($, byKey('preview.violation.i1_violation'));
    await waitFor($, byKey('preview.i1Hint'));
    expect(byKey('guided.offer'), findsOneWidget);
    expect(
      $.tester.widget<FilledButton>(byKey('wizard.confirm')).onPressed,
      isNull,
    );
    await tapKey($, 'wizard.cancel');
    await waitGone($, byKey('wizard'));
  });
}
