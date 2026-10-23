// SPDX-License-Identifier: Apache-2.0

/// The import sources over REST (docs/spec/22-import-migration.md 8, 11, 12.1, 13; brief M8-08
/// Design notes 1 to 4).
///
/// The console is the only place for the owner steps of `import.manage`, because the command
/// line logs in as a `cli` device that never provides step-up (22 8). The types carry what the
/// server answers: a source is the same `ImportSource` that the event `import.progress` carries
/// ([ImportSourceSnapshot.fromJson] reads both), and the screens show counters and structure
/// only, never content, file names or names of people (22 11, 14).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';

import '../data/access_repository.dart' show I1Resolution, adminCall;

/// The state of an import source (`ImportSource.state`, 22 7.4 and 13.5).
enum ImportState {
  /// Created, not started yet.
  draft,

  /// The owner allows imports; it does not say that a tool run is active (M8-03 note 3).
  running,

  /// Batches and cursor writes are refused until the next start.
  paused,

  /// Every imported channel is led here; the mirror ended.
  completed,

  /// Cancelled (`stats.server.cancelled`) or failed.
  failed;

  /// The state of the wire value [value]; an unknown value reads as [failed], which offers no
  /// action, so a newer server never gets a step the console does not know.
  static ImportState of(String? value) => values.firstWhere(
    (s) => s.name == value,
    orElse: () => ImportState.failed,
  );
}

/// The leading side of a mapped channel (`lead`, 22 12.1).
enum ImportLead {
  /// The source leads; the channel here is read-only and mirrored.
  discord,

  /// This server leads; the channel is writable and has its operator.
  bothy;

  /// The lead of the wire value [value] (the mapping's default is `discord`).
  static ImportLead of(Object? value) =>
      value == 'bothy' ? ImportLead.bothy : ImportLead.discord;
}

/// One channel of the stored mapping whose target is not `skip` (22 5.2).
final class MappedChannel {
  /// Creates the entry.
  const MappedChannel({
    required this.discordId,
    required this.lead,
    this.createName,
    this.existingId,
    this.plannedAgentHandle,
  });

  /// The Discord channel ID (`discord_channel`).
  final String discordId;

  /// The name of a channel the import creates (`target.create.name`), or null.
  final String? createName;

  /// The existing channel the import fills (`target.existing`), or null.
  final String? existingId;

  /// The handle of `planned_operator` (`agent:<handle>`), or null for `none`.
  final String? plannedAgentHandle;

  /// The stored lead.
  final ImportLead lead;
}

/// One counter of `stats` (22 11): its [path] below the namespace (`channel.posted`) and its
/// number.
final class ImportCounter {
  /// Creates the counter.
  const ImportCounter(this.path, this.value);

  /// The member path, joined with dots, for example `message.posted`.
  final String path;

  /// The value.
  final num value;
}

/// The latest snapshot of one import source (`ImportSource`): the payload of `getImport`, of
/// every `import.manage` answer and of the event `import.progress` (brief M8-08 Design note 1).
///
/// Only the parts a screen may show are read: the counters are the numbers of `stats` (a text
/// or any other value is left out, so no name or content of the source can reach a screen,
/// 22 11 and 14), the mapping gives the channels with their Discord IDs, Bothy names, planned
/// operators and leads; the people of the mapping are never read.
final class ImportSourceSnapshot {
  /// Creates the snapshot.
  const ImportSourceSnapshot({
    required this.id,
    required this.kind,
    required this.name,
    required this.state,
    required this.config,
    required this.stats,
    required this.createdAt,
    this.startedAt,
    this.finishedAt,
  });

  /// Reads the JSON object of `ImportSource` (snake case, as the server sends it).
  ///
  /// Throws a [FormatException] when a required member is missing.
  factory ImportSourceSnapshot.fromJson(Map<String, Object?> json) {
    String text(String key) => switch (json[key]) {
      final String s => s,
      _ => throw FormatException('ImportSource.$key is missing'),
    };
    DateTime? time(String key) => switch (json[key]) {
      final String s => DateTime.tryParse(s),
      _ => null,
    };
    return ImportSourceSnapshot(
      id: text('id'),
      kind: text('kind'),
      name: text('name'),
      state: ImportState.of(json['state'] as String?),
      config: _object(json['config']),
      stats: _object(json['stats']),
      createdAt: time('created_at') ?? DateTime.fromMillisecondsSinceEpoch(0),
      startedAt: time('started_at'),
      finishedAt: time('finished_at'),
    );
  }

