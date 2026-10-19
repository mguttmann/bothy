// SPDX-License-Identifier: Apache-2.0

/// The places of the administration and their paths below the app's `/admin` (desktop) or the
/// console's root (docs/spec/01-product.md 5.2 A1; brief M4-26 Scope).
library;

/// The entries of A1 in their order (01 5.2). Only an entry with a screen is listed; the later
/// tickets add theirs (grants M4-27, agents M4-28, audit M4-29, hosts M4-37, apps M6-08,
/// statistics M6-06, instance M6-09).
enum AdminSection {
  /// People, owners and their details.
  people('people', hasScreen: true),

  /// Groups and their members.
  groups('groups', hasScreen: true),

  /// Grants per target, with "Who has access?" (M4-27).
  grants('grants', hasScreen: true),

  /// The queue of waiting devices.
  devices('devices', hasScreen: true),

  /// Agents (M4-28).
  agents('agents'),

  /// Apps (M6-08).
  apps('apps'),

  /// The audit log (M4-29).
  audit('audit', hasScreen: true),

  /// Statistics (M6-06).
  statistics('statistics'),

  /// Instance settings (M6-09).
  instance('instance'),

  /// Hosts of the terminal, for owners (M4-37).
  hosts('hosts', hasScreen: true);

  const AdminSection(this.segment, {this.hasScreen = false});

  /// The path segment, for example `people`.
  final String segment;

  /// Whether this build has a screen for the entry; the shell lists only these.
  final bool hasScreen;

  /// The entries the shell lists.
  static List<AdminSection> get listed => [
    for (final s in values)
      if (s.hasScreen) s,
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
  /// (in [AdminSection.grants]) or host (in [AdminSection.hosts]), or null for the list.
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
      if (section == AdminSection.devices || section == AdminSection.audit) {
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
