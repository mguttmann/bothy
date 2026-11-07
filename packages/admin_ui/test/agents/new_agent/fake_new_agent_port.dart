// SPDX-License-Identifier: Apache-2.0

/// An in-memory [NewAgentPort] with the server rules that fix the step order of 26 8.2 and
/// 8.3: `tool_class_too_low` for a `stdio` server below `host` (26 4.3 item 7),
/// `definition_needs_host` for a class below `host` while the stored definition has one
/// (R-BA-06), `host_tool_class_ceiling` above the runner's `hosts_agents` (26 3.1 item 3), and
/// `createAgent` and `createChannel` answered from their `Idempotency-Key`; for editing (A1-09)
/// the versions, `setHost` with `host_tool_class_ceiling`, `hosted_holder_mismatch` and
/// `host_not_capable`, `disable`, and a stale `If-Match` of a definition write as
/// `version_conflict`. Every call, its [AgentChangeKind] and every audit row is recorded;
/// [failNext] makes the next call of an operation fail once.
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';

/// The runner Dev runner (hosts up to `host`, organization).
const HostingRunner devRunner = HostingRunner(
  id: 'runner-1',
  name: 'Dev runner',
  ceiling: AgentToolClass.host,
);

/// Alex's personal runner that reports `personal`.
const HostingRunner alexRunner = HostingRunner(
  id: 'runner-2',
  name: 'Alex laptop',
  ceiling: AgentToolClass.scoped,
  billingMode: 'personal',
  holderId: 'alex',
  modesSupported: ['personal', 'organization'],
);

/// A template with a `stdio` server and the wish `host`.
const NewAgentTemplate filesTemplate = NewAgentTemplate(
  slug: 'file-helper',
  name: 'File helper',
  description: 'Reads and sorts the files of a folder',
  version: '1.0.0',
  requiredSecrets: ['FILES_TOKEN'],
  definition: {
    'instructions': 'Sort the files Alex names.',
    'skills': [
      {
        'name': 'sort',
        'description': 'Sorts files by date',
        'body': 'Sort the files by their date.',
      },
    ],
    'mcp_servers': [
      {
        'name': 'files',
        'transport': 'stdio',
        'command': 'npx',
        'package': '@example/mcp-files@1.4.2',
        'env': [
          {'name': 'FILES_TOKEN', 'secret_ref': 'env:FILES_TOKEN'},
        ],
      },
    ],
    'settings': {'tool_class_wish': 'host'},
    'origin': {
      'kind': 'template',
      'template_slug': 'file-helper',
      'template_version': '1.0.0',
    },
  },
);

/// A template without MCP servers.
const NewAgentTemplate writingTemplate = NewAgentTemplate(
  slug: 'writing-helper',
  name: 'Writing helper',
  description: 'Helps with texts',
  version: '1.0.0',
  definition: {
    'instructions': 'Help with texts.',
    'skills': <Object?>[],
    'mcp_servers': <Object?>[],
    'settings': {'tool_class_wish': 'chat'},
    'origin': {
      'kind': 'template',
      'template_slug': 'writing-helper',
      'template_version': '1.0.0',
    },
  },
);

/// The standard offer.
const NewAgentOffer standardOffer = NewAgentOffer(
  templates: [writingTemplate, filesTemplate],
  runners: [devRunner, alexRunner],
);

class _Agent {
  _Agent(this.id, this.name);
  final String id;
  final String name;
  String displayName = '';
  String description = '';
  bool active = true;
  AgentToolClass toolClass = AgentToolClass.chat;
  int version = 1;
  String billing = 'organization';
  String? holder;
  String hostedBy = '';
  final List<Map<String, Object?>> definitions = [];
  final List<DraftOrigin> origins = [];

  NewAgentState get state => NewAgentState(
    id: id,
    version: version,
    toolClass: toolClass,
    billingMode: billing,
    holderId: billing == 'personal' ? holder : null,
    name: displayName.isEmpty ? name : displayName,
    handle: name,
    description: description,
    hostedBy: hostedBy,
    active: active,
  );
}

/// The fake port.
class FakeNewAgentPort implements NewAgentPort {
  /// Creates the port with [offerValue] (null hides the entry) and [contextValue].
  FakeNewAgentPort({
    this.offerValue = standardOffer,
    this.contextValue = const NewAgentContext(
      meId: 'alex',
      onlyOwner: true,
      spaces: [NewAgentSpace('space-1', 'Team')],
      ownerNames: {'alex': 'Alex'},
    ),
  });

