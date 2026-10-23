// SPDX-License-Identifier: Apache-2.0

// The import calls of the fake repository (brief M8-08 Tests), behaving like the server of
// M8-03: the state machine of note 3 (a repeated transition answers unchanged, every other one
// is 409 import_state), completion only when every imported channel is led here (409 with the
// reason of a lead that is not here), setImportLead with 404 for a channel the mapping does not name, 409
// channel_not_imported before its item is posted and read-only set or lifted with the lead,
// setChannelOperator with If-Match (412) and the I1 guard of the agents fake, and a mapping
// whose planned operator names an unknown agent refused with its pointer.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';

/// The import calls and whether 05 gives their action step-up (`import.manage` and
/// `channel.operator.set` have it; the reads have none).
const Map<String, bool> importStepUpOf = {
  'createImportSource': true,
  'startImport': true,
  'pauseImport': true,
  'cancelImport': true,
  'completeImport': true,
  'setImportLead': true,
  'setImportChannelOperator': true,
};

SelfActionFailed _conflict(String reason) =>
    SelfActionFailed(ApiProblem(status: 409, code: 'conflict', reason: reason));

const SelfActionFailed _notFound = SelfActionFailed(
  ApiProblem(status: 404, code: 'not_found'),
);

/// The mapping of the fixture source: guild 100, two new channels in a new space ("tickets"
/// with the planned operator Texter, the forum "questions" without one), an existing channel
/// that is never imported and a skipped one; the people of the mapping carry only handles.
Map<String, Object?> fakeMapping({String plannedOperator = 'agent:texter'}) => {
  'version': 1,
  'guild_id': '100',
  'defaults': {
    'unmapped_human': 'placeholder',
    'unmapped_bot': 'placeholder',
    'uncategorized_space': '200',
  },
  'spaces': [
    {
      'discord_category': '200',
      'target': {
        'create': {'name': 'Imported'},
      },
    },
  ],
  'channels': [
    {
      'discord_channel': '301',
      'target': {
        'create': {'name': 'tickets'},
      },
      'planned_operator': plannedOperator,
      'lead': 'discord',
    },
    {
      'discord_channel': '302',
      'target': {
        'create': {'name': 'questions'},
      },
      'planned_operator': 'none',
      'lead': 'discord',
    },
    {
      'discord_channel': '303',
      'target': {'existing': '0192f4a0-7b1c-7000-8000-000000000303'},
      'planned_operator': 'none',
      'lead': 'discord',
    },
    {'discord_channel': '304', 'target': 'skip'},
  ],
  'users': [
    {'discord_user': '401', 'target': 'user:alex'},
  ],
};

