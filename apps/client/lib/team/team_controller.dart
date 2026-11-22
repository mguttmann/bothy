// SPDX-License-Identifier: Apache-2.0

/// The state of the team dialog and the `/team` command over a [TeamPort] (09 3.9.1, 3.9.7):
/// the team as the server answered it last, whether an operation is on the way, and the last
/// refusal. One operation at a time; a refused one keeps the team as it was and names the
/// server's reason. Every change calls [TeamController.onChanged], which the app uses to read
/// the team of the header again.
library;

import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';

import 'team_port.dart';

/// The controller of the team of [channel].
final class TeamController extends ChangeNotifier {
  /// Creates the controller; [newKey] makes the `Idempotency-Key` of an add or a stop.
  TeamController({
    required this.channel,
    required this.port,
    required this.newKey,
    this.onChanged,
  });

  /// The team channel (or the channel that may become one).
  final ChannelId channel;

  /// The server side.
  final TeamPort port;

  /// Makes an idempotency key.
  final String Function() newKey;

  /// Called after every change the server accepted.
  final VoidCallback? onChanged;

  TeamView? _team;
  bool _loaded = false;
  bool _busy = false;
  TeamFailure? _failure;
  String? _addKey;
  String? _addKeyFor;

  /// The team, or null for a channel without team mode (or before [load]).
  TeamView? get team => _team;

  /// Whether the channel is a team channel.
  bool get enabled => _team != null;

  /// Whether [load] answered once.
  bool get loaded => _loaded;

  /// Whether an operation is on the way.
  bool get busy => _busy;

  /// The refusal of the last operation, or null.
  TeamFailure? get failure => _failure;

  List<TeamAgent>? _agents;

  /// The active agents of the server for the add form, or null before [load] or when their
  /// read failed (the dialog then offers the stored agents).
  List<TeamAgent>? get agents => _agents;

  /// Reads the team and, beside it, the agents of the add form. A failed read of the team keeps
  /// what was shown and sets [failure]; a failed read of the agents only leaves [agents] null.
  Future<void> load() => _run(() async {
    final agents = _readAgents();
    _team = await port.read(channel);
    _loaded = true;
    _agents = await agents;
  }, changes: false);

  Future<List<TeamAgent>?> _readAgents() async {
    try {
      return await port.agents();
    } on TeamFailure {
      return null;
    }
  }

  /// Switches team mode on ([on] true, the caller becomes the sponsor) or off.
  Future<bool> setEnabled({required bool on}) => _run(() async {
    if (on) {
      _team = await port.enable(channel);
    } else {
      await port.disable(channel);
      _team = null;
    }
  });

  /// Adds [agent] working in [home] as [handle]. A repeated press for the same three values
  /// after a lost answer sends the same idempotency key, so it adds nobody twice; after a
  /// refusal of the server the next press is a new request.
  Future<bool> add({
    required PrincipalId agent,
    required ChannelId home,
    required String handle,
  }) {
    final keyFor = '${agent.value}:${home.value}:$handle';
    if (_addKey == null || _addKeyFor != keyFor) {
      _addKey = newKey();
      _addKeyFor = keyFor;
    }
    final key = _addKey!;
    return _run(() async {
      final member = await port.add(
        channel,
        agent: agent,
        homeChannel: home,
        handle: handle,
        idempotencyKey: key,
      );
      _team = _team?.withMember(member);
      _addKey = null;
      _addKeyFor = null;
    }).then((ok) {
      // The server stores a refusal under its key (04 5.2): only a lost answer may repeat the
      // key, a refused add asks anew with a new one.
      if (!ok && !(_failure?.offline ?? true)) {
        _addKey = null;
        _addKeyFor = null;
      }
      return ok;
    });
  }

  /// Removes the member [memberId].
  Future<bool> remove(String memberId) => _run(() async {
    await port.remove(channel, memberId);
    _team = _team?.without(memberId);
  });

  /// Stops every member run and pauses member inputs.
  Future<bool> stopAll() => _run(() async {
    await port.stopAll(channel, idempotencyKey: newKey());
    _team = _team?.paused(TeamPause.stopped);
  });

  bool _disposed = false;

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }

  @override
  void notifyListeners() {
    // An answer may arrive after the dialog closed.
    if (!_disposed) super.notifyListeners();
  }

  Future<bool> _run(Future<void> Function() call, {bool changes = true}) async {
    if (_busy) return false;
    _busy = true;
    _failure = null;
    notifyListeners();
    var ok = false;
    try {
      await call();
      ok = true;
    } on TeamFailure catch (e) {
      _failure = e;
    } finally {
      _busy = false;
      notifyListeners();
    }
    if (ok && changes) onChanged?.call();
    return ok;
  }
}
