// SPDX-License-Identifier: Apache-2.0

/// The dialog "Add forums for existing folders" of the desktop (01 6.10; design note
/// `docs/design/new-project.md` section 6; docs/decisions.d/M4-44.md D-M4-44-5): the runner
/// choice, the folders of the runner without a forum as checked boxes, which are the preview of
/// what will be created, and after "Add forums" the state and the failure sentence of every
/// folder, "Try again" and the notice of a runner that has not finished.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import 'new_project_dialog.dart' show newProjectFailureText;
import 'new_project_flow.dart';
import 'new_project_port.dart';
import 'project_sync_flow.dart';

/// Opens the dialog for [space] with the services of the signed-in session; returns the forums
/// it created when every checked folder is ready, null when the person closed it. Does nothing
/// without a session.
Future<List<ChannelId>?> showProjectSyncDialog(
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
    purpose: (_) => l.stepUpPurposeProjectSync,
    reason: l.projectSyncAuditReason,
  );
  final dialog = ProjectSyncDialog(
    spaceId: space.id,
    spaceName: space.name,
    spaceRunner: space.defaultOperatorAgentId,
    port: port,
    source: port,
    runners: port.runners,
    pollInterval: config.newProjectPollInterval,
    wait: config.newProjectWait,
    now: ref.read(appPortsProvider).clock.now,
    newKey: services.ids.next,
  );
  return showDialog<List<ChannelId>>(
    context: context,
    barrierDismissible: false,
    builder: (_) => dialog,
  );
}

/// The sentence for a failed load of the folder list.
String serverFailureText(AppLocalizations l, NewProjectFailure failure) =>
    switch (failure.kind) {
      NewProjectFailureKind.offline => l.newProjectErrOffline,
      NewProjectFailureKind.stepUpCancelled => l.newProjectErrStepUp,
      _ => problemText(l, failure.code ?? '', reason: failure.reason),
    };

/// The dialog over [port] and [source]; [runners] lists the agents to choose from.
class ProjectSyncDialog extends StatefulWidget {
  /// Creates the dialog of the space [spaceId] named [spaceName] with the default operator
  /// [spaceRunner]; the durations are `ui.new_project_poll_interval` and `ui.new_project_wait`.
  const ProjectSyncDialog({
    super.key,
    required this.spaceId,
    required this.spaceName,
    required this.spaceRunner,
    required this.port,
    required this.source,
    required this.runners,
    required this.pollInterval,
    required this.wait,
    required this.now,
    required this.newKey,
  });

  /// The space.
  final SpaceId spaceId;

  /// Its name.
  final String spaceName;

  /// Its default operator, or null.
  final PrincipalId? spaceRunner;

  /// The server side of the steps.
  final NewProjectPort port;

  /// The folder lists.
  final ProjectFolderSource source;

  /// The runners to choose from.
  final Future<List<ProjectRunner>> Function() runners;

  /// `ui.new_project_poll_interval`.
  final Duration pollInterval;

  /// `ui.new_project_wait`.
  final Duration wait;

  /// The clock.
  final DateTime Function() now;

  /// Makes an idempotency key.
  final String Function() newKey;

  @override
  State<ProjectSyncDialog> createState() => _ProjectSyncDialogState();
}

class _ProjectSyncDialogState extends State<ProjectSyncDialog> {
  late final ProjectSyncFlow _flow;
  List<ProjectRunner>? _runners;
  bool _popped = false;

  @override
  void initState() {
    super.initState();
    _flow = ProjectSyncFlow(
      port: widget.port,
      source: widget.source,
      space: widget.spaceId,
      spaceRunner: widget.spaceRunner,
      pollInterval: widget.pollInterval,
      wait: widget.wait,
      now: widget.now,
      newKey: widget.newKey,
    )..addListener(_changed);
    unawaited(_loadRunners());
    unawaited(_flow.load());
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
    setState(() {});
    if (_flow.finished && !_popped) {
      _popped = true;
      Navigator.of(context).pop(_flow.created);
    }
  }

  void _chooseRunner(PrincipalId? id) {
    _flow.runner = id;
    unawaited(_flow.load());
  }

