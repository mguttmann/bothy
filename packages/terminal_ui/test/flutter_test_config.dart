// SPDX-License-Identifier: Apache-2.0

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// Golden setup of terminal_ui, the same as design_system (docs/spec/15-client-architecture.md 20;
/// docs/DECISIONS.md D-M4-04-5).
///
/// CI goldens (text as blocks, Ahem) run on every machine and are compared everywhere with
/// [DsGoldenFileComparator] (tolerance for anti-aliasing between hosts, docs/DECISIONS.md PL/5);
/// platform goldens with the real fonts run only on macOS. The bundled fonts are loaded once
/// so platform goldens and widget tests render Atkinson Hyperlegible and the terminal
/// font JetBrains Mono.
Future<void> testExecutable(FutureOr<void> Function() testMain) async {
  TestWidgetsFlutterBinding.ensureInitialized();
  await _loadBundledFonts();
  goldenFileComparator = DsGoldenFileComparator(
    (goldenFileComparator as LocalFileComparator).basedir.resolve('test.dart'),
  );
  return AlchemistConfig.runWithConfig(
    config: AlchemistConfig(
      ciGoldensConfig: const CiGoldensConfig(
        enabled: true,
        filePathResolver: _goldenPath,
      ),
      platformGoldensConfig: PlatformGoldensConfig(
        enabled: Platform.isMacOS,
        filePathResolver: _goldenPath,
      ),
    ),
    run: testMain,
  );
}

/// Golden tests live in `test/goldens/`, their files in `test/goldens/ci/` and
/// `test/goldens/macos/` (the path is relative to the test file).
String _goldenPath(String fileName, String environmentName) =>
    '${environmentName.toLowerCase()}/$fileName.png';

/// Loads every family of the asset font manifest of this package.
///
/// The families of this package itself (the terminal font) are listed under their plain name
/// here, while the apps register them as `packages/terminal_ui/<family>`, the name
/// `terminalFontFamily` uses; they are loaded under both names, so the tests resolve the font
/// the way the apps do.
Future<void> _loadBundledFonts() async {
  final manifest = jsonDecode(
    await rootBundle.loadString('FontManifest.json'),
  ) as List<dynamic>;
  for (final entry in manifest.cast<Map<String, dynamic>>()) {
    final family = entry['family'] as String;
    final fonts = (entry['fonts'] as List<dynamic>)
        .cast<Map<String, dynamic>>();
    for (final name in [
      family,
      if (!family.startsWith('packages/')) 'packages/terminal_ui/$family',
    ]) {
      final loader = FontLoader(name);
      for (final font in fonts) {
        // The font manifest stores asset keys URL-encoded; the bundle expects them decoded.
        loader.addFont(
          rootBundle.load(Uri.decodeFull(font['asset'] as String)),
        );
      }
      await loader.load();
    }
  }
}
