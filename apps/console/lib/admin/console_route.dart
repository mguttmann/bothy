// SPDX-License-Identifier: Apache-2.0

/// The places of the administration below the pages of the console (brief M6-04 Design note 1,
/// docs/decisions.d/M6-04.md D-M6-04-1): the pages of M6-02 host the screens of `admin_ui`, and a
/// detail adds the ID of its person, group, agent or app to the page's path (M6-08 adds the apps
/// and the incoming webhooks below the apps page).
library;

import 'package:admin_ui/admin_ui.dart';

import '../routes.dart';

/// The path segment of "View as person" below the people page.
const String viewAsSegment = 'view-as';

/// The path segment of the access grants below the groups page.
const String grantsSegment = 'grants';

/// The path segment of the incoming webhooks below the apps page (brief M6-08); the channel is
/// chosen on the screen and never enters the path (ADR 0028).
const String incomingWebhooksSegment = 'incoming-webhooks';

/// One place of the console: a [page] of the route table and, on an administration page with a
/// screen of `admin_ui`, the [admin] place it shows, or the stored view ([viewAs]).
///
/// The target of the access grants stays out of the path: its key names a channel or a space,
/// and the console has no route for content (ADR 0028), so `/groups/grants` is the path of every
/// grants place and the target lives only in the app.
///
/// Example: `ConsoleRoute.parse('/people/<id>')!.admin` is the detail of that person;
/// `ConsoleRoute.forAdmin(const AdminLocation(AdminSection.grants)).path` is `/groups/grants`.
final class ConsoleRoute {
  /// Creates the place.
  const ConsoleRoute(this.page, {this.admin, this.viewAs = false});

  /// The page of the route table (and the entry of the navigation).
  final ConsolePage page;

  /// The place of the administration the page shows, or null for a page without one.
  final AdminLocation? admin;

  /// Whether the place is the structure of the stored "View as person" (07 9.2).
  final bool viewAs;

  /// The place a page opens at: the list of its screen, or no place for a page whose screen
  /// comes with a later ticket (01 5.5).
  factory ConsoleRoute.ofPage(ConsolePage page) =>
      ConsoleRoute(page, admin: _start[page]);

  /// The place of [location]: the page whose navigation entry holds its section.
  factory ConsoleRoute.forAdmin(AdminLocation location) =>
      ConsoleRoute(switch (location.section) {
        AdminSection.people => ConsolePage.people,
        AdminSection.groups || AdminSection.grants => ConsolePage.groups,
        AdminSection.devices => ConsolePage.devices,
        AdminSection.agents => ConsolePage.agents,
        AdminSection.audit => ConsolePage.audit,
        AdminSection.hosts => ConsolePage.hosts,
        AdminSection.apps || AdminSection.webhooks => ConsolePage.apps,
        AdminSection.statistics => ConsolePage.stats,
        AdminSection.instance => ConsolePage.instance,
      }, admin: location);

  /// The first place of each page that hosts a screen of `admin_ui` in this build.
  static const Map<ConsolePage, AdminLocation> _start = {
    ConsolePage.people: AdminLocation.start,
    ConsolePage.groups: AdminLocation(AdminSection.groups),
    ConsolePage.devices: AdminLocation(AdminSection.devices),
    ConsolePage.agents: AdminLocation(AdminSection.agents),
    ConsolePage.audit: AdminLocation(AdminSection.audit),
    ConsolePage.apps: AdminLocation(AdminSection.apps),
  };

  /// The pages whose detail has an ID in the path, with the section of the detail.
  static const Map<ConsolePage, AdminSection> _details = {
    ConsolePage.people: AdminSection.people,
    ConsolePage.groups: AdminSection.groups,
    ConsolePage.agents: AdminSection.agents,
    ConsolePage.apps: AdminSection.apps,
  };

  /// The place of [path] (relative to the console's base, without query), or null for a path
  /// that names nothing.
  static ConsoleRoute? parse(String path) {
    final page = ConsolePage.fromPath(path);
    if (page != null) return ConsoleRoute.ofPage(page);
    final parts = path.split('/').where((s) => s.isNotEmpty).toList();
    if (parts.length != 2) return null;
    final parent = ConsolePage.fromPath('/${parts.first}');
    if (parent == null) return null;
    final child = parts[1];
    if (parent == ConsolePage.people && child == viewAsSegment) {
      return const ConsoleRoute(ConsolePage.people, viewAs: true);
    }
    if (parent == ConsolePage.groups && child == grantsSegment) {
      return const ConsoleRoute(
        ConsolePage.groups,
        admin: AdminLocation(AdminSection.grants),
      );
    }
    if (parent == ConsolePage.apps && child == incomingWebhooksSegment) {
      return const ConsoleRoute(
        ConsolePage.apps,
        admin: AdminLocation(AdminSection.webhooks),
      );
    }
    final section = _details[parent];
    if (section == null) return null;
    return ConsoleRoute(parent, admin: AdminLocation(section, id: child));
  }

  /// The path relative to the console's base, for example `/people/<id>`.
  String get path {
    final at = admin;
    if (viewAs) return '${ConsolePage.people.path}/$viewAsSegment';
    if (at == null) return page.path;
    if (at.section == AdminSection.grants) {
      return '${ConsolePage.groups.path}/$grantsSegment';
    }
    if (at.section == AdminSection.webhooks) {
      return '${ConsolePage.apps.path}/$incomingWebhooksSegment';
    }
    final id = at.id;
    return id == null ? page.path : '${page.path}/$id';
  }

  /// The path in the browser, for example `/console/people/<id>`.
  String get fullPath => '$consoleBasePath$path';

  @override
  bool operator ==(Object other) =>
      other is ConsoleRoute &&
      other.page == page &&
      other.admin == admin &&
      other.viewAs == viewAs;

  @override
  int get hashCode => Object.hash(page, admin, viewAs);

  @override
  String toString() => 'ConsoleRoute($path)';
}
