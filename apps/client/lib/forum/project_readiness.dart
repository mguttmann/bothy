// SPDX-License-Identifier: Apache-2.0

/// The banner of a project forum that is not ready (01 6.9 item 8; 09 18.12 and 18.16;
/// docs/decisions.d/F-PROJEKT-BEREIT.md): the cause from `ready_reason`, that the runner is
/// offline, and the action that fixes the cause. Desktop and phone show the same banner over
/// the post list and over a chat of the forum.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as wire;
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../new_project/new_project_dialog.dart' show newProjectFailureText;
import '../new_project/new_project_flow.dart';
import '../new_project/new_project_port.dart'
    show codeRunnerAdapterKind, newProjectFailureOf;
import '../notifications_settings/providers.dart' show isOwnerProvider;
import 'providers.dart';

/// The ready reason the server derives for an operator that cannot serve a project (09 18.16).
const String reasonOperatorCannotServe = 'operator_cannot_serve_project';

/// The ready reason of a folder the runner did not create (09 18.12).
const String reasonWorkspaceExists = 'workspace_exists';

/// What the banner shows of a project forum.
@immutable
final class ProjectReadinessView {
  /// Creates the view.
  const ProjectReadinessView({
    required this.ready,
    this.reason,
    this.runnerName,
    this.runnerOffline = false,
    this.fixRunner,
  });

  /// Whether the runner reported the project ready.
  final bool ready;

  /// `ready_reason`, or null.
  final String? reason;

  /// The display name of the forum's effective operator, or null without one.
  final String? runnerName;

  /// Whether that agent is offline.
  final bool runnerOffline;

  /// The only active agent that can serve a project, the target of "Make NAME the runner";
  /// null when there is none or more than one.
  final ProjectRunner? fixRunner;
}

/// The server side of the banner; the actions throw [NewProjectFailure].
abstract interface class ProjectReadinessPort {
  /// The readiness of the binding of [forum], or null when the forum has none or the reader
  /// may not read it (`getChannelBehavior` needs `manage`).
  Future<ProjectReadinessView?> read(ChannelId forum);

  /// Makes [agent] the operator of [forum] (`setChannelOperator`) with the audit [reason]; the
  /// step-up prompt names [purpose].
  Future<void> setRunner(
    ChannelId forum,
    PrincipalId agent, {
    required String reason,
    required String purpose,
  });

  /// Binds [forum] to the folder [workspaceRef] and keeps the other values of its binding
  /// (`putProjectBinding`).
  Future<void> renameFolder(
    ChannelId forum,
    String workspaceRef, {
    required String reason,
    required String purpose,
  });
}

/// [ProjectReadinessPort] over the client of the signed-in session (DPoP, refresh and the
/// step-up interceptor apply).
final class ApiProjectReadinessPort implements ProjectReadinessPort {
  /// Creates the port.
  ApiProjectReadinessPort(this._api);

  final wire.ApiClient _api;

  @override
  Future<ProjectReadinessView?> read(ChannelId forum) async {
    final wire.ProjectBinding? binding;
    try {
      binding = (await _api.getChannelsApi().getChannelBehavior(
        channelId: forum.value,
      )).data?.projectBinding;
    } on Object {
      return null;
    }
    if (binding == null) return null;
    if (binding.ready) return const ProjectReadinessView(ready: true);
    String? name;
    var offline = false;
    try {
      final channel = (await _api.getChannelsApi().getChannel(
        channelId: forum.value,
      )).data!.channel;
      final operator = channel.operator_.effectiveAgentId;
      if (operator != null) {
        final agent = (await _api.getAgentsApi().getAgent(agentId: operator))
            .data!;
        name = agent.displayName;
        offline = agent.presence.state == wire.AgentRuntimeState.offline;
      }
    } on Object {
      // Without the operator the banner still names the cause.
    }
    ProjectRunner? fix;
    if (binding.readyReason == reasonOperatorCannotServe) {
      fix = await _onlyRunner();
    }
    return ProjectReadinessView(
      ready: false,
      reason: binding.readyReason,
      runnerName: name,
      runnerOffline: offline,
      fixRunner: fix,
    );
  }

