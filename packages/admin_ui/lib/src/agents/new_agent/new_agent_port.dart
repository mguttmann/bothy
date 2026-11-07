// SPDX-License-Identifier: Apache-2.0

/// The server side of "New agent" (docs/spec/26-builtin-agents.md 6.1, 7.3, 8.1, 8.2; brief
/// A1-08 Scope): the port [NewAgentPort] with its value types and [ApiNewAgentPort] over the
/// generated client.
///
/// Every operation runs through the client of the signed-in session, so DPoP or the console's
/// cookie, the refresh and the step-up interceptor apply and the step-up prompt appears where
/// the server demands it (07 2.13: `agent.create`, `agent.tool_class.set`, `agent.update` and
/// `agent.billing_mode.set` have step-up). The port decides no permission: whether the entry
/// shows is the server's answer (`listAgentTemplates` and `hosts_agents` reach owners only).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart'
    show AuthException, StepUpUnavailable, stepUpPurposeExtra;
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';

import 'new_agent_draft.dart';

/// The `adapter_kind` of a built-in agent (26 3.1).
const String builtinAdapterKind = 'claude_agent';

/// A runner that hosts built-in agents (`presence.hosts_agents`, 26 8.1).
final class HostingRunner {
  /// Creates the entry.
  const HostingRunner({
    required this.id,
    required this.name,
    required this.ceiling,
    this.billingMode = 'organization',
    this.holderId,
    this.modesSupported = const [],
  });

  /// The runner agent's ID.
  final String id;

  /// Its display name.
  final String name;

  /// The highest tool class it loads for a hosted agent (26 3.1 item 3).
  final AgentToolClass ceiling;

  /// Its own billing mode, `personal` or `organization`.
  final String billingMode;

  /// Its credential holder with `personal`, else null.
  final String? holderId;

  /// The modes it reports in `billing_modes_supported`.
  final List<String> modesSupported;
}

/// A template of `listAgentTemplates` (26 7.3).
final class NewAgentTemplate {
  /// Creates the entry.
  const NewAgentTemplate({
    required this.slug,
    required this.name,
    required this.description,
    required this.version,
    required this.definition,
    this.privateSource = false,
    this.requiredSecrets = const [],
  });

  /// The folder name.
  final String slug;

  /// One line for the picker.
  final String name;

  /// One line on what an agent from it does.
  final String description;

  /// The template's version.
  final String version;

  /// The draft as `agent_definition.v1` JSON.
  final Map<String, Object?> definition;

  /// Whether it comes from `agents.templates_dir` of the installation.
  final bool privateSource;

  /// The secret names it needs on the runner host.
  final List<String> requiredSecrets;
}

/// What "New agent" needs to show: the templates and the runners that host agents. The entry is
/// shown only with an offer (26 8.1).
final class NewAgentOffer {
  /// Creates the offer.
  const NewAgentOffer({required this.templates, required this.runners});

  /// The templates, shipped first.
  final List<NewAgentTemplate> templates;

  /// The active runners that report `hosts_agents`; never empty.
  final List<HostingRunner> runners;
}

/// A space the channel can be created in.
final class NewAgentSpace {
  /// Creates the entry.
  const NewAgentSpace(this.id, this.name);

  /// The space ID.
  final String id;

  /// Its name.
  final String name;
}

/// What the preview needs about the signed-in owner (10 10.3 item 3) and the spaces.
final class NewAgentContext {
  /// Creates the context.
  const NewAgentContext({
    required this.meId,
    required this.onlyOwner,
    this.spaces = const [],
  });

  /// The signed-in person.
  final String meId;

  /// Whether that person is the only owner.
  final bool onlyOwner;

  /// The spaces the person sees.
  final List<NewAgentSpace> spaces;
}

/// The agent as the steps need it (`Agent`).
final class NewAgentState {
  /// Creates the state.
  const NewAgentState({
    required this.id,
    required this.version,
    required this.toolClass,
    this.billingMode = 'organization',
    this.holderId,
  });

  /// The agent ID.
  final String id;

  /// The version for `If-Match`.
  final int version;

  /// The agent's tool class.
  final AgentToolClass toolClass;

  /// The configured billing mode.
  final String billingMode;

  /// The credential holder with `personal`.
  final String? holderId;
}