  /// The source ID.
  final String id;

  /// `discord` or `archive`.
  final String kind;

  /// The name the owner gave the source.
  final String name;

  /// The state.
  final ImportState state;

  /// The stored mapping (`import_source.<kind>.v1`).
  final Map<String, Object?> config;

  /// The counters (`server` and `tool` namespaces).
  final Map<String, Object?> stats;

  /// When the source was created.
  final DateTime createdAt;

  /// The first start, or null.
  final DateTime? startedAt;

  /// The end (completed or failed), or null.
  final DateTime? finishedAt;

  static Map<String, Object?> _object(Object? v) =>
      v is Map ? v.cast<String, Object?>() : const {};

  /// Whether the owner cancelled the source (`stats.server.cancelled`).
  bool get cancelled => _object(stats['server'])['cancelled'] == true;

  /// The server's counters (`stats.server.<item_kind>.<state>`, `source_changed`), sorted.
  List<ImportCounter> get serverCounters => _counters(stats['server']);

  /// The tool's counters (`stats.tool`), sorted.
  List<ImportCounter> get toolCounters => _counters(stats['tool']);

  static List<ImportCounter> _counters(Object? root) {
    final out = <ImportCounter>[];
    void walk(Object? node, String path) {
      if (node is num) {
        out.add(ImportCounter(path, node));
      } else if (node is Map) {
        for (final e in node.entries) {
          walk(e.value, path.isEmpty ? '${e.key}' : '$path.${e.key}');
        }
      }
      // Texts, lists and flags are no counters; they are never shown (22 11, 14).
    }

    walk(root, '');
    out.sort((a, b) => a.path.compareTo(b.path));
    return out;
  }

  /// The channels of the mapping whose target is not `skip`, in the mapping's order.
  List<MappedChannel> get channels {
    final list = config['channels'];
    if (list is! List) return const [];
    return [
      for (final c in list)
        if (c is Map && c['discord_channel'] is String && c['target'] != 'skip')
          MappedChannel(
            discordId: c['discord_channel']! as String,
            createName: switch (c['target']) {
              {'create': {'name': final String n}} => n,
              _ => null,
            },
            existingId: switch (c['target']) {
              {'existing': final String id} => id,
              _ => null,
            },
            plannedAgentHandle: switch (c['planned_operator']) {
              final String p when p.startsWith('agent:') => p.substring(6),
              _ => null,
            },
            lead: ImportLead.of(c['lead']),
          ),
    ];
  }
}

/// One item of a source (`ImportItem`): its origin key (Discord IDs only, 22 7.1), kind,
/// target and state.
final class ImportItemEntry {
  /// Creates the item.
  const ImportItemEntry({
    required this.id,
    required this.originKey,
    required this.itemKind,
    required this.state,
    this.targetId,
    this.error,
  });

  /// The item ID.
  final String id;

  /// The origin key, for example `channel:1029384000`.
  final String originKey;

  /// `space`, `channel`, `forum_tag`, `thread`, `message`, `file`, `user`, `reaction`, `pin`.
  final String itemKind;

  /// `pending`, `posted`, `skipped` or `failed`.
  final String state;

  /// The target's ID, or null.
  final String? targetId;

  /// The server's error of a failed item, or null.
  final String? error;
}

/// One channel of this server as the import screen needs it (`getChannel`): its name, the
/// read-only flag, the operator after inheritance and the version for `If-Match`.
final class ImportChannelState {
  /// Creates the state.
  const ImportChannelState({
    required this.id,
    required this.name,
    required this.readOnly,
    required this.version,
    this.operatorAgentId,
  });

  /// The channel ID.
  final String id;

  /// The channel name (structure, 22 11).
  final String name;

  /// Whether the channel is read-only (05 rule 1.8).
  final bool readOnly;

  /// The operator after inheritance (`operator.effective_agent_id`), or null for none.
  final String? operatorAgentId;

  /// The version for `If-Match`.
  final int version;
}

/// One page of sources (`listImports`).
final class ImportSourcesPage {
  /// Creates the page.
  const ImportSourcesPage(this.items, {this.nextCursor});

