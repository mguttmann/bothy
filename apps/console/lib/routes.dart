// SPDX-License-Identifier: Apache-2.0

/// The route table of the console (brief M6-02 Design note 1, docs/DECISIONS.md D-M6-02-1).
library;

import 'package:l10n/l10n.dart';

/// The base path the server serves the console under; the routes are relative to it.
const String consoleBasePath = '/console';

/// Every page of the console: the login and step-up pages and one page per navigation entry of
/// docs/spec/01-product.md 5.5. This enum is the whole router configuration; there is no route
/// for channels, threads or messages (ADR 0028), and `routes_test.dart` holds that.
enum ConsolePage {
  /// E2, the login.
  login('/login'),

  /// Target of the OIDC login redirect (`app_redirect_uri`).
  loginCallback('/login/callback'),

  /// The step-up after the login (08 7.5).
  stepUp('/step-up'),

  /// Target of the OIDC re-login of the step-up.
  stepUpCallback('/step-up/callback'),

  /// E7, logged out.
  loggedOut('/logged-out'),

  /// Live.
  live('/live'),

  /// Statistics.
  stats('/stats'),

  /// Hosts and terminal.
  hosts('/hosts'),

  /// People.
  people('/people'),

  /// Groups and access grants.
  groups('/groups'),

  /// Devices.
  devices('/devices'),

  /// Agents and costs.
  agents('/agents'),

  /// Apps and webhooks.
  apps('/apps'),

  /// Audit.
  audit('/audit'),

  /// Import. Not named `import`: a line that starts with `import` is a directive for
  /// tools/arch-check, which would read the path as an import URI.
  dataImport('/import'),

  /// Instance and branding.
  instance('/instance'),

  /// Status.
  status('/status'),

  /// Operations (Grafana): the dashboard of `console.grafana_embed_url` in an iframe, only for
  /// owners and only when the key is set (docs/spec/25-visualization.md 9.2, brief M6-17).
  grafana('/grafana');

  const ConsolePage(this.path);

  /// The path relative to [consoleBasePath], as the router sees it below the `<base href>`.
  final String path;

  /// The path in the browser, for example `/console/stats`.
  String get fullPath => '$consoleBasePath$path';

  /// Whether the page is an administration page, which needs a session with a step-up.
  bool get isAdministration => index >= ConsolePage.live.index;

  /// The page of [path] (relative to the base, without query), or null for any other path.
  static ConsolePage? fromPath(String path) {
    for (final page in values) {
      if (page.path == path) return page;
    }
    return null;
  }
}

/// The navigation entries in the order of 01 5.5.
const List<ConsolePage> consoleNavigation = [
  ConsolePage.live,
  ConsolePage.stats,
  ConsolePage.hosts,
  ConsolePage.people,
  ConsolePage.groups,
  ConsolePage.devices,
  ConsolePage.agents,
  ConsolePage.apps,
  ConsolePage.audit,
  ConsolePage.dataImport,
  ConsolePage.instance,
  ConsolePage.status,
];

/// The navigation entries of a console: [consoleNavigation], followed by
/// [ConsolePage.grafana] when [grafana] is true, that is when `getInstance` names
/// `grafana_embed_url` (owners, with `console.grafana_embed_url` set; 25 9.2 item 6).
List<ConsolePage> consoleNavigationFor({required bool grafana}) =>
    grafana ? [...consoleNavigation, ConsolePage.grafana] : consoleNavigation;

/// The text of the navigation entry of [page] in [l].
///
/// Throws an [ArgumentError] for a page that is not in [consoleNavigation] and not
/// [ConsolePage.grafana].
String navigationLabel(AppLocalizations l, ConsolePage page) => switch (page) {
  ConsolePage.live => l.consoleNavLive,
  ConsolePage.stats => l.consoleNavStats,
  ConsolePage.hosts => l.consoleNavHosts,
  ConsolePage.people => l.consoleNavPeople,
  ConsolePage.groups => l.consoleNavGroups,
  ConsolePage.devices => l.consoleNavDevices,
  ConsolePage.agents => l.consoleNavAgents,
  ConsolePage.apps => l.consoleNavApps,
  ConsolePage.audit => l.consoleNavAudit,
  ConsolePage.dataImport => l.consoleNavImport,
  ConsolePage.instance => l.consoleNavInstance,
  ConsolePage.status => l.consoleNavStatus,
  ConsolePage.grafana => l.consoleNavGrafana,
  _ => throw ArgumentError.value(page, 'page', 'not a navigation entry'),
};
