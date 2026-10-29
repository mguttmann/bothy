// SPDX-License-Identifier: Apache-2.0

/// The new project dialog of the desktop (01 6.9; design note `docs/design/new-project.md`;
/// docs/decisions.d/M4-43.md D-M4-43-7 to D-M4-43-11): the four modes, the name suggested from
/// the URL or the idea, the runner choice, the steps with their states, one sentence for a
/// failed step, "Try again" and the notice of a runner that has not finished.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import 'new_project_flow.dart';
import 'new_project_port.dart';

/// What a finished dialog created: the forum and, for an idea, the outbox entry of its post.
final class NewProjectResult {
  /// Creates the result.
  const NewProjectResult({required this.forum, this.sentEntry});

  /// The new forum.
  final ChannelId forum;

  /// The outbox entry of the posted idea, or null.
  final String? sentEntry;
}

/// Opens the dialog for [space] with the services of the signed-in session; returns the
/// result when every step is done, null when the person closed it. Does nothing without a
/// session.
Future<NewProjectResult?> showNewProjectDialog(
  BuildContext context,
  WidgetRef ref,
  Space space,
) async {
  final services = ref.read(signedInServicesProvider);
  final engine = ref.read(syncEngineProvider);
  final db = ref.read(localDatabaseProvider);
  if (services == null || engine == null || db == null) return null;
  final l = AppLocalizations.of(context);
  final config = ref.read(appConfigProvider);
  final port = ApiNewProjectPort(
    api: services.auth.api,
    outbox: engine.outbox,
    dao: db.outboxDao,
    purpose: l.stepUpPurposeNewProject,
    reason: l.newProjectAuditReason,
  );
  final dialog = NewProjectDialog(
    spaceId: space.id,
    spaceName: space.name,
    spaceRunner: space.defaultOperatorAgentId,
    port: port,
    runners: port.runners,
    pollInterval: config.newProjectPollInterval,
    wait: config.newProjectWait,
    nameMaxChars: config.newProjectNameMaxChars,
    now: ref.read(appPortsProvider).clock.now,
    newKey: services.ids.next,
  );
  return showDialog<NewProjectResult>(
    context: context,
    barrierDismissible: false,
    builder: (_) => dialog,
  );
}

/// The sentence for [failure] of [step] (01 6.9 item 5).
String newProjectFailureText(
  AppLocalizations l,
  NewProjectStep step,
  NewProjectFailure failure,
) {
  switch (failure.kind) {
    case NewProjectFailureKind.stepUpCancelled:
      return l.newProjectErrStepUp;
    case NewProjectFailureKind.offline:
      return l.newProjectErrOffline;
    case NewProjectFailureKind.problem:
      final code = failure.code ?? '';
      if (code == 'conflict' && step == NewProjectStep.createForum) {
        return l.newProjectErrNameTaken;
      }
      if (code == 'conflict' && step == NewProjectStep.bindProject) {
        return l.newProjectErrFolderBound;
      }
      return problemText(l, code, reason: failure.reason);
    case NewProjectFailureKind.notReady:
      return switch (failure.code) {
        'workspace_exists' => l.newProjectErrWorkspaceExists,
        'clone_failed' => l.newProjectErrCloneFailed,
        'init_failed' => l.newProjectErrInitFailed,
        'provision_timeout' => l.newProjectErrTimeout,
        'provision_refused' => l.newProjectErrRefused,
        'workspace_missing' => l.newProjectErrMissing,
        'not_a_git_worktree' => l.newProjectErrNotGit,
        'not_writable' => l.newProjectErrNotWritable,
        'outside_root' => l.newProjectErrOutsideRoot,
        final code => l.newProjectErrNotReady(code ?? ''),
      };
  }
}