  /// What [offer] answers.
  NewAgentOffer? offerValue;

  /// What [context] answers.
  NewAgentContext contextValue;

  /// Every call in order, for example `setToolClass host` or `putDefinition`.
  final List<String> calls = [];

  /// The audit rows the server wrote, one per action that changed or created something.
  final List<String> audit = [];

  /// The errors of the next call of an operation, used once each: usually a
  /// [NewAgentFailure], any other object stands for a malformed answer.
  final Map<String, Object> failNext = {};

  /// The answers of [interview], in order.
  final List<InterviewReply> interviewReplies = [];

  /// The turns and `want_draft` of every interview request.
  final List<(List<InterviewTurn>, bool)> interviewRequests = [];

  /// While set, every call waits until it completes ([release]).
  Completer<void>? gate;

  /// The bodies of `putDefinition` in order.
  final List<Map<String, Object?>> putBodies = [];

  /// The `If-Match` of every `putDefinition` in order.
  final List<int?> putIfMatches = [];

  /// The [AgentChangeKind] of every write in order, as `operation kind`.
  final List<String> changes = [];

  /// The runners `setHost` and the class steps know besides those of the offer.
  final List<HostingRunner> extraRunners = [];

  /// The reasons of every `disable`.
  final List<String> disableReasons = [];

  /// The agents by ID.
  final Map<String, _Agent> _agents = {};
  final Map<String, String> _createKeys = {};
  final Map<String, String> _channelKeys = {};

  /// The channels by ID with their operator.
  final Map<String, String?> channels = {};

  /// How many agents exist.
  int get agentCount => _agents.length;

  /// The tool class of [id].
  AgentToolClass classOf(String id) => _agents[id]!.toolClass;

  /// The stored definitions of [id].
  List<Map<String, Object?>> definitionsOf(String id) =>
      _agents[id]!.definitions;

  /// The billing of [id].
  String billingOf(String id) => _agents[id]!.billing;

  /// The only agent, for tests that create one.
  String get onlyAgent => _agents.keys.single;

  /// The hosting runner of [id].
  String hostOf(String id) => _agents[id]!.hostedBy;

  /// Whether [id] is active.
  bool activeOf(String id) => _agents[id]!.active;

  /// Adds an existing built-in agent [id] (handle [handle]) on [hostedBy] with the stored
  /// [definitions], oldest first, as if created earlier; no call or audit row is recorded.
  void addAgent(
    String id, {
    String handle = 'notes',
    String displayName = 'Notes',
    String hostedBy = 'runner-1',
    AgentToolClass toolClass = AgentToolClass.chat,
    String billing = 'organization',
    String? holder,
    List<Map<String, Object?>> definitions = const [],
  }) {
    _agents[id] = _Agent(id, handle)
      ..displayName = displayName
      ..description = 'Keeps the notes'
      ..hostedBy = hostedBy
      ..toolClass = toolClass
      ..billing = billing
      ..holder = holder
      ..version = 1 + definitions.length;
    for (final d in definitions) {
      _agents[id]!
        ..definitions.add(d)
        ..origins.add(DraftOrigin.manual);
    }
  }

  /// Stores [definition] as a new version of [id] the way another device would.
  void changeBehind(String id, Map<String, Object?> definition) {
    _agents[id]!
      ..definitions.add(definition)
      ..origins.add(DraftOrigin.manual)
      ..version += 1;
  }

  List<HostingRunner> get _runners => [
    ...(offerValue ?? standardOffer).runners,
    ...extraRunners,
  ];

  /// Lets the held calls go on.
  void release() {
    final g = gate;
    gate = null;
    g?.complete();
  }

  Future<void> _enter(String op) async {
    calls.add(op);
    if (gate case final g?) await g.future;
    final name = op.split(' ').first;
    final f = failNext.remove(name);
    if (f != null) throw f;
  }

  @override
  Future<NewAgentOffer?> offer() async {
    final f = failNext.remove('offer');
    if (f != null) throw f;
    return offerValue;
  }

  @override
  Future<NewAgentContext> context() async => contextValue;

