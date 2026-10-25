// SPDX-License-Identifier: Apache-2.0

/// The polite announcements of the run graph: the end of a node and state changes, at most
/// once per `a11y.live_announce_min_interval`, never every patch (docs/spec/01-product.md 8,
/// rows "Live content" and "Run graph and control room"; design note D-RV-12).
library;

import 'run_graph_model.dart';

/// Builds the text of one change: the node and its new state.
typedef RunGraphAnnouncement = String Function(RunGraphNode node);

/// Compares each new graph with the one before and announces what changed state.
///
/// The first graph is only remembered. A node that is new (it arrived with a patch) is not
/// announced, nor is a patch that changes durations, tokens or titles; only a node whose state
/// changed (including the end of a node) is. Changes within [minInterval] of the last
/// announcement wait and go out together, newest state per node, when [flush] runs after
/// [pendingDue]. Time comes from [clock], so tests run on a fake clock.
///
/// ```dart
/// final a = RunGraphAnnouncer(minInterval: config.liveAnnounceMinInterval,
///     clock: clock.now, sink: announce, text: (n) => '${n.name}: ${n.state}');
/// a.observe(graph); // on every derived graph
/// ```
final class RunGraphAnnouncer {
  /// Creates the announcer.
  RunGraphAnnouncer({
    required this.minInterval,
    required this.clock,
    required this.sink,
    required this.text,
  });

  /// `a11y.live_announce_min_interval`.
  final Duration minInterval;

  /// The current time.
  final DateTime Function() clock;

  /// Receives each announcement.
  final void Function(String message) sink;

  /// The text of one changed node.
  final RunGraphAnnouncement text;

  Map<String, RunGraphState>? _states;
  DateTime? _lastSentAt;
  final Map<String, String> _pending = {};

  /// When the waiting changes may go out, or null when none wait.
  DateTime? get pendingDue =>
      _pending.isEmpty ? null : _lastSentAt?.add(minInterval);

  /// Records [graph]; announces the state changes since the last graph or keeps them waiting.
  void observe(RunGraphModel graph) {
    final next = <String, RunGraphState>{};
    final changed = <RunGraphNode>[];
    final before = _states;
    void walk(RunGraphNode n) {
      next[n.id] = n.state;
      final old = before?[n.id];
      if (old != null && old != n.state) changed.add(n);
      n.children.forEach(walk);
    }

    walk(graph.root);
    _states = next;
    if (changed.isEmpty) return;
    for (final n in changed) {
      // A later change of the same node replaces its waiting text.
      _pending.remove(n.id);
      _pending[n.id] = text(n);
    }
    final last = _lastSentAt;
    if (last == null || !clock().isBefore(last.add(minInterval))) _send();
  }

  /// Sends the waiting changes when they are due; does nothing otherwise.
  void flush() {
    final due = pendingDue;
    if (due == null || clock().isBefore(due)) return;
    _send();
  }

  void _send() {
    final message = _pending.values.join('; ');
    _pending.clear();
    _lastSentAt = clock();
    sink(message);
  }
}
