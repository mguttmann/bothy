// SPDX-License-Identifier: Apache-2.0

// The typed editors of the instance settings are held equal to their sources (brief M6-09
// Design note 1; Ruling D-M6-09-1): every key of the table of the configuration reference
// section 6 with its type, the accent keys of ADR 0044 and of the server
// (`kernel/designkeys`), and the languages of `locale.default` that the server accepts.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter_test/flutter_test.dart';

/// The repository root (the directory with `docs/operations/configuration.md`).
Directory _root() {
  var dir = Directory.current;
  while (!File('${dir.path}/docs/operations/configuration.md').existsSync()) {
    dir = dir.parent;
  }
  return dir;
}

String _read(String path) => File('${_root().path}/$path').readAsStringSync();

/// The rows of the table of section 6: key and type.
List<(String, String)> _section6() {
  final text = _read('docs/operations/configuration.md');
  final start = text.indexOf('## 6. Instance settings');
  final end = text.indexOf('\n## 7.', start);
  final rows = <(String, String)>[];
  for (final line in text.substring(start, end).split('\n')) {
    final m = RegExp(r'^\| `([a-z_.]+)` \| ([^|]+) \|').firstMatch(line);
    if (m == null) continue;
    rows.add((m[1]!, m[2]!.trim()));
  }
  return rows;
}

void main() {
  test('the table holds exactly the keys of configuration reference 6', () {
    final doc = _section6();
    expect(doc, isNotEmpty);
    expect([for (final s in instanceSettingSchemas) (s.key, s.docType)], doc);
  });

  test('branding keys are written only by the branding operations', () {
    for (final s in instanceSettingSchemas) {
      expect(
        s.route,
        s.key.startsWith('branding.')
            ? SettingRoute.branding
            : SettingRoute.settings,
        reason: s.key,
      );
    }
  });

  test('the accent keys are those of ADR 0044, 01 12 and the server', () {
    final server = _read(
      'services/server/internal/kernel/designkeys/accent.go',
    );
    final list = RegExp(r'accentColors = \[\]string\{([^}]*)\}')
        .firstMatch(server)![1]!;
    final keys = RegExp(r'"([a-z]+)"').allMatches(list).map((m) => m[1]!);
    expect(accentPaletteKeys.toSet(), keys.toSet());
    expect(accentPaletteKeys, hasLength(7));
    final adr = _read(
      'docs/adr/0044-working-name-and-branding-as-configuration.md',
    );
    final spec = _read('docs/spec/01-product.md');
    for (final k in accentPaletteKeys) {
      expect(adr, contains('`$k`'), reason: 'ADR 0044 names $k');
      expect(spec, contains('`$k`'), reason: '01 12 names $k');
    }
  });

  test('locale.default offers the languages the server accepts', () {
    final server = _read('services/server/internal/instance/settings.go');
    final list = RegExp(r'supportedLocales = \[\]string\{([^}]*)\}')
        .firstMatch(server)![1]!;
    final tags = RegExp(r'"([a-z-]+)"').allMatches(list).map((m) => m[1]!);
    expect(supportedLanguageTags.toSet(), tags.toSet());
  });

  test('an unknown key has no schema', () {
    expect(settingSchemaOf('push.future_setting'), isNull);
    expect(settingSchemaOf('locale.default')!.kind, SettingKind.locale);
  });
}