  @override
  Future<InterviewReply> interview(
    List<InterviewTurn> turns, {
    bool wantDraft = false,
  }) async {
    interviewRequests.add((List.of(turns), wantDraft));
    await _enter('interview');
    if (interviewReplies.isEmpty) {
      return const InterviewReply(
        outcome: InterviewOutcome.fallback,
        fallbackReason: 'builder_not_configured',
      );
    }
    return interviewReplies.removeAt(0);
  }

  @override
  Future<NewAgentState> createAgent({
    required String handle,
    required String displayName,
    required String description,
    required String hostedBy,
    required String idempotencyKey,
  }) async {
    await _enter('createAgent');
    final known = _createKeys[idempotencyKey];
    if (known != null) return _agents[known]!.state;
    if (_agents.values.any((a) => a.name == handle)) {
      throw const NewAgentFailure.problem('conflict', reason: 'handle_taken');
    }
    final id = 'agent-${_agents.length + 1}';
    _agents[id] = _Agent(id, handle)
      ..displayName = displayName
      ..description = description
      ..hostedBy = hostedBy;
    _createKeys[idempotencyKey] = id;
    audit.add('agent.create');
    return _agents[id]!.state;
  }

  @override
  Future<NewAgentState> agent(String agentId) async {
    await _enter('agent');
    return _agents[agentId]!.state;
  }

  @override
  Future<NewAgentState> setToolClass(
    NewAgentState agent,
    AgentToolClass toolClass, {
    required String name,
    AgentChangeKind change = AgentChangeKind.create,
  }) async {
    changes.add('setToolClass ${change.name}');
    await _enter('setToolClass ${toolClass.wire}');
    final a = _agents[agent.id]!;
    if (agent.version != a.version) {
      throw const NewAgentFailure.problem('version_conflict');
    }
    final host = [
      ...standardOffer.runners,
      ..._runners,
    ].firstWhere((r) => r.id == a.hostedBy);
    if (toolClass.isAbove(host.ceiling)) {
      throw const NewAgentFailure.problem(
        'conflict',
        reason: 'host_tool_class_ceiling',
      );
    }
    final current = a.definitions.isEmpty ? null : a.definitions.last;
    final stdio = ((current?['mcp_servers'] as List?) ?? const []).any(
      (s) => s is Map && s['transport'] == 'stdio',
    );
    if (toolClass != AgentToolClass.host && stdio) {
      throw const NewAgentFailure.problem(
        'conflict',
        reason: 'definition_needs_host',
      );
    }
    a
      ..toolClass = toolClass
      ..version += 1;
    audit.add('agent.tool_class.set');
    return a.state;
  }

  @override
  Future<StoredDefinition?> definition(String agentId) async {
    await _enter('definition');
    final a = _agents[agentId];
    if (a == null || a.definitions.isEmpty) return null;
    return StoredDefinition(a.definitions.length, a.definitions.last);
  }

  @override
  Future<StoredDefinition> putDefinition(
    String agentId,
    Map<String, Object?> definition, {
    int? ifMatch,
    required String name,
    AgentChangeKind change = AgentChangeKind.create,
  }) async {
    putBodies.add(definition);
    putIfMatches.add(ifMatch);
    changes.add('putDefinition ${change.name}');
    await _enter('putDefinition');
    final a = _agents[agentId]!;
    final n = a.definitions.length;
    if (n > 0 && ifMatch == null) {
      throw const NewAgentFailure.problem('precondition_required');
    }
    if (ifMatch != null && ifMatch != n) {
      throw const NewAgentFailure.problem('version_conflict');
    }
    final servers = (definition['mcp_servers'] as List?) ?? const [];
    if (a.toolClass != AgentToolClass.host) {
      final errors = [
        for (final (i, s) in servers.indexed)
          if (s is Map && s['transport'] == 'stdio')
            FieldIssue('/mcp_servers/$i/transport', 'tool_class_too_low'),
      ];
      if (errors.isNotEmpty) {
        throw NewAgentFailure.problem('validation_failed', errors: errors);
      }
    }
    if (n == 0 || !sameDefinition(a.definitions.last, definition)) {
      a
        ..definitions.add(definition)
        ..origins.add(
          DraftOrigin.values.firstWhere(
            (o) => o.name == (definition['origin'] as Map?)?['kind'],
            orElse: () => DraftOrigin.manual,
          ),
        )
        ..version += 1;
      audit.add('agent.update');
    }
    return StoredDefinition(a.definitions.length, a.definitions.last);
  }

