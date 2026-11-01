// SPDX-License-Identifier: Apache-2.0

/// The server side of a new project (01 6.9; docs/decisions.d/M4-43.md D-M4-43-1): the existing
/// operations `createChannel`, `getChannel` with `setChannelOperator`, `putProjectBinding` and
/// `getChannelBehavior` through the client of the signed-in session (DPoP, refresh and the
/// step-up interceptor apply, so the step-up prompt appears where the server demands it), and
/// the idea through the outbox as the forum's new post draft and its first post (01 6.8 item 8).
/// The folders of a runner without a binding come from `listAgentProjectFolders` (01 6.10), the
/// rename of a project forum goes through `updateChannel` (01 6.11, M4-44).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart'
    show AuthException, StepUpUnavailable, stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:domain/domain.dart' as d;
import 'package:local_store/local_store.dart' show OutboxDao;
import 'package:sync_engine/sync_engine.dart' show Outbox;

import '../composer/draft_sync.dart';
import '../sync/thread_ops.dart';
import 'new_project_flow.dart';
import 'project_sync_flow.dart';

/// A runner the dialog offers: an active agent.
final class ProjectRunner {
  /// Creates the entry.
  const ProjectRunner({required this.id, required this.name});

  /// The agent.
  final d.PrincipalId id;

  /// Its display name.
  final String name;
}

/// [NewProjectPort] over [ApiClient] and the outbox.
///
/// Example:
/// ```dart
/// final port = ApiNewProjectPort(api: services.auth.api, outbox: engine.outbox,
///     dao: db.outboxDao, purpose: l.stepUpPurposeNewProject, reason: l.newProjectAuditReason);
/// ```
final class ApiNewProjectPort implements NewProjectPort, ProjectFolderSource {
  /// Creates the port; [purpose] gives the text of the step-up prompt for the project's name
  /// and [reason] is the audit reason of the binding and the runner assignment (D-M4-43-7).
  ApiNewProjectPort({
    required this._api,
    required this._outbox,
    required this._dao,
    required this._purpose,
    required this._reason,
  });

  final ApiClient _api;
  final Outbox _outbox;
  final OutboxDao _dao;
  final String Function(String name) _purpose;
  final String _reason;

  /// The name of the forum this port created, for the step-up text of the later steps.
  String _name = '';

  Map<String, Object?> get _extra => {stepUpPurposeExtra: _purpose(_name)};

