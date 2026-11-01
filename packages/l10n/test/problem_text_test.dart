// SPDX-License-Identifier: Apache-2.0

// Problem codes (brief M4-06 Design note 5): every code of `x-problem-codes` in
// docs/api/openapi.yaml has its own text, so a new code in the specification needs a text in
// the same change; an unknown code shows the generic text with the code.

import 'dart:io';
import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support/repo.dart';

/// The codes of `x-problem-codes`, in the order of the file.
List<String> specCodes() {
  final lines = File('${repoRoot().path}/docs/api/openapi.yaml')
      .readAsLinesSync();
  final start = lines.indexWhere((l) => l.trim() == 'x-problem-codes:');
  expect(
    start,
    isNonNegative,
    reason: 'docs/api/openapi.yaml has no x-problem-codes',
  );
  final indent = lines[start].indexOf('x');
  final codes = <String>[];
  for (final line in lines.skip(start + 1)) {
    final text = line.trimLeft();
    if (text.isNotEmpty &&
        line.length - text.length <= indent &&
        !text.startsWith('- ')) {
      break;
    }
    final match = RegExp(r'^- code: (\S+)$').firstMatch(text);
    if (match != null) {
      codes.add(match.group(1)!);
    }
  }
  return codes;
}

/// The reasons of `terminal_denied` that `x-problem-codes` names (24 13): the backticked words
/// after "`reason` is" in the description of the code.
List<String> terminalDeniedReasons() {
  final text = File('${repoRoot().path}/docs/api/openapi.yaml')
      .readAsStringSync();
  final start = text.indexOf('- code: terminal_denied');
  expect(start, isNonNegative, reason: 'terminal_denied is not listed');
  final entry = text.substring(start, text.indexOf('client_action', start));
  final reasons = entry.substring(
    entry.indexOf('`reason` is') + '`reason` is'.length,
  );
  return [
    for (final m in RegExp(r'`([a-z0-9_]+)`').allMatches(reasons)) m.group(1)!,
  ];
}

void main() {
  final locales = AppLocalizations.supportedLocales;

  test('the specification lists the reasons of terminal_denied', () {
    expect(
      terminalDeniedReasons(),
      containsAll(['no_session_user', 'profile_unknown', 'session_limit']),
    );
  });

  test('the specification lists the problem codes', () {
    expect(
      specCodes(),
      containsAll(['not_found', 'forbidden', 'setup_required']),
    );
  });

  for (final locale in locales) {
    final l = lookupAppLocalizations(locale);

    test(
      'every problem code of the specification has its own text ($locale)',
      () {
        final fallbacks = specCodes().where(
          (c) => problemText(l, c) == l.problemFallback(c),
        );
        expect(fallbacks, isEmpty, reason: 'codes without an ARB message');
      },
    );

    test('codes map to distinct texts ($locale)', () {
      final texts = specCodes().map((c) => problemText(l, c)).toSet();
      expect(texts.length, specCodes().length);
    });

    test('a reason selects the specific text ($locale)', () {
      expect(
        problemText(l, 'forbidden', reason: 'capability_missing'),
        l.problemForbiddenCapabilityMissing,
      );
      expect(
        problemText(l, 'forbidden', reason: 'capability_missing'),
        isNot(l.problemForbidden),
      );
      expect(
        problemText(l, 'forbidden', reason: 'something_new'),
        l.problemForbidden,
      );
    });

    test('a refused project folder of a delete has its own text ($locale)', () {
      // 01 6.18 item 4: the runner's last folder check does not allow the move.
      expect(
        problemText(l, 'conflict', reason: 'project_folder_refused'),
        l.problemConflictProjectFolderRefused,
      );
      expect(
        problemText(l, 'conflict', reason: 'project_folder_refused'),
        isNot(l.problemConflict),
      );
    });

    test('the refusals of a step-up have their own texts ($locale)', () {
      // SEC-STEPUP-LOCKOUT (08 6.2 and 13): a locked password path and a password changed during
      // the attempt; a session that cannot confirm at all (07 4.1 step_up_not_possible).
      final texts = {
        problemText(l, 'step_up_required', reason: 'password_locked'),
        problemText(l, 'step_up_required', reason: 'password_changed'),
        problemText(l, 'forbidden', reason: 'step_up_not_possible'),
      };
      expect(texts, hasLength(3));
      expect(
        texts,
        isNot(
          anyOf(
            contains(l.problemStepUpRequired),
            contains(l.problemForbidden),
          ),
        ),
      );
      expect(
        problemText(l, 'step_up_required', reason: 'password_locked'),
        l.problemStepUpRequiredPasswordLocked,
      );
    });

    test('every reason of terminal_denied has its own text ($locale)', () {
      final reasons = terminalDeniedReasons();
      final texts = {
        for (final r in reasons) problemText(l, 'terminal_denied', reason: r),
      };
      expect(texts, hasLength(reasons.length));
      expect(texts, isNot(contains(l.problemTerminalDenied)));
    });

    test('an unknown code shows the fallback with the code ($locale)', () {
      final text = problemText(l, 'future_code');
      expect(text, l.problemFallback('future_code'));
      expect(text, contains('future_code'));
    });
  }

  test('the specific and the fallback texts read as written', () {
    final en = lookupAppLocalizations(const Locale('en'));
    final de = lookupAppLocalizations(const Locale('de'));
    expect(problemText(en, 'not_found'), 'Not found or no access');
    expect(
      problemText(en, 'future_code'),
      'Something went wrong (future_code)',
    );
    expect(
      problemText(de, 'future_code'),
      'Etwas ist schiefgegangen (future_code)',
    );
    expect(
      problemText(en, 'forbidden', reason: 'capability_missing'),
      'You lack an extra permission for this',
    );
  });
}