/// The import calls of the fake; [change], [refused] and [i1Guard] are those of the repository.
mixin FakeImport {
  /// Records the change [op] and asks for the step-up as the server would.
  Future<void> change(String op, String args, String purpose);

  /// Throws the refusal of [op], if any.
  void refused(String op);

  /// The I1 guard of the agents fake.
  void i1Guard(String op, I1Resolution? resolve);

  /// The agents by ID (the planned operator is resolved by handle).
  Map<String, AgentRecord> get agentRecordsById;

  /// The sources by ID as JSON (`ImportSource`), newest last.
  final Map<String, Map<String, Object?>> importsById = {};

  /// The items per source.
  final Map<String, List<ImportItemEntry>> importItemsById = {};

  /// The channels the import created, by ID.
  final Map<String, ImportChannelState> importChannelsById = {};

  int _imports = 0;

  /// Adds a source named [name] in [state] with [config] (default [fakeMapping]) and [stats];
  /// returns its ID.
  String seedImport(
    String name, {
    String state = 'draft',
    Map<String, Object?>? config,
    Map<String, Object?> stats = const {},
  }) {
    final id = 'import-${++_imports}';
    importsById[id] = {
      'id': id,
      'kind': 'discord',
      'name': name,
      'config': config ?? fakeMapping(),
      'state': state,
      'stats': stats,
      'cursor': const <String, Object?>{},
      'created_at': DateTime.utc(2026, 10, 17, 8).toIso8601String(),
    };
    importItemsById[id] = [];
    return id;
  }

  /// Posts the channel items of [discordIds] of source [id] as the import tool would: each
  /// creates a read-only channel (lead `discord`) without operator and counts as posted.
  void postChannels(String id, List<String> discordIds) {
    final source = importsById[id]!;
    for (final d in discordIds) {
      final entry = _mapped(source, d)!;
      final channelId = 'ch-$d';
      final create = (entry['target']! as Map)['create'] as Map?;
      importChannelsById[channelId] = ImportChannelState(
        id: channelId,
        name: (create?['name'] as String?) ?? 'existing-$d',
        readOnly: entry['lead'] != ImportLead.here.wire,
        version: 1,
      );
      importItemsById[id]!.add(
        ImportItemEntry(
          id: 'item-$d',
          originKey: 'channel:$d',
          itemKind: 'channel',
          state: 'posted',
          targetId: channelId,
        ),
      );
    }
    final stats = (source['stats']! as Map).cast<String, Object?>();
    final server = ((stats['server'] as Map?) ?? const {})
        .cast<String, Object?>();
    final channel = ((server['channel'] as Map?) ?? const {})
        .cast<String, Object?>();
    final posted = ((channel['posted'] as num?) ?? 0) + discordIds.length;
    source['stats'] = <String, Object?>{
      ...stats,
      'server': <String, Object?>{
        ...server,
        'channel': <String, Object?>{...channel, 'posted': posted},
      },
    };
  }

  /// Sets the operator of the channel of [discordId] of source [id] to [agentId] without a
  /// request (what another owner did).
  void operatorElsewhere(String id, String discordId, String? agentId) {
    final c = importChannelsById['ch-$discordId']!;
    importChannelsById[c.id] = ImportChannelState(
      id: c.id,
      name: c.name,
      readOnly: c.readOnly,
      operatorAgentId: agentId,
      version: c.version + 1,
    );
  }

  /// The source [id] as the client reads it.
  ImportSourceSnapshot importSnapshot(String id) =>
      ImportSourceSnapshot.fromJson(importsById[id]!);

  Map<String, Object?>? _mapped(Map<String, Object?> source, String d) {
    final channels = (source['config']! as Map)['channels']! as List;
    for (final c in channels) {
      if (c is Map && c['discord_channel'] == d && c['target'] != 'skip') {
        return c.cast<String, Object?>();
      }
    }
    return null;
  }

  Map<String, Object?> _source(String id) =>
      importsById[id] ?? (throw _notFound);

  /// Every source, newest first.
  Future<ImportSourcesPage> importSources({String? cursor}) async {
    refused('importSources');
    return ImportSourcesPage([
      for (final id in importsById.keys.toList().reversed) importSnapshot(id),
    ]);
  }

  /// One source.
  Future<ImportSourceSnapshot> importSource(String id) async {
    refused('importSource');
    _source(id);
    return importSnapshot(id);
  }

  /// Creates a source; a planned operator whose agent is unknown is `400` at its pointer.
  Future<ImportSourceSnapshot> createImportSource({
    required String name,
    required Map<String, Object?> config,
    required String purpose,
  }) async {
    await change('createImportSource', name, purpose);
    final errors = <ProblemFieldError>[];
    final channels = config['channels'];
    if (channels is List) {
      for (final (i, c) in channels.indexed) {
        final op = c is Map ? c['planned_operator'] : null;
        if (op is String &&
            op.startsWith('agent:') &&
            !agentRecordsById.values.any((a) => a.handle == op.substring(6))) {
          errors.add(
            ProblemFieldError(
              pointer: '/config/channels/$i/planned_operator',
              code: 'unknown_handle',
            ),
          );
        }
      }
    }
    if (errors.isNotEmpty) {
      throw SelfActionFailed(
        ApiProblem(status: 400, code: 'validation_failed', errors: errors),
      );
    }
    final id = seedImport(name, config: config);
    return importSnapshot(id);
  }

  Future<ImportSourceSnapshot> _move(
    String op,
    String id,
    String purpose, {
    required Set<String> from,
    required String to,
  }) async {
    await change(op, id, purpose);
    final s = _source(id);
    if (s['state'] == to && to != 'failed' && to != 'completed') {
      return importSnapshot(id);
    }
    if (!from.contains(s['state'])) throw _conflict('import_state');
    s['state'] = to;
    return importSnapshot(id);
  }

  /// `draft` or `paused` to `running`.
  Future<ImportSourceSnapshot> startImport(
    String id, {
    required String purpose,
  }) => _move(
    'startImport',
    id,
    purpose,
    from: {'draft', 'paused'},
    to: 'running',
  );

  /// `running` to `paused`.
  Future<ImportSourceSnapshot> pauseImport(
    String id, {
    required String purpose,
  }) => _move('pauseImport', id, purpose, from: {'running'}, to: 'paused');

  /// `draft`, `running` or `paused` to `failed` with `cancelled`.
  Future<ImportSourceSnapshot> cancelImport(
    String id, {
    required String purpose,
  }) async {
    final after = await _move(
      'cancelImport',
      id,
      purpose,
      from: {'draft', 'running', 'paused'},
      to: 'failed',
    );
    final s = _source(id);
    s['stats'] = {
      ...(s['stats']! as Map).cast<String, Object?>(),
      'server': {
        ...((s['stats']! as Map)['server'] as Map? ?? const {})
            .cast<String, Object?>(),
        'cancelled': true,
      },
    };
    return after.state == ImportState.failed ? importSnapshot(id) : after;
  }

  /// `running` or `paused` to `completed` when every imported channel is led here.
  Future<ImportSourceSnapshot> completeImport(
    String id, {
    required String purpose,
  }) async {
    await change('completeImport', id, purpose);
    final s = _source(id);
    if (!{'running', 'paused'}.contains(s['state'])) {
      throw _conflict('import_state');
    }
    for (final item in importItemsById[id]!) {
      if (item.itemKind != 'channel' || item.state != 'posted') continue;
      final m = _mapped(s, item.originKey.substring('channel:'.length));
      if (m != null && m['lead'] != ImportLead.here.wire) {
        throw _conflict('lead_not_${ImportLead.here.wire}');
      }
    }
    s['state'] = 'completed';
    return importSnapshot(id);
  }

  /// Sets the lead and the read-only flag of the channel's target together.
  Future<ImportSourceSnapshot> setImportLead(
    String id, {
    required String discordChannel,
    required ImportLead lead,
    required String purpose,
  }) async {
    await change('setImportLead', '$id $discordChannel ${lead.name}', purpose);
    final s = _source(id);
    if (s['state'] == 'completed' || s['state'] == 'failed') {
      throw _conflict('import_state');
    }
    final m = _mapped(s, discordChannel) ?? (throw _notFound);
    final item = importItemsById[id]!
        .where((i) => i.originKey == 'channel:$discordChannel')
        .firstOrNull;
    if (item == null || item.state != 'posted') {
      throw _conflict('channel_not_imported');
    }
    if (m['lead'] == lead.wire) return importSnapshot(id);
    m['lead'] = lead.wire;
    final c = importChannelsById[item.targetId]!;
    importChannelsById[c.id] = ImportChannelState(
      id: c.id,
      name: c.name,
      readOnly: lead == ImportLead.discord,
      operatorAgentId: c.operatorAgentId,
      version: c.version + 1,
    );
    return importSnapshot(id);
  }

  /// Every item of the source in [state].
  Future<List<ImportItemEntry>> importItems(String id, {String? state}) async {
    refused('importItems');
    _source(id);
    return [
      for (final i in importItemsById[id]!)
        if (state == null || i.state == state) i,
    ];
  }

  /// One channel.
  Future<ImportChannelState> importChannel(String channelId) async {
    refused('importChannel');
    return importChannelsById[channelId] ?? (throw _notFound);
  }

  /// Sets the operator with `If-Match`; I1 through the agents fake.
  Future<ImportChannelState> setImportChannelOperator(
    String channelId, {
    required String? agentId,
    required int version,
    required String reason,
    I1Resolution? resolveI1,
    required String purpose,
  }) async {
    await change(
      'setImportChannelOperator',
      '$channelId ${agentId ?? '-'} $reason',
      purpose,
    );
    final c = importChannelsById[channelId] ?? (throw _notFound);
    if (c.version != version) {
      throw const SelfActionFailed(
        ApiProblem(status: 412, code: 'version_conflict'),
      );
    }
    i1Guard('setImportChannelOperator', resolveI1);
    return importChannelsById[channelId] = ImportChannelState(
      id: c.id,
      name: c.name,
      readOnly: c.readOnly,
      operatorAgentId: agentId,
      version: c.version + 1,
    );
  }
}