/// A stored definition version (`AgentDefinitionVersion`).
final class StoredDefinition {
  /// Creates the version.
  const StoredDefinition(this.version, this.json);

  /// The version number, the `If-Match` of the next write.
  final int version;

  /// The stored definition.
  final Map<String, Object?> json;
}

/// A field error of the server (`FieldError`): its JSON Pointer and code.
final class FieldIssue {
  /// Creates the issue.
  const FieldIssue(this.pointer, this.code);

  /// The JSON Pointer.
  final String pointer;

  /// The code, for example `tool_class_too_low`.
  final String code;

  @override
  bool operator ==(Object other) =>
      other is FieldIssue && other.pointer == pointer && other.code == code;

  @override
  int get hashCode => Object.hash(pointer, code);

  @override
  String toString() => '$pointer:$code';
}

/// Who said a turn of the interview.
enum InterviewRole {
  /// A question of the builder.
  builder,

  /// An answer of the person.
  owner,
}

/// One turn of the interview, kept by the client (26 6.1 item 1).
final class InterviewTurn {
  /// Creates the turn.
  const InterviewTurn(this.role, this.text);

  /// Who said it.
  final InterviewRole role;

  /// What was said.
  final String text;
}

/// The outcome of one interview request.
enum InterviewOutcome {
  /// One or two questions.
  questions,

  /// A draft.
  draft,

  /// No builder answer; [InterviewReply.fallbackReason] says why.
  fallback,
}

/// The answer of `interviewAgent` (`AgentInterviewAnswer`).
final class InterviewReply {
  /// Creates the answer.
  const InterviewReply({
    required this.outcome,
    this.questions = const [],
    this.draft,
    this.errors = const [],
    this.fallbackReason,
    this.costMicros,
    this.currency,
  });

  /// The outcome.
  final InterviewOutcome outcome;

  /// The builder's questions.
  final List<String> questions;

  /// The draft as `AgentDraft` JSON.
  final Map<String, Object?>? draft;

  /// The draft's findings.
  final List<FieldIssue> errors;

  /// `builder_not_configured`, `builder_offline`, `builder_timeout` or `builder_invalid`.
  final String? fallbackReason;

  /// The builder's cost estimate of this turn, in micro units of [currency].
  final int? costMicros;

  /// The currency of [costMicros].
  final String? currency;
}

/// How a call failed.
enum NewAgentFailureKind {
  /// The server answered with a problem.
  problem,

  /// The person cancelled the step-up prompt.
  stepUpCancelled,

  /// No answer from the server.
  offline,
}

/// The failure of a port call.
final class NewAgentFailure implements Exception {
  /// A problem of the server with [code], optional [reason] and field [errors].
  const NewAgentFailure.problem(
    String this.code, {
    this.reason,
    this.errors = const [],
  }) : kind = NewAgentFailureKind.problem;

  /// The step-up prompt was cancelled.
  const NewAgentFailure.stepUpCancelled()
    : kind = NewAgentFailureKind.stepUpCancelled,
      code = null,
      reason = null,
      errors = const [];

  /// The server did not answer.
  const NewAgentFailure.offline()
    : kind = NewAgentFailureKind.offline,
      code = null,
      reason = null,
      errors = const [];

  /// The kind.
  final NewAgentFailureKind kind;

  /// The problem code, or null.
  final String? code;

  /// The problem's `reason`, or null.
  final String? reason;

  /// The field errors of a `400 validation_failed`.
  final List<FieldIssue> errors;

  @override
  String toString() => 'NewAgentFailure($kind, $code, $reason, $errors)';
}

/// The server side of "New agent"; every call throws [NewAgentFailure].
abstract interface class NewAgentPort {
  /// The offer, or null when the entry stays hidden: the server does not answer
  /// `listAgentTemplates` (an older server, or a caller who is no owner) or no active runner
  /// reports `hosts_agents` (26 8.1).
  Future<NewAgentOffer?> offer();

  /// The signed-in owner and the spaces (`getMe`, `listOwners`, `listSpaces`).
  Future<NewAgentContext> context();

  /// Sends the interview so far (`interviewAgent`); [wantDraft] asks for the draft now.
  Future<InterviewReply> interview(
    List<InterviewTurn> turns, {
    bool wantDraft = false,
  });

  /// Creates the built-in agent on [hostedBy] (`createAgent`, step-up).
  Future<NewAgentState> createAgent({
    required String handle,
    required String displayName,
    required String description,
    required String hostedBy,
    required String idempotencyKey,
  });