  /// The only active agent that can serve a project, or null.
  Future<ProjectRunner?> _onlyRunner() async {
    final found = <ProjectRunner>[];
    try {
      String? cursor;
      do {
        final page = (await _api.getAgentsApi().listAgents(cursor: cursor))
            .data!;
        for (final a in page.items) {
          if (a.status == wire.AgentStatusEnum.active &&
              a.adapterKind == codeRunnerAdapterKind) {
            found.add(
              ProjectRunner(id: PrincipalId.parse(a.id), name: a.displayName),
            );
          }
        }
        cursor = page.nextCursor;
      } while (cursor != null);
    } on Object {
      return null;
    }
    return found.length == 1 ? found.single : null;
  }

  @override
  Future<void> setRunner(
    ChannelId forum,
    PrincipalId agent, {
    required String reason,
    required String purpose,
  }) => _guard(() async {
    final channels = _api.getChannelsApi();
    final current = (await channels.getChannel(channelId: forum.value)).data!;
    await channels.setChannelOperator(
      channelId: forum.value,
      ifMatch: '"${current.channel.version}"',
      operatorSet: wire.OperatorSet(
        (b) => b
          ..mode = wire.OperatorMode.agent
          ..agentId = agent.value
          ..reason = reason,
      ),
      extra: {stepUpPurposeExtra: purpose},
    );
  });

  @override
  Future<void> renameFolder(
    ChannelId forum,
    String workspaceRef, {
    required String reason,
    required String purpose,
  }) => _guard(() async {
    final channels = _api.getChannelsApi();
    final current = (await channels.getChannelBehavior(channelId: forum.value))
        .data
        ?.projectBinding;
    if (current == null) {
      throw const NewProjectFailure.problem('not_found');
    }
    await channels.putProjectBinding(
      channelId: forum.value,
      ifMatch: '"${current.version}"',
      projectBindingPut: wire.ProjectBindingPut(
        (b) => b
          ..workspaceRef = workspaceRef
          ..provision = current.provision
          ..repositoryUrl = current.repositoryUrl
          ..defaultBranch = current.defaultBranch
          ..runnerConfig.replace(current.runnerConfig)
          ..reason = reason,
      ),
      extra: {stepUpPurposeExtra: purpose},
    );
  });

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

/// The sentence of the cause [reason] in the banner (09 18.12, 18.16).
String projectReasonText(AppLocalizations l, String? reason) =>
    switch (reason) {
      null || '' => l.projectReasonWaiting,
      reasonOperatorCannotServe => l.projectReasonOperatorCannotServe,
      'provisioning' => l.projectReasonProvisioning,
      final code => newProjectFailureText(
        l,
        NewProjectStep.prepareFolder,
        NewProjectFailure.notReady(code),
      ),
    };

/// The banner of the project forum [forum]: nothing while the project is ready or the reader
/// may not read the binding; otherwise the cause, an offline runner and, for an owner
/// (`isOwnerProvider`), the action that fixes the cause, and "Check again" for everyone.
class ProjectReadinessBanner extends ConsumerStatefulWidget {
  /// Creates the banner.
  const ProjectReadinessBanner({
    super.key,
    required this.forum,
    required this.forumName,
  });

  /// The forum.
  final ChannelId forum;

  /// Its name, for the step-up prompt.
  final String forumName;

