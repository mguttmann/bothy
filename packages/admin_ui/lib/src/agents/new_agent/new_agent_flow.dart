// SPDX-License-Identifier: Apache-2.0

/// The steps of "New agent" with resume (docs/spec/01-product.md 6.20 items 4 and 6;
/// docs/spec/26-builtin-agents.md 8.2 to 8.4, R-BA-11, BA-T16; brief A1-08 Design note 3,
/// brief A1-09 Design notes 2 and 3).
///
/// Each step is its own action with its own audit entry; nothing is rolled back. Whether a step
/// is done is not a stored flag but a comparison of its target in the edited preview with what
/// the agent has (class, current definition, billing mode, channel and operator), so "Try
/// again" runs exactly the steps whose target is not reached yet, in the order of 26 8.2, and
/// creates nothing twice: `createAgent` and `createChannel` repeat with the same
/// `Idempotency-Key` while their request is unchanged, and the other steps are writes with
/// `If-Match`. A class above the agent's current one is set before the definition, a lower one
/// after it (26 8.3, 8.4), so a `stdio` server is stored under `host` and a removed one no
/// longer blocks the way back to `chat` (R-BA-06).
///
/// The same flow edits an existing built-in agent ([NewAgentFlow.forAgent], A1-09): "Edit" and
/// "Restore" ([NewAgentMode.edit]) run only the class and definition steps, the definition
/// with `If-Match` of the version the edit started from, so a change of another device in
/// between is `412 version_conflict` and [NewAgentFlow.reload] starts again from the current
/// version; "Show" ([NewAgentMode.view]) changes nothing; "Continue setup"
/// ([NewAgentMode.setup]) resumes the steps of 26 8.2 for an agent without a definition.
library;

import 'package:flutter/foundation.dart';

import 'agent_edit_changes.dart';
import 'new_agent_draft.dart';
import 'new_agent_port.dart';

/// What the flow does with the agent.
enum NewAgentMode {
  /// "New agent": every step of 26 8.2 from `agent.create` on.
  create,

  /// "Continue setup" of an agent without a definition: the steps after `agent.create`, with
  /// the agent's identity and runner fixed (26 8.3).
  setup,

  /// "Edit" or "Restore": the class and definition steps of 26 8.4.
  edit,

  /// "Show" of a stored version: read-only, no step.
  view,
}

/// The steps of 26 8.2.
enum NewAgentStep {
  /// `agent.create` with `hosted_by` and `adapter_kind = 'claude_agent'`.
  createAgent,

  /// `agent.tool_class.set`: before the definition when the class rises, after it when it falls.
  setClass,

  /// `putAgentDefinition`.
  storeDefinition,

  /// `agent.billing_mode.set`, only for `personal` or to undo it.
  setBilling,

  /// `channel.create`, only when a channel is asked for.
  createChannel,

  /// `channel.operator.set` with the new agent.
  setOperator,
}

/// The state of a step (01 6.20 item 4).
enum NewAgentStepState {
  /// Not started.
  waiting,

  /// Running now.
  running,

  /// Done.
  done,

  /// Failed; [NewAgentFlow.failure] says why.
  failed,
}

/// Why `personal` is not offered (10 10.3 item 3), or [offered].
enum PersonalOffer {
  /// `personal` may be chosen.
  offered,

  /// The signed-in person is not the only owner.
  notOnlyOwner,

  /// The hosting runner itself is not `personal`.
  runnerNotPersonal,

  /// The runner's credential holder is someone else.
  notHolder,

  /// The runner does not report `personal` in `billing_modes_supported`.
  notSupported,
}

/// The JSON Pointers of the preview fields that are not part of the definition; the definition's
/// own fields use the pointers of `agent_definition.v1` (for example `/skills/0/body`).
abstract final class PreviewField {
  /// The display name.
  static const String name = '/name';

  /// The handle.
  static const String handle = '/handle';