/// The dialog over [port]; [runners] lists the agents to choose from.
class NewProjectDialog extends StatefulWidget {
  /// Creates the dialog of the space [spaceId] named [spaceName] with the default operator
  /// [spaceRunner]; the durations and the limit are `ui.new_project_poll_interval`,
  /// `ui.new_project_wait` and `ui.new_project_name_max_chars`.
  const NewProjectDialog({
    super.key,
    required this.spaceId,
    required this.spaceName,
    required this.spaceRunner,
    required this.port,
    required this.runners,
    required this.pollInterval,
    required this.wait,
    required this.nameMaxChars,
    required this.now,
    required this.newKey,
  });

  /// The space.
  final SpaceId spaceId;

  /// Its name.
  final String spaceName;

  /// Its default operator, or null.
  final PrincipalId? spaceRunner;

  /// The server side.
  final NewProjectPort port;

  /// The runners to choose from.
  final Future<List<ProjectRunner>> Function() runners;

  /// `ui.new_project_poll_interval`.
  final Duration pollInterval;

  /// `ui.new_project_wait`.
  final Duration wait;

  /// `ui.new_project_name_max_chars`.
  final int nameMaxChars;

  /// The clock.
  final DateTime Function() now;

  /// Makes an idempotency key.
  final String Function() newKey;

  @override
  State<NewProjectDialog> createState() => _NewProjectDialogState();
}

class _NewProjectDialogState extends State<NewProjectDialog> {
  late final NewProjectFlow _flow;
  final TextEditingController _name = TextEditingController();
  final TextEditingController _url = TextEditingController();
  final TextEditingController _folder = TextEditingController();
  final TextEditingController _idea = TextEditingController();
  List<ProjectRunner>? _runners;
  bool _tried = false;
  bool _popped = false;

  @override
  void initState() {
    super.initState();
    _flow = NewProjectFlow(
      port: widget.port,
      space: widget.spaceId,
      spaceRunner: widget.spaceRunner,
      pollInterval: widget.pollInterval,
      wait: widget.wait,
      nameMaxChars: widget.nameMaxChars,
      now: widget.now,
      newKey: widget.newKey,
    )..addListener(_changed);
    unawaited(_loadRunners());
  }

  Future<void> _loadRunners() async {
    List<ProjectRunner> list;
    try {
      list = await widget.runners();
    } on Object {
      list = const [];
    }
    if (mounted) setState(() => _runners = list);
  }

  void _changed() {
    if (!mounted) return;
    // The flow suggests a name and a folder; the fields follow while the person has not typed.
    if (_name.text != _flow.name) _name.text = _flow.name;
    if (_folder.text != _flow.folder) _folder.text = _flow.folder;
    setState(() {});
    // Once only: the flow notifies again after its last step while the route is still closing.
    if (_flow.finished && !_popped) {
      _popped = true;
      Navigator.of(
        context,
      ).pop(NewProjectResult(forum: _flow.forum!, sentEntry: _flow.sentEntry));
    }
  }

  Future<void> _start() async {
    setState(() => _tried = true);
    await _flow.run();
  }

