// SPDX-License-Identifier: Apache-2.0

// The status page (brief M7-21 Tests): every component state with its word, the component of an
// agent exactly as the server returns it, open incidents, maintenance windows and the history
// with their Markdown texts, the maintenance mode apart from outages, the empty page, "Server
// unreachable" from the connection state and from a read without an answer, live reloads, and
// the owner forms with their validation and the bodies of their operations; a person without
// `status.manage` sees no form.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));

/// A source over [r] whose changes and connection state the test drives.
final class _Source implements StatusSource {
  _Source(this.r);

  final FakeAdminRepository r;
  final StreamController<void> changed = StreamController<void>.broadcast();
  final StreamController<bool> lost = StreamController<bool>.broadcast();

  @override
  Future<StatusPage> load() => r.status();

  @override
  Stream<void> get changes => changed.stream;

  @override
  Stream<bool> get unreachable => lost.stream;
}

Future<_Source> _pump(
  WidgetTester tester,
  FakeAdminRepository r, {
  bool manage = true,
}) async {
  final source = _Source(r);
  addTearDown(() async {
    await source.changed.close();
    await source.lost.close();
  });
  await pumpInApp(
    tester,
    StatusScreen(
      source: source,
      manage: manage ? r : null,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  );
  return source;
}

Finder _key(String key) => find.byKey(ValueKey(key));

Finder _in(String key, Finder f) => find.descendant(of: _key(key), matching: f);

Future<void> _tap(WidgetTester tester, String key) async {
  await tester.ensureVisible(_key(key));
  await tester.tap(_key(key));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('every component state has its word; the agent as returned', (
    tester,
  ) async {
    await _pump(tester, _repository());
    final states = {
      fakeApiComponent: 'Operational',
      fakePushComponent: 'Degraded',
      fakeFilesComponent: 'Maintenance',
      fakeSearchComponent: 'Partial outage',
      fakeLotseComponent: 'Major outage',
    };
    for (final MapEntry(key: id, value: word) in states.entries) {
      expect(
        _in('status.component.$id.state', find.text(word)),
        findsOneWidget,
        reason: id,
      );
    }
    expect(find.textContaining(RegExp(r'Lotse\W* \(agent\)')), findsOneWidget);
    expect(_in('status.overall', find.text('Degraded')), findsOneWidget);
    expect(_key('status.maintenanceMode'), findsNothing);
  });

  testWidgets('without the agent in the answer no agent component shows', (
    tester,
  ) async {
    final r = _repository()
      ..statusPage = fakeStatusPage(fakeStatusJson(withAgent: false));
    await _pump(tester, r);
    expect(_key('status.component.$fakeLotseComponent'), findsNothing);
    expect(find.textContaining('Lotse'), findsNothing);
  });

  testWidgets('incidents, maintenance windows and history apart', (
    tester,
  ) async {
    await _pump(tester, _repository());
    expect(
      _in('status.incidents.open', _key('status.incident.$fakeOpenIncident')),
      findsOneWidget,
    );
    expect(
      _in(
        'status.maintenanceWindows',
        _key('status.incident.$fakeMaintenance'),
      ),
      findsOneWidget,
    );
    expect(
      _in('status.history', _key('status.incident.$fakeResolvedIncident')),
      findsOneWidget,
    );
    // The open incident carries its impact; the maintenance window none.
    expect(
      tester.widget<Text>(_key('status.incident.$fakeOpenIncident.meta')).data,
      'Incident · Minor impact · Monitoring',
    );
    expect(
      tester.widget<Text>(_key('status.incident.$fakeMaintenance.meta')).data,
      'Maintenance · Scheduled',
    );
    // Owner texts are Markdown in the renderer's safe subset; newest update first.
    expect(find.textContaining('A fix is out.'), findsOneWidget);
    expect(find.textContaining('iPhone'), findsOneWidget);
    expect(find.textContaining(RegExp(r'Affects \W*Push')), findsOneWidget);
    final newer = tester.getTopLeft(find.textContaining('A fix is out.'));
    final older = tester.getTopLeft(find.textContaining('reports'));
    expect(newer.dy, lessThan(older.dy));
  });

  testWidgets('the maintenance mode shows as maintenance, not as an outage', (
    tester,
  ) async {
    final r = _repository()
      ..statusPage = fakeStatusPage(fakeStatusJson(overall: 'maintenance'));
    await _pump(tester, r);
    expect(_key('status.maintenanceMode'), findsOneWidget);
    expect(_in('status.overall', find.text('Maintenance')), findsOneWidget);
  });

  testWidgets('one component in maintenance claims no read-only instance', (
    tester,
  ) async {
    final json = fakeStatusJson(overall: 'maintenance', withAgent: false);
    for (final c in json['components']! as List<Object?>) {
      (c! as Map<String, Object?>)['state'] = 'operational';
    }
    ((json['components']! as List<Object?>).first!
            as Map<String, Object?>)['state'] =
        'maintenance';
    await _pump(tester, _repository()..statusPage = fakeStatusPage(json));
    final banner = tester.widget<DsBanner>(_key('status.maintenanceMode'));
    expect(
      banner.text,
      'Maintenance is in progress. Parts of the instance may be limited until it ends.',
    );
    expect(banner.text, isNot(contains('changes')));
  });

  testWidgets('an empty page says so', (tester) async {
    final r = _repository()
      ..statusPage = fakeStatusPage({
        'overall_state': 'operational',
        'components': <Object?>[],
        'incidents': <Object?>[],
      });
    await _pump(tester, r);
    expect(_key('status.empty'), findsOneWidget);
    expect(_key('status.incidents.none'), findsOneWidget);
    expect(_key('status.maintenanceWindows'), findsNothing);
    expect(_key('status.history'), findsNothing);
  });

  testWidgets(
    'an unreachable server hides every state until a new answer comes',
    (tester) async {
      final r = _repository();
      final source = await _pump(tester, r);
      expect(_key('status.component.$fakeApiComponent'), findsOneWidget);

      source.lost.add(true);
      await tester.pumpAndSettle();
      expect(find.text('Server unreachable'), findsOneWidget);
      expect(_key('status.component.$fakeApiComponent'), findsNothing);
      expect(_key('status.overall'), findsNothing);

      final reads = r.statusReads;
      source.lost.add(false);
      await tester.pumpAndSettle();
      expect(r.statusReads, reads + 1);
      expect(_key('status.unreachable'), findsNothing);
      expect(_key('status.component.$fakeApiComponent'), findsOneWidget);
    },
  );

  testWidgets('a read without an answer shows Server unreachable', (
    tester,
  ) async {
    final r = _repository()..statusError = const SelfActionFailed(null);
    await _pump(tester, r);
    expect(_key('status.unreachable'), findsOneWidget);
    expect(_key('status.overall'), findsNothing);
  });

  testWidgets('a refused read shows the server reason, not an outage', (
    tester,
  ) async {
    final r = _repository()
      ..statusError = const SelfActionFailed(
        ApiProblem(status: 403, code: 'forbidden'),
      );
    await _pump(tester, r);
    expect(_key('admin.denied'), findsOneWidget);
    expect(_key('status.unreachable'), findsNothing);
  });

  testWidgets('a change reloads the page', (tester) async {
    final r = _repository();
    final source = await _pump(tester, r);
    r.statusPage = fakeStatusPage(fakeStatusJson(overall: 'major_outage'));
    source.changed.add(null);
    await tester.pumpAndSettle();
    expect(_in('status.overall', find.text('Major outage')), findsOneWidget);
  });

  testWidgets('the console source reads again after its interval', (
    tester,
  ) async {
    final r = _repository();
    await pumpInApp(
      tester,
      StatusScreen(
        source: PollingStatusSource(
          loader: r.status,
          interval: const Duration(seconds: 30),
        ),
        timeZone: 'UTC',
        clock: testClock,
      ),
    );
    final reads = r.statusReads;
    await tester.pump(const Duration(seconds: 30));
    await tester.pumpAndSettle();
    expect(r.statusReads, reads + 1);
    // The periodic timer ends with the page.
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets('a person without status.manage sees no form', (tester) async {
    final r = _repository()..callerOwner = false;
    await _pump(tester, r);
    expect(_key('status.addComponent'), findsNothing);
    expect(_key('status.openIncident'), findsNothing);
    expect(_key('status.component.$fakeApiComponent.edit'), findsNothing);
    expect(_key('status.incident.$fakeOpenIncident.update'), findsNothing);
  });

  testWidgets('the read-only page asks nobody for the owner flag', (
    tester,
  ) async {
    await _pump(tester, _repository(), manage: false);
    expect(_key('status.addComponent'), findsNothing);
  });

  testWidgets('a new component needs a name and sends its fields', (
    tester,
  ) async {
    final r = _repository();
    await _pump(tester, r);
    await _tap(tester, 'status.addComponent');
    final save = tester.widget<FilledButton>(_key('confirm.submit'));
    expect(save.onPressed, isNull, reason: 'no name yet');
    await tester.enterText(_key('status.form.name'), 'Calendar');
    await tester.pumpAndSettle();
    await _tap(tester, 'confirm.submit');
    final (op, body) = r.statusCalls.single;
    expect(op, 'createStatusComponent');
    final input = body as StatusComponentInput;
    expect(input.name, 'Calendar');
    expect(input.state, StatusComponentState.operational);
    expect(input.description, isNull);
    expect(input.healthSource, isNull);
    expect(input.agentId, isNull);
    expect(find.text('The status page is updated.'), findsOneWidget);
  });

  testWidgets('a name of more than 200 characters is refused', (tester) async {
    await _pump(tester, _repository());
    await _tap(tester, 'status.addComponent');
    await tester.enterText(_key('status.form.name'), 'x' * 201);
    await tester.pumpAndSettle();
    expect(find.text('At most 200 characters.'), findsOneWidget);
    expect(
      tester.widget<FilledButton>(_key('confirm.submit')).onPressed,
      isNull,
    );
  });

  testWidgets('a change sends the name and only what changed', (tester) async {
    final r = _repository();
    await _pump(tester, r);
    await _tap(tester, 'status.component.$fakePushComponent.edit');
    await tester.enterText(_key('status.form.name'), 'Push notifications');
    await tester.pumpAndSettle();
    await _tap(tester, 'confirm.submit');
    final (op, body) = r.statusCalls.single;
    expect(op, 'updateStatusComponent $fakePushComponent');
    final input = body as StatusComponentInput;
    expect(input.name, 'Push notifications');
    expect(input.state, isNull);
    expect(input.description, isNull);
    expect(input.healthSource, isNull);
    expect(input.afterComponentId, isNull);
  });

  testWidgets('delete asks once and calls deleteStatusComponent', (
    tester,
  ) async {
    final r = _repository();
    await _pump(tester, r);
    await _tap(tester, 'status.component.$fakeSearchComponent.delete');
    expect(find.text('Delete Search?'), findsOneWidget);
    await _tap(tester, 'confirm.submit');
    expect(r.statusCalls.single, (
      'deleteStatusComponent',
      fakeSearchComponent,
    ));
  });

  testWidgets('a maintenance window with its components and text', (
    tester,
  ) async {
    final r = _repository();
    await _pump(tester, r);
    await _tap(tester, 'status.openIncident');
    await _tap(tester, 'status.form.kind.maintenance');
    await tester.enterText(_key('status.form.title'), 'Database upgrade');
    await _tap(tester, 'status.form.component.$fakeApiComponent');
    expect(
      tester.widget<FilledButton>(_key('confirm.submit')).onPressed,
      isNull,
      reason: 'no text yet',
    );
    await tester.enterText(_key('status.form.body'), 'Sync pauses briefly.');
    await tester.pumpAndSettle();
    await _tap(tester, 'confirm.submit');
    final (op, body) = r.statusCalls.single;
    expect(op, 'createStatusIncident');
    final input = body as StatusIncidentCreate;
    expect(input.kind, StatusIncidentKind.maintenance);
    expect(input.state, StatusIncidentState.scheduled);
    expect(input.impact, StatusIncidentImpact.none);
    expect(input.title, 'Database upgrade');
    expect(input.componentIds.toList(), [fakeApiComponent]);
    expect(input.bodyMd, 'Sync pauses briefly.');
    expect(input.startedAt, testNow);
  });

  testWidgets('an update sends its state and text', (tester) async {
    final r = _repository();
    await _pump(tester, r);
    await _tap(tester, 'status.incident.$fakeOpenIncident.update');
    await _tap(tester, 'status.form.incidentState');
    await tester.tap(find.text('Resolved').last);
    await tester.pumpAndSettle();
    await tester.enterText(_key('status.form.body'), 'Delays are resolved.');
    await tester.pumpAndSettle();
    await _tap(tester, 'confirm.submit');
    final (op, body) = r.statusCalls.single;
    expect(op, 'addStatusIncidentUpdate $fakeOpenIncident');
    final input = body as StatusIncidentUpdateCreate;
    expect(input.state, StatusIncidentState.resolved);
    expect(input.bodyMd, 'Delays are resolved.');
  });
}