  /// The description.
  static const String description = '/description';

  /// The hosting runner.
  static const String host = '/hosted_by';

  /// The cost cap.
  static const String costCap = '/settings/max_cost_usd_per_run';

  /// The channel's space.
  static const String channelSpace = '/channel/space';

  /// The channel's name.
  static const String channelName = '/channel/name';
}

final RegExp _handlePattern = RegExp(r'^[a-z0-9][a-z0-9._-]*$');

/// What a finished dialog created.
final class NewAgentResult {
  /// Creates the result.
  const NewAgentResult({required this.agentId, this.channelId});

  /// The new agent.
  final String agentId;

  /// The new channel, or null when none was asked for.
  final String? channelId;
}

/// The preview and the steps of one new agent.
///
/// Example:
/// ```dart
/// final flow = NewAgentFlow(port: port, offer: offer, newKey: ids.next)
///   ..useTemplate(offer.templates.first);
/// flow.edit((d) => d.toolClass = AgentToolClass.host);
/// await flow.run();
/// ```
final class NewAgentFlow extends ChangeNotifier {
  /// Creates the flow over [port] for [offer]; [newKey] makes an `Idempotency-Key` (UUIDv7);
  /// [context] is the signed-in owner and the spaces, which [setContext] can give later.
  NewAgentFlow({
    required this._port,
    required this.offer,
    required this._newKey,
    this._context,
  }) : mode = NewAgentMode.create,
       _runner = offer.runners.length == 1 ? offer.runners.single : null;

  /// Creates the flow for the existing built-in [agent] on [runner] (A1-09).
  ///
  /// With a current definition [stored] it edits ([NewAgentMode.edit]); [content] is an older
  /// version to restore, edited against [stored]; [readOnly] shows [content] or [stored]
  /// without a step ([NewAgentMode.view]). Without [stored] it continues the setup
  /// ([NewAgentMode.setup]): the person picks a way in, the agent keeps its identity.
  ///
  /// Example:
  /// ```dart
  /// final flow = NewAgentFlow.forAgent(port: port, offer: offer, newKey: ids.next,
  ///     agent: await port.agent(id), runner: runner, stored: await port.definition(id));
  /// flow.edit((d) => d.instructions = 'Keep the notes of Robin.');
  /// await flow.run(); // putDefinition with If-Match of the stored version
  /// ```
  NewAgentFlow.forAgent({
    required this._port,
    required this.offer,
    required this._newKey,
    required NewAgentState agent,
    required HostingRunner this._runner,
    StoredDefinition? stored,
    Map<String, Object?>? content,
    bool readOnly = false,
    this._context,
  }) : mode = readOnly
           ? NewAgentMode.view
           : stored == null && content == null
           ? NewAgentMode.setup
           : NewAgentMode.edit,
       _agent = agent,
       _stored = stored,
       _base = stored?.version,
       _baseJson = stored?.json,
       _billing = agent.billingMode {
    final json = content ?? stored?.json;
    if (json != null) _draft = _draftOf(json, agent);
  }

  final NewAgentPort _port;
  final String Function() _newKey;

  /// What the flow does.
  final NewAgentMode mode;

  /// The templates and runners.
  final NewAgentOffer offer;

  NewAgentContext? _context;
  NewAgentDraft? _draft;
  HostingRunner? _runner;
  String _billing = 'organization';

  NewAgentState? _agent;
  StoredDefinition? _stored;
  String? _channelId;
  bool _operatorSet = false;
  bool _classAfter = false;

  /// The version and content an edit started from: the `If-Match` of its definition write and
  /// the "before" of [changes].
  int? _base;
  Map<String, Object?>? _baseJson;
  bool _ran = false;
  bool _classRan = false;
  bool _definitionRan = false;

  String? _createKey;
  String? _createRequest;
  String? _channelKey;
  String? _channelRequest;

