// SPDX-License-Identifier: Apache-2.0

/// The labels and names around grants: levels, capabilities, targets, days, and the names of
/// the people, groups, agents, spaces and channels an answer names only by ID
/// (docs/spec/07-permissions.md 9.1, 9.3; brief M4-27 Design notes 1 and 2).
///
/// Labels only: the sentence of an effect is the server's (07 9.3).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import '../data/admin_repository.dart';

/// The label of [level], for example "Trigger agents".
String levelLabel(AppLocalizations l, AccessLevel level) => switch (level) {
  AccessLevel.none => l.adminLevelNone,
  AccessLevel.read => l.adminLevelRead,
  AccessLevel.write => l.adminLevelWrite,
  AccessLevel.trigger => l.adminLevelTrigger,
};

/// The label of [capability], for example "Upload files".
String capabilityLabel(AppLocalizations l, AccessCapability capability) =>
    switch (capability) {
      AccessCapability.approveActions => l.capabilityApproveActions,
      AccessCapability.upload => l.adminCapabilityUpload,
      AccessCapability.export => l.adminCapabilityExport,
      AccessCapability.manage => l.adminCapabilityManage,
      AccessCapability.terminal => l.adminCapabilityTerminal,
    };

/// The level with its capabilities, for example "Trigger agents, Upload files".
String levelWithCapabilities(
  AppLocalizations l,
  AccessLevel level,
  List<AccessCapability> capabilities,
) => [
  levelLabel(l, level),
  for (final c in capabilities) capabilityLabel(l, c),
].join(', ');

/// The label of [target], for example "Channel Marketing/Kampagnen".
String targetLabel(AppLocalizations l, AdminTarget target) =>
    switch (target.kind) {
      AdminTargetKind.instance => l.adminTargetInstance,
      AdminTargetKind.space => l.adminTargetSpace(target.name),
      AdminTargetKind.channel => l.adminTargetChannel(target.path),
    };

/// The last day of a validity that ends at [end] (exclusive), seen in [timeZone] relative to
/// [now]: "Dec 31, 2026" for an end at the start of 2027-01-01.
String lastDayLabel(
  BuildContext context,
  DateTime end, {
  required DateTime now,
  required String timeZone,
}) {
  final format = L10nFormat(Localizations.localeOf(context));
  final last = end.subtract(const Duration(seconds: 1));
  try {
    return format.relativeDay(last, now: now, timeZone: timeZone);
  } on ArgumentError {
    return format.relativeDay(last, now: now, timeZone: 'UTC');
  }
}

/// The names of what the grant answers name by ID, loaded from the directory lists the caller
/// may read (`listUsers`, `listGroups`, `listAgents`, `listSpaces`, `listSpaceChannels`). A
/// list the server refuses stays empty, and its IDs show as they are.
final class AccessNames {
  /// Creates the names from loaded lists.
  AccessNames({
    this.people = const [],
    this.groups = const [],
    this.agents = const [],
    this.targets = const [],
  });

  /// Loads every list of [repository]; a refused list stays empty.
  static Future<AccessNames> load(AdminRepository repository) async {
    Future<List<T>> orEmpty<T>(Future<List<T>> Function() load) async {
      try {
        return await load();
      } on Object {
        return const [];
      }
    }

    Future<List<AdminGroup>> allGroups() async {
      final out = <AdminGroup>[];
      String? cursor;
      do {
        final page = await repository.groups(cursor: cursor);
        out.addAll(page.items);
        cursor = page.nextCursor;
      } while (cursor != null);
      return out;
    }

    return AccessNames(
      people: await orEmpty(() => allPeople(repository)),
      groups: await orEmpty(allGroups),
      agents: await orEmpty(repository.agents),
      targets: await orEmpty(repository.targets),
    );
  }

  /// The people.
  final List<AdminPerson> people;

  /// The groups.
  final List<AdminGroup> groups;

  /// The agents.
  final List<AdminAgent> agents;

  /// The spaces and channels.
  final List<AdminTarget> targets;

  /// The name of the person or agent [id], or [id].
  String principal(String id) =>
      people.where((p) => p.id == id).firstOrNull?.displayName ??
      agents.where((a) => a.id == id).firstOrNull?.displayName ??
      id;

  /// The name of the group [id], or [id].
  String group(String id) =>
      groups.where((g) => g.id == id).firstOrNull?.name ?? id;

  /// The path of the space or channel [id] (`Marketing/Kampagnen`), or [id].
  String target(String id) =>
      targets.where((t) => t.id == id).firstOrNull?.path ?? id;
}
