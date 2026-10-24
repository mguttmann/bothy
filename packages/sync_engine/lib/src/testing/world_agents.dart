// SPDX-License-Identifier: Apache-2.0

/// The agent interface of the fake world (brief M4-21 Scope "Fake server handlers"): the
/// registered commands with their option suggestions, the invocations, the cards with
/// components and their interactions, the agent sessions per thread and the live box anchors.
///
/// The rules follow the server of M2-15 (components, 09 11), M2-16 (commands, 09 10) and
/// M2-17 (autocomplete, 09 10.5) as far as the client sees them; what the fake leaves out is
/// named on each member.
library;

import 'world.dart';

/// Suggestions of an agent for one option: the option name, the typed value and the other
/// option values in; `{name, value}` choices out (API `CommandChoice`).
typedef FakeAutocomplete = List<JsonMap> Function(
  String option,
  String value,
  JsonMap others,
);

/// The agent interface state of one [FakeWorld]; [FakeAgentInterface.of] gives the one of a
/// world.
final class FakeAgentInterface {
  FakeAgentInterface._(this.world);

  static final Expando<FakeAgentInterface> _of = Expando();

  /// The agent interface of [world], created on first use.
  static FakeAgentInterface of(FakeWorld world) =>
      _of[world] ??= FakeAgentInterface._(world);

  /// The world.
  final FakeWorld world;

  /// The registered commands by ID (API `Command`).
  final Map<String, JsonMap> commands = {};

  /// The option suggestions by command ID.
  final Map<String, FakeAutocomplete> suggestions = {};

  /// Every `autocompleteCommand` request in order: command ID, option and value.
  final List<({String commandId, String option, String value})>
  autocompleteRequests = [];

  /// Every accepted `invokeCommand` in order (API `CommandInvoke` with the command and the
  /// person).
  final List<JsonMap> invocations = [];

  /// The interactions by `principal|client_interaction_id` (API `Interaction`).
  final Map<String, JsonMap> interactions = {};

  /// The agent sessions per thread (API `AgentSession`), oldest first.
  final Map<String, List<JsonMap>> sessions = {};

  /// Registers the command [name] of [ownerId] on the instance (or [scopeKind] and
  /// [scopeId]) and appends `command.changed`; returns its ID.
  String registerCommand(
    String ownerId,
    String name, {
    String description = '',
    List<JsonMap> options = const [],
    bool control = false,
    String scopeKind = 'instance',
    String? scopeId,
    FakeAutocomplete? autocomplete,
  }) {
    final id = world.nextId();
    final command = <String, Object?>{
      'id': id,
      'owner_principal_id': ownerId,
      'name': name,
      'description': description,
      'scope_kind': scopeKind,
      'scope_id': ?scopeId,
      'options': options,
      'required_level': 'trigger',
      'control': control,
      'version': 1,
    };
    commands[id] = command;
    if (autocomplete != null) suggestions[id] = autocomplete;
    world.append(
      'command.changed',
      resource: {'kind': 'command', 'id': id, 'version': 1},
      scope: {
        if (scopeKind == 'channel') 'channel_id': scopeId,
        if (scopeKind == 'space') 'space_id': scopeId,
      },
      payload: {'command': command, 'change': 'registered'},
      actor: ownerId,
    );
    return id;
  }

  /// The commands offered in [channelId] (`listChannelCommands`): those of agents that see
  /// the channel, on the instance, the channel's space or the channel; the operator's first,
  /// then by lower-case name, owner and ID. The fake has no guest rule and no app commands.
  List<JsonMap> commandsOf(String channelId) {
    final channel = world.channels[channelId]!;
    final operator =
        (channel.json['operator'] as JsonMap?)?['effective_agent_id'];
    bool applies(JsonMap c) => switch (c['scope_kind']) {
      'instance' => true,
      'space' => c['scope_id'] == channel.spaceId,
      'channel' => c['scope_id'] == channelId,
      _ => false,
    };
    final list = [
      for (final c in commands.values)
        if (applies(c) &&
            world.seesChannel(c['owner_principal_id']! as String, channelId))
          c,
    ];
    int group(JsonMap c) => c['owner_principal_id'] == operator ? 0 : 1;
    list.sort((a, b) {
      var r = group(a).compareTo(group(b));
      if (r == 0) {
        r = (a['name']! as String).toLowerCase().compareTo(
          (b['name']! as String).toLowerCase(),
        );
      }
      if (r == 0) {
        r = (a['owner_principal_id']! as String).compareTo(
          b['owner_principal_id']! as String,
        );
      }
      if (r == 0) r = (a['id']! as String).compareTo(b['id']! as String);
      return r;
    });
    return list;
  }

