// SPDX-License-Identifier: Apache-2.0

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';
import 'admin_fakes.dart';

/// A browser whose console is signed in with a step-up; [session] gives the session body.
TestBrowser signedIn({Map<String, Object?> Function()? session}) {
  final b = TestBrowser();
  adminRoutes(b.server);
  b.server.routes['GET /api/v1/console/session'] = (_) =>
      Answer(200, (session ?? adminSession)());
  b.server.routes['GET /api/v1/groups'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  return b;
}

/// A browser window of 1600 by 1000 logical pixels.
void browserWindow(WidgetTester tester) {
  tester.view.physicalSize = const Size(1600, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
}

void main() {
  group('the pages host the screens of admin_ui', () {
    testWidgets('People shows the list of A1 at the width of the browser', (
      tester,
    ) async {
      browserWindow(tester);
      final b = signedIn();
      await pumpConsole(tester, b, location: '/people');
      expect(find.byType(AdminBody), findsOneWidget);
      expect(find.byType(PeopleList), findsOneWidget);
      expect(find.text('Sam'), findsWidgets);
      // The console's own navigation, not the sidebar of the desktop's AdminShell.
      expect(find.byType(AdminShell), findsNothing);
      expect(find.byKey(const ValueKey('admin.nav.people')), findsNothing);
      expect(tester.getSize(find.byType(PeopleList)).width, greaterThan(880));
    });

    testWidgets('a detail of admin_ui changes the place and the path', (
      tester,
    ) async {
      final b = signedIn();
      final c = await pumpConsole(tester, b, location: '/people');
      c.openAdmin(const AdminLocation(AdminSection.people, id: samId));
      await tester.pumpAndSettle();
      expect(c.page, ConsolePage.people);
      expect(c.path, '/people/$samId');
      expect(find.byType(PersonDetail), findsOneWidget);
    });

    testWidgets('Groups has the tabs Groups and Access grants', (tester) async {
      final b = signedIn();
      final c = await pumpConsole(tester, b, location: '/groups');
      expect(find.byType(GroupsList), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('console.groups.tab.grants')));
      await tester.pumpAndSettle();
      expect(c.path, '/groups/grants');
      expect(c.page, ConsolePage.groups);
      expect(find.byType(GrantsList), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('console.groups.tab.groups')));
      await tester.pumpAndSettle();
      expect(c.path, '/groups');
    });

    testWidgets('Apps has the tabs Apps and Incoming webhooks', (tester) async {
      final b = signedIn();
      final c = await pumpConsole(tester, b, location: '/apps');
      expect(find.byType(AppsList), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('console.apps.tab.incoming')));
      await tester.pumpAndSettle();
      expect(c.path, '/apps/incoming-webhooks');
      expect(c.page, ConsolePage.apps);
      expect(find.byType(IncomingWebhooksView), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('console.apps.tab.apps')));
      await tester.pumpAndSettle();
      expect(c.path, '/apps');
    });

    testWidgets('the manifest of an app comes from the browser file input', (
      tester,
    ) async {
      final b = signedIn();
      await pumpConsole(tester, b, location: '/apps');
      final picker = tester
          .widget<AppsList>(find.byType(AppsList))
          .pickManifest;
      expect(picker, isNotNull);
      b.files.next = ChosenFile(
        'repo-bridge.yaml',
        Uint8List.fromList(utf8.encode('slug: repo-bridge\n')),
      );
      final file = await picker!();
      expect(file!.name, 'repo-bridge.yaml');
      expect(utf8.decode(file.bytes), 'slug: repo-bridge\n');
      expect(b.files.choices.single, (
        '.json,.yaml,.yml,application/json',
        'manifest',
      ));
      b.files.next = null;
      expect(await picker(), isNull);
    });

    testWidgets('Groups has the tabs Permission hints and Expiring soon', (
      tester,
    ) async {
      browserWindow(tester);
      final b = signedIn();
      b.server.routes['GET /api/v1/access/hints'] = (_) => const Answer(200, {
        'items': <Object>[],
        'labels': {
          'principals': <Object>[],
          'groups': <Object>[],
          'spaces': <Object>[],
          'channels': <Object>[],
        },
      });
      b.server.routes['GET /api/v1/access/expiring'] = (_) =>
          const Answer(200, {
            'items': <Object>[],
            'labels': {
              'principals': <Object>[],
              'groups': <Object>[],
              'spaces': <Object>[],
              'channels': <Object>[],
            },
          });
      final c = await pumpConsole(tester, b, location: '/groups');
      expect(find.text('Permission hints'), findsOneWidget);
      expect(find.text('Expiring soon'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('console.groups.tab.hints')));
      await tester.pumpAndSettle();
      expect(c.path, '/groups/hints');
      expect(c.page, ConsolePage.groups);
      expect(find.byType(PermissionHintsView), findsOneWidget);
      expect(find.text('No permission hints.'), findsOneWidget);
      expect(
        tester.getSize(find.byType(PermissionHintsView)).width,
        greaterThan(880),
      );

      await tester.tap(
        find.byKey(const ValueKey('console.groups.tab.expiring')),
      );
      await tester.pumpAndSettle();
      expect(c.path, '/groups/expiring');
      expect(find.byType(ExpiringAccessView), findsOneWidget);
      expect(find.text('Nothing expires soon.'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('console.groups.tab.groups')));
      await tester.pumpAndSettle();
      expect(c.path, '/groups');
    });

    testWidgets('a page of a later ticket keeps its placeholder', (
      tester,
    ) async {
      final b = signedIn();
      // Live has its page since M6-16; Status keeps the placeholder.
      await pumpConsole(tester, b, location: '/status');
      expect(find.byType(AdminBody), findsNothing);
      expect(
        find.text('This page comes with a later version of the console.'),
        findsOneWidget,
      );
    });

    testWidgets('the audit export goes to a browser download', (tester) async {
      final b = signedIn();
      await pumpConsole(tester, b, location: '/audit');
      expect(find.byType(AuditLogView), findsOneWidget);
      final view = tester.widget<AuditLogView>(find.byType(AuditLogView));
      expect(view.saveTarget, isNotNull);
    });
  });

  group('View as person', () {
    final banner = find.byType(ViewAsBanner);

    testWidgets('the banner with name and time stands on every page', (
      tester,
    ) async {
      final b = signedIn(
        session: () => adminSession(
          viewAsEnds: DateTime.now().add(const Duration(minutes: 10)),
        ),
      );
      final c = await pumpConsole(tester, b, location: '/people');
      for (final page in consoleNavigation) {
        c.open(page);
        await tester.pumpAndSettle();
        expect(banner, findsOneWidget, reason: page.path);
        expect(
          find.text('Viewing as Sam (read-only). Nothing is sent. Logged.'),
          findsOneWidget,
        );
        expect(find.byKey(const ValueKey('viewAs.remaining')), findsOneWidget);
      }
    });

    testWidgets('End view ends the stored view and the banner goes', (
      tester,
    ) async {
      var stored = true;
      final b = signedIn(
        session: () => adminSession(
          viewAsEnds: stored
              ? DateTime.now().add(const Duration(minutes: 10))
              : null,
        ),
      );
      b.server.routes['DELETE /api/v1/console/session/view-as'] = (_) {
        stored = false;
        return const Answer(204);
      };
      await pumpConsole(tester, b, location: '/agents');
      expect(banner, findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('viewAs.end')));
      await tester.pumpAndSettle();
      final end = b.server.to('DELETE', '/api/v1/console/session/view-as');
      expect(end.single.headers['x-console-request'], '1');
      expect(banner, findsNothing);
    });

    testWidgets(
      'the end time ends the view without a step-up: the first read after it is refused once',
      (tester) async {
        var reads = 0;
        final b = signedIn();
        b.server.routes['GET /api/v1/console/session'] = (_) {
          reads++;
          if (reads == 1) {
            return Answer(
              200,
              adminSession(
                viewAsEnds: DateTime.now().add(const Duration(seconds: 1)),
              ),
            );
          }
          if (reads == 2) {
            return const Answer(401, {
              'code': 'step_up_required',
              'status': 401,
            });
          }
          return Answer(200, adminSession());
        };
        final c = await pumpConsole(tester, b, location: '/people');
        expect(banner, findsOneWidget);
        await tester.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 1200)),
        );
        await tester.pump(const Duration(seconds: 1));
        await tester.pumpAndSettle();
        expect(reads, 3);
        expect(banner, findsNothing);
        expect(
          find.byKey(const ValueKey('consoleStepUp.dialog')),
          findsNothing,
        );
        expect(b.server.to('POST', '/api/v1/auth/step-up/challenge'), isEmpty);
        expect(c.state, isA<ConsoleSignedIn>());
      },
    );

    testWidgets(
      'starting from a person shows the structure and the spaces Sam does not see',
      (tester) async {
        var stored = false;
        final b = signedIn(
          session: () => adminSession(
            viewAsEnds: stored
                ? DateTime.now().add(const Duration(minutes: 15))
                : null,
          ),
        );
        final viewAs = b.server.routes['GET /api/v1/users/$samId/view-as']!;
        b.server.routes['GET /api/v1/users/$samId/view-as'] = (r) {
          stored = true;
          return viewAs(r);
        };
        final c = await pumpConsole(tester, b, location: '/people');
        final admin = ConsoleApp.adminOf(
          tester.element(find.byType(Navigator).first),
        );
        unawaited(
          admin.startViewAs(
            const AdminPersonFixture().sam,
            purpose: 'View as Sam',
          ),
        );
        await tester.pumpAndSettle();
        expect(c.path, '/people/view-as');
        expect(find.byType(ViewAsStructure), findsOneWidget);
        expect(
          find.byKey(const ValueKey('viewAs.space.$supportId')),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('viewAs.channel.$generalId')),
          findsOneWidget,
        );
        expect(find.text('Not visible for Sam: Engineering'), findsOneWidget);
        expect(banner, findsOneWidget);
        // The owner's spaces were read before the view started.
        final order = [
          for (final r in b.server.seen)
            if (r.path == '/api/v1/spaces' ||
                r.path == '/api/v1/users/$samId/view-as')
              r.path,
        ];
        expect(order.first, '/api/v1/spaces');
        // No route of the structure opens content (ADR 0028).
        expect(
          find.byType(InkWell).evaluate().where((e) {
            final w = e.widget as InkWell;
            return w.onTap != null &&
                find
                    .descendant(
                      of: find.byType(ViewAsStructure),
                      matching: find.byWidget(w),
                    )
                    .evaluate()
                    .isNotEmpty;
          }),
          isEmpty,
        );
      },
    );

    testWidgets('a stored view ends before a new one starts', (tester) async {
      var stored = true;
      final b = signedIn(
        session: () => adminSession(
          viewAsEnds: stored
              ? DateTime.now().add(const Duration(minutes: 15))
              : null,
        ),
      );
      b.server.routes['DELETE /api/v1/console/session/view-as'] = (_) {
        stored = false;
        return const Answer(204);
      };
      final viewAs = b.server.routes['GET /api/v1/users/$samId/view-as']!;
      b.server.routes['GET /api/v1/users/$samId/view-as'] = (r) {
        stored = true;
        return viewAs(r);
      };
      await pumpConsole(tester, b, location: '/people');
      final admin = ConsoleApp.adminOf(
        tester.element(find.byType(Navigator).first),
      );
      unawaited(
        admin.startViewAs(
          const AdminPersonFixture().sam,
          purpose: 'View as Sam',
        ),
      );
      await tester.pumpAndSettle();
      final order = [
        for (final r in b.server.seen)
          if (r.method == 'DELETE' || r.path.endsWith('/view-as'))
            '${r.method} ${r.path}',
      ];
      expect(order, [
        'DELETE /api/v1/console/session/view-as',
        'GET /api/v1/users/$samId/view-as',
      ]);
      expect(
        b.server.to('DELETE', '/api/v1/console/session/view-as'),
        hasLength(1),
      );
      expect(find.byType(ViewAsStructure), findsOneWidget);
    });

    testWidgets(
      'after a reload the banner stays and the structure says why it is missing',
      (tester) async {
        final b = signedIn(
          session: () => adminSession(
            viewAsEnds: DateTime.now().add(const Duration(minutes: 5)),
          ),
        );
        await pumpConsole(tester, b, location: '/people/view-as');
        expect(banner, findsOneWidget);
        expect(find.byKey(const ValueKey('viewAs.later')), findsOneWidget);
      },
    );

    testWidgets(
      'after a logout the next login in the tab sees nothing of the view',
      (tester) async {
        var stored = false;
        final b = signedIn(
          session: () => adminSession(
            viewAsEnds: stored
                ? DateTime.now().add(const Duration(minutes: 15))
                : null,
          ),
        );
        final viewAs = b.server.routes['GET /api/v1/users/$samId/view-as']!;
        b.server.routes['GET /api/v1/users/$samId/view-as'] = (r) {
          stored = true;
          return viewAs(r);
        };
        b.server.routes['POST /api/v1/auth/logout'] = (_) {
          stored = false;
          return const Answer(204);
        };
        b.server.routes['POST /api/v1/auth/password/login'] = (_) =>
            Answer(200, adminSession());
        final c = await pumpConsole(tester, b, location: '/people');
        final admin = ConsoleApp.adminOf(
          tester.element(find.byType(Navigator).first),
        );
        unawaited(
          admin.startViewAs(
            const AdminPersonFixture().sam,
            purpose: 'View as Sam',
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(ViewAsStructure), findsOneWidget);

        await tester.tap(find.text('Log out'));
        await tester.pumpAndSettle();
        c.logInAgain();
        unawaited(c.loginWithPassword('robin', 'pw-Aa1!', '492039'));
        await tester.pumpAndSettle();
        expect(c.state, isA<ConsoleSignedIn>());
        expect(admin.snapshot, isNull);
        expect(admin.notVisible, isEmpty);
        expect(c.path, isNot('/people/view-as'));
        c.openRoute(const ConsoleRoute(ConsolePage.people, viewAs: true));
        await tester.pumpAndSettle();
        expect(find.byType(ViewAsStructure), findsNothing);
        expect(find.text('Not visible for Sam: Engineering'), findsNothing);
      },
    );

    testWidgets('a reload after the end of a view keeps the session', (
      tester,
    ) async {
      var reads = 0;
      final b = signedIn();
      b.server.routes['GET /api/v1/console/session'] = (_) {
        reads++;
        // The first request after the end of a stored view ends it and answers this (07 9.2).
        if (reads == 1) {
          return const Answer(401, {'code': 'step_up_required', 'status': 401});
        }
        return Answer(200, adminSession());
      };
      final c = await pumpConsole(tester, b, location: '/people');
      expect(reads, 2);
      expect(c.state, isA<ConsoleSignedIn>());
      expect(find.byType(PeopleList), findsOneWidget);
    });

    testWidgets('a refused view shows the reason of the server', (
      tester,
    ) async {
      final b = signedIn();
      b.server.routes['GET /api/v1/users/$samId/view-as'] = (_) => const Answer(
        403,
        {'code': 'forbidden', 'status': 403, 'reason': 'owner_only'},
      );
      await pumpConsole(tester, b, location: '/people');
      final admin = ConsoleApp.adminOf(
        tester.element(find.byType(Navigator).first),
      );
      unawaited(
        admin.startViewAs(
          const AdminPersonFixture().sam,
          purpose: 'View as Sam',
        ),
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('console.viewAs.failed')),
        findsOneWidget,
      );
      expect(find.byType(ViewAsStructure), findsNothing);
    });
  });

  test(
    'the download target saves the export once with the media type of its name',
    () async {
      final downloads = FakeDownloads();
      final target = downloadSaveTarget(downloads);
      final sink = (await target('audit-2026-10-01-2026-10-08.csv'))!;
      await sink.addStream(
        Stream.fromIterable([
          utf8.encode('occurred_at,action\n'),
          utf8.encode('2026-10-01T09:30:00Z,group.create\n'),
        ]),
      );
      await sink.close();
      await sink.close();
      expect(downloads.saved, hasLength(1));
      expect(downloads.saved.single.type, 'text/csv');
      expect(
        utf8.decode(downloads.saved.single.bytes),
        'occurred_at,action\n2026-10-01T09:30:00Z,group.create\n',
      );
      final nd = (await target('audit.ndjson'))!;
      await nd.addStream(Stream.value(Uint8List.fromList([123, 125])));
      await nd.close();
      expect(downloads.saved.last.type, 'application/x-ndjson');
    },
  );
}

/// The people of the fixtures as admin_ui reads them.
class AdminPersonFixture {
  /// Creates the fixture.
  const AdminPersonFixture();

  /// Sam, an active person.
  AdminPerson get sam => AdminPerson(
    id: samId,
    handle: 'sam',
    displayName: 'Sam',
    status: AdminPersonStatus.active,
    isOwner: false,
    createdAt: DateTime.utc(2026, 10, 10, 9, 30),
  );
}
