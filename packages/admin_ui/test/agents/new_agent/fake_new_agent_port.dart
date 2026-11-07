// SPDX-License-Identifier: Apache-2.0

/// An in-memory [NewAgentPort] with the server rules that fix the step order of 26 8.2 and
/// 8.3: `tool_class_too_low` for a `stdio` server below `host` (26 4.3 item 7),
/// `definition_needs_host` for a class below `host` while the stored definition has one
/// (R-BA-06), `host_tool_class_ceiling` above the runner's `hosts_agents` (26 3.1 item 3), and
/// `createAgent` and `createChannel` answered from their `Idempotency-Key`. Every call and every
/// audit row is recorded; [failNext] makes the next call of an operation fail once.
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
  AgentToolClass toolClass = AgentToolClass.chat;
  int version = 1;
  String billing = 'organization';
  String? holder;
  String hostedBy = '';
  final List<Map<String, Object?>> definitions = [];

  NewAgentState get state => NewAgentState(
    id: id,
    version: version,
    toolClass: toolClass,
    billingMode: billing,
    holderId: billing == 'personal' ? holder : null,
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

  /// The failures of the next call of an operation, used once each.
  final Map<String, NewAgentFailure> failNext = {};

  /// The answers of [interview], in order.
  final List<InterviewReply> interviewReplies = [];

  /// The turns and `want_draft` of every interview request.
  final List<(List<InterviewTurn>, bool)> interviewRequests = [];

  /// While set, every call waits until it completes ([release]).
  Completer<void>? gate;

  /// The bodies of `putDefinition` in order.
  final List<Map<String, Object?>> putBodies = [];

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
  Future<NewAgentOffer?> offer() async => offerValue;

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
    _agents[id] = _Agent(id, handle)..hostedBy = hostedBy;
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
  }) async {
    await _enter('setToolClass ${toolClass.wire}');
    final a = _agents[agent.id]!;
    if (agent.version != a.version) {
      throw const NewAgentFailure.problem('version_conflict');
    }
    final host = standardOffer.runners.firstWhere((r) => r.id == a.hostedBy);
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
  }) async {
    putBodies.add(definition);
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
  }) async {
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
  }) async {
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
  }) async {
    await _enter('setOperator');
    channels[channelId] = agentId;
    audit.add('channel.operator.set');
  }

  /// The calls that change something, without the reads.
  List<String> get writes => [
    for (final c in calls)
      if (c != 'agent' && c != 'definition' && c != 'interview') c,
  ];
}
