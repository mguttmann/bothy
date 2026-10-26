// SPDX-License-Identifier: Apache-2.0

/// The places of the administration and their paths below the app's `/admin` (desktop) or the
/// console's root (docs/spec/01-product.md 5.2 A1; brief M4-26 Scope).
library;

/// The entries of A1 in their order (01 5.2). Only an entry with a screen is listed; the later
/// tickets add theirs (grants M4-27, agents M4-28, audit M4-29, hosts M4-37, permission hints
/// and expiring access M6-07, apps M6-08, statistics M6-06, instance M6-09, status page M7-21,
/// import M8-08).
enum AdminSection {
  /// People, owners and their details.
  people('people', hasScreen: true),

  /// Groups and their members.
  groups('groups', hasScreen: true),

  /// Grants per target, with "Who has access?" (M4-27).
  grants('grants', hasScreen: true),

  /// The list "Permission hints" of 07 9.5 (M6-07).
  hints('hints', hasScreen: true),

  /// The list "Expiring soon" of 07 9.4 (M6-07).
  expiring('expiring', hasScreen: true),

  /// The queue of waiting devices.
  devices('devices', hasScreen: true),

  /// Agents with their backend switch and tokens (M4-28).
  agents('agents', hasScreen: true),

  /// Apps with their consent, lifecycle and outgoing webhooks (M6-08).
  apps('apps', hasScreen: true),

  /// The incoming webhooks per channel (M6-08); the channel is chosen on the screen, never in
  /// the path.
  webhooks('incoming-webhooks', hasScreen: true),

  /// The audit log (M4-29).
  audit('audit', hasScreen: true),

  /// Statistics, A7 (M6-06).
  statistics('statistics', hasScreen: true),

  /// Instance settings, branding, login providers and feature flags, for owners (M6-09).
  instance('instance', hasScreen: true),

  /// The status page with the owner forms of `status.manage` (M7-21).
  status('status', hasScreen: true),

  /// Hosts of the terminal, for owners (M4-37).
  hosts('hosts', hasScreen: true),

  /// The import sources, their channels and the owner steps of `import.manage` (M8-08). Web
  /// only (01 3.7, row "Import from Discord"): the console hosts it on its page "Import"; the
  /// desktop shell does not list it.
  dataImport('import', hasScreen: true, webOnly: true);

  const AdminSection(
    this.segment, {
    this.hasScreen = false,
    this.webOnly = false,
  });

  /// The path segment, for example `people`.
  final String segment;

  /// Whether this build has a screen for the entry; the shell lists only these.
  final bool hasScreen;

  /// Whether only the web console offers the entry (01 3.7); [listed] leaves it out, so the
  /// desktop shell neither lists nor opens it.
  final bool webOnly;

  /// The entries the desktop shell lists.
  static List<AdminSection> get listed => [
    for (final s in values)
      if (s.hasScreen && !s.webOnly) s,
  ];
}

/// A place in the administration: a [section] and, for a detail, the [id] of its person or
/// group, or the key of the open target of the grants (`channel.<id>`, `space.<id>`,
/// `instance`).
///
/// Example: `AdminLocation.parse(['people', id])` is the detail of that person;
/// `AdminLocation(AdminSection.groups).path` is `/groups`.
final class AdminLocation {
  /// Creates the location.
  const AdminLocation(this.section, {this.id});

  /// The start of the administration: the people list.
  static const AdminLocation start = AdminLocation(AdminSection.people);

  /// The section.
  final AdminSection section;

  /// The person (in [AdminSection.people]), group (in [AdminSection.groups]), target key
  /// (in [AdminSection.grants]), agent (in [AdminSection.agents]), app (in
  /// [AdminSection.apps]), host (in [AdminSection.hosts]) or import source (in
  /// [AdminSection.dataImport]), or null for the list.
  final String? id;

  /// Parses the path [segments] below the administration's root; null for a path that names no
  /// screen of this build (the app shows "Not found or no access", 01 6.7). No segment is the
  /// [start].
  static AdminLocation? parse(List<String> segments) {
    final parts = segments.where((s) => s.isNotEmpty).toList();
    if (parts.isEmpty) return start;
    final section = AdminSection.listed
        .where((s) => s.segment == parts.first)
        .firstOrNull;
    if (section == null || parts.length > 2) return null;
    if (parts.length == 2) {
      // These lists have no detail; their filters stay in the screen.
      if (const {
        AdminSection.devices,
        AdminSection.audit,
        AdminSection.webhooks,
        AdminSection.hints,
        AdminSection.expiring,
        AdminSection.instance,
        AdminSection.statistics,
        AdminSection.status,
      }.contains(section)) {
        return null;
      }
      return AdminLocation(section, id: parts[1]);
    }
    return AdminLocation(section);
  }

  /// The path below the root, for example `/people/<id>`.
  String get path =>
      id == null ? '/${section.segment}' : '/${section.segment}/$id';

  @override
  bool operator ==(Object other) =>
      other is AdminLocation && other.section == section && other.id == id;

  @override
  int get hashCode => Object.hash(section, id);

  @override
  String toString() => 'AdminLocation($path)';
}