  /// The agent as the server has it (`getAgent`).
  Future<NewAgentState> agent(String agentId);

  /// Sets the tool class (`setAgentToolClass`, step-up); [name] is the agent's name for the
  /// prompt and the reason.
  Future<NewAgentState> setToolClass(
    NewAgentState agent,
    AgentToolClass toolClass, {
    required String name,
  });

  /// The current definition (`getAgentDefinition`), or null while there is none.
  Future<StoredDefinition?> definition(String agentId);

  /// Stores [definition] (`putAgentDefinition`, step-up); [ifMatch] is the current version,
  /// null for the first one.
  Future<StoredDefinition> putDefinition(
    String agentId,
    Map<String, Object?> definition, {
    int? ifMatch,
    required String name,
  });

  /// Sets the billing mode (`setAgentBillingMode`, step-up); [holderId] for `personal`.
  Future<NewAgentState> setBilling(
    NewAgentState agent,
    String mode, {
    String? holderId,
    required String name,
  });

  /// Creates the text channel [channelName] in [spaceId] (`createChannel`) and returns its ID.
  Future<String> createChannel(
    String spaceId,
    String channelName, {
    required String idempotencyKey,
    required String name,
  });

  /// Makes [agentId] the operator of [channelId] (`getChannel`, `setChannelOperator`).
  Future<void> setOperator(
    String channelId,
    String agentId, {
    required String name,
  });
}

/// [NewAgentPort] over [ApiClient].
///
/// Example:
/// ```dart
/// final port = ApiNewAgentPort(session.api,
///     purpose: l.stepUpPurposeNewAgent, reason: l.newAgentAuditReason);
/// final offer = await port.offer();
/// ```
final class ApiNewAgentPort implements NewAgentPort {
  /// Creates the port; [purpose] gives the step-up text for the agent's name and [reason] the
  /// audit reason of the class, billing and operator steps.
  ApiNewAgentPort(this._api, {required this._purpose, required this._reason});

  final ApiClient _api;
  final String Function(String name) _purpose;
  final String Function(String name) _reason;

  AgentsApi get _agents => _api.getAgentsApi();

  Map<String, Object?> _extra(String name) => {
    stepUpPurposeExtra: _purpose(name),
  };

  @override
  Future<NewAgentOffer?> offer() async {
    try {
      final templates = (await _agents.listAgentTemplates()).data!.items;
      final runners = <HostingRunner>[];
      String? cursor;
      do {
        final page = (await _agents.listAgents(cursor: cursor)).data!;
        for (final a in page.items) {
          final ceiling = a.presence.hostsAgents;
          if (a.status != AgentStatusEnum.active || ceiling == null) continue;
          runners.add(
            HostingRunner(
              id: a.id,
              name: a.displayName,
              ceiling: AgentToolClass.of(_toolClassWire(ceiling)),
              billingMode: a.billingMode == BillingMode.personal
                  ? 'personal'
                  : 'organization',
              holderId: a.credentialHolderId,
              modesSupported: [
                for (final m
                    in a.presence.billingModesSupported ??
                        BuiltList<BillingMode>())
                  m == BillingMode.personal ? 'personal' : 'organization',
              ],
            ),
          );
        }
        cursor = page.nextCursor;
      } while (cursor != null);
      if (runners.isEmpty) return null;
      return NewAgentOffer(
        templates: [for (final t in templates) _template(t)],
        runners: runners,
      );
    } on Object {
      // An older server (404), a caller who is no owner (403) or no answer: no entry.
      return null;
    }
  }

  @override
  Future<NewAgentContext> context() => _guard(() async {
    final me = (await _api.getMeApi().getMe()).data!;
    final owners = (await _api.getUsersApi().listOwners()).data!.items;
    final spaces = (await _api.getSpacesApi().listSpaces()).data!.items;
    final id = me.principal.id;
    return NewAgentContext(
      meId: id,
      onlyOwner: owners.length == 1 && owners.single.user.id == id,
      spaces: [for (final s in spaces) NewAgentSpace(s.space.id, s.space.name)],
    );
  });

