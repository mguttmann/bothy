// SPDX-License-Identifier: Apache-2.0

// The audit list (brief M4-29 Tests): each filter sets its query parameter of
// `listAuditEvents`, pages follow the cursor, times show in the person's time zone, a tap opens
// the details, and a `403` shows the server's reason without other controls.

import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late FakeAdminRepository repository;

  setUp(() {
    repository = FakeAdminRepository(prompt: RecordingPrompt())
      ..stepUpFresh = true;
  });

  Future<void> submit(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
  }

  Future<void> pick(WidgetTester tester, String dropdown, String item) async {
    await tester.tap(find.byKey(ValueKey(dropdown)));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ValueKey(item)).last);
    await tester.pumpAndSettle();
  }

  testWidgets('the list shows the entries newest first with their columns', (
    tester,
  ) async {
    await pumpAudit(tester, repository);

    expect(repository.auditRequests.single, contains('cursor='));
    final rows = find.byWidgetPredicate(
      (w) =>
          w.key is ValueKey<String> &&
          (w.key! as ValueKey<String>).value.startsWith('audit.entry.'),
    );
    expect(
      [for (final w in tester.widgetList(rows)) (w.key! as ValueKey).value],
      ['audit.entry.3', 'audit.entry.2', 'audit.entry.1'],
    );
    final sam = find.byKey(const ValueKey('audit.entry.2'));
    expect(
      find.descendant(of: sam, matching: find.text('Sam')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: sam, matching: find.text('agent.invoke')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: sam, matching: find.text('denied')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: sam, matching: find.text('missing_grant')),
      findsOneWidget,
    );
    final alex = find.byKey(const ValueKey('audit.entry.3'));
    expect(
      find.descendant(of: alex, matching: find.text('Add an owner')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: alex, matching: find.text('owner.add')),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: alex,
        matching: find.text('user 0192f4a0-7b1c-7000-8000-000000000303'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('each filter sets its parameter', (tester) async {
    await pumpAudit(tester, repository);

    await submit(tester, 'audit.filter.action', 'grant.create');
    expect(repository.auditRequests.last, contains('action=grant.create '));
    expect(find.byKey(const ValueKey('audit.entry.1')), findsOneWidget);
    expect(find.byKey(const ValueKey('audit.entry.3')), findsNothing);
    await submit(tester, 'audit.filter.action', '');

    await pick(tester, 'audit.filter.actor', 'audit.filter.actor.sam');
    expect(repository.auditRequests.last, contains('actor=sam '));
    await pick(tester, 'audit.filter.result', 'audit.filter.result.deny');
    expect(repository.auditRequests.last, contains('decision=deny '));

    await tester.enterText(
      find.byKey(const ValueKey('audit.filter.kind')),
      'run',
    );
    await tester.enterText(
      find.byKey(const ValueKey('audit.filter.id')),
      '0192f4a0-7b1c-7000-8000-000000000202',
    );
    await tester.tap(find.byKey(const ValueKey('audit.filter.apply')));
    await tester.pumpAndSettle();
    expect(
      repository.auditRequests.last,
      allOf(
        contains('kind=run '),
        contains('id=0192f4a0-7b1c-7000-8000-000000000202 '),
        contains('actor=sam '),
        contains('decision=deny '),
      ),
    );
    expect(find.byKey(const ValueKey('audit.entry.2')), findsOneWidget);
  });

  testWidgets('the time range is read in the person time zone', (tester) async {
    await pumpAudit(tester, repository);

    await tester.enterText(
      find.byKey(const ValueKey('audit.filter.from')),
      '2026-10-17',
    );
    await submit(tester, 'audit.filter.to', '2026-10-17');
    // Europe/Berlin is UTC+2 in October: the day starts at 22:00 UTC the day before; "to" is
    // the start of the day after, so the last day counts.
    expect(
      repository.auditRequests.last,
      allOf(
        contains('from=2026-10-16T22:00:00.000Z '),
        contains('to=2026-10-17T22:00:00.000Z '),
      ),
    );
    expect(find.byKey(const ValueKey('audit.entry.1')), findsNothing);
    expect(find.byKey(const ValueKey('audit.entry.2')), findsOneWidget);

    final requests = repository.auditRequests.length;
    await submit(tester, 'audit.filter.from', '2026-02-30');
    expect(find.text('Enter a date as YYYY-MM-DD.'), findsOneWidget);
    expect(repository.auditRequests, hasLength(requests));
  });

  testWidgets('pages follow the cursor', (tester) async {
    repository.auditPageSize = 2;
    await pumpAudit(tester, repository);

    expect(find.byKey(const ValueKey('audit.entry.1')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('audit.more')));
    await tester.pumpAndSettle();
    expect(repository.auditRequests.last, endsWith('cursor=2'));
    expect(find.byKey(const ValueKey('audit.entry.1')), findsOneWidget);
    expect(find.byKey(const ValueKey('audit.entry.3')), findsOneWidget);
    expect(find.byKey(const ValueKey('audit.more')), findsNothing);
  });

  testWidgets('times show in the person time zone', (tester) async {
    await pumpAudit(tester, repository);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('audit.entry.3')),
        matching: find.text('Today 10:00\u202fAM'),
      ),
      findsOneWidget,
    );

    await pumpAudit(tester, repository, timeZone: 'America/New_York');
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('audit.entry.3')),
        matching: find.text('Today 4:00\u202fAM'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('a tap shows the details of the entry', (tester) async {
    await pumpAudit(tester, repository);
    await tester.tap(find.byKey(const ValueKey('audit.entry.2')));
    await tester.pumpAndSettle();

    final detail = find.byKey(const ValueKey('audit.detail'));
    expect(detail, findsOneWidget);
    for (final text in [
      'Entry no. 2',
      '4bf92f35-77b3-4da6-a3ce-000000000002',
      'missing_grant',
      'b2' * 32,
      'a1' * 32,
    ]) {
      expect(
        find.descendant(of: detail, matching: find.text(text)),
        findsOneWidget,
        reason: text,
      );
    }
    expect(
      find.descendant(of: detail, matching: find.textContaining('said')),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('audit.detail.close')));
    await tester.pumpAndSettle();
    expect(detail, findsNothing);
  });

  testWidgets('a 403 shows the server reason and no controls', (tester) async {
    repository.refuse['auditEvents'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pumpAudit(tester, repository);

    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('audit.check')), findsNothing);
    expect(find.byKey(const ValueKey('audit.filter.apply')), findsNothing);
    expect(find.byKey(const ValueKey('audit.export')), findsNothing);
  });

  testWidgets('the view offers no change of an entry', (tester) async {
    await pumpAudit(tester, repository, saveTarget: MemoryTarget().call);
    await tester.tap(find.byKey(const ValueKey('audit.entry.3')));
    await tester.pumpAndSettle();
    final buttons = find.descendant(
      of: find.byKey(const ValueKey('audit.detail')),
      matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
    );
    expect(buttons, findsOneWidget, reason: 'only "Close"');
  });
}