  /// The sources, newest first.
  final List<ImportSourceSnapshot> items;

  /// The cursor of the next page, or null.
  final String? nextCursor;
}

/// The import calls of the console (tag `Import` of `docs/api/openapi.yaml`, with `getChannel`
/// and `setChannelOperator` for the operator step of 22 13.3).
///
/// Every changing call carries the purpose text the step-up dialog shows when the server answers
/// `401 step_up_required`; the screen never asks on its own. Errors: [SelfActionFailed] with the
/// server's problem (a `409 i1_violation` with its conflicts), [StepUpCancelled] when the person
/// cancels the step-up.
abstract interface class ImportRepository {
  /// One page of sources (`listImports`).
  Future<ImportSourcesPage> importSources({String? cursor});

  /// The source [id] (`getImport`).
  Future<ImportSourceSnapshot> importSource(String id);

  /// Creates a source of `kind = discord` named [name] with the mapping [config] (the JSON of
  /// the owner's YAML) (`createImport`, step-up).
  Future<ImportSourceSnapshot> createImportSource({
    required String name,
    required Map<String, Object?> config,
    required String purpose,
  });

  /// Moves the source to `running` (`startImport`, step-up).
  Future<ImportSourceSnapshot> startImport(String id, {required String purpose});

  /// Moves the source to `paused` (`pauseImport`, step-up).
  Future<ImportSourceSnapshot> pauseImport(String id, {required String purpose});

  /// Ends the source as `failed` with `cancelled` (`cancelImport`, step-up).
  Future<ImportSourceSnapshot> cancelImport(String id, {required String purpose});

  /// Moves the source to `completed` (`completeImport`, step-up).
  Future<ImportSourceSnapshot> completeImport(
    String id, {
    required String purpose,
  });

  /// Sets the lead of the mapped channel [discordChannel] (`setImportLead`, step-up), which
  /// sets or lifts read-only of its target in the same transaction.
  Future<ImportSourceSnapshot> setImportLead(
    String id, {
    required String discordChannel,
    required ImportLead lead,
    required String purpose,
  });

  /// Every item of the source in [state] (all pages of `listImportItems`).
  Future<List<ImportItemEntry>> importItems(String id, {String? state});

  /// The channel [channelId] (`getChannel`).
  Future<ImportChannelState> importChannel(String channelId);

  /// Sets the operator of [channelId] to [agentId], or to none when [agentId] is null
  /// (`setChannelOperator` with `If-Match` on [version], step-up); [resolveI1] is the guided
  /// way out of an I1 conflict in the same request (05 rule 1.6).
  Future<ImportChannelState> setImportChannelOperator(
    String channelId, {
    required String? agentId,
    required int version,
    required String reason,
    I1Resolution? resolveI1,
    required String purpose,
  });
}

/// [ImportRepository] over the generated client.
///
/// Example:
/// ```dart
/// final imports = ApiImportRepository(api, newIdempotencyKey: ids.next);
/// final page = await imports.importSources();
/// ```
final class ApiImportRepository implements ImportRepository {
  /// Creates the repository over `api` (the client of the signed-in session, so the cookie,
  /// CSRF and the step-up adapter apply); [newIdempotencyKey] returns a fresh UUIDv7.
  ApiImportRepository(
    this._api, {
    required String Function() newIdempotencyKey,
  }) : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  /// The page size of `listImportItems`.
  static const int _itemPage = 200;

  ImportApi get _imports => _api.getImportApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  ImportSourceSnapshot _snapshot(ImportSource s) => ImportSourceSnapshot.fromJson(
    (_api.serializers.serializeWith(ImportSource.serializer, s)! as Map)
        .cast<String, Object?>(),
  );

  static ImportChannelState _channel(Channel c) => ImportChannelState(
    id: c.id,
    name: c.name,
    readOnly: c.readOnly,
    operatorAgentId: c.operator_.effectiveAgentId,
    version: c.version,
  );

  @override
  Future<ImportSourcesPage> importSources({String? cursor}) =>
      adminCall(() async {
        final page = (await _imports.listImports(cursor: cursor)).data!;
        return ImportSourcesPage([
          for (final s in page.items) _snapshot(s),
        ], nextCursor: page.nextCursor);
      });

  @override
  Future<ImportSourceSnapshot> importSource(String id) => adminCall(
    () async => _snapshot((await _imports.getImport(importId: id)).data!),
  );