  @override
  Future<InterviewReply> interview(
    List<InterviewTurn> turns, {
    bool wantDraft = false,
  }) => _guard(() async {
    final answer = (await _agents.interviewAgent(
      agentInterviewRequest: AgentInterviewRequest(
        (b) => b
          ..turns.addAll([
            for (final t in turns)
              AgentInterviewTurn(
                (x) => x
                  ..role = t.role == InterviewRole.builder
                      ? AgentInterviewTurnRoleEnum.builder
                      : AgentInterviewTurnRoleEnum.owner
                  ..text = t.text,
              ),
          ])
          ..wantDraft = wantDraft ? true : null,
      ),
    )).data!;
    final draft = answer.draft;
    return InterviewReply(
      outcome: switch (answer.outcome) {
        AgentInterviewAnswerOutcomeEnum.questions => InterviewOutcome.questions,
        AgentInterviewAnswerOutcomeEnum.draft => InterviewOutcome.draft,
        _ => InterviewOutcome.fallback,
      },
      questions: answer.questions?.toList() ?? const [],
      draft: draft == null
          ? null
          : _json(
              standardSerializers.serializeWith(AgentDraft.serializer, draft),
            ),
      errors: [
        for (final e in answer.errors ?? BuiltList<FieldError>())
          FieldIssue(e.pointer, e.code),
      ],
      fallbackReason: switch (answer.fallbackReason) {
        null => null,
        AgentInterviewAnswerFallbackReasonEnum.builderNotConfigured =>
          'builder_not_configured',
        AgentInterviewAnswerFallbackReasonEnum.builderOffline =>
          'builder_offline',
        AgentInterviewAnswerFallbackReasonEnum.builderTimeout =>
          'builder_timeout',
        _ => 'builder_invalid',
      },
      costMicros: answer.costEstimate?.amountMicros,
      currency: answer.costEstimate?.currency,
    );
  });

  @override
  Future<NewAgentState> createAgent({
    required String handle,
    required String displayName,
    required String description,
    required String hostedBy,
    required String idempotencyKey,
  }) => _guard(
    () async => _state(
      (await _agents.createAgent(
        idempotencyKey: idempotencyKey,
        agentCreate: AgentCreate(
          (b) => b
            ..handle = handle
            ..displayName = displayName
            ..description = description
            ..adapterKind = builtinAdapterKind
            ..hostedBy = hostedBy,
        ),
        extra: _extra(displayName),
      )).data!,
    ),
  );

  @override
  Future<NewAgentState> agent(String agentId) => _guard(
    () async => _state((await _agents.getAgent(agentId: agentId)).data!),
  );

  @override
  Future<NewAgentState> setToolClass(
    NewAgentState agent,
    AgentToolClass toolClass, {
    required String name,
  }) => _guard(
    () async => _state(
      (await _agents.setAgentToolClass(
        agentId: agent.id,
        ifMatch: '"${agent.version}"',
        toolClassSet: ToolClassSet(
          (b) => b
            ..toolClass = switch (toolClass) {
              AgentToolClass.chat => ToolClass.chat,
              AgentToolClass.scoped => ToolClass.scoped,
              AgentToolClass.host => ToolClass.host,
            }
            ..reason = _reason(name),
        ),
        extra: _extra(name),
      )).data!,
    ),
  );

  @override
  Future<StoredDefinition?> definition(String agentId) async {
    try {
      final v = (await _agents.getAgentDefinition(agentId: agentId)).data!;
      return _stored(v);
    } on DioException catch (e) {
      // A built-in agent without a definition answers 404 (26 4.1).
      if (e.response?.statusCode == 404) return null;
      throw newAgentFailureOf(e) ?? const NewAgentFailure.offline();
    } on Object catch (e) {
      throw newAgentFailureOf(e) ?? const NewAgentFailure.offline();
    }
  }

  @override
  Future<StoredDefinition> putDefinition(
    String agentId,
    Map<String, Object?> definition, {
    int? ifMatch,
    required String name,
  }) => _guard(
    () async => _stored(
      (await _agents.putAgentDefinition(
        agentId: agentId,
        ifMatch: ifMatch == null ? null : '"$ifMatch"',
        agentDefinition: standardSerializers.deserializeWith(
          AgentDefinition.serializer,
          definition,
        )!,
        extra: _extra(name),
      )).data!,
      fallback: definition,
    ),
  );

