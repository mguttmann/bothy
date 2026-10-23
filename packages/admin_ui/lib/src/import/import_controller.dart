// SPDX-License-Identifier: Apache-2.0

/// The state of the import screens: the latest snapshot per source and, for an open source,
/// its channels, failed items and agents (brief M8-08 Design notes 1 and 3; 15 3.1 rule 3).
library;

import 'package:flutter/foundation.dart';

import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import 'import_repository.dart';

/// Where a mapped channel stands (22 13.1), as its row shows it.
enum ImportChannelPhase {
  /// The channel item is not `posted` yet: no lead action (the server would answer `409
  /// conflict` with `reason` `channel_not_imported`).
  notImported,

  /// The source leads; the channel here is the read-only mirror.
  mirror,

  /// This server leads and the operator is the planned one.
  hereOperatorOk,

  /// This server leads and the channel has no operator although one is planned.
  hereNoOperator,

  /// This server leads and the operator is another agent than the planned one (R-IM-08).
  hereOtherOperator,
}

/// One row of the channel table: the mapped channel, its posted channel item, the channel of
/// this server and the planned agent.
final class ImportChannelRow {
  /// Creates the row.
  const ImportChannelRow({
    required this.mapped,
    this.item,
    this.channel,
    this.channelError,
    this.plannedAgent,
  });

  /// The entry of the mapping.
  final MappedChannel mapped;

  /// The `posted` channel item (`channel:<discord id>`), or null while not imported.
  final ImportItemEntry? item;

  /// The channel of this server, or null (not imported, or `getChannel` failed).
  final ImportChannelState? channel;

  /// The failure of `getChannel`, or null.
  final Object? channelError;

  /// The agent of `planned_operator`, or null for `none` or an agent that is not listed.
  final AgentRecord? plannedAgent;

  /// The name of the channel here: the channel's name, else the name the mapping creates, else null
  /// (an existing channel that could not be read).
  String? get targetName => channel?.name ?? mapped.createName;

  /// Whether the channel is imported (its channel item is `posted`).
  bool get imported => item != null;

  /// The phase of the row; the lead is the stored one of the snapshot.
  ImportChannelPhase get phase {
    if (!imported) return ImportChannelPhase.notImported;
    if (mapped.lead == ImportLead.discord) return ImportChannelPhase.mirror;
    final operator = channel?.operatorAgentId;
    if (operator == null) {
      return mapped.plannedAgentHandle == null
          ? ImportChannelPhase.hereOperatorOk
          : ImportChannelPhase.hereNoOperator;
    }
    return operator == plannedAgent?.id
        ? ImportChannelPhase.hereOperatorOk
        : ImportChannelPhase.hereOtherOperator;
  }
}

/// The details of one source beyond its snapshot.
final class ImportSourceDetails {
  /// Creates the details.
  const ImportSourceDetails({
    required this.rows,
    required this.failedItems,
    required this.agents,
  });

  /// One row per mapped channel whose target is not `skip`, in the mapping's order.
  final List<ImportChannelRow> rows;

  /// The items in state `failed`.
  final List<ImportItemEntry> failedItems;

  /// The agents the server lists (for the operator step).
  final List<AgentRecord> agents;

  /// Whether completion may be offered: every imported channel is led here (22 13.5).
  bool get allImportedLedHere =>
      rows.every((r) => !r.imported || r.mapped.lead == ImportLead.here);
}

/// Holds the latest [ImportSourceSnapshot] per source ID and the details of the open source.
///
/// A snapshot comes from `getImport` (after an action, on the refresh control and when the tab
/// becomes visible) or, in a client with a gateway, from the event `import.progress`, whose
/// payload is the same `ImportSource`; [apply] takes either. The newest snapshot always wins,
/// also one whose counters are lower than the shown ones, because the server is the source of
/// the counters (brief M8-08 Design note 1). There is no timer: the console has no gateway and
/// no configuration key sets an interval (fixed requirement 16).
///
/// Example:
/// ```dart
/// final c = ImportController(repository);
/// await c.loadList();
/// await c.refresh(c.sources.first.id);
/// ```
class ImportController extends ChangeNotifier {
  /// Creates the controller over [repository].
  ImportController(this.repository);

  /// The administration's calls.
  final AdminRepository repository;

  final Map<String, ImportSourceSnapshot> _byId = {};
  final List<String> _order = [];
  final Map<String, ImportSourceDetails> _details = {};

  /// The sources in the order of the list (newest first), each in its latest snapshot.
  List<ImportSourceSnapshot> get sources => [
    for (final id in _order) ?_byId[id],
  ];

  /// The latest snapshot of [id], or null.
  ImportSourceSnapshot? source(String id) => _byId[id];

  /// The details of [id] from its last [refresh], or null.
  ImportSourceDetails? details(String id) => _details[id];

  /// Takes [snapshot] as the newest state of its source and tells the listeners.
  void apply(ImportSourceSnapshot snapshot) {
    _byId[snapshot.id] = snapshot;
    if (!_order.contains(snapshot.id)) _order.insert(0, snapshot.id);
    notifyListeners();
  }

  /// Reads the event payload [json] of `import.progress` (an `ImportSource`) and applies it.
  ///
  /// Throws a [FormatException] for a payload without the required members.
  void applyProgress(Map<String, Object?> json) =>
      apply(ImportSourceSnapshot.fromJson(json));

  /// Loads every page of `listImports` and replaces the list.
  ///
  /// Errors of the repository pass to the caller.
  Future<void> loadList() async {
    final all = <ImportSourceSnapshot>[];
    String? cursor;
    do {
      final page = await repository.importSources(cursor: cursor);
      all.addAll(page.items);
      cursor = page.nextCursor;
    } while (cursor != null);
    _order
      ..clear()
      ..addAll(all.map((s) => s.id));
    for (final s in all) {
      _byId[s.id] = s;
    }
    notifyListeners();
  }

  /// Reads the source [id] (`getImport`) and its details: the channel items, the failed items,
  /// the agents and the channel of every imported row (`getChannel`; a failure stays in the
  /// row).
  ///
  /// Errors of `getImport` and the item lists pass to the caller; the agents are optional (a
  /// refused list leaves the planned agent unresolved).
  Future<void> refresh(String id) async {
    final snapshot = await repository.importSource(id);
    final items = await repository.importItems(id);
    List<AgentRecord> agents;
    try {
      agents = await repository.agentRecords();
    } on Object {
      agents = const [];
    }
    final posted = <String, ImportItemEntry>{
      for (final i in items)
        if (i.itemKind == 'channel' &&
            i.state == 'posted' &&
            i.originKey.startsWith('channel:'))
          i.originKey.substring('channel:'.length): i,
    };
    final rows = <ImportChannelRow>[];
    for (final m in snapshot.channels) {
      final item = posted[m.discordId];
      final planned = agents
          .where((a) => a.handle == m.plannedAgentHandle)
          .firstOrNull;
      ImportChannelState? channel;
      Object? error;
      final target = item?.targetId;
      if (target != null) {
        try {
          channel = await repository.importChannel(target);
        } on Object catch (e) {
          error = e;
        }
      }
      rows.add(
        ImportChannelRow(
          mapped: m,
          item: item,
          channel: channel,
          channelError: error,
          plannedAgent: planned,
        ),
      );
    }
    _details[id] = ImportSourceDetails(
      rows: rows,
      failedItems: [
        for (final i in items)
          if (i.state == 'failed') i,
      ],
      agents: agents,
    );
    apply(snapshot);
  }
}
