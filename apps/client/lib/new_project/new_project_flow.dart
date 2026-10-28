// SPDX-License-Identifier: Apache-2.0

/// The steps of a new project from the app (01 6.9; design note `docs/design/new-project.md`;
/// docs/decisions.d/M4-43.md).
///
/// The flow composes actions that exist (D-M4-43-1): create the forum (`channel.create`),
/// assign the runner when it is not the space's default operator (`channel.operator.set`),
/// bind the project with `provision` (`channel.project.bind`), read the readiness the runner
/// reports until it is ready or names a failure (`getChannelBehavior`, 09 18.12), and for an
/// idea post it as the first chat (`thread.create` through the outbox). A failed step stops
/// the flow; [NewProjectFlow.run] again resumes at the failed step with the IDs and the
/// idempotency key it already has, so nothing is created twice (D-M4-43-8).
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';

import 'project_name.dart';

/// The four ways to start a project (01 6.9 item 2).
enum NewProjectMode {
  /// A new empty folder with `git init` (`provision` `init`).
  empty,

  /// A clone of an `https` repository (`provision` `clone`).
  clone,

  /// A folder that exists under the runner's projects folder (`provision` `none`).
  existing,

  /// An empty project whose first chat is the idea (`provision` `init`).
  idea,
}

/// The steps the dialog lists (01 6.9 item 3).
enum NewProjectStep {
  /// `createChannel` with kind `forum`.
  createForum,

  /// `setChannelOperator`, only when the runner is not the space's default operator.
  assignRunner,

  /// `putProjectBinding` with `provision`.
  bindProject,

  /// The runner creates or checks the folder; the flow reads the readiness.
  prepareFolder,

  /// The idea as the first post, only for [NewProjectMode.idea].
  postIdea,
}

/// The state of one step.
enum NewProjectStepState {
  /// Not started.
  waiting,

  /// Running now.
  running,

  /// Done.
  done,

  /// Failed; [NewProjectFlow.failure] says why.
  failed,
}

/// The fields of the dialog.
enum NewProjectField {
  /// The mode.
  mode,

  /// The name of the forum and of a created folder.
  name,

  /// The repository URL of [NewProjectMode.clone].
  url,

  /// The folder of the binding: typed for [NewProjectMode.existing], otherwise the name until
  /// the runner refuses it.
  folder,

  /// The idea of [NewProjectMode.idea].
  idea,

  /// The runner agent.
  runner,
}

/// How a step failed.
enum NewProjectFailureKind {
  /// The server refused the request with a problem [NewProjectFailure.code].
  problem,

  /// The person cancelled the step-up prompt.
  stepUpCancelled,

  /// No answer from the server.
  offline,

  /// The runner reported the reason [NewProjectFailure.code] (09 18.12).
  notReady,
}

/// The failure of a step; the port throws it, the flow keeps it.
@immutable
final class NewProjectFailure implements Exception {
  /// A problem of the server with its [code] and optional [reason].
  const NewProjectFailure.problem(String this.code, {this.reason})
    : kind = NewProjectFailureKind.problem;

  /// The step-up prompt was cancelled.
  const NewProjectFailure.stepUpCancelled()
    : kind = NewProjectFailureKind.stepUpCancelled,
      code = null,
      reason = null;

  /// The server did not answer.
  const NewProjectFailure.offline()
    : kind = NewProjectFailureKind.offline,
      code = null,
      reason = null;

  /// The runner reported the readiness reason [code].
  const NewProjectFailure.notReady(String this.code)
    : kind = NewProjectFailureKind.notReady,
      reason = null;

  /// The kind.
  final NewProjectFailureKind kind;

  /// The problem code or the readiness reason, or null.
  final String? code;

  /// The problem's `reason`, or null.
  final String? reason;

  @override
  String toString() => 'NewProjectFailure($kind, $code)';
}

/// The readiness of a binding (`ProjectBinding.ready` and `ready_reason`).
@immutable
final class BindingReadiness {
  /// Creates the readiness.
  const BindingReadiness({required this.ready, this.reason});

  /// Whether the runner reported the project ready.
  final bool ready;