  bool _busy = false;
  bool _attempted = false;
  bool _closed = false;
  NewAgentStep? _running;
  NewAgentStep? _failedStep;
  NewAgentFailure? _failure;
  Map<String, List<String>> _fieldErrors = const {};
  int _costMicros = 0;
  String? _currency;
  bool _hasCost = false;

  /// The signed-in owner and the spaces, or null until known.
  NewAgentContext? get context => _context;

  /// Gives the context once it is read.
  void setContext(NewAgentContext value) {
    _context = value;
    if (personalOffer != PersonalOffer.offered) _billing = 'organization';
    _notify();
  }

  /// The draft the preview edits, or null before a way in was chosen.
  NewAgentDraft? get draft => _draft;

  /// Starts the preview from [template] (`origin.kind = template`).
  void useTemplate(NewAgentTemplate template) => _use(
    NewAgentDraft.fromTemplate(
      slug: template.slug,
      name: template.name,
      description: template.description,
      version: template.version,
      definition: template.definition,
      requiredSecrets: template.requiredSecrets,
    ),
  );

  /// Starts the preview from the builder's [reply] with its draft, its findings at their fields
  /// and the [costMicros] of the whole interview in [currency].
  void useInterview(InterviewReply reply, {int? costMicros, String? currency}) {
    final spaces = _context?.spaces ?? const <NewAgentSpace>[];
    _use(
      NewAgentDraft.fromInterview(
        reply.draft ?? const {},
        spaceIdOf: (name) =>
            spaces.where((s) => s.name == name).firstOrNull?.id,
      ),
      errors: reply.errors,
    );
    if (costMicros != null) {
      _hasCost = true;
      _costMicros = costMicros;
      _currency = currency;
    }
  }

  /// Starts the preview from the empty form.
  void useEmpty() => _use(NewAgentDraft.empty());

  void _use(NewAgentDraft draft, {List<FieldIssue> errors = const []}) {
    if (_busy) return;
    switch (mode) {
      case NewAgentMode.create:
        if (_agent != null) return;
      case NewAgentMode.setup:
        // The agent exists: a way in gives the content, the identity stays the agent's.
        if (_ran) return;
        final a = _agent!;
        draft
          ..name = a.name
          ..handle = a.handle
          ..description = a.description;
      case NewAgentMode.edit || NewAgentMode.view:
        return;
    }
    _draft = draft;
    _fieldErrors = _group(errors);
    _notify();
  }

  /// The draft of a stored definition [json] of [agent]; a restored content with a `stdio`
  /// server preselects `host`, which it needs (26 4.4 item 4).
  static NewAgentDraft _draftOf(
    Map<String, Object?> json,
    NewAgentState agent,
  ) {
    final d = NewAgentDraft.fromStored(
      json,
      name: agent.name,
      handle: agent.handle,
      description: agent.description,
      toolClass: agent.toolClass,
    );
    if (d.hasStdio) d.toolClass = AgentToolClass.host;
    return d;
  }

  /// The interview's cost estimate so far in micro units, or null without an interview.
  int? get costMicros => _hasCost ? _costMicros : null;

  /// The currency of [costMicros].
  String? get currency => _currency;

  /// Changes the draft through [change]; does nothing while a step runs. The identity is locked
  /// once the agent exists, the channel once it is created ([canEditIdentity],
  /// [canEditChannel]); [change] may still touch them, so the locked values are put back.
  void edit(void Function(NewAgentDraft draft) change) {
    final d = _draft;
    if (d == null || !canEditContent) return;
    final identity = (d.name, d.handle, d.description);
    final channel = (d.channelWanted, d.channelSpaceId, d.channelName);
    change(d);
    if (!canEditIdentity) {
      d
        ..name = identity.$1
        ..handle = identity.$2
        ..description = identity.$3;
    }
    if (!canEditChannel) {
      d
        ..channelWanted = channel.$1
        ..channelSpaceId = channel.$2
        ..channelName = channel.$3;
    }
    _notify();
  }

