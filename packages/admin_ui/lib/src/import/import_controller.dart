// SPDX-License-Identifier: Apache-2.0

/// The state of the import screens: the latest snapshot per source and, for an open source,
/// its channels, failed items and agents (brief M8-08 Design notes 1 and 3; 15 3.1 rule 3).
library;

import 'dart:async';

import 'package:flutter/foundation.dart';

import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../data/self_repository.dart' show SelfActionFailed;
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

  /// Whether the imported channel is no longer live here: archived or deleted, in an archived
  /// space, or `getChannel` answered `404` (gone for good). Its lead can no longer change and it
  /// does not block completion (22 13.5).
  bool get notLive {
    if (!imported) return false;
    final c = channel;
    if (c != null) return !c.live;
    final e = channelError;
    return e is SelfActionFailed && e.problem?.status == 404;
  }

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

  /// Whether completion may be offered: every imported channel that is still live here is led
  /// here (22 13.5).
  bool get allImportedLedHere => rows.every(
    (r) => !r.imported || r.notLive || r.mapped.lead == ImportLead.here,
  );
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
/// One pass of [refresh] runs per source at a time, and it reads only the items the page
/// shows as far as `listImportItems` lets it (ruling AUD-AM1/5).
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

  /// The running pass of [refresh] per source, and the sources asked for again meanwhile.
  final Map<String, Future<void>> _running = {};
  final Set<String> _again = {};
  bool _disposed = false;

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }

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
  /// One pass runs per source at a time: a call while a pass runs gets the future of that pass,
  /// and one more pass follows it, so what changed meanwhile reaches the view and the requests of
  /// an action, the refresh control and a tab that becomes visible never read side by side.
  ///
  /// Errors of `getImport` and the item lists pass to the caller, and to every caller that
  /// waits for the same pass; the agents are optional (a refused list leaves the planned agent
  /// unresolved).
  Future<void> refresh(String id) {
    final running = _running[id];
    if (running != null) {
      _again.add(id);
      return running;
    }
    return _running[id] = _passes(id);
  }

  Future<void> _passes(String id) async {
    try {
      do {
        _again.remove(id);
        await _read(id);
      } while (_again.contains(id) && !_disposed);
    } finally {
      _again.remove(id);
      // The futures of the callers are done when this one is, so the next call starts anew.
      unawaited(_running.remove(id));
    }
  }

  /// The posted channel item of every mapped channel of [snapshot], by Discord ID.
  ///
  /// `listImportItems` filters by state but not by item kind (docs/decisions.d/M8-08.md
  /// D-M8-08-1), so the posted items are read page by page, oldest first, only until every
  /// mapped channel has its item: the import tool writes the channels before their content, so
  /// that is the first pages of an import whose channels exist. While a mapped channel has no
  /// posted item the read still goes through every posted item.
  Future<Map<String, ImportItemEntry>> _postedChannels(
    ImportSourceSnapshot snapshot,
  ) async {
    final wanted = {for (final m in snapshot.channels) m.discordId};
    final posted = <String, ImportItemEntry>{};
    if (wanted.isEmpty) return posted;
    await repository.importItems(
      snapshot.id,
      state: 'posted',
      enough: (page) {
        for (final i in page) {
          if (i.itemKind == 'channel' && i.originKey.startsWith('channel:')) {
            posted[i.originKey.substring('channel:'.length)] = i;
          }
        }
        return wanted.every(posted.containsKey);
      },
    );
    return posted;
  }

  Future<void> _read(String id) async {
    final snapshot = await repository.importSource(id);
    final failed = await repository.importItems(id, state: 'failed');
    final posted = await _postedChannels(snapshot);
    List<AgentRecord> agents;
    try {
      agents = await repository.agentRecords();
    } on Object {
      agents = const [];
    }
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
      failedItems: failed,
      agents: agents,
    );
    apply(snapshot);
  }
}
