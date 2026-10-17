// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter_test/flutter_test.dart';

// flutter test runs with the package folder as the working directory.
bool _bundled(String? asset) => asset != null && File(asset).existsSync();

void main() {
  test('every semantic icon resolves in its mapped weight and in fill', () {
    expect(dsSemanticIcons, isNotEmpty);
    for (final MapEntry(key: name, value: glyph) in dsSemanticIcons.entries) {
      expect(
        _bundled(DsIconAssets.lookup(glyph)),
        isTrue,
        reason: '$name (${glyph.weight.name}/${glyph.name})',
      );
      expect(
        _bundled(DsIconAssets.lookup(glyph, filled: true)),
        isTrue,
        reason: '$name (fill/${glyph.name})',
      );
    }
  });

  test('every space icon resolves', () {
    expect(DsIcons.spaceIcons, isNotEmpty);
    for (final name in DsIcons.spaceIcons) {
      expect(_bundled(DsIconAssets.space(name)), isTrue, reason: name);
    }
  });

  test('the lookup file names no glyph that the mapping lacks', () {
    final mapped = {
      for (final glyph in dsSemanticIcons.values) ...[
        '${glyph.weight.name}/${glyph.name}',
        'fill/${glyph.name}',
      ],
      for (final name in DsIcons.spaceIcons) 'regular/$name',
    };
    expect(DsIconAssets.byKey.keys.toSet(), mapped);
  });

  test('every asset file under assets/icons is in the lookup', () {
    final files = Directory('assets/icons')
        .listSync(recursive: true)
        .whereType<File>()
        .map((f) => f.path.replaceAll(r'\', '/'))
        .where((p) => p.endsWith('.vec'))
        .toSet();
    expect(files, DsIconAssets.byKey.values.toSet());
  });
}
