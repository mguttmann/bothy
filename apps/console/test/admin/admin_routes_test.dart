// SPDX-License-Identifier: Apache-2.0

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:flutter_test/flutter_test.dart';

const _id = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';

void main() {
  group('the administration pages host the places of admin_ui', () {
    test('each page of A1 to A6 starts at its list', () {
      expect(ConsoleRoute.parse('/people')!.admin, AdminLocation.start);
      expect(
        ConsoleRoute.parse('/groups')!.admin,
        const AdminLocation(AdminSection.groups),
      );
      expect(
        ConsoleRoute.parse('/devices')!.admin,
        const AdminLocation(AdminSection.devices),
      );
      expect(
        ConsoleRoute.parse('/agents')!.admin,
        const AdminLocation(AdminSection.agents),
      );
      expect(
        ConsoleRoute.parse('/audit')!.admin,
        const AdminLocation(AdminSection.audit),
      );
    });

    test('details of people, groups and agents have a path', () {
      expect(
        ConsoleRoute.parse('/people/$_id'),
        ConsoleRoute(
          ConsolePage.people,
          admin: const AdminLocation(AdminSection.people, id: _id),
        ),
      );
      expect(
        ConsoleRoute.parse('/groups/$_id')!.admin,
        const AdminLocation(AdminSection.groups, id: _id),
      );
      expect(
        ConsoleRoute.parse('/agents/$_id')!.admin,
        const AdminLocation(AdminSection.agents, id: _id),
      );
      for (final r in [
        ConsoleRoute.parse('/people/$_id')!,
        ConsoleRoute.parse('/groups/$_id')!,
        ConsoleRoute.parse('/agents/$_id')!,
      ]) {
        expect(ConsoleRoute.parse(r.path), r);
      }
    });

    test('the grants belong to the groups page and keep their target out of the path', () {
      final grants = ConsoleRoute.parse('/groups/grants')!;
      expect(grants.page, ConsolePage.groups);
      expect(grants.admin, const AdminLocation(AdminSection.grants));
      final target = ConsoleRoute(
        ConsolePage.groups,
        admin: const AdminLocation(AdminSection.grants, id: 'channel.$_id'),
      );
      expect(target.path, '/groups/grants');
      expect(ConsoleRoute.forAdmin(target.admin!).page, ConsolePage.groups);
    });

    test('apps have a list and a detail, the incoming webhooks a place of their own', () {
      expect(
        ConsoleRoute.parse('/apps')!.admin,
        const AdminLocation(AdminSection.apps),
      );
      expect(
        ConsoleRoute.parse('/apps/$_id')!.admin,
        const AdminLocation(AdminSection.apps, id: _id),
      );
      final incoming = ConsoleRoute.parse('/apps/incoming-webhooks')!;
      expect(incoming.page, ConsolePage.apps);
      expect(incoming.admin, const AdminLocation(AdminSection.webhooks));
      expect(incoming.path, '/apps/incoming-webhooks');
      expect(
        ConsoleRoute.forAdmin(const AdminLocation(AdminSection.webhooks)),
        incoming,
      );
      expect(ConsoleRoute.parse('/apps/incoming-webhooks/$_id'), isNull);
      for (final r in [ConsoleRoute.parse('/apps/$_id')!, incoming]) {
        expect(ConsoleRoute.parse(r.path), r);
      }
    });

    test(
      'the permission hints and expiring access belong to the groups page',
      () {
        final hints = ConsoleRoute.parse('/groups/hints')!;
        expect(hints.page, ConsolePage.groups);
        expect(hints.admin, const AdminLocation(AdminSection.hints));
        final expiring = ConsoleRoute.parse('/groups/expiring')!;
        expect(expiring.page, ConsolePage.groups);
        expect(expiring.admin, const AdminLocation(AdminSection.expiring));
        for (final section in [AdminSection.hints, AdminSection.expiring]) {
          final route = ConsoleRoute.forAdmin(AdminLocation(section));
          expect(route.page, ConsolePage.groups, reason: section.name);
          expect(route.path, '/groups/${section.segment}');
          expect(ConsoleRoute.parse(route.path), route);
        }
      },
    );

    test('view as person has its own place below people', () {
      final r = ConsoleRoute.parse('/people/view-as')!;
      expect(r.page, ConsolePage.people);
      expect(r.viewAs, isTrue);
      expect(r.path, '/people/view-as');
    });

    test('pages without a screen of this ticket have no place', () {
      for (final p in [
        ConsolePage.live,
        ConsolePage.stats,
        ConsolePage.hosts,
      ]) {
        expect(ConsoleRoute.parse(p.path)!.admin, isNull, reason: p.path);
      }
    });

    test('a path that names nothing gives null', () {
      for (final path in [
        '/people/$_id/extra',
        '/devices/$_id',
        '/audit/$_id',
        '/stats/$_id',
        '/login/$_id',
        '/channels/$_id',
        '/threads/$_id',
        '/',
      ]) {
        expect(ConsoleRoute.parse(path), isNull, reason: path);
      }
    });

    test('every place of the administration maps to its navigation entry', () {
      final pages = {
        AdminSection.people: ConsolePage.people,
        AdminSection.groups: ConsolePage.groups,
        AdminSection.grants: ConsolePage.groups,
        AdminSection.hints: ConsolePage.groups,
        AdminSection.expiring: ConsolePage.groups,
        AdminSection.devices: ConsolePage.devices,
        AdminSection.agents: ConsolePage.agents,
        AdminSection.audit: ConsolePage.audit,
        AdminSection.hosts: ConsolePage.hosts,
        AdminSection.apps: ConsolePage.apps,
        AdminSection.webhooks: ConsolePage.apps,
      };
      for (final e in pages.entries) {
        expect(
          ConsoleRoute.forAdmin(AdminLocation(e.key)).page,
          e.value,
          reason: e.key.name,
        );
      }
    });
  });

  // ADR 0028 "Confirmation": no path of the administration names content.
  test('no administration path names a channel, thread, message or space', () {
    const forbidden = [
      'channel',
      'thread',
      'message',
      'space',
      '/t/',
      '/c/',
      '/s/',
    ];
    final routes = [
      for (final s in AdminSection.listed) ...[
        ConsoleRoute.forAdmin(AdminLocation(s)),
        ConsoleRoute.forAdmin(
          AdminLocation(
            s,
            // The grants keep the key of their target, which names a channel or space.
            id: s == AdminSection.grants ? 'channel.$_id' : _id,
          ),
        ),
      ],
      ConsoleRoute.parse('/people/view-as')!,
    ];
    for (final r in routes) {
      final path = '${r.fullPath}/';
      for (final word in forbidden) {
        expect(path.contains(word), isFalse, reason: '$path contains $word');
      }
    }
  });
}