  @override
  void dispose() {
    _flow
      ..removeListener(_changed)
      ..dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final flow = _flow;
    final retry =
        flow.started && !flow.busy && (flow.hasFailures || flow.hasUnfinished);
    return AlertDialog(
      key: const ValueKey('projectSync.dialog'),
      title: Text(l.projectSyncTitle(dsIsolate(widget.spaceName))),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              _runnerField(l, flow),
              const SizedBox(height: DsSpace.s12),
              ..._list(l, colors, flow),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('projectSync.close'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(flow.started ? l.newProjectClose : l.newProjectCancel),
        ),
        if (!flow.started)
          FilledButton(
            key: const ValueKey('projectSync.add'),
            onPressed: flow.canStart ? () => unawaited(flow.run()) : null,
            child: Text(l.projectSyncAdd),
          )
        else if (retry)
          FilledButton(
            key: const ValueKey('projectSync.retry'),
            onPressed: () => unawaited(flow.run()),
            child: Text(l.newProjectRetry),
          ),
      ],
    );
  }

  List<Widget> _list(
    AppLocalizations l,
    DsColors colors,
    ProjectSyncFlow flow,
  ) {
    Widget note(String key, String text, {bool danger = false}) => Text(
      text,
      key: ValueKey(key),
      style: dsTextStyle(DsType.body)
          .copyWith(color: danger ? colors.dangerFg : colors.textSecondary),
    );
    if (flow.runner == null) {
      return [note('projectSync.chooseRunner', l.projectSyncChooseRunner)];
    }
    if (flow.loadFailure case final f?) {
      return [
        note('projectSync.loadFailed', serverFailureText(l, f), danger: true),
      ];
    }
    if (flow.loading || !flow.loaded) {
      return [note('projectSync.loading', l.projectSyncLoading)];
    }
    if (flow.rows.isEmpty) {
      return [
        note(
          flow.reported ? 'projectSync.empty' : 'projectSync.notReported',
          flow.reported ? l.projectSyncEmpty : l.projectSyncNotReported,
        ),
      ];
    }
    return [
      note('projectSync.intro', l.projectSyncIntro),
      const SizedBox(height: DsSpace.s8),
      for (final r in flow.rows) _FolderRow(row: r, flow: flow),
    ];
  }

  Widget _runnerField(AppLocalizations l, ProjectSyncFlow flow) {
    final runners = _runners ?? const <ProjectRunner>[];
    final space = widget.spaceRunner;
    return DsDropdownField<PrincipalId>(
      key: ValueKey('projectSync.runner.${runners.length}'),
      label: l.newProjectRunner,
      enabled: !flow.started && !flow.busy && runners.isNotEmpty,
      initialSelection: flow.runner,
      entries: [
        for (final r in runners)
          DropdownMenuEntry(
            value: r.id,
            label: r.id == space ? l.newProjectRunnerSpace(r.name) : r.name,
          ),
      ],
      onSelected: _chooseRunner,
    );
  }
}

/// One folder: its checkbox before the start, then its state and, on a failure, its sentence.
class _FolderRow extends StatelessWidget {
  const _FolderRow({required this.row, required this.flow});

  final ProjectSyncRow row;
  final ProjectSyncFlow flow;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final state = row.state;
    final stateName = switch (state) {
      ProjectSyncRowState.waiting => 'waiting',
      ProjectSyncRowState.skipped => 'skipped',
      ProjectSyncRowState.running => 'running',
      ProjectSyncRowState.waitingForRunner => 'runner',
      ProjectSyncRowState.ready => 'ready',
      ProjectSyncRowState.failed => 'failed',
      ProjectSyncRowState.notFinished => 'unfinished',
    };
    final failure = row.failure;
    final step = row.failedStep;
    final detail = switch (state) {
      ProjectSyncRowState.waitingForRunner => l.projectSyncWaitingRunner,
      ProjectSyncRowState.ready => l.projectSyncReady,
      ProjectSyncRowState.notFinished => l.projectSyncNotFinished,
      ProjectSyncRowState.failed when failure != null && step != null =>
        newProjectFailureText(l, step, failure),
      _ => null,
    };
    final mark = switch (state) {
      ProjectSyncRowState.running => const SizedBox.square(
        dimension: DsSpace.s16,
        child: CircularProgressIndicator(strokeWidth: 2),
      ),
      ProjectSyncRowState.ready => Icon(
        Icons.check,
        size: DsSpace.s16,
        color: colors.textPrimary,
      ),
      ProjectSyncRowState.failed => Icon(
        Icons.close,
        size: DsSpace.s16,
        color: colors.dangerFg,
      ),
      _ => Icon(
        Icons.circle_outlined,
        size: DsSpace.s16,
        color: colors.textSecondary,
      ),
    };
    final lead = flow.started
        ? mark
        : Checkbox(
            key: ValueKey('projectSync.check.${row.name}'),
            value: row.checked,
            onChanged: flow.busy
                ? null
                : (v) => flow.setChecked(row.name, checked: v ?? false),
          );
    return MergeSemantics(
      child: Semantics(
        label: l.projectSyncRowState(row.name, stateName),
        child: Padding(
          key: ValueKey('projectSync.row.${row.name}'),
          padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              lead,
              const SizedBox(width: DsSpace.s8),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(row.name, style: dsTextStyle(DsType.body)),
                    if (detail != null)
                      Text(
                        detail,
                        key: ValueKey('projectSync.detail.${row.name}'),
                        style: dsTextStyle(DsType.bodySm).copyWith(
                          color: state == ProjectSyncRowState.failed
                              ? colors.dangerFg
                              : colors.textSecondary,
                        ),
                      ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