  /// The hosting runner; editable until `agent.create` succeeded (26 8.3).
  HostingRunner? get runner => _runner;
  set runner(HostingRunner? value) {
    if (!canEditHost) return;
    _runner = value;
    if (personalOffer != PersonalOffer.offered) _billing = 'organization';
    _notify();
  }

  /// The billing mode to set: `organization` (I1 default) or `personal`.
  String get billing => _billing;
  set billing(String value) {
    // An edit keeps the billing mode; "Switch backend" of the agent card changes it (10 9).
    if (_busy || mode == NewAgentMode.edit || mode == NewAgentMode.view) {
      return;
    }
    if (value == 'personal' && personalOffer != PersonalOffer.offered) return;
    _billing = value;
    _notify();
  }

  /// Whether `personal` is offered and, if not, why (10 10.3 item 3).
  PersonalOffer get personalOffer {
    final r = _runner;
    final c = _context;
    if (c == null || !c.onlyOwner) return PersonalOffer.notOnlyOwner;
    if (r == null || r.billingMode != 'personal') {
      return PersonalOffer.runnerNotPersonal;
    }
    if (r.holderId != c.meId) return PersonalOffer.notHolder;
    if (!r.modesSupported.contains('personal')) {
      return PersonalOffer.notSupported;
    }
    return PersonalOffer.offered;
  }

  /// Whether the name, handle and description may change: until the agent exists.
  bool get canEditIdentity => _agent == null && !_busy;

  /// Whether the hosting runner may change: until `agent.create` succeeded (26 8.3); an
  /// existing agent moves with "Change runner" (`agent.host.set`, 26 8.4).
  bool get canEditHost => _agent == null && !_busy;

  /// Whether the channel may change: until it is created; an edit creates no channel.
  bool get canEditChannel =>
      _channelId == null &&
      !_busy &&
      (mode == NewAgentMode.create || mode == NewAgentMode.setup);

  /// Whether the definition, the class and the settings may change: not while a step runs and
  /// never in [NewAgentMode.view].
  bool get canEditContent => !_busy && mode != NewAgentMode.view;

  /// The changed parts of an edit against the version it started from, then the class
  /// (26 8.4); empty outside [NewAgentMode.edit].
  List<DefinitionChange> get changes {
    final d = _draft;
    final before = _baseJson;
    if (mode != NewAgentMode.edit || d == null || before == null) {
      return const [];
    }
    final from = currentClass;
    return [
      ...definitionChanges(before, d.definitionJson()),
      if (d.toolClass != from)
        DefinitionChange(
          DefinitionChangeKind.toolClass,
          d.toolClass.wire,
          from.wire,
        ),
    ];
  }

  /// Whether an edit has something to save.
  bool get hasChanges => changes.isNotEmpty;

  /// Whether the draft has a local (`stdio`) MCP server.
  bool get hasLocalServers => _draft?.hasStdio ?? false;

  /// Removes every local (`stdio`) MCP server from the draft, the way out of
  /// `definition_needs_host` (R-BA-06).
  void removeLocalServers() =>
      edit((d) => d.servers.removeWhere((s) => s.isStdio));

  /// The created agent, or null.
  NewAgentState? get agent => _agent;

  /// The created channel, or null.
  String? get channelId => _channelId;

  /// Whether a step runs.
  bool get busy => _busy;

  /// Whether "Create" was pressed at least once, also when [invalidFields] stopped it; the
  /// preview marks the invalid fields from then on.
  bool get attempted => _attempted;

  /// Whether a step ran or runs; for an existing agent, whether "Save" or "Continue setup" was
  /// pressed.
  bool get started => mode == NewAgentMode.create
      ? _agent != null || _busy || _failedStep != null
      : _ran || _busy || _failedStep != null;

