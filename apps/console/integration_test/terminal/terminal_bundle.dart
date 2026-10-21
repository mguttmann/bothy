// SPDX-License-Identifier: Apache-2.0

// The entry point of the `patrol` web test of the terminal in the console (brief M6-18 Design
// note 5), the same hand-written bundle as integration_test/admin/admin_bundle.dart (patrol_cli
// has no command that builds a web bundle for another server). The Dockerfile target
// console-e2e-terminal-web builds it with the flags of the real console, the server serves it
// under its enforced CSP, and tools/console-e2e (phase 3, headless/terminal_suites.py) drives it
// in Chromium. Update it with the pin of patrol in apps/console/pubspec.yaml.
//
// The internal imports are the ones the generated bundle uses; patrol has no public API for a
// hand-written bundle.
// ignore_for_file: implementation_imports, invalid_use_of_internal_member, depend_on_referenced_packages

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';
import 'package:patrol/src/platform/contracts/contracts.dart';
import 'package:patrol/src/platform/mobile/patrol_runtime_ports.dart';
import 'package:test_api/src/backend/invoker.dart';

import 'terminal_test.dart' as terminal_test;

/// Registers the one terminal test and serves it to the browser side.
Future<void> main() async {
  final explored = Completer<DartGroupEntry>();
  final automator = PlatformAutomator(
    config: PlatformAutomatorConfig.defaultConfig(),
  );
  await automator.initialize();
  final binding = PatrolBinding.ensureInitialized(automator);

  // Runs first and reads the tree of groups and tests (the hack of the generated bundle around
  // https://github.com/dart-lang/test/issues/1998).
  test('patrol_test_explorer', () {
    final top = Invoker.current!.liveTest.groups.first;
    explored.complete(createDartTestGroup(top));
  });

  group('terminal_test', terminal_test.main);

  final service = PatrolAppService(
    topLevelDartTestGroup: await explored.future,
  );
  binding.patrolAppService = service;
  await PatrolRuntimePorts.ensureLoaded();
  await runAppService(service);
  await automator.markPatrolAppServiceReady();
  await service.testExecutionCompleted;
}