  /// Posts a card of [agentId] into [threadId] (`kind = card`, 09 11): the message with
  /// [text], then its [components] (`ComponentCreate`-like maps with `custom_id`, `type`,
  /// `definition` and optional `single_use`, `required_capability`, `restricted_to`,
  /// `expires_at`) bound to [runId]; the components follow as `message.updated`, as the
  /// server writes the card row before its components (09 11.4). Returns the message ID.
  String postCard(
    String agentId,
    String threadId,
    String text,
    List<JsonMap> components, {
    String? runId,
  }) {
    final created = world.createMessage(agentId, threadId, {
      'client_msg_id': world.nextId(),
      'content_md': text,
      'kind': 'card',
    }, origin: 'agent');
    final id = created.message['id']! as String;
    final stored = <JsonMap>[];
    for (var i = 0; i < components.length; i++) {
      final c = components[i];
      final capability = c['required_capability'] as String?;
      stored.add({
        'id': world.nextId(),
        'custom_id': c['custom_id'],
        'type': c['type'],
        'definition': c['definition'],
        'position': i + 1,
        'single_use': c['single_use'] ?? true,
        'required_capability': ?capability,
        // 09 11.1: `run_starter` is the default with `approve_actions` (SEC-AI-4).
        'restricted_to':
            c['restricted_to'] ??
            (capability == 'approve_actions' ? 'run_starter' : 'any'),
        'run_id': ?runId,
        'state': 'active',
        'expires_at': ?c['expires_at'],
        'version': 1,
      });
    }
    world.updateMessageFields(id, {'components': stored}, actor: agentId);
    return id;
  }

  /// Writes the live box anchor of [runId] (`kind = run`, 09 5.4): an agent message without
  /// text behind the triggering message, then its `run_id` as `message.updated`.
  String postAnchor(String agentId, String runId) {
    final run = world.runs[runId]!;
    final created = world.createMessage(agentId, run['thread_id']! as String, {
      'client_msg_id': world.nextId(),
      'content_md': '',
      'kind': 'run',
    }, origin: 'agent');
    final id = created.message['id']! as String;
    world.updateMessageFields(id, {'run_id': runId}, actor: agentId);
    return id;
  }

  /// The component [componentId] with its message, or null.
  ({JsonMap message, JsonMap component})? componentById(String componentId) {
    for (final m in world.messages.values) {
      for (final c in (m['components'] as List<Object?>? ?? const [])) {
        if (c is JsonMap && c['id'] == componentId) {
          return (message: m, component: c);
        }
      }
    }
    return null;
  }

  /// Whether [componentId] was used (a card's script waits for it).
  bool used(String componentId) =>
      componentById(componentId)?.component['state'] == 'used';

  /// The current agent session of [agentId] in [threadId], created on first use (09 9).
  JsonMap currentSession(String threadId, String agentId) {
    final list = sessions.putIfAbsent(threadId, () => []);
    for (final s in list) {
      if (s['is_current'] == true) return s;
    }
    return _newSession(threadId, agentId);
  }

  /// Ends the current session of [threadId] with [reason] and starts a new one, bound to
  /// [externalRef] when given (`resetAgentSession`, `bindAgentSession`).
  JsonMap replaceSession(
    String threadId,
    String agentId, {
    required String reason,
    String? externalRef,
  }) {
    for (final s in sessions.putIfAbsent(threadId, () => [])) {
      if (s['is_current'] == true) {
        s
          ..['is_current'] = false
          ..['ended_at'] = world.stamp()
          ..['end_reason'] = reason
          ..['version'] = (s['version']! as int) + 1;
      }
    }
    return _newSession(threadId, agentId, externalRef: externalRef);
  }

  /// Adds an earlier session of [agentId] in [threadId] with [externalRef] (`/resume`
  /// chooses it).
  JsonMap addEarlierSession(
    String threadId,
    String agentId,
    String externalRef,
  ) {
    final s = _session(threadId, agentId, externalRef: externalRef)
      ..['is_current'] = false
      ..['ended_at'] = world.stamp()
      ..['end_reason'] = 'reset';
    sessions.putIfAbsent(threadId, () => []).insert(0, s);
    return s;
  }

  JsonMap _newSession(String threadId, String agentId, {String? externalRef}) {
    final s = _session(threadId, agentId, externalRef: externalRef);
    sessions.putIfAbsent(threadId, () => []).add(s);
    return s;
  }

  JsonMap _session(String threadId, String agentId, {String? externalRef}) => {
    'id': world.nextId(),
    'thread_id': threadId,
    'agent_id': agentId,
    'external_session_ref': ?externalRef,
    'is_current': true,
    'started': externalRef != null,
    'cost': {'amount_micros': 0, 'currency': 'EUR', 'is_estimate': true},
    'created_at': world.stamp(),
    'version': 1,
  };
}
