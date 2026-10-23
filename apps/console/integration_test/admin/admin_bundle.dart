// SPDX-License-Identifier: Apache-2.0

// The entry point of the `patrol` web suites of the console administration (brief M6-04 Design
// note 4, docs/decisions.d/M6-04.md D-M6-04-4). `patrol_cli` has no command that builds a web
// bundle for another server, so this file takes the place of the `test_bundle.dart` it generates: the
// same steps as the template of patrol_cli 4.8.0 for patrol 4.10.0 (an explorer test, then the
// app service the browser side calls through window.__patrol__getTests and
// window.__patrol__runTest). The Dockerfile target console-e2e-web builds it with the flags of
// the real console, the server serves it under its enforced CSP, and tools/console-e2e drives
// it in Chromium. Update it with the pin of patrol in apps/console/pubspec.yaml.
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

import 'a1_people_test.dart' as a1_people_test;
import 'a2_grants_test.dart' as a2_grants_test;
import 'a3_who_has_access_test.dart' as a3_who_has_access_test;
import 'a4_view_as_test.dart' as a4_view_as_test;
import 'a5_agents_test.dart' as a5_agents_test;
import 'a6_audit_test.dart' as a6_audit_test;
import 'a7_apps_test.dart' as a7_apps_test;
import 'a8_incoming_webhooks_test.dart' as a8_incoming_webhooks_test;

/// Registers the flows in the order they run (they share one server: A2 grants Sam on
/// Marketing before A4 looks at Sam's view, A5 switches the agent A2 found in I1; A7 installs
/// the only app before A8 adds the incoming app) and serves them to the browser side.
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

  group('a1_people_test', a1_people_test.main);
  group('a2_grants_test', a2_grants_test.main);
  group('a3_who_has_access_test', a3_who_has_access_test.main);
  group('a4_view_as_test', a4_view_as_test.main);
  group('a5_agents_test', a5_agents_test.main);
  group('a6_audit_test', a6_audit_test.main);
  group('a7_apps_test', a7_apps_test.main);
  group('a8_incoming_webhooks_test', a8_incoming_webhooks_test.main);

  final service = PatrolAppService(
    topLevelDartTestGroup: await explored.future,
  );
  binding.patrolAppService = service;
  await PatrolRuntimePorts.ensureLoaded();
  await runAppService(service);
  await automator.markPatrolAppServiceReady();
  await service.testExecutionCompleted;
}