  @override
  void dispose() {
    _flow
      ..removeListener(_changed)
      ..dispose();
    _name.dispose();
    _url.dispose();
    _folder.dispose();
    _idea.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final flow = _flow;
    final started = flow.forum != null || flow.busy || flow.failedStep != null;
    final invalid = flow.invalidFields;
    String? error(NewProjectField f, String text, {required bool filled}) =>
        invalid.contains(f) && (_tried || filled) ? text : null;
    final failure = flow.failure;
    final failedStep = flow.failedStep;
    return AlertDialog(
      key: const ValueKey('newProject.dialog'),
      title: Text(l.newProjectIn(dsIsolate(widget.spaceName))),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l.newProjectModeLabel,
                style: dsTextStyle(DsType.bodyStrong),
              ),
              const SizedBox(height: DsSpace.s8),
              Wrap(
                spacing: DsSpace.s8,
                runSpacing: DsSpace.s8,
                children: [
                  for (final m in NewProjectMode.values)
                    ChoiceChip(
                      key: ValueKey('newProject.mode.${m.name}'),
                      label: Text(_modeLabel(l, m)),
                      selected: flow.mode == m,
                      onSelected: flow.canEdit(NewProjectField.mode)
                          ? (_) => flow.mode = m
                          : null,
                    ),
                ],
              ),
              const SizedBox(height: DsSpace.s12),
              if (flow.mode == NewProjectMode.clone)
                _field(
                  key: 'newProject.url',
                  controller: _url,
                  label: l.newProjectUrl,
                  enabled: flow.canEdit(NewProjectField.url),
                  onChanged: (v) => flow.url = v,
                  error: error(
                    NewProjectField.url,
                    l.newProjectUrlInvalid,
                    filled: _url.text.isNotEmpty,
                  ),
                ),
              if (flow.mode == NewProjectMode.idea)
                _field(
                  key: 'newProject.idea',
                  controller: _idea,
                  label: l.newProjectIdea,
                  enabled: flow.canEdit(NewProjectField.idea),
                  onChanged: (v) => flow.idea = v,
                  lines: 4,
                  error: error(
                    NewProjectField.idea,
                    l.newProjectIdeaRequired,
                    filled: false,
                  ),
                ),
              _field(
                key: 'newProject.name',
                controller: _name,
                label: l.newProjectName,
                helper: l.newProjectNameHint,
                enabled: flow.canEdit(NewProjectField.name),
                onChanged: (v) => flow.name = v,
                error: error(
                  NewProjectField.name,
                  l.newProjectNameInvalid(widget.nameMaxChars),
                  filled: _name.text.isNotEmpty,
                ),
              ),
              // A created folder takes the name; once the forum exists the name is locked, so
              // the folder is shown to change it after a taken folder (01 6.9 item 5).
              if (flow.mode == NewProjectMode.existing || flow.forum != null)
                _field(
                  key: 'newProject.folder',
                  controller: _folder,
                  label: l.newProjectFolder,
                  enabled: flow.canEdit(NewProjectField.folder),
                  onChanged: (v) => flow.folder = v,
                  error: error(
                    NewProjectField.folder,
                    l.newProjectFolderInvalid,
                    filled: _folder.text.isNotEmpty,
                  ),
                ),
              _runnerField(l, flow),
              if (error(
                    NewProjectField.runner,
                    l.newProjectRunnerMissing,
                    filled: false,
                  )
                  case final e?)
                Text(
                  e,
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.dangerFg),
                ),
              if (started) ...[
                const SizedBox(height: DsSpace.s12),
                for (final step in flow.steps)
                  _StepLine(
                    label: _stepLabel(l, flow, step),
                    state: flow.stateOf(step),
                  ),
              ],
              if (failure != null && failedStep != null) ...[
                const SizedBox(height: DsSpace.s8),
                Text(
                  newProjectFailureText(l, failedStep, failure),
                  key: const ValueKey('newProject.failure'),
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.dangerFg),
                ),
              ],
              if (flow.slow && failure == null) ...[
                const SizedBox(height: DsSpace.s8),
                Text(
                  l.newProjectSlow,
                  key: const ValueKey('newProject.slow'),
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textSecondary),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        if (!started)
          TextButton(
            key: const ValueKey('newProject.cancel'),
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l.newProjectCancel),
          )
        else
          TextButton(
            key: const ValueKey('newProject.close'),
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l.newProjectClose),
          ),
        if (!started)
          FilledButton(
            key: const ValueKey('newProject.create'),
            onPressed: flow.busy ? null : () => unawaited(_start()),
            child: Text(l.newProjectCreate),
          )
        else if (failure != null)
          FilledButton(
            key: const ValueKey('newProject.retry'),
            onPressed: flow.busy ? null : () => unawaited(_start()),
            child: Text(l.newProjectRetry),
          ),
      ],
    );
  }

  Widget _field({
    required String key,
    required TextEditingController controller,
    required String label,
    required bool enabled,
    required ValueChanged<String> onChanged,
    String? helper,
    String? error,
    int lines = 1,
  }) => Padding(
    padding: const EdgeInsets.only(bottom: DsSpace.s12),
    child: TextField(
      key: ValueKey(key),
      controller: controller,
      enabled: enabled,
      minLines: lines,
      maxLines: lines == 1 ? 1 : null,
      onChanged: onChanged,
      decoration: InputDecoration(
        labelText: label,
        helperText: helper,
        errorText: error,
        errorMaxLines: 3,
      ),
    ),
  );

  Widget _runnerField(AppLocalizations l, NewProjectFlow flow) {
    final runners = _runners ?? const <ProjectRunner>[];
    final space = widget.spaceRunner;
    return DsDropdownField<PrincipalId>(
      // A new field once the runners arrive, so the preselected runner shows its label.
      key: ValueKey('newProject.runner.${runners.length}'),
      label: l.newProjectRunner,
      enabled: flow.canEdit(NewProjectField.runner) && runners.isNotEmpty,
      initialSelection: flow.runner,
      entries: [
        for (final r in runners)
          DropdownMenuEntry(
            value: r.id,
            label: r.id == space ? l.newProjectRunnerSpace(r.name) : r.name,
          ),
      ],
      onSelected: (id) => flow.runner = id,
    );
  }

  static String _modeLabel(AppLocalizations l, NewProjectMode m) => switch (m) {
    NewProjectMode.empty => l.newProjectModeEmpty,
    NewProjectMode.clone => l.newProjectModeClone,
    NewProjectMode.existing => l.newProjectModeExisting,
    NewProjectMode.idea => l.newProjectModeIdea,
  };

  static String _stepLabel(
    AppLocalizations l,
    NewProjectFlow flow,
    NewProjectStep s,
  ) => switch (s) {
    NewProjectStep.createForum => l.newProjectStepForum,
    NewProjectStep.assignRunner => l.newProjectStepRunner,
    NewProjectStep.bindProject => l.newProjectStepBind,
    NewProjectStep.prepareFolder =>
      flow.folderCloning
          ? l.newProjectStepCloning
          : flow.folderCreating
          ? l.newProjectStepCreating
          : l.newProjectStepFolder,
    NewProjectStep.postIdea => l.newProjectStepIdea,
  };
}

