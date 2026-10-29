// SPDX-License-Identifier: Apache-2.0

/// "Add forums for existing folders" (01 6.10; design note `docs/design/new-project.md`
/// section 6; docs/decisions.d/M4-44.md D-M4-44-4 and D-M4-44-5).
///
/// The flow lists the folders a runner reported under its projects folder that no forum is
/// bound to (`listAgentProjectFolders`), lets the person uncheck some, and for each checked
/// folder runs the steps of a new project in mode "Existing folder" with the folder name as
/// forum name: create the forum, assign the runner when it is not the space's default
/// operator, bind the folder with `provision` `none`. One folder after the other, so the first
/// step-up covers the rest; a failed folder does not stop the others. Then it reads the
/// readiness of every bound forum until it is ready, names a reason or `ui.new_project_wait`
/// passed. [ProjectSyncFlow.run] again resumes the failed and unfinished folders with the forum
/// and the idempotency key each already has, so no folder gets a second forum.
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';

import 'new_project_flow.dart';

/// The folders of a runner without a binding (schema `ProjectFolderList`).
@immutable
final class RunnerFolders {
  /// Creates the list.
  const RunnerFolders({required this.names, required this.reported});

  /// The folder names in name order.
  final List<String> names;

  /// Whether the runner reported its folders at least once.
  final bool reported;
}

/// The server side that lists a runner's folders; throws [NewProjectFailure].
abstract interface class ProjectFolderSource {
  /// The folders of [runner] that no forum is bound to (`listAgentProjectFolders`).
  Future<RunnerFolders> folders(PrincipalId runner);
}

/// What the person sees of one folder.
enum ProjectSyncRowState {
  /// Checked, not started.
  waiting,

  /// Unchecked; nothing is created for it.
  skipped,

  /// A step of the folder runs now.
  running,

  /// Bound; the runner has not reported it ready yet.
  waitingForRunner,

  /// Ready: the forum is a project.
  ready,

  /// A step failed; [ProjectSyncRow.failure] says why.
  failed,

  /// Bound, but not ready within `ui.new_project_wait`.
  notFinished,
}

/// One folder of the list with its progress.
final class ProjectSyncRow {
  ProjectSyncRow._(this.name);

  /// The folder name, also the forum's name and the binding's `workspace_ref`.
  final String name;

  /// Whether a forum is created for it.
  bool get checked => _checked;
  bool _checked = true;

  /// The state.
  ProjectSyncRowState get state => _state;
  ProjectSyncRowState _state = ProjectSyncRowState.waiting;

  /// The step that failed, or null.
  NewProjectStep? get failedStep => _failedStep;
  NewProjectStep? _failedStep;

  /// Why [failedStep] failed, or null.
  NewProjectFailure? get failure => _failure;
  NewProjectFailure? _failure;

  /// The created forum, or null before.
  ChannelId? get forum => _forum;
  ChannelId? _forum;

  String? _key;
  bool _assigned = false;
  bool _bound = false;

  bool get _open =>
      _checked &&
      _state != ProjectSyncRowState.ready &&
      _state != ProjectSyncRowState.skipped;
}

/// The state and the steps of "Add forums for existing folders".
///
/// Example:
/// ```dart
/// final flow = ProjectSyncFlow(port: port, source: port, space: space.id,
///     spaceRunner: space.defaultOperatorAgentId, pollInterval: config.newProjectPollInterval,
///     wait: config.newProjectWait, now: clock.now, newKey: ids.next);
/// await flow.load();
/// flow.setChecked('scratch', checked: false);
/// await flow.run();
/// ```
final class ProjectSyncFlow extends ChangeNotifier {
  /// Creates the flow in [space] with the default operator [spaceRunner], which is the
  /// preselected runner (D-M4-43-11). [pollInterval] is `ui.new_project_poll_interval`, [wait]
  /// `ui.new_project_wait`; [now] and [sleep] are the clock, and [newKey] makes the
  /// `Idempotency-Key` of a forum, one per folder.
  ProjectSyncFlow({
    required this._port,
    required this._source,
    required this.space,
    required this.spaceRunner,
    required this._pollInterval,
    required this._wait,
    required this._now,
    required this._newKey,
    Future<void> Function(Duration)? sleep,
  }) : _givenSleep = sleep,
       _runner = spaceRunner;

