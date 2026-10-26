// SPDX-License-Identifier: Apache-2.0

// The status page in the browser (brief M7-21 Tests): the owner sees the standard components
// that the worker's round created, opens an incident on one of them, sees it under the open
// incidents, resolves it with an update and sees it move to the history; a maintenance window
// shows apart from the incidents.

import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

/// The page of `getStatus` as the owner sees it.
Future<Map<String, Object?>> _page() async {
  final answer = await consoleApi('GET', '/status');
  final json = answer.json;
  if (answer.status != 200 || json is! Map<String, Object?>) {
    throw e2eFailure('getStatus answered ${answer.status}');
  }
  return json;
}

/// The ID of the incident titled [title], or a failure.
Future<String> _incidentId(String title) async {
  final incidents = ((await _page())['incidents']! as List)
      .cast<Map<String, Object?>>();
  final found = incidents.where((i) => i['title'] == title);
  if (found.isEmpty) throw e2eFailure('no incident "$title" on the page');
  return found.first['id']! as String;
}

/// The ID of the component named [name], waiting for the worker's first round.
Future<String> _componentId(PatrolIntegrationTester $, String name) async {
  final end = DateTime.now().add(const Duration(minutes: 2));
  while (DateTime.now().isBefore(end)) {
    final components = ((await _page())['components']! as List)
        .cast<Map<String, Object?>>();
    final found = components.where((c) => c['name'] == name);
    if (found.isNotEmpty) return found.first['id']! as String;
    await $.tester.pump(const Duration(seconds: 1));
  }
  throw e2eFailure('the component "$name" did not appear');
}

void main() {
  patrolTest('status page: components, an incident and its resolution', (
    $,
  ) async {
    final messages = await _componentId($, 'Messages and sync');
    await openConsole($, '/status');
    await waitFor(
      $,
      byKey('status.component.$messages'),
      timeout: const Duration(minutes: 2),
    );
    for (final name in ['Push notifications', 'Files', 'Search']) {
      expect(find.textContaining(name), findsWidgets, reason: name);
    }

    await tapKey($, 'status.openIncident');
    await typeKey($, 'status.form.title', 'Messages arrive late');
    await tapKey($, 'status.form.component.$messages');
    await typeKey($, 'status.form.body', 'We are looking into **delays**.');
    await tapKey($, 'confirm.submit');
    await waitFor($, find.text('The status page is updated.'));
    final incident = await _incidentId('Messages arrive late');
    await waitFor(
      $,
      find.descendant(
        of: byKey('status.incidents.open'),
        matching: byKey('status.incident.$incident'),
      ),
    );

    await tapKey($, 'status.incident.$incident.update');
    await tapKey($, 'status.form.incidentState');
    await tapOn($, find.text('Resolved'));
    await typeKey($, 'status.form.body', 'Delays are resolved.');
    await tapKey($, 'confirm.submit');
    await waitFor(
      $,
      find.descendant(
        of: byKey('status.history'),
        matching: byKey('status.incident.$incident'),
      ),
    );
  });

  patrolTest('status page: a maintenance window apart from incidents', (
    $,
  ) async {
    await openConsole($, '/status');
    await tapKey($, 'status.openIncident');
    await tapKey($, 'status.form.kind.maintenance');
    await typeKey($, 'status.form.title', 'Storage upgrade');
    await typeKey($, 'status.form.body', 'Uploads pause for one hour.');
    await tapKey($, 'confirm.submit');
    await waitFor($, find.text('The status page is updated.'));
    final window = await _incidentId('Storage upgrade');
    await waitFor(
      $,
      find.descendant(
        of: byKey('status.maintenanceWindows'),
        matching: byKey('status.incident.$window'),
      ),
    );
  });
}