  @override
  Future<NewAgentState> setBilling(
    NewAgentState agent,
    String mode, {
    String? holderId,
    required String name,
    AgentChangeKind change = AgentChangeKind.create,
  }) async {
    changes.add('setBilling ${change.name}');
    await _enter('setBilling $mode');
    final a = _agents[agent.id]!;
    if (agent.version != a.version) {
      throw const NewAgentFailure.problem('version_conflict');
    }
    a
      ..billing = mode
      ..holder = holderId
      ..version += 1;
    audit.add('agent.billing_mode.set');
    return a.state;
  }

  @override
  Future<String> createChannel(
    String spaceId,
    String channelName, {
    required String idempotencyKey,
    required String name,
    AgentChangeKind change = AgentChangeKind.create,
  }) async {
    changes.add('createChannel ${change.name}');
    await _enter('createChannel');
    final known = _channelKeys[idempotencyKey];
    if (known != null) return known;
    final id = 'channel-${channels.length + 1}';
    channels[id] = null;
    _channelKeys[idempotencyKey] = id;
    audit.add('channel.create');
    return id;
  }

  @override
  Future<void> setOperator(
    String channelId,
    String agentId, {
    required String name,
    AgentChangeKind change = AgentChangeKind.create,
  }) async {
    changes.add('setOperator ${change.name}');
    await _enter('setOperator');
    channels[channelId] = agentId;
    audit.add('channel.operator.set');
  }

  @override
  Future<List<NewAgentState>> builtinAgents() async {
    await _enter('builtinAgents');
    return [for (final a in _agents.values) a.state];
  }

  @override
  Future<List<DefinitionVersionInfo>> versions(String agentId) async {
    await _enter('versions');
    final a = _agents[agentId]!;
    return [
      for (var n = a.definitions.length; n >= 1; n--)
        DefinitionVersionInfo(
          version: n,
          createdAt: DateTime.utc(2026, 11, 7, 9, n),
          createdBy: 'alex',
          origin: a.origins[n - 1],
        ),
    ];
  }

  @override
  Future<StoredDefinition> definitionVersion(
    String agentId,
    int version,
  ) async {
    await _enter('definitionVersion');
    final a = _agents[agentId]!;
    return StoredDefinition(version, a.definitions[version - 1]);
  }

  @override
  Future<NewAgentState> setHost(
    NewAgentState agent,
    String runnerId, {
    required String name,
  }) async {
    changes.add('setHost host');
    await _enter('setHost');
    final a = _agents[agent.id]!;
    if (agent.version != a.version) {
      throw const NewAgentFailure.problem('version_conflict');
    }
    final runner = _runners.where((r) => r.id == runnerId).firstOrNull;
    if (runner == null) {
      throw const NewAgentFailure.problem(
        'validation_failed',
        errors: [FieldIssue('/hosted_by', 'host_not_capable')],
      );
    }
    if (a.toolClass.isAbove(runner.ceiling)) {
      throw const NewAgentFailure.problem(
        'conflict',
        reason: 'host_tool_class_ceiling',
      );
    }
    if (a.billing == 'personal' &&
        (runner.billingMode != 'personal' ||
            runner.holderId != a.holder ||
            !runner.modesSupported.contains('personal'))) {
      throw const NewAgentFailure.problem(
        'conflict',
        reason: 'hosted_holder_mismatch',
      );
    }
    a
      ..hostedBy = runnerId
      ..version += 1;
    audit.add('agent.host.set');
    return a.state;
  }

  @override
  Future<NewAgentState> disable(
    String agentId, {
    required String reason,
    required String name,
  }) async {
    changes.add('disable disable');
    disableReasons.add(reason);
    await _enter('disable');
    final a = _agents[agentId]!
      ..active = false
      ..version += 1;
    audit.add('agent.disable');
    return a.state;
  }

  /// The calls that change something, without the reads.
  List<String> get writes => [
    for (final c in calls)
      if (!const {
        'agent',
        'definition',
        'interview',
        'builtinAgents',
        'versions',
        'definitionVersion',
      }.contains(c))
        c,
  ];
}