  final NewProjectPort _port;
  final ProjectFolderSource _source;
  final Duration _pollInterval;
  final Duration _wait;
  final DateTime Function() _now;
  final String Function() _newKey;
  final Future<void> Function(Duration)? _givenSleep;
  Timer? _timer;
  Completer<void>? _wake;

  /// The space.
  final SpaceId space;

  /// The space's default operator, or null.
  final PrincipalId? spaceRunner;

  PrincipalId? _runner;
  List<ProjectSyncRow> _rows = const [];
  bool _reported = true;
  bool _loading = false;
  bool _loaded = false;
  NewProjectFailure? _loadFailure;
  bool _busy = false;
  bool _started = false;
  bool _closed = false;
  int _loads = 0;

  /// The runner whose folders are listed; locked once a forum exists.
  PrincipalId? get runner => _runner;
  set runner(PrincipalId? value) {
    if (_started || _busy || value == _runner) return;
    _runner = value;
    _rows = const [];
    _loaded = false;
    notifyListeners();
  }

  /// The folders in name order.
  List<ProjectSyncRow> get rows => _rows;

  /// The row of the folder [name].
  ProjectSyncRow row(String name) => _rows.firstWhere((r) => r.name == name);

  /// Whether the runner reported its folders at least once.
  bool get reported => _reported;

  /// Whether the list is loading.
  bool get loading => _loading;

  /// Whether a list of the current runner arrived.
  bool get loaded => _loaded;

  /// Why the list could not be loaded, or null.
  NewProjectFailure? get loadFailure => _loadFailure;

  /// Whether a step runs now.
  bool get busy => _busy;

  /// Whether "Add forums" has been pressed.
  bool get started => _started;

  /// Whether "Add forums" may start: a list with a checked folder, not running.
  bool get canStart => !_started && !_busy && _rows.any((r) => r.checked);

  /// Whether every checked folder is ready.
  bool get finished =>
      _started &&
      _rows
          .where((r) => r.checked)
          .every((r) => r.state == ProjectSyncRowState.ready);

  /// Whether a folder failed.
  bool get hasFailures =>
      _rows.any((r) => r.state == ProjectSyncRowState.failed);

  /// Whether a folder is bound but not ready within `ui.new_project_wait`.
  bool get hasUnfinished =>
      _rows.any((r) => r.state == ProjectSyncRowState.notFinished);

  /// The forums created so far, in list order.
  List<ChannelId> get created => [for (final r in _rows) ?r.forum];

  /// Checks or unchecks the folder [name]; only before "Add forums".
  void setChecked(String name, {required bool checked}) {
    if (_started || _busy) return;
    row(name)
      .._checked = checked
      .._state = checked
          ? ProjectSyncRowState.waiting
          : ProjectSyncRowState.skipped;
    notifyListeners();
  }

  /// Loads the folders of [runner]; a failure is kept in [loadFailure].
  Future<void> load() async {
    final runner = _runner;
    if (runner == null || _started || _closed) return;
    final load = ++_loads;
    _loading = true;
    _loadFailure = null;
    notifyListeners();
    try {
      final list = await _source.folders(runner);
      if (_closed || load != _loads || runner != _runner) return;
      _rows = [for (final n in list.names) ProjectSyncRow._(n)];
      _reported = list.reported;
      _loaded = true;
    } on NewProjectFailure catch (e) {
      if (_closed || load != _loads) return;
      _rows = const [];
      _loadFailure = e;
    } finally {
      if (!_closed && load == _loads) {
        _loading = false;
        notifyListeners();
      }
    }
  }