  /// The failed step, or null.
  NewAgentStep? get failedStep => _failedStep;

  /// Why [failedStep] failed, or null.
  NewAgentFailure? get failure => _failure;

  /// The server's field errors by JSON Pointer of the preview (codes in order).
  Map<String, List<String>> get fieldErrors => _fieldErrors;

  /// The field errors at [pointer] and below it.
  List<String> errorsAt(String pointer) => [
    for (final MapEntry(:key, :value) in _fieldErrors.entries)
      if (key == pointer) ...value,
  ];

  /// The tool class of the agent as the flow knows it, `chat` before it exists.
  AgentToolClass get currentClass => _agent?.toolClass ?? AgentToolClass.chat;

  /// The class the preview asks for.
  AgentToolClass get targetClass => _draft?.toolClass ?? AgentToolClass.chat;

  /// The steps in their order (26 8.2, 8.3): the class step before the definition when the class
  /// rises and after it when it falls; billing only for `personal` or to undo it; the channel
  /// steps only for a channel.
  List<NewAgentStep> get steps {
    final d = _draft;
    final target = targetClass;
    final current = currentClass;
    if (mode == NewAgentMode.edit || mode == NewAgentMode.view) {
      // 26 8.4: only what the edit changes, each a visible step; a raised class before the
      // definition, a lowered one after it.
      final classShown = target != current || _classRan;
      final after =
          current.isAbove(target) || (target == current && _classAfter);
      final definitionShown =
          _definitionRan || !_reached(NewAgentStep.storeDefinition);
      return [
        if (classShown && !after) NewAgentStep.setClass,
        if (definitionShown) NewAgentStep.storeDefinition,
        if (classShown && after) NewAgentStep.setClass,
      ];
    }
    final classShown =
        target != AgentToolClass.chat ||
        current != AgentToolClass.chat ||
        _classAfter;
    final after = current.isAbove(target) || (target == current && _classAfter);
    final billingShown =
        _billing == 'personal' ||
        (_agent != null && _agent!.billingMode != 'organization');
    final channelShown = (d?.channelWanted ?? false) || _channelId != null;
    return [
      NewAgentStep.createAgent,
      if (classShown && !after) NewAgentStep.setClass,
      NewAgentStep.storeDefinition,
      if (classShown && after) NewAgentStep.setClass,
      if (billingShown) NewAgentStep.setBilling,
      if (channelShown) ...[
        NewAgentStep.createChannel,
        NewAgentStep.setOperator,
      ],
    ];
  }

  /// The state of [step].
  NewAgentStepState stateOf(NewAgentStep step) {
    if (_running == step) return NewAgentStepState.running;
    if (_failedStep == step) return NewAgentStepState.failed;
    return _reached(step) ? NewAgentStepState.done : NewAgentStepState.waiting;
  }

  /// Whether every step is done after "Create", "Save" or "Continue setup".
  bool get finished =>
      _ran && _agent != null && !_busy && steps.every(_reached);

  /// What the dialog returns once [finished].
  NewAgentResult? get result => finished
      ? NewAgentResult(agentId: _agent!.id, channelId: _channelId)
      : null;

  bool _reached(NewAgentStep step) {
    final agent = _agent;
    final d = _draft;
    if (agent == null || d == null) return false;
    return switch (step) {
      NewAgentStep.createAgent => true,
      NewAgentStep.setClass => agent.toolClass == d.toolClass,
      NewAgentStep.storeDefinition => sameDefinition(
        _stored?.json,
        d.definitionJson(),
      ),
      NewAgentStep.setBilling =>
        agent.billingMode == _billing &&
            (_billing != 'personal' || agent.holderId == _runner?.holderId),
      NewAgentStep.createChannel => _channelId != null,
      NewAgentStep.setOperator => _operatorSet,
    };
  }

