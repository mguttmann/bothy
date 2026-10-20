// SPDX-License-Identifier: Apache-2.0

/// Typing and agent presence in memory (15 7.1 `EphemeralStore`, 04 12, brief M4-14 Design
/// note 6). Nothing here reaches the local database: ephemeral events are lost on reconnect by
/// design (04 2.1), and presence is seeded again from the next `ready`.
library;

import 'dart:async';

import 'package:auth/auth.dart';
import 'package:domain/domain.dart';

/// Who types where and how the visible agents are, in memory.
///
/// Example:
/// ```dart
/// store.watchTyping(threadId).listen((who) => showTyping(who));
/// store.typingStarted(threadId, alexId, expiresAt);
/// ```
final class EphemeralStore {
  /// Creates the store; [typingTtl] (`sync.typing_ttl`, D-W-115) caps how long an indicator
  /// lasts, whatever `expires_at` the server sends.
  EphemeralStore({required this._clock, required this._typingTtl});

  final Clock _clock;
  final Duration _typingTtl;
  final Map<ThreadId, Map<PrincipalId, DateTime>> _typing = {};
  final Map<PrincipalId, AgentPresence> _presence = {};
  final StreamController<ThreadId> _typingChanges =
      StreamController<ThreadId>.broadcast(sync: true);
  final StreamController<PrincipalId> _presenceChanges =
      StreamController<PrincipalId>.broadcast(sync: true);

  /// Counts typing entries, so the expiry of an older entry never removes a renewed one.
  int _typingGen = 0;
  final Map<(ThreadId, PrincipalId), int> _typingGens = {};

  /// The principals typing in [threadId] now.
  Set<PrincipalId> typingIn(ThreadId threadId) =>
      Set.unmodifiable(_typing[threadId]?.keys ?? const <PrincipalId>[]);

  /// The typing principals of [threadId] after every change, starting with the current set.
  Stream<Set<PrincipalId>> watchTyping(ThreadId threadId) async* {
    yield typingIn(threadId);
    yield* _typingChanges.stream
        .where((t) => t == threadId)
        .map((_) => typingIn(threadId));
  }

  /// `typing.started` (04 12 item 1): [principal] types in [threadId] until [expiresAt], at the
  /// latest `now + sync.typing_ttl`.
  void typingStarted(
    ThreadId threadId,
    PrincipalId principal,
    DateTime expiresAt,
  ) {
    final now = _clock.now();
    final cap = now.add(_typingTtl);
    final until = expiresAt.isAfter(cap) ? cap : expiresAt;
    if (!until.isAfter(now)) return;
    _typing.putIfAbsent(threadId, () => {})[principal] = until;
    final gen = ++_typingGen;
    _typingGens[(threadId, principal)] = gen;
    _typingChanges.add(threadId);
    unawaited(
      _clock.delay(until.difference(now)).then((_) {
        if (_typingGens[(threadId, principal)] == gen) {
          _removeTyping(threadId, principal);
        }
      }),
    );
  }

  /// A message of [principal] in [threadId] ends its indicator (04 12 item 1).
  void messageFrom(ThreadId threadId, PrincipalId principal) =>
      _removeTyping(threadId, principal);

  void _removeTyping(ThreadId threadId, PrincipalId principal) {
    final entries = _typing[threadId];
    if (entries == null || entries.remove(principal) == null) return;
    _typingGens.remove((threadId, principal));
    if (entries.isEmpty) _typing.remove(threadId);
    _typingChanges.add(threadId);
  }

  /// Drops every typing indicator; a lost connection loses them (04 2.1).
  void clearTyping() {
    final threads = _typing.keys.toList();
    _typing.clear();
    _typingGens.clear();
    threads.forEach(_typingChanges.add);
  }

  /// The presence of [agentId], or null when unknown.
  AgentPresence? presenceOf(PrincipalId agentId) => _presence[agentId];

  /// The presence of [agentId] after every change, starting with the current value.
  Stream<AgentPresence?> watchPresence(PrincipalId agentId) async* {
    yield presenceOf(agentId);
    yield* _presenceChanges.stream
        .where((a) => a == agentId)
        .map((_) => presenceOf(agentId));
  }

  /// Replaces every presence with [presence] from the snapshot of `ready` (04 12 item 2).
  void seedPresence(Map<PrincipalId, AgentPresence> presence) {
    final changed = {..._presence.keys, ...presence.keys};
    _presence
      ..clear()
      ..addAll(presence);
    changed.forEach(_presenceChanges.add);
  }

  /// `agent.presence_changed` (04 12 item 2).
  void presenceChanged(PrincipalId agentId, AgentPresence presence) {
    _presence[agentId] = presence;
    _presenceChanges.add(agentId);
  }

  /// Closes the streams.
  Future<void> dispose() async {
    // Not awaited: a paused listener would hold the done event (D-M4-19-14).
    unawaited(_typingChanges.close());
    unawaited(_presenceChanges.close());
  }
}
