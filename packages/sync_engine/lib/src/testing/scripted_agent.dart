// SPDX-License-Identifier: Apache-2.0

/// The scripted agent of the fake server (brief M4-17 Design note 6): it turns a mention of an
/// agent into the run of 04 16.5, step by step, as a test script says.
library;

import 'dart:async';

import 'world.dart';
import 'world_agents.dart';

/// One step of an [AgentScript].
sealed class AgentStep {
  const AgentStep();

  /// The agent took the invocation: `trigger_state` `queued` (04 16.5).
  const factory AgentStep.queue() = _Queue;

  /// The run starts: `run.state_changed` `running` and `trigger_state` `working`.
  const factory AgentStep.start() = _Start;

  /// The adapter patches the run document with [ops] (`run.update`, 04 11.2).
  const factory AgentStep.patch(List<JsonMap> ops) = _Patch;

  /// Waits until the triggering person sent a follow-up into the thread.
  const factory AgentStep.awaitFollowup() = _AwaitFollowup;

  /// The follow-up reached the runtime: `followups/<id>/state` `delivered` and
  /// `trigger_state` `followup_delivered` on the follow-up message.
  const factory AgentStep.deliverFollowup() = _DeliverFollowup;

  /// The runtime absorbed the follow-up: `followups/<id>/state` `absorbed`.
  const factory AgentStep.absorbFollowup() = _AbsorbFollowup;

  /// The agent posts [text] into the thread (origin `agent`).
  const factory AgentStep.reply(String text) = _Reply;

  /// The run ends with [state] (`completed`, `failed`) and `trigger_state` `done` or
  /// `failed`.
  const factory AgentStep.finish({String state}) = _Finish;

  /// The agent posts a card with [text] and [components] bound to the run (09 11; M4-21);
  /// see `FakeAgentInterface.postCard` for the component maps.
  const factory AgentStep.card(String text, List<JsonMap> components) = _Card;

  /// Waits until a component of the last card of the run was used (an interaction).
  const factory AgentStep.awaitInteraction() = _AwaitInteraction;
}

final class _Card extends AgentStep {
  const _Card(this.text, this.components);

  final String text;

  final List<JsonMap> components;
}

final class _AwaitInteraction extends AgentStep {
  const _AwaitInteraction();
}

final class _Queue extends AgentStep {
  const _Queue();
}

final class _Start extends AgentStep {
  const _Start();
}

final class _Patch extends AgentStep {
  const _Patch(this.ops);

  final List<JsonMap> ops;
}

final class _AwaitFollowup extends AgentStep {
  const _AwaitFollowup();
}

final class _DeliverFollowup extends AgentStep {
  const _DeliverFollowup();
}

final class _AbsorbFollowup extends AgentStep {
  const _AbsorbFollowup();
}

final class _Reply extends AgentStep {
  const _Reply(this.text);

  final String text;
}

final class _Finish extends AgentStep {
  const _Finish({this.state = 'completed'});

  final String state;
}

/// The steps of a run and the pause before each one.
final class AgentScript {
  /// Creates the script.
  const AgentScript(
    this.steps, {
    this.pause = const Duration(milliseconds: 20),
  });

  /// The steps in order.
  final List<AgentStep> steps;

  /// The pause before every step.
  final Duration pause;

  /// The flow of 04 16.5: queue, start, a status line and a step, a follow-up delivered and
  /// absorbed, a reply and the end.
  static AgentScript followUpRun(
    String stepId,
    String startedAt, {
    String reply = 'Done.',
  }) => AgentScript([
    const AgentStep.queue(),
    const AgentStep.start(),
    const AgentStep.patch([
      {'op': 'add', 'path': '/status_line', 'value': 'Reading the thread'},
    ]),
    AgentStep.patch([
      {
        'op': 'add',
        'path': '/steps/$stepId',
        'value': {
          'id': stepId,
          'ordinal': 1,
          'kind': 'tool_call',
          'name': 'search',
          'title': 'Search the notes',
          'status': 'running',
          'started_at': startedAt,
        },
      },
    ]),
    const AgentStep.awaitFollowup(),
    const AgentStep.deliverFollowup(),
    const AgentStep.absorbFollowup(),
    AgentStep.patch([
      {'op': 'replace', 'path': '/steps/$stepId/status', 'value': 'completed'},
      {'op': 'replace', 'path': '/status_line', 'value': 'Writing the answer'},
    ]),
    AgentStep.reply(reply),
    const AgentStep.finish(),
  ]);
}

/// An agent of the world that answers mentions with the run of its [script].
///
/// A message of a person that mentions [agentId] in a channel the agent sees gets
/// `trigger_state` `received` and a queued run in the same step; a message of the triggering
/// person in the thread of an active run is a follow-up (`followups/<message id>` with state
/// `queued`). Then the steps run one after another, each after [AgentScript.pause]. A run
/// that ends early (`cancelRun`) stops its script. Nothing is random.
///
/// Example:
/// ```dart
/// final agent = ScriptedAgent(world, agentId: robin, script: AgentScript.followUpRun(
///   world.nextId(), world.stamp()));
/// agent.attach();
/// ```
final class ScriptedAgent {
  /// Creates the agent; it acts after [attach]. With [anchors] every run gets its live box
  /// anchor (`kind = run`, 09 5.4) right after it is queued, as the server writes it in the
  /// same transaction; tests of tickets before M4-21 run without.
  ScriptedAgent(
    this.world, {
    required this.agentId,
    required this.script,
    this.anchors = false,
  });

  /// Whether runs get their anchor message.
  final bool anchors;

  /// The cards this agent posted, by run ID (message IDs in order).
  final Map<String, List<String>> cards = {};