  /// The fields that block "Create", by the pointer of the preview; the server checks the
  /// rest and answers at its field.
  Set<String> get invalidFields {
    final d = _draft;
    if (d == null) return const {};
    final cap = d.costCapValue;
    return {
      if (d.name.trim().isEmpty) PreviewField.name,
      if (!_handlePattern.hasMatch(d.handle)) PreviewField.handle,
      if (_runner == null) PreviewField.host,
      if (cap != null && (cap.isNaN || cap <= 0)) PreviewField.costCap,
      if (d.channelWanted && d.channelSpaceId == null)
        PreviewField.channelSpace,
      if (d.channelWanted && d.channelName.trim().isEmpty)
        PreviewField.channelName,
    };
  }

  /// Runs every step whose target is not reached yet, in the order of [steps]; returns when all
  /// are done, a step failed or the flow was disposed. Before a retry it reads the agent and its
  /// current definition again, so the comparison uses what the agent has (26 8.3). Does nothing
  /// with [invalidFields] or while it runs.
  Future<void> run() async {
    final d = _draft;
    if (d == null || _busy || _closed || mode == NewAgentMode.view) return;
    if (!_attempted) {
      _attempted = true;
      _notify();
    }
    if (invalidFields.isNotEmpty) return;
    _ran = true;
    _busy = true;
    _failedStep = null;
    _failure = null;
    _fieldErrors = const {};
    _notify();
    try {
      if (_agent case final agent?) {
        try {
          _agent = await _port.agent(agent.id);
          _stored = await _port.definition(agent.id);
        } on Object catch (e) {
          _fail(_firstOpen() ?? NewAgentStep.createAgent, e);
          return;
        }
      }
      // Each pass runs one step; a step that runs but does not reach its target ends the run,
      // so a server that answers unexpectedly can never loop.
      for (var pass = 0; pass <= NewAgentStep.values.length; pass++) {
        if (_closed) return;
        final step = _firstOpen();
        if (step == null) return;
        _running = step;
        _notify();
        try {
          await _do(step, d);
        } on Object catch (e) {
          _running = null;
          _fail(step, e);
          return;
        }
        _running = null;
        if (!_reached(step)) {
          _fail(step, const NewAgentFailure.problem('internal_error'));
          return;
        }
        _notify();
      }
    } finally {
      _busy = false;
      _running = null;
      _notify();
    }
  }

  /// Reads the agent and its current definition again and starts the edit from there; the
  /// edits made here are dropped. The answer to `412 version_conflict` of an edit (06 8: a
  /// person decides; 26 4.1 item 2). Does nothing outside [NewAgentMode.edit] or while a step
  /// runs.
  Future<void> reload() async {
    final a = _agent;
    if (a == null || _busy || _closed || mode != NewAgentMode.edit) return;
    _busy = true;
    _notify();
    try {
      final agent = await _port.agent(a.id);
      final stored = await _port.definition(a.id);
      _agent = agent;
      _stored = stored;
      _base = stored?.version;
      _baseJson = stored?.json;
      if (stored != null) _draft = _draftOf(stored.json, agent);
      _failedStep = null;
      _failure = null;
      _fieldErrors = const {};
      _ran = false;
      _attempted = false;
      _classRan = false;
      _definitionRan = false;
      _classAfter = false;
    } on Object catch (e) {
      _fail(_failedStep ?? NewAgentStep.storeDefinition, e);
    } finally {
      _busy = false;
      _notify();
    }
  }

  AgentChangeKind get _change => switch (mode) {
    NewAgentMode.setup => AgentChangeKind.setup,
    NewAgentMode.edit || NewAgentMode.view => AgentChangeKind.edit,
    NewAgentMode.create => AgentChangeKind.create,
  };

  NewAgentStep? _firstOpen() {
    for (final s in steps) {
      if (!_reached(s)) return s;
    }
    return null;
  }

