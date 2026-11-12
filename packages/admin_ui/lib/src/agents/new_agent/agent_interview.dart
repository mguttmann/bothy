// SPDX-License-Identifier: Apache-2.0

/// The interview with the builder agent (docs/spec/01-product.md 6.20 item 2;
/// docs/spec/26-builtin-agents.md 6.1; brief A1-08 Design note 5).
///
/// The client keeps the interview: the builder's questions and the person's answers, oldest
/// first, sent whole with every request (26 6.1 item 1). One request per turn; while it runs
/// the view shows a spinner, and the answer replaces it with the builder's questions, the draft
/// or the fallback with its reason. "Use what we have" sets `want_draft`. The cost estimate is
/// the sum of the estimates of every answer so far.
///
/// The server takes at most `agents.interview.max_turns` turns and refuses a longer interview
/// with `too_many_items` on `/turns` before it asks the builder. The interview then ends: the
/// same request goes out once more with `want_draft` and the turns that fit ([fittingTurns]),
/// so a builder that keeps asking leads to a draft and never to an error the person cannot
/// leave (ruling AUD-AA1/1).
library;

import 'dart:math' as math;

import 'package:flutter/foundation.dart';

import 'new_agent_port.dart';

/// The interview of one dialog over [NewAgentPort.interview].
///
/// Example:
/// ```dart
/// final interview = AgentInterview(port)..start();
/// await interview.answer('Notes of our weekly meeting');
/// if (interview.draft != null) flow.useInterview(interview.draft!);
/// ```
final class AgentInterview extends ChangeNotifier {
  /// Creates the interview over the given [NewAgentPort].
  AgentInterview(this._port);

  final NewAgentPort _port;
  final List<InterviewTurn> _turns = [];
  List<String> _questions = const [];
  InterviewReply? _draft;
  String? _fallback;
  NewAgentFailure? _error;
  bool _pending = false;
  bool _closed = false;
  int _costMicros = 0;
  String? _currency;
  bool _hasCost = false;

  /// The most turns the server took in one request of this interview: that many fit
  /// `agents.interview.max_turns`, which the client cannot read.
  int _taken = 0;

  /// The turns so far, oldest first: the builder's questions and the person's answers.
  List<InterviewTurn> get turns => List.unmodifiable(_turns);

  /// The open questions of the last answer, empty when none is open.
  List<String> get questions => _questions;

  /// The answer that carried a draft, or null.
  InterviewReply? get draft => _draft;

  /// The fallback reason (`builder_not_configured`, `builder_offline`, `builder_timeout`,
  /// `builder_invalid`), or null.
  String? get fallbackReason => _fallback;

  /// The failure of the last request (for example a turn the server found too long), or null.
  NewAgentFailure? get error => _error;

  /// Whether a request runs.
  bool get pending => _pending;

  /// The sum of the cost estimates so far in micro units of [currency], or null before the
  /// first estimate.
  int? get costMicros => _hasCost ? _costMicros : null;

  /// The currency of [costMicros].
  String? get currency => _currency;

  /// Asks the builder for its first questions.
  Future<void> start() => _send(wantDraft: false);

  /// Sends [text] as the person's answer to the open questions; an empty answer or one while a
  /// request runs does nothing.
  Future<void> answer(String text) async {
    final t = text.trim();
    if (t.isEmpty || _pending || _fallback != null) return;
    _turns.add(InterviewTurn(InterviewRole.owner, t));
    await _send(wantDraft: false);
  }

  /// "Use what we have": asks for the draft now (`want_draft`), with [text] as a last answer
  /// when it is not empty.
  Future<void> useWhatWeHave([String text = '']) async {
    if (_pending || _fallback != null) return;
    final t = text.trim();
    if (t.isNotEmpty) _turns.add(InterviewTurn(InterviewRole.owner, t));
    await _send(wantDraft: true);
  }

  Future<void> _send({required bool wantDraft}) async {
    _pending = true;
    _error = null;
    _notify();
    try {
      final reply = await _ask(List.of(_turns), wantDraft: wantDraft);
      if (_closed) return;
      if (reply.costMicros case final micros?) {
        _hasCost = true;
        _costMicros += micros;
        _currency = reply.currency ?? _currency;
      }
      switch (reply.outcome) {
        case InterviewOutcome.questions:
          _questions = reply.questions;
          for (final q in reply.questions) {
            _turns.add(InterviewTurn(InterviewRole.builder, q));
          }
        case InterviewOutcome.draft:
          _questions = const [];
          _draft = reply;
        case InterviewOutcome.fallback:
          _questions = const [];
          _fallback = reply.fallbackReason ?? 'builder_invalid';
      }
    } on Object catch (e) {
      // Any other error (a malformed answer) shows as an internal error, as in the flow.
      _error = e is NewAgentFailure
          ? e
          : const NewAgentFailure.problem('internal_error');
      // The answer that was not taken goes back to the person, who may shorten it.
      if (_turns.isNotEmpty && _turns.last.role == InterviewRole.owner) {
        _turns.removeLast();
      }
    } finally {
      _pending = false;
      _notify();
    }
  }

  /// Sends [turns]; an interview the server finds too long is sent once more as a request for
  /// the draft with the turns that fit.
  Future<InterviewReply> _ask(
    List<InterviewTurn> turns, {
    required bool wantDraft,
  }) async {
    try {
      final reply = await _port.interview(turns, wantDraft: wantDraft);
      _taken = math.max(_taken, turns.length);
      return reply;
    } on NewAgentFailure catch (e) {
      if (!_tooManyTurns(e)) rethrow;
      // At least one turn fits: the lowest value of the key is 1 (26 14).
      final fitting = fittingTurns(turns, math.max(_taken, 1));
      if (fitting.length == turns.length) rethrow;
      return _port.interview(fitting, wantDraft: true);
    }
  }

  static bool _tooManyTurns(NewAgentFailure e) =>
      e.errors.any((i) => i.pointer == '/turns' && i.code == 'too_many_items');

  void _notify() {
    if (!_closed) notifyListeners();
  }

  @override
  void dispose() {
    _closed = true;
    super.dispose();
  }
}

/// The at most [fit] turns of [turns] a request for the draft carries when the interview is
/// longer than the server takes, in their order: first the builder's questions at the end that
/// nobody answered are left out, then its questions from the oldest on, and only then the
/// person's answers from the oldest on, so the person's own words reach the builder as long as
/// they fit.
///
/// Example: with `fit` 3, `[q1, q2, a1, q3, a2]` becomes `[a1, q3, a2]`.
List<InterviewTurn> fittingTurns(List<InterviewTurn> turns, int fit) {
  final kept = List.of(turns);
  while (kept.length > fit &&
      kept.isNotEmpty &&
      kept.last.role == InterviewRole.builder) {
    kept.removeLast();
  }
  for (final role in [InterviewRole.builder, InterviewRole.owner]) {
    while (kept.length > fit) {
      final oldest = kept.indexWhere((t) => t.role == role);
      if (oldest < 0) break;
      kept.removeAt(oldest);
    }
  }
  return kept;
}