  @override
  ConsumerState<ProjectReadinessBanner> createState() =>
      _ProjectReadinessBannerState();
}

class _ProjectReadinessBannerState
    extends ConsumerState<ProjectReadinessBanner> {
  ProjectReadinessView? _view;
  bool _busy = false;
  NewProjectFailure? _failure;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  @override
  void didUpdateWidget(ProjectReadinessBanner old) {
    super.didUpdateWidget(old);
    if (old.forum != widget.forum) {
      _view = null;
      unawaited(_load());
    }
  }

  Future<void> _load() async {
    final port = ref.read(projectReadinessPortProvider);
    if (port == null) return;
    final view = await port.read(widget.forum);
    if (mounted) setState(() => _view = view);
  }

  Future<void> _act(Future<void> Function(ProjectReadinessPort port) fn) async {
    final port = ref.read(projectReadinessPortProvider);
    if (port == null || _busy) return;
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      await fn(port);
    } on NewProjectFailure catch (f) {
      if (mounted) setState(() => _failure = f);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    await _load();
  }

  Future<void> _rename(AppLocalizations l) async {
    final name = await showDialog<String>(
      context: context,
      builder: (_) => const _FolderNameDialog(),
    );
    if (name == null || name.trim().isEmpty) return;
    await _act(
      (port) => port.renameFolder(
        widget.forum,
        name.trim(),
        reason: l.newProjectAuditReason,
        purpose: l.stepUpPurposeNewProject(widget.forumName),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final view = _view;
    if (view == null || view.ready) return const SizedBox.shrink();
    final l = AppLocalizations.of(context);
    // The actions are owner actions with step-up (05 channel.operator.set, channel.project.bind).
    final owner = ref.watch(isOwnerProvider).value ?? false;
    final tone = dsToneColors(context, DsTone.warning);
    final colors = context.dsColors;
    final fix = view.fixRunner;
    final failure = _failure;
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s8,
        vertical: DsSpace.s4,
      ),
      child: Semantics(
        container: true,
        liveRegion: true,
        child: Container(
          key: const ValueKey('project.banner'),
          padding: const EdgeInsets.all(DsSpace.s8),
          decoration: BoxDecoration(
            color: tone.subtle,
            border: Border.all(color: tone.border),
            borderRadius: BorderRadius.circular(DsRadius.sm),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l.projectNotReady(projectReasonText(l, view.reason)),
                key: const ValueKey('project.banner.reason'),
                style: dsTextStyle(DsType.bodyStrong).copyWith(color: tone.fg),
              ),
              if (view.runnerOffline && view.runnerName != null)
                Text(
                  l.projectRunnerOffline(view.runnerName!),
                  key: const ValueKey('project.banner.offline'),
                  style: dsTextStyle(DsType.bodySm),
                ),
              if (failure != null)
                Text(
                  l.projectActionFailed(
                    newProjectFailureText(
                      l,
                      NewProjectStep.bindProject,
                      failure,
                    ),
                  ),
                  key: const ValueKey('project.banner.failure'),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.dangerFg),
                ),
              const SizedBox(height: DsSpace.s4),
              Wrap(
                spacing: DsSpace.s8,
                runSpacing: DsSpace.s4,
                children: [
                  if (owner &&
                      view.reason == reasonOperatorCannotServe &&
                      fix != null)
                    DsButton(
                      key: const ValueKey('project.banner.setRunner'),
                      label: l.projectActionSetRunner(fix.name),
                      loading: _busy,
                      onPressed: _busy
                          ? null
                          : () => unawaited(
                              _act(
                                (port) => port.setRunner(
                                  widget.forum,
                                  fix.id,
                                  reason: l.newProjectAuditReason,
                                  purpose: l.stepUpPurposeNewProject(
                                    widget.forumName,
                                  ),
                                ),
                              ),
                            ),
                    ),
                  if (owner && view.reason == reasonWorkspaceExists)
                    DsButton(
                      key: const ValueKey('project.banner.rename'),
                      label: l.projectActionRenameFolder,
                      loading: _busy,
                      onPressed: _busy ? null : () => unawaited(_rename(l)),
                    ),
                  TextButton(
                    key: const ValueKey('project.banner.check'),
                    onPressed: _busy ? null : () => unawaited(_load()),
                    child: Text(l.projectActionCheckAgain),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Asks for a new folder name.
class _FolderNameDialog extends StatefulWidget {
  const _FolderNameDialog();

  @override
  State<_FolderNameDialog> createState() => _FolderNameDialogState();
}

class _FolderNameDialogState extends State<_FolderNameDialog> {
  final TextEditingController _name = TextEditingController();

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return AlertDialog(
      title: Text(l.projectRenameFolderTitle),
      content: TextField(
        key: const ValueKey('project.rename.field'),
        controller: _name,
        autofocus: true,
        decoration: InputDecoration(labelText: l.projectRenameFolderTitle),
        onSubmitted: (v) => Navigator.of(context).pop(v),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.newProjectCancel),
        ),
        FilledButton(
          key: const ValueKey('project.rename.save'),
          onPressed: () => Navigator.of(context).pop(_name.text),
          child: Text(l.projectRenameFolderSave),
        ),
      ],
    );
  }
}