/// One step with its state: a mark and the label, read as one sentence.
class _StepLine extends StatelessWidget {
  const _StepLine({required this.label, required this.state});

  final String label;
  final NewProjectStepState state;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final stateName = switch (state) {
      NewProjectStepState.waiting => 'waiting',
      NewProjectStepState.running => 'running',
      NewProjectStepState.done => 'finished',
      NewProjectStepState.failed => 'failed',
    };
    final mark = switch (state) {
      NewProjectStepState.running => const SizedBox.square(
        dimension: DsSpace.s16,
        child: CircularProgressIndicator(strokeWidth: 2),
      ),
      NewProjectStepState.done => Icon(
        Icons.check,
        size: DsSpace.s16,
        color: colors.textPrimary,
      ),
      NewProjectStepState.failed => Icon(
        Icons.close,
        size: DsSpace.s16,
        color: colors.dangerFg,
      ),
      NewProjectStepState.waiting => Icon(
        Icons.circle_outlined,
        size: DsSpace.s16,
        color: colors.textSecondary,
      ),
    };
    return MergeSemantics(
      child: Semantics(
        label: l.newProjectStepState(label, stateName),
        excludeSemantics: true,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
          child: Row(
            children: [
              mark,
              const SizedBox(width: DsSpace.s8),
              Expanded(child: Text(label, style: dsTextStyle(DsType.body))),
            ],
          ),
        ),
      ),
    );
  }
}
