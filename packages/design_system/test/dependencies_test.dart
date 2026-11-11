// SPDX-License-Identifier: Apache-2.0

// The package brings no test library into the dependencies of an app
// (AUD-pkg-design-system-01, finding design-system-5). `lib/testing.dart` needs alchemist and
// flutter_test; as regular dependencies of this package they were dependencies of every app,
// and the apps bundled the Roboto files alchemist declares for its goldens.
//
// The test reads the package graph `pub get` writes for the workspace. The Flutter tool reads
// the same file when it builds an app bundle: a package reachable from the app only through
// dev dependencies contributes no asset and no font.

import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// The package graph of the workspace; `flutter test` runs in the package folder.
final File _graph = File('../../.dart_tool/package_graph.json');

/// The packages of the test setup that must stay out of an app.
const _testLibraries = ['alchemist', 'flutter_test'];

void main() {
  final packages = {
    for (final p
        in ((jsonDecode(_graph.readAsStringSync())
                    as Map<String, dynamic>)['packages']
                as List)
            .cast<Map<String, dynamic>>())
      p['name'] as String: p,
  };

  List<String> list(String package, String key) =>
      ((packages[package] ?? (throw StateError('no package $package')))[key]
                  as List? ??
              const [])
          .cast<String>();

  /// Every package [root] reaches through regular dependencies, without [root].
  Set<String> shipped(String root) {
    final seen = <String>{};
    final open = [...list(root, 'dependencies')];
    while (open.isNotEmpty) {
      final name = open.removeLast();
      if (seen.add(name)) open.addAll(list(name, 'dependencies'));
    }
    return seen;
  }

  test('the test libraries are dev dependencies of the package', () {
    expect(
      list('design_system', 'devDependencies'),
      containsAll(_testLibraries),
    );
    expect(
      list('design_system', 'dependencies').where(_testLibraries.contains),
      isEmpty,
    );
  });

  test('the package ships no test library to whoever depends on it', () {
    expect(
      shipped('design_system')
          .where((name) => _testLibraries.contains(name) || name == 'test_api'),
      isEmpty,
    );
  });

  test('no app ships alchemist and its fonts', () {
    for (final app in ['client', 'console']) {
      expect(shipped(app), contains('design_system'), reason: app);
      expect(shipped(app), isNot(contains('alchemist')), reason: app);
    }
  });
}