  /// The reason the runner reported with [ready] false, or null.
  final String? reason;
}

/// The server side of the flow; every method throws [NewProjectFailure].
abstract interface class NewProjectPort {
  /// Creates the forum [name] in [space] with [idempotencyKey] and returns its ID.
  Future<ChannelId> createForum(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  });

  /// Sets [agent] as the operator of [forum].
  Future<void> assignRunner(ChannelId forum, PrincipalId agent);

  /// Binds [forum] to [workspaceRef] with [provision] (`none`, `init`, `clone`) and, for a
  /// clone, [repositoryUrl].
  Future<void> bind(
    ChannelId forum, {
    required String workspaceRef,
    required String provision,
    String? repositoryUrl,
  });

  /// The readiness of the binding of [forum], or null while there is none.
  Future<BindingReadiness?> readiness(ChannelId forum);

  /// Stores [idea] as the person's draft of a new post in [forum] (01 6.8 item 8).
  Future<void> saveIdea(ChannelId forum, String idea);

  /// Sends [idea] as the first post of [forum] (empty title, `clear_draft`) and returns its
  /// outbox entry.
  Future<String> postIdea(ChannelId forum, String idea);
}

/// The state and the steps of one new project.
///
/// Example:
/// ```dart
/// final flow = NewProjectFlow(port: port, space: space.id,
///     spaceRunner: space.defaultOperatorAgentId, pollInterval: config.newProjectPollInterval,
///     wait: config.newProjectWait, nameMaxChars: config.newProjectNameMaxChars,
///     now: clock.now)
///   ..mode = NewProjectMode.clone
///   ..url = 'https://git.example.com/team/website.git';
/// await flow.run();
/// ```
final class NewProjectFlow extends ChangeNotifier {
  /// Creates the flow in [space]; [spaceRunner] is the space's default operator, the runner
  /// preselected and inherited by the forum (D-M4-43-11). [pollInterval] is
  /// `ui.new_project_poll_interval`, [wait] `ui.new_project_wait`, [nameMaxChars]
  /// `ui.new_project_name_max_chars`; [now] and [sleep] are the clock (tests pass fakes), and
  /// [newKey] makes the `Idempotency-Key` (a UUIDv7) of the forum: one per name, so a retry of
  /// the same request is answered from the server's record and a changed name is a new request.
  NewProjectFlow({
    required this._port,
    required this.space,
    required this.spaceRunner,
    required this._pollInterval,
    required this._wait,
    required this.nameMaxChars,
    required this._now,
    required this._newKey,
    Future<void> Function(Duration)? sleep,
  }) : _givenSleep = sleep,
       _runner = spaceRunner;

  final NewProjectPort _port;
  final Duration _pollInterval;
  final Duration _wait;
  final DateTime Function() _now;
  final Future<void> Function(Duration)? _givenSleep;
  Timer? _timer;
  Completer<void>? _wake;
  final String Function() _newKey;
  String? _key;
  String? _keyName;

  /// The space.
  final SpaceId space;

  /// The space's default operator, or null.
  final PrincipalId? spaceRunner;

  /// `ui.new_project_name_max_chars`.
  final int nameMaxChars;

  NewProjectMode _mode = NewProjectMode.empty;
  String _name = '';
  bool _nameTyped = false;
  String _url = '';
  String _folder = '';
  bool _folderTyped = false;
  String _idea = '';
  PrincipalId? _runner;

  final Map<NewProjectStep, NewProjectStepState> _states = {};
  NewProjectStep? _failedStep;
  NewProjectFailure? _failure;
  ChannelId? _forum;
  String? _sentEntry;
  bool _ideaSaved = false;
  bool _busy = false;
  bool _slow = false;
  bool _closed = false;

  /// The mode; locked once the forum exists.
  NewProjectMode get mode => _mode;
  set mode(NewProjectMode value) {
    if (!canEdit(NewProjectField.mode) || value == _mode) return;
    _mode = value;
    if (value != NewProjectMode.existing) {
      // A created folder takes the name; a folder typed for "existing" would be hidden here.
      _folderTyped = false;
      _folder = _name;
    }
    _suggest();
    notifyListeners();
  }

