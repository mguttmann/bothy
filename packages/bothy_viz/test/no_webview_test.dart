// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// The package part of VZ-T16 (docs/spec/25-visualisierung.md 12; SEC-VZ-1): no WebView, no
/// HtmlElementView and no iframe in the sources of the package and its harness.
void main() {
  test('no WebView, HtmlElementView or iframe in lib/, harness/ or test/', () {
    final pattern = RegExp(
      r'webview|htmlelementview|iframe',
      caseSensitive: false,
    );
    final self = File('test/no_webview_test.dart').absolute.path;
    final hits = <String>[];
    for (final dir in ['lib', 'harness', 'test']) {
      final root = Directory(dir);
      if (!root.existsSync()) continue;
      for (final file in root.listSync(recursive: true).whereType<File>()) {
        final source =
            file.path.endsWith('.dart') || file.path.endsWith('.yaml');
        final generated =
            file.path.contains('/.dart_tool/') || file.path.contains('/build/');
        if (!source || generated || file.absolute.path == self) continue;
        final lines = file.readAsLinesSync();
        for (var i = 0; i < lines.length; i++) {
          // Comments may name what is banned; imports and code may not.
          final line = lines[i].trimLeft();
          if (line.startsWith('//') || line.startsWith('#')) continue;
          if (pattern.hasMatch(line)) hits.add('${file.path}:${i + 1}');
        }
      }
    }
    expect(hits, isEmpty);
  });
}
