// SPDX-License-Identifier: Apache-2.0

/// The conversation of one search assistant popup (docs/spec/14-search.md 4.10): the questions
/// and their answers, kept only while the popup is open. Each question carries the earlier ones
/// with the terms and the threads of their answers, so "no, the older one" keeps the subject.
library;

import 'package:flutter/foundation.dart';

import 'assist_port.dart';

/// The most earlier questions a question carries: the default of `search.assist.max_turns`.
const int assistMaxTurns = 6;

/// One question and, once it came, its answer or failure.
final class AssistExchange {
  /// Creates the exchange of [question], pending.
  AssistExchange(this.question);

  /// The question.
  final String question;

  /// The answer, or null while pending or after a failure.
  AssistAnswer? answer;

  /// The failure, or null.
  AssistFailureKind? failure;

  /// The wait the server named with [AssistFailureKind.rateLimited], or null.
  Duration? retryAfter;

  /// Whether the answer is still awaited.
  bool get pending => answer == null && failure == null;
}

/// The questions of one popup over [port] (null while signed out: every question fails).
///
/// Example:
/// ```dart
/// final conversation = AssistConversation(port);
/// await conversation.ask('the takeover of the tool');
/// ```
class AssistConversation extends ChangeNotifier {
  /// Creates an empty conversation.
  AssistConversation(this.port);

  /// The search assistant.
  final AssistPort? port;

  final List<AssistExchange> _exchanges = [];

  /// The exchanges, oldest first.
  List<AssistExchange> get exchanges => List.unmodifiable(_exchanges);

  /// Whether a question is pending.
  bool get busy => _exchanges.isNotEmpty && _exchanges.last.pending;

  bool _disposed = false;

  /// Asks [question] (trimmed; an empty one or one while another is pending does nothing).
  Future<void> ask(String question) async {
    final q = question.trim();
    if (q.isEmpty || busy) return;
    final history = [
      for (final e in _exchanges)
        if (e.answer case final answer?)
          AssistTurn(
            q: e.question,
            terms: answer.terms,
            threadIds: [for (final h in answer.hits) h.threadId],
          ),
    ];
    final turns = history.length > assistMaxTurns
        ? history.sublist(history.length - assistMaxTurns)
        : history;
    final exchange = AssistExchange(q);
    _exchanges.add(exchange);
    notifyListeners();
    final p = port;
    try {
      if (p == null) throw const AssistFailure(AssistFailureKind.failed);
      exchange.answer = await p.ask(q, turns);
    } on AssistFailure catch (f) {
      exchange
        ..failure = f.kind
        ..retryAfter = f.retryAfter;
    } on Object {
      exchange.failure = AssistFailureKind.failed;
    }
    if (!_disposed) notifyListeners();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }
}
