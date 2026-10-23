// SPDX-License-Identifier: Apache-2.0

// The plain texts of the apps screens (brief M6-08): every scope of 05 2.6 has its meaning, a
// scope of a newer server says that this version has none, every ManifestFileProblem and every
// installation state has a text, and the table of 11 4.3 has the six rows for each action.

import 'dart:io' as io;

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

/// The scopes of the table in 05 2.6, read from the specification at run time, so a scope
/// added there without a meaning here fails this test.
List<String> specScopes() {
  var dir = io.Directory.current.absolute;
  while (!io.File('${dir.path}/docs/spec/05-actions.md').existsSync()) {
    if (dir.parent.path == dir.path) throw StateError('no docs/spec/05');
    dir = dir.parent;
  }
  final spec = io.File('${dir.path}/docs/spec/05-actions.md')
      .readAsStringSync();
  final start = spec.indexOf('### 2.6');
  final end = spec.indexOf('\n### ', start + 1);
  return {
    for (final m in RegExp(
      r'\| `([a-z]+:[a-z_]+)` \|',
    ).allMatches(spec.substring(start, end)))
      m.group(1)!,
  }.toList();
}

void main() {
  late AppLocalizations en;
  late AppLocalizations de;

  setUpAll(() async {
    en = await AppLocalizations.delegate.load(const Locale('en'));
    de = await AppLocalizations.delegate.load(const Locale('de'));
  });

  test('every scope of 05 2.6 has a meaning in both languages', () {
    final scopes = specScopes();
    expect(scopes, hasLength(greaterThanOrEqualTo(9)));
    for (final l in [en, de]) {
      for (final s in scopes) {
        expect(appScopeKnown(s), isTrue, reason: s);
        expect(appScopeLabel(l, s), isNot(l.adminAppScopeUnknown), reason: s);
      }
    }
    expect(
      appScopeLabel(en, 'messages:read'),
      'Read messages and threads and search in them',
    );
  });

  test('a scope of a newer server has no meaning', () {
    expect(appScopeKnown('reports:read'), isFalse);
    expect(appScopeLabel(en, 'reports:read'), en.adminAppScopeUnknown);
  });

  test('every file problem and every state has its own text', () {
    final problems = {
      for (final p in ManifestFileProblem.values)
        manifestFileProblemText(en, p),
    };
    expect(problems, hasLength(ManifestFileProblem.values.length));
    final states = {
      for (final s in AppInstallState.values) appStateLabel(en, s),
    };
    expect(states, hasLength(AppInstallState.values.length));
    expect(appStateLabel(de, AppInstallState.suspended), 'Angehalten');
  });

  test('the table of 11 4.3 has six rows per action', () {
    for (final action in AppLifecycleAction.values) {
      final rows = lifecycleEffects(en, action);
      expect(rows.map((r) => r.part), [
        'Tokens',
        'Deliveries',
        'Commands',
        'Components',
        'Access grants and incoming webhooks',
        'Messages of the app',
      ]);
    }
    expect(
      lifecycleEffects(en, AppLifecycleAction.suspend).map((r) => r.effect),
      [
        'Blocked, not revoked',
        'Paused, the position is kept',
        'Hidden',
        'Disabled',
        'Remain; incoming webhooks answer 404 until the app is resumed',
        'Remain',
      ],
    );
    expect(
      lifecycleEffects(en, AppLifecycleAction.uninstall).map((r) => r.effect),
      [
        'Revoked',
        'Subscriptions are disabled',
        'Removed',
        'Disabled',
        'Revoked',
        'Remain; the author is kept and the app is disabled',
      ],
    );
    expect(
      lifecycleEffects(en, AppLifecycleAction.resume).map((r) => r.effect),
      [
        'Work again',
        'Continue from the kept position',
        'Shown again',
        'Enabled again',
        'Unchanged; incoming webhooks accept calls again',
        'Unchanged',
      ],
    );
  });
}
