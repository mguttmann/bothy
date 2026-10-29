// SPDX-License-Identifier: Apache-2.0

/// The server side of a new project (01 6.9; docs/decisions.d/M4-43.md D-M4-43-1): the existing
/// operations `createChannel`, `getChannel` with `setChannelOperator`, `putProjectBinding` and
/// `getChannelBehavior` through the client of the signed-in session (DPoP, refresh and the
/// step-up interceptor apply, so the step-up prompt appears where the server demands it), and
/// the idea through the outbox as the forum's new post draft and its first post (01 6.8 item 8).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show AuthException, stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:domain/domain.dart' as d;
import 'package:local_store/local_store.dart' show OutboxDao;
import 'package:sync_engine/sync_engine.dart' show Outbox;

import '../composer/draft_sync.dart';
import '../sync/thread_ops.dart';
import 'new_project_flow.dart';

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
final class ApiNewProjectPort implements NewProjectPort {
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

  /// Runs [call] and turns its errors into [NewProjectFailure]: a cancelled step-up or a lost
  /// session, no answer, or the server's problem.
  static Future<T> _guard<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      if (e.error is AuthException) {
        throw const NewProjectFailure.stepUpCancelled();
      }
      final problem = e.response == null ? null : ApiProblem.of(e);
      if (problem == null) throw const NewProjectFailure.offline();
      throw NewProjectFailure.problem(problem.code, reason: problem.reason);
    } on AuthException {
      throw const NewProjectFailure.stepUpCancelled();
    }
  }
}
