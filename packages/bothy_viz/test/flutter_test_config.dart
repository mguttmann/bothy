// SPDX-License-Identifier: Apache-2.0

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

/// Golden setup of this package, the same as design_system (docs/spec/15-client-architecture.md 20;
/// docs/DECISIONS.md D-M4-04-5).
///
/// CI goldens (text as blocks, Ahem) run on every machine and are compared everywhere with
/// [DsGoldenFileComparator] (tolerance for anti-aliasing between hosts, docs/DECISIONS.md PL/5);
/// platform goldens with the real fonts run only on macOS. The bundled fonts are loaded once
/// so platform goldens and widget tests render Atkinson Hyperlegible.
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
Future<void> _loadBundledFonts() async {
  final manifest = jsonDecode(
    await rootBundle.loadString('FontManifest.json'),
  ) as List<dynamic>;
  for (final entry in manifest.cast<Map<String, dynamic>>()) {
    final loader = FontLoader(entry['family'] as String);
    for (final font
        in (entry['fonts'] as List<dynamic>).cast<Map<String, dynamic>>()) {
      // The font manifest stores asset keys URL-encoded; the bundle expects them decoded.
      loader.addFont(rootBundle.load(Uri.decodeFull(font['asset'] as String)));
    }
    await loader.load();
  }
}