  /// Registers the command [name] of this agent on the instance (09 10.2) with option
  /// suggestions from [suggest] (09 10.5); returns its ID.
  String registerCommand(
    String name, {
    String description = '',
    List<JsonMap> options = const [],
    bool control = false,
    FakeAutocomplete? suggest,
  }) => FakeAgentInterface.of(world).registerCommand(
    agentId,
    name,
    description: description,
    options: options,
    control: control,
    autocomplete: suggest,
  );

  /// The world.
  final FakeWorld world;

  /// The agent principal.
  final String agentId;

  /// What every run does.
  final AgentScript script;

  /// The runs this agent started, by ID.
  final List<String> runIds = [];

  /// Completes when the script of the newest run ended.
  Future<void> get done => _done.future;

  Completer<void> _done = Completer<void>();
  final Map<String, List<String>> _followups = {};
  bool _attached = false;

  /// Starts listening to created messages.
  void attach() {
    if (_attached) return;
    _attached = true;
    world.triggerChecks.add(_triggerState);
    world.messageHooks.add(_onMessage);
  }

  /// Stops listening.
  void detach() {
    _attached = false;
    world.triggerChecks.remove(_triggerState);
    world.messageHooks.remove(_onMessage);
  }

  bool _mentions(JsonMap message) =>
      (message['mention_ids']! as List<Object?>).contains(agentId) &&
      world.seesChannel(agentId, message['channel_id']! as String);

  JsonMap? _ownActiveRun(JsonMap message) {
    final run = world.activeRunOf(message['thread_id']! as String);
    return run != null && run['agent_id'] == agentId ? run : null;
  }

  bool _isFollowup(JsonMap message, FakePerson author) {
    final run = _ownActiveRun(message);
    return run != null &&
        run['triggered_by'] == author.id &&
        !_mentions(message);
  }

  String? _triggerState(JsonMap message, FakePerson author) {
    if (author.id == agentId || author.kind != 'user') return null;
    return _mentions(message) || _isFollowup(message, author)
        ? 'received'
        : null;
  }

  void _onMessage(JsonMap message, FakePerson author) {
    if (author.id == agentId || message['trigger_state'] != 'received') return;
    final id = message['id']! as String;
    final active = _ownActiveRun(message);
    if (active != null &&
        active['triggered_by'] == author.id &&
        !_mentions(message)) {
      final runId = active['id']! as String;
      _followups.putIfAbsent(runId, () => []).add(id);
      world.patchRun(runId, [
        {
          'op': 'add',
          'path': '/followups/$id',
          'value': {'message_id': id, 'state': 'queued'},
        },
      ]);
      return;
    }
    if (!_mentions(message) || active != null) return;
    final run = world.createRun(
      agentId: agentId,
      triggeringMessageId: id,
      triggeredBy: author.id,
    );
    final runId = run['id']! as String;
    runIds.add(runId);
    if (anchors) FakeAgentInterface.of(world).postAnchor(agentId, runId);
    _done = Completer<void>();
    unawaited(_play(runId, id));
  }

  Future<void> _play(String runId, String messageId) async {
    try {
      for (final step in script.steps) {
        await Future<void>.delayed(script.pause);
        if (!_attached) return;
        if (world.activeRunOf(
              world.runs[runId]!['thread_id']! as String,
            )?['id'] !=
            runId) {
          return;
        }
        await _apply(step, runId, messageId);
      }
    } finally {
      if (!_done.isCompleted) _done.complete();
    }
  }

  Future<void> _apply(AgentStep step, String runId, String messageId) async {
    final run = world.runs[runId]!;
    switch (step) {
      case _Queue():
        world.setTriggerState(messageId, 'queued');
      case _Start():
        world.setRunState(runId, 'running', triggerState: 'working');
        world.setTriggerState(messageId, 'working');
      case _Patch(:final ops):
        world.patchRun(runId, ops);
      case _AwaitFollowup():
        while ((_followups[runId] ?? const []).isEmpty) {
          if (!_attached) return;
          await Future<void>.delayed(script.pause);
        }
      case _DeliverFollowup():
        final f = _followups[runId]!.first;
        world.patchRun(runId, [
          {
            'op': 'replace',
            'path': '/followups/$f/state',
            'value': 'delivered',
          },
        ]);
        world.setTriggerState(f, 'followup_delivered', actor: agentId);
      case _AbsorbFollowup():
        final f = _followups[runId]!.first;
        world.patchRun(runId, [
          {'op': 'replace', 'path': '/followups/$f/state', 'value': 'absorbed'},
        ]);
      case _Reply(:final text):
        world.createMessage(agentId, run['thread_id']! as String, {
          'client_msg_id': world.nextId(),
          'content_md': text,
        }, origin: 'agent');
      case _Card(:final text, :final components):
        final id = FakeAgentInterface.of(world).postCard(
          agentId,
          run['thread_id']! as String,
          text,
          components,
          runId: runId,
        );
        cards.putIfAbsent(runId, () => []).add(id);
      case _AwaitInteraction():
        final ui = FakeAgentInterface.of(world);
        bool used() {
          final card = cards[runId]?.last;
          final comps = card == null
              ? const <Object?>[]
              : world.messages[card]!['components']! as List<Object?>;
          return comps.any((c) => c is JsonMap && ui.used(c['id']! as String));
        }

        while (!used()) {
          if (!_attached) return;
          await Future<void>.delayed(script.pause);
        }
      case _Finish(:final state):
        final trigger = state == 'completed' ? 'done' : 'failed';
        world.setRunState(runId, state, triggerState: trigger);
        world.setTriggerState(messageId, trigger);
    }
  }
}