  /// The name of the forum and of a created folder.
  String get name => _name;
  set name(String value) {
    if (!canEdit(NewProjectField.name)) return;
    _name = value;
    _nameTyped = true;
    if (!_folderTyped) _folder = value;
    notifyListeners();
  }

  /// The repository URL of a clone; the name follows it until the person types one.
  String get url => _url;
  set url(String value) {
    if (!canEdit(NewProjectField.url)) return;
    _url = value.trim();
    _suggest();
    notifyListeners();
  }

  /// The folder of the binding, relative to the runner's projects folder: for an existing
  /// project the folder the person names, otherwise the name unless the person changed it
  /// after the runner refused the folder (a taken folder, 01 6.9 item 5).
  String get folder => _folder;
  set folder(String value) {
    if (!canEdit(NewProjectField.folder)) return;
    _folder = value;
    _folderTyped = true;
    notifyListeners();
  }

  /// The idea; the name follows its first words until the person types one.
  String get idea => _idea;
  set idea(String value) {
    if (!canEdit(NewProjectField.idea)) return;
    _idea = value;
    _suggest();
    notifyListeners();
  }

  /// The runner agent; the space's default operator unless the person chose another.
  PrincipalId? get runner => _runner;
  set runner(PrincipalId? value) {
    if (!canEdit(NewProjectField.runner)) return;
    _runner = value;
    notifyListeners();
  }

  void _suggest() {
    if (_nameTyped || _forum != null) return;
    final suggestion = switch (_mode) {
      NewProjectMode.clone => suggestProjectName(
        url: _url,
        maxChars: nameMaxChars,
      ),
      NewProjectMode.idea => suggestProjectName(
        idea: _idea,
        maxChars: nameMaxChars,
      ),
      _ => _name,
    };
    _name = suggestion;
    if (!_folderTyped) _folder = suggestion;
  }

  /// The steps of the current mode and runner.
  List<NewProjectStep> get steps => [
    NewProjectStep.createForum,
    if (_needsRunner) NewProjectStep.assignRunner,
    NewProjectStep.bindProject,
    NewProjectStep.prepareFolder,
    if (_mode == NewProjectMode.idea) NewProjectStep.postIdea,
  ];

  bool get _needsRunner => _runner != null && _runner != spaceRunner;

  /// The state of [step].
  NewProjectStepState stateOf(NewProjectStep step) =>
      _states[step] ?? NewProjectStepState.waiting;

  /// The failed step, or null.
  NewProjectStep? get failedStep => _failedStep;

  /// Why [failedStep] failed, or null.
  NewProjectFailure? get failure => _failure;

  /// The created forum, or null before.
  ChannelId? get forum => _forum;

  /// The outbox entry of the posted idea, or null.
  String? get sentEntry => _sentEntry;

  /// Whether a step runs now.
  bool get busy => _busy;

  /// Whether the runner has not reported the project ready within `ui.new_project_wait`.
  bool get slow => _slow;

  /// Whether every step is done.
  bool get finished =>
      steps.every((s) => stateOf(s) == NewProjectStepState.done);

  /// Whether the folder step clones (its label reads "Cloning").
  bool get folderCloning => _mode == NewProjectMode.clone;

  /// Whether the folder step creates a folder (its label reads "Creating folder").
  bool get folderCreating =>
      _mode == NewProjectMode.empty || _mode == NewProjectMode.idea;

  /// Whether [field] may still change: the fields of a done step are locked, except the
  /// binding's fields after the runner refused them (they are bound again on "Try again").
  bool canEdit(NewProjectField field) {
    if (_busy) return false;
    bool open(NewProjectStep s) => stateOf(s) != NewProjectStepState.done;
    final rebind = _failedStep == NewProjectStep.prepareFolder;
    return switch (field) {
      NewProjectField.mode || NewProjectField.name => _forum == null,
      NewProjectField.runner =>
        _forum == null || open(NewProjectStep.assignRunner),
      NewProjectField.url ||
      NewProjectField.folder => open(NewProjectStep.bindProject) || rebind,
      NewProjectField.idea => open(NewProjectStep.postIdea) && !_ideaSaved,
    };
  }