  /// Runs the steps of every checked folder that is not ready, one folder after the other,
  /// then waits for the readiness of the bound ones (D-M4-44-5). Returns when every folder is
  /// ready, failed or not finished within `ui.new_project_wait`, or the flow was disposed.
  Future<void> run() async {
    if (_busy || _closed || !_rows.any((r) => r._open)) return;
    _busy = true;
    _started = true;
    notifyListeners();
    try {
      for (final r in _rows.where((r) => r._open)) {
        if (_closed) return;
        r
          .._failedStep = null
          .._failure = null;
        await _steps(r);
      }
      await _waitReady();
    } finally {
      _busy = false;
      if (!_closed) notifyListeners();
    }
  }

  bool get _needsRunner => _runner != null && _runner != spaceRunner;

  /// Creates, assigns and binds one folder; a failure marks only this row.
  Future<void> _steps(ProjectSyncRow r) async {
    var step = NewProjectStep.createForum;
    _set(r, ProjectSyncRowState.running);
    try {
      if (r._forum == null) {
        r._key ??= _newKey();
        r._forum = await _port.createForum(
          space,
          r.name,
          idempotencyKey: r._key!,
        );
      }
      if (_needsRunner && !r._assigned) {
        step = NewProjectStep.assignRunner;
        await _port.assignRunner(r._forum!, _runner!);
        r._assigned = true;
      }
      if (!r._bound) {
        step = NewProjectStep.bindProject;
        await _port.bind(r._forum!, workspaceRef: r.name, provision: 'none');
        r._bound = true;
      }
      _set(r, ProjectSyncRowState.waitingForRunner);
    } on Object catch (e) {
      if (_closed) return;
      _fail(r, step, e);
    }
  }

  /// Reads the readiness of the bound rows until each is ready or failed, or [_wait] passed;
  /// the rows still waiting then show that the runner has not finished.
  Future<void> _waitReady() async {
    final start = _now();
    while (!_closed) {
      final pending = _rows
          .where((r) => r.state == ProjectSyncRowState.waitingForRunner)
          .toList();
      if (pending.isEmpty) return;
      for (final r in pending) {
        try {
          final b = await _port.readiness(r._forum!);
          if (_closed) return;
          final reason = b?.reason;
          if (b != null && b.ready) {
            _set(r, ProjectSyncRowState.ready);
          } else if (reason != null && reason != 'provisioning') {
            _fail(
              r,
              NewProjectStep.prepareFolder,
              NewProjectFailure.notReady(reason),
            );
          }
        } on Object catch (e) {
          if (_closed) return;
          _fail(r, NewProjectStep.prepareFolder, e);
        }
      }
      if (!_rows.any((r) => r.state == ProjectSyncRowState.waitingForRunner)) {
        return;
      }
      if (_now().difference(start) >= _wait) {
        for (final r in _rows) {
          if (r.state == ProjectSyncRowState.waitingForRunner) {
            _set(r, ProjectSyncRowState.notFinished);
          }
        }
        return;
      }
      await (_givenSleep ?? _sleep)(_pollInterval);
    }
  }

  void _fail(ProjectSyncRow r, NewProjectStep step, Object e) {
    r
      .._failedStep = step
      // Any other error (a malformed answer) fails the folder too, so "Try again" stays.
      .._failure = e is NewProjectFailure
          ? e
          : const NewProjectFailure.problem('internal_error');
    _set(r, ProjectSyncRowState.failed);
  }

  void _set(ProjectSyncRow r, ProjectSyncRowState s) {
    r._state = s;
    if (!_closed) notifyListeners();
  }

  /// Waits [d] on a timer that [dispose] cancels.
  Future<void> _sleep(Duration d) {
    final wake = _wake = Completer<void>();
    _timer = Timer(d, () {
      if (!wake.isCompleted) wake.complete();
    });
    return wake.future;
  }

  @override
  void dispose() {
    _closed = true;
    _timer?.cancel();
    final wake = _wake;
    if (wake != null && !wake.isCompleted) wake.complete();
    super.dispose();
  }
}