  void _fail(NewAgentStep step, Object error) {
    if (_closed) return;
    final failure = error is NewAgentFailure
        ? error
        // Any other error (a malformed answer) fails the step too, so the person keeps "Try
        // again" instead of a step that runs forever.
        : const NewAgentFailure.problem('internal_error');
    _failedStep = step;
    _failure = failure;
    _fieldErrors = _group([
      for (final e in failure.errors)
        FieldIssue(_pointerOf(step, e.pointer), e.code),
    ]);
  }

  /// The preview pointer of a server pointer of [step]: the definition's pointers are the
  /// preview's, `createAgent` and `createChannel` name their own members.
  static String _pointerOf(NewAgentStep step, String pointer) => switch (step) {
    NewAgentStep.createAgent => switch (pointer) {
      '/display_name' => PreviewField.name,
      _ => pointer,
    },
    NewAgentStep.createChannel => switch (pointer) {
      '/name' => PreviewField.channelName,
      _ => pointer,
    },
    _ => pointer,
  };

  Future<void> _do(NewAgentStep step, NewAgentDraft d) async {
    final name = d.name.trim();
    switch (step) {
      case NewAgentStep.createAgent:
        final runner = _runner!;
        final request = [d.handle, name, d.description, runner.id].join('\n');
        // One key per request: a repeat of the same request is answered from the server's
        // record, a changed handle is a new request.
        if (_createKey == null || _createRequest != request) {
          _createKey = _newKey();
          _createRequest = request;
        }
        _agent = await _port.createAgent(
          handle: d.handle,
          displayName: name,
          description: d.description,
          hostedBy: runner.id,
          idempotencyKey: _createKey!,
        );
      case NewAgentStep.setClass:
        // A lower class goes after the definition and stays there in the list once done.
        _classAfter = currentClass.isAbove(d.toolClass);
        _classRan = true;
        final fresh = await _port.agent(_agent!.id);
        _agent = fresh;
        if (fresh.toolClass == d.toolClass) return;
        _agent = await _port.setToolClass(
          fresh,
          d.toolClass,
          name: name,
          change: _change,
        );
      case NewAgentStep.storeDefinition:
        _definitionRan = true;
        _stored = await _port.putDefinition(
          _agent!.id,
          d.definitionJson(),
          // An edit writes against the version it started from, so a change of another device
          // in between is `412 version_conflict` instead of being overwritten (26 4.1).
          ifMatch: mode == NewAgentMode.edit ? _base : _stored?.version,
          name: name,
          change: _change,
        );
        _base = _stored!.version;
        _baseJson = _stored!.json;
        // The write raised the agent's version (`agent.updated`); the next `If-Match` needs it.
        _agent = await _port.agent(_agent!.id);
      case NewAgentStep.setBilling:
        _agent = await _port.setBilling(
          await _port.agent(_agent!.id),
          _billing,
          holderId: _billing == 'personal' ? _runner?.holderId : null,
          name: name,
          change: _change,
        );
      case NewAgentStep.createChannel:
        final request = [d.channelSpaceId, d.channelName.trim()].join('\n');
        if (_channelKey == null || _channelRequest != request) {
          _channelKey = _newKey();
          _channelRequest = request;
        }
        _channelId = await _port.createChannel(
          d.channelSpaceId!,
          d.channelName.trim(),
          idempotencyKey: _channelKey!,
          name: name,
          change: _change,
        );
      case NewAgentStep.setOperator:
        await _port.setOperator(
          _channelId!,
          _agent!.id,
          name: name,
          change: _change,
        );
        _operatorSet = true;
    }
  }

  static Map<String, List<String>> _group(List<FieldIssue> errors) {
    final out = <String, List<String>>{};
    for (final e in errors) {
      (out[e.pointer] ??= []).add(e.code);
    }
    return out;
  }

  void _notify() {
    if (!_closed) notifyListeners();
  }

  @override
  void dispose() {
    _closed = true;
    super.dispose();
  }
}