  /// The fields that block the start (01 6.9 item 2).
  Set<NewProjectField> get invalidFields => {
    if (!isProjectName(_name, maxChars: nameMaxChars)) NewProjectField.name,
    if (_mode == NewProjectMode.clone && !isCloneUrl(_url)) NewProjectField.url,
    if (_mode == NewProjectMode.existing
        ? !isFolderRef(_folder)
        // A created folder that is the name shares the name's error.
        : _folder != _name && !isProjectName(_folder, maxChars: nameMaxChars))
      NewProjectField.folder,
    if (_mode == NewProjectMode.idea && _idea.trim().isEmpty)
      NewProjectField.idea,
    if (_runner == null) NewProjectField.runner,
  };

  String get _workspaceRef => _folder;

  String get _provision => switch (_mode) {
    NewProjectMode.existing => 'none',
    NewProjectMode.clone => 'clone',
    NewProjectMode.empty || NewProjectMode.idea => 'init',
  };

  /// Runs the steps from the first one not done; returns when all are done, a step failed, or
  /// the flow was disposed. Does nothing with [invalidFields] or while it runs.
  Future<void> run() async {
    if (_busy || _closed || invalidFields.isNotEmpty) return;
    _busy = true;
    if (_failedStep == NewProjectStep.prepareFolder) {
      // The runner refused the binding's values; they may have changed, so bind again.
      _states.remove(NewProjectStep.bindProject);
    }
    _failedStep = null;
    _failure = null;
    _slow = false;
    notifyListeners();
    try {
      for (final step in steps) {
        if (stateOf(step) == NewProjectStepState.done) continue;
        _set(step, NewProjectStepState.running);
        try {
          await _do(step);
        } on NewProjectFailure catch (f) {
          if (_closed) return;
          _failedStep = step;
          _failure = f;
          _set(step, NewProjectStepState.failed);
          return;
        }
        if (_closed) return;
        _set(step, NewProjectStepState.done);
      }
    } finally {
      _busy = false;
      if (!_closed) notifyListeners();
    }
  }

  void _set(NewProjectStep step, NewProjectStepState state) {
    _states[step] = state;
    if (!_closed) notifyListeners();
  }

  Future<void> _do(NewProjectStep step) async {
    switch (step) {
      case NewProjectStep.createForum:
        if (_key == null || _keyName != _name) {
          _key = _newKey();
          _keyName = _name;
        }
        _forum = await _port.createForum(space, _name, idempotencyKey: _key!);
      case NewProjectStep.assignRunner:
        await _port.assignRunner(_forum!, _runner!);
      case NewProjectStep.bindProject:
        if (_mode == NewProjectMode.idea && !_ideaSaved) {
          await _port.saveIdea(_forum!, _idea);
          _ideaSaved = true;
        }
        await _port.bind(
          _forum!,
          workspaceRef: _workspaceRef,
          provision: _provision,
          repositoryUrl: _mode == NewProjectMode.clone ? _url : null,
        );
      case NewProjectStep.prepareFolder:
        await _waitReady();
      case NewProjectStep.postIdea:
        _sentEntry = await _port.postIdea(_forum!, _idea);
    }
  }

  /// Reads the readiness until ready, a failure reason or the end of the flow (09 18.12): no
  /// reason and `provisioning` mean the runner has not finished.
  Future<void> _waitReady() async {
    final start = _now();
    while (!_closed) {
      final r = await _port.readiness(_forum!);
      if (r != null && r.ready) return;
      final reason = r?.reason;
      if (reason != null && reason != 'provisioning') {
        throw NewProjectFailure.notReady(reason);
      }
      if (!_slow && _now().difference(start) >= _wait) {
        _slow = true;
        if (!_closed) notifyListeners();
      }
      await (_givenSleep ?? _sleep)(_pollInterval);
    }
  }

  /// Waits [d] on a timer that [dispose] cancels, so a closed dialog leaves no timer behind.
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