  @override
  Future<ImportSourceSnapshot> createImportSource({
    required String name,
    required Map<String, Object?> config,
    required String purpose,
  }) => adminCall(() async {
    // The mapping travels as the plain JSON the dialog converted, so the server's closed schema
    // sees exactly the owner's file (unknown members are reported at their pointer).
    final response = await _api.dio.request<Object>(
      '/imports',
      data: {'kind': 'discord', 'name': name, 'config': config},
      options: Options(
        method: 'POST',
        contentType: 'application/json',
        headers: {'Idempotency-Key': _newKey()},
        extra: {..._secure, ..._purpose(purpose)},
      ),
    );
    return ImportSourceSnapshot.fromJson(
      (response.data! as Map).cast<String, Object?>(),
    );
  });

  /// The security requirement of the import operations as the generated client sends it, so
  /// the auth interceptors sign the request sent with a plain JSON body too.
  static const Map<String, Object?> _secure = {
    'secure': <Map<String, String>>[
      {
        'type': 'apiKey',
        'name': 'consoleSession',
        'keyName': '__Host-console_session',
        'where': '',
      },
      {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
      {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
    ],
  };

  @override
  Future<ImportSourceSnapshot> startImport(
    String id, {
    required String purpose,
  }) => adminCall(
    () async => _snapshot(
      (await _imports.startImport(
        importId: id,
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<ImportSourceSnapshot> pauseImport(
    String id, {
    required String purpose,
  }) => adminCall(
    () async => _snapshot(
      (await _imports.pauseImport(
        importId: id,
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<ImportSourceSnapshot> cancelImport(
    String id, {
    required String purpose,
  }) => adminCall(
    () async => _snapshot(
      (await _imports.cancelImport(
        importId: id,
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<ImportSourceSnapshot> completeImport(
    String id, {
    required String purpose,
  }) => adminCall(
    () async => _snapshot(
      (await _imports.completeImport(
        importId: id,
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<ImportSourceSnapshot> setImportLead(
    String id, {
    required String discordChannel,
    required ImportLead lead,
    required String purpose,
  }) => adminCall(
    () async => _snapshot(
      (await _imports.setImportLead(
        importId: id,
        importLeadSet: ImportLeadSet(
          (b) => b
            ..sourceChannel = discordChannel
            ..lead = ImportLeadSetLeadEnum.valueOf(lead.name),
        ),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<List<ImportItemEntry>> importItems(String id, {String? state}) =>
      adminCall(() async {
        final out = <ImportItemEntry>[];
        String? cursor;
        do {
          final page = (await _imports.listImportItems(
            importId: id,
            cursor: cursor,
            limit: _itemPage,
            state: state == null ? null : ImportItemState.valueOf(state),
          )).data!;
          for (final i in page.items) {
            // The wire values (`forum_tag`), not the names of the generated enums.
            final json = (_api.serializers.serializeWith(ImportItem.serializer, i)!
                    as Map)
                .cast<String, Object?>();
            out.add(
              ImportItemEntry(
                id: i.id,
                originKey: i.originKey,
                itemKind: json['item_kind']! as String,
                state: json['state']! as String,
                targetId: i.targetId,
                error: i.error,
              ),
            );
          }
          cursor = page.nextCursor;
        } while (cursor != null);
        return out;
      });

  @override
  Future<ImportChannelState> importChannel(String channelId) => adminCall(
    () async => _channel(
      (await _api.getChannelsApi().getChannel(
        channelId: channelId,
      )).data!.channel,
    ),
  );

  @override
  Future<ImportChannelState> setImportChannelOperator(
    String channelId, {
    required String? agentId,
    required int version,
    required String reason,
    I1Resolution? resolveI1,
    required String purpose,
  }) => adminCall(
    () async => _channel(
      (await _api.getChannelsApi().setChannelOperator(
        channelId: channelId,
        ifMatch: '"$version"',
        operatorSet: OperatorSet(
          (b) => b
            ..mode = agentId == null ? OperatorMode.none : OperatorMode.agent
            ..agentId = agentId
            ..reason = reason
            ..resolveI1 = resolveI1?.wire.toBuilder(),
        ),
        extra: _purpose(purpose),
      )).data!,
    ),
  );
}
