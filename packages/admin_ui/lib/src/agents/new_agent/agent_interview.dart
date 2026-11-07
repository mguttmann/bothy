// SPDX-License-Identifier: Apache-2.0

/// The interview with the builder agent (docs/spec/01-product.md 6.20 item 2;
/// docs/spec/26-builtin-agents.md 6.1; brief A1-08 Design note 5).
///
/// The client keeps the interview: the builder's questions and the person's answers, oldest
/// first, sent whole with every request (26 6.1 item 1). One request per turn; while it runs
/// the view shows a spinner, and the answer replaces it with the builder's questions, the draft
/// or the fallback with its reason. "Use what we have" sets `want_draft`. The cost estimate is
/// the sum of the estimates of every answer so far.
library;

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
      final reply = await _port.interview(
        List.of(_turns),
        wantDraft: wantDraft,
      );
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

  void _notify() {
    if (!_closed) notifyListeners();
  }

  @override
  void dispose() {
    _closed = true;
    super.dispose();
  }
}