  /// The active agents in the order of the server (the runner choice, D-M4-43-11).
  Future<List<ProjectRunner>> runners() => _guard(() async {
    final out = <ProjectRunner>[];
    String? cursor;
    do {
      final page = (await _api.getAgentsApi().listAgents(cursor: cursor)).data!;
      for (final a in page.items) {
        if (a.status == AgentStatusEnum.active) {
          out.add(
            ProjectRunner(id: d.PrincipalId.parse(a.id), name: a.displayName),
          );
        }
      }
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<RunnerFolders> folders(d.PrincipalId runner) => _guard(() async {
    final list = (await _api.getAgentsApi().listAgentProjectFolders(
      agentId: runner.value,
    )).data!;
    return RunnerFolders(
      names: [for (final f in list.items) f.name],
      reported: list.reported,
    );
  });

  @override
  Future<d.ChannelId> createForum(
    d.SpaceId space,
    String name, {
    required String idempotencyKey,
  }) => _guard(() async {
    _name = name;
    final created = await _api.getChannelsApi().createChannel(
      spaceId: space.value,
      idempotencyKey: idempotencyKey,
      channelCreate: ChannelCreate(
        (b) => b
          ..kind = ChannelCreateKindEnum.forum
          ..name = name,
      ),
    );
    return d.ChannelId.parse(created.data!.id);
  });

  @override
  Future<void> assignRunner(d.ChannelId forum, d.PrincipalId agent) =>
      _guard(() async {
        final channels = _api.getChannelsApi();
        final current = (await channels.getChannel(channelId: forum.value))
            .data!;
        await channels.setChannelOperator(
          channelId: forum.value,
          ifMatch: '"${current.channel.version}"',
          operatorSet: OperatorSet(
            (b) => b
              ..mode = OperatorMode.agent
              ..agentId = agent.value
              ..reason = _reason,
          ),
          extra: _extra,
        );
      });

  @override
  Future<void> bind(
    d.ChannelId forum, {
    required String workspaceRef,
    required String provision,
    String? repositoryUrl,
  }) => _guard(() async {
    await _api.getChannelsApi().putProjectBinding(
      channelId: forum.value,
      projectBindingPut: ProjectBindingPut(
        (b) => b
          ..workspaceRef = workspaceRef
          ..provision = ProjectProvision.valueOf(provision)
          ..repositoryUrl = repositoryUrl
          ..reason = _reason,
      ),
      extra: _extra,
    );
  });

  @override
  Future<BindingReadiness?> readiness(d.ChannelId forum) => _guard(() async {
    final behavior = (await _api.getChannelsApi().getChannelBehavior(
      channelId: forum.value,
    )).data;
    final b = behavior?.projectBinding;
    if (b == null) return null;
    return BindingReadiness(ready: b.ready, reason: b.readyReason);
  });

  @override
  Future<void> saveIdea(d.ChannelId forum, String idea) =>
      DraftWriter.forum(outbox: _outbox, dao: _dao, forum: forum).write(idea);

  @override
  Future<String> postIdea(d.ChannelId forum, String idea) async {
    // An empty title: the runner names the chat after its first answer (09 12).
    final entry = await _outbox.enqueue(
      ThreadOps.postRequest(
        forum: forum,
        title: '',
        contentMd: idea,
        clearDraft: true,
      ),
    );
    return entry.id;
  }

  /// Runs [call] and turns its errors into [NewProjectFailure] ([newProjectFailureOf]).
  static Future<T> _guard<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on Object catch (e) {
      final failure = newProjectFailureOf(e);
      if (failure == null) rethrow;
      throw failure;
    }
  }
}

/// The [NewProjectFailure] of [error] of a port call, or null for an error that is no answer
/// of the server or the login layer: a session without any step-up method
/// ([StepUpUnavailable], for example a Windows device without a local account) is the server's
/// `403 forbidden` with reason `step_up_not_possible`; another error of the login layer (a
/// cancelled step-up, a lost session) is a cancel; a `DioException` without an answer is
/// offline; otherwise the server's problem with its code and reason.
NewProjectFailure? newProjectFailureOf(Object error) {
  final own = error is DioException ? error.error : error;
  if (own is StepUpUnavailable) {
    return const NewProjectFailure.problem(
      'forbidden',
      reason: 'step_up_not_possible',
    );
  }
  if (own is AuthException) return const NewProjectFailure.stepUpCancelled();
  if (error is! DioException) return null;
  final problem = error.response == null ? null : ApiProblem.of(error);
  if (problem == null) return const NewProjectFailure.offline();
  return NewProjectFailure.problem(problem.code, reason: problem.reason);
}

/// The server side of "Rename project" (01 6.11; D-M4-44-6); every method throws
/// [NewProjectFailure].
abstract interface class RenameProjectPort {
  /// The current version of [forum], the `If-Match` of the rename.
  Future<int> version(d.ChannelId forum);

  /// Renames [forum] to [name] at [version] with [idempotencyKey]; the folder and the binding
  /// stay.
  Future<void> rename(
    d.ChannelId forum,
    String name, {
    required int version,
    required String idempotencyKey,
  });
}

/// [RenameProjectPort] over `getChannel` and `updateChannel`.
final class ApiRenameProjectPort implements RenameProjectPort {
  /// Creates the port over the client of the signed-in session.
  ApiRenameProjectPort(this._api);

  final ApiClient _api;

  @override
  Future<int> version(d.ChannelId forum) => ApiNewProjectPort._guard(() async {
    final view = (await _api.getChannelsApi().getChannel(
      channelId: forum.value,
    )).data!;
    return view.channel.version;
  });

  @override
  Future<void> rename(
    d.ChannelId forum,
    String name, {
    required int version,
    required String idempotencyKey,
  }) => ApiNewProjectPort._guard(() async {
    await _api.getChannelsApi().updateChannel(
      channelId: forum.value,
      ifMatch: '"$version"',
      idempotencyKey: idempotencyKey,
      channelUpdate: ChannelUpdate((b) => b..name = name),
    );
  });
}