  @override
  Future<NewAgentState> setBilling(
    NewAgentState agent,
    String mode, {
    String? holderId,
    required String name,
  }) => _guard(
    () async => _state(
      (await _agents.setAgentBillingMode(
        agentId: agent.id,
        ifMatch: '"${agent.version}"',
        billingModeSet: BillingModeSet(
          (b) => b
            ..billingMode = mode == 'personal'
                ? BillingMode.personal
                : BillingMode.organization
            ..credentialHolderId = mode == 'personal' ? holderId : null
            ..reason = _reason(name),
        ),
        extra: _extra(name),
      )).data!,
    ),
  );

  @override
  Future<String> createChannel(
    String spaceId,
    String channelName, {
    required String idempotencyKey,
    required String name,
  }) => _guard(
    () async => (await _api.getChannelsApi().createChannel(
      spaceId: spaceId,
      idempotencyKey: idempotencyKey,
      channelCreate: ChannelCreate(
        (b) => b
          ..kind = ChannelCreateKindEnum.text
          ..name = channelName,
      ),
      extra: _extra(name),
    )).data!.id,
  );

  @override
  Future<void> setOperator(
    String channelId,
    String agentId, {
    required String name,
  }) => _guard(() async {
    final channels = _api.getChannelsApi();
    final current = (await channels.getChannel(channelId: channelId)).data!;
    await channels.setChannelOperator(
      channelId: channelId,
      ifMatch: '"${current.channel.version}"',
      operatorSet: OperatorSet(
        (b) => b
          ..mode = OperatorMode.agent
          ..agentId = agentId
          ..reason = _reason(name),
      ),
      extra: _extra(name),
    );
  });

  static NewAgentTemplate _template(AgentTemplate t) => NewAgentTemplate(
    slug: t.slug,
    name: t.name,
    description: t.description,
    version: t.version,
    privateSource: t.source_ == AgentTemplateSource_Enum.private,
    requiredSecrets: t.requiredSecrets.toList(),
    definition: _json(
      standardSerializers.serializeWith(
        AgentDefinition.serializer,
        t.definition,
      ),
    ),
  );

  static NewAgentState _state(Agent a) => NewAgentState(
    id: a.id,
    version: a.version,
    toolClass: AgentToolClass.of(_toolClassWire(a.toolClass)),
    billingMode: a.billingMode == BillingMode.personal
        ? 'personal'
        : 'organization',
    holderId: a.credentialHolderId,
  );

  static StoredDefinition _stored(
    AgentDefinitionVersion v, {
    Map<String, Object?>? fallback,
  }) {
    final d = v.definition;
    return StoredDefinition(
      v.version,
      d == null
          ? (fallback ?? const {})
          : _json(
              standardSerializers.serializeWith(AgentDefinition.serializer, d),
            ),
    );
  }

  static String _toolClassWire(ToolClass c) => switch (c) {
    ToolClass.host => 'host',
    ToolClass.scoped => 'scoped',
    _ => 'chat',
  };

  static Map<String, Object?> _json(Object? value) =>
      value is Map ? value.cast<String, Object?>() : const {};

  static Future<T> _guard<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on Object catch (e) {
      final failure = newAgentFailureOf(e);
      if (failure == null) rethrow;
      throw failure;
    }
  }
}

/// The [NewAgentFailure] of [error] of a port call, or null for an error that is no answer of
/// the server or the login layer: a session without any step-up method ([StepUpUnavailable])
/// is the server's `403 forbidden` with reason `step_up_not_possible`; another error of the
/// login layer (a cancelled step-up) is a cancel; a `DioException` without an answer is
/// offline; otherwise the server's problem with its code, reason and field errors.
NewAgentFailure? newAgentFailureOf(Object error) {
  if (error is NewAgentFailure) return error;
  final own = error is DioException ? error.error : error;
  if (own is StepUpUnavailable) {
    return const NewAgentFailure.problem(
      'forbidden',
      reason: 'step_up_not_possible',
    );
  }
  if (own is AuthException) return const NewAgentFailure.stepUpCancelled();
  if (error is! DioException) return null;
  final response = error.response;
  if (response == null) return const NewAgentFailure.offline();
  final problem = ApiProblem.of(error) ?? ApiProblem.fromResponse(response);
  return NewAgentFailure.problem(
    problem.code,
    reason: problem.reason,
    errors: [for (final e in problem.errors) FieldIssue(e.pointer, e.code)],
  );
}
