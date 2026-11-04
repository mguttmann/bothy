// SPDX-License-Identifier: Apache-2.0

/// The port of the search assistant (docs/spec/14-search.md 4.10, docs/design/search-assistant.md
/// 4): one question with the earlier questions of the popup, and the answer as the popup shows
/// it. The API adapter maps the generated client and its problems to these types, so the popup
/// and its tests never see Dio.
library;

import 'package:api_client/api_client.dart' as rest;
import 'package:built_collection/built_collection.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

import '../search/search_port.dart' show SearchSegment;

/// An earlier question of the popup, sent back with the next one: the question, the terms of
/// its answer and the threads it showed.
final class AssistTurn {
  /// Creates the turn.
  const AssistTurn({
    required this.q,
    this.terms = const [],
    this.threadIds = const [],
  });

  /// The question.
  final String q;

  /// The terms of its answer.
  final List<String> terms;

  /// The threads its answer showed.
  final List<ThreadId> threadIds;
}

/// One session of an answer.
final class AssistHit {
  /// Creates the hit.
  const AssistHit({
    required this.threadId,
    required this.channelId,
    required this.channelName,
    required this.at,
    required this.segments,
    this.messageId,
    this.threadTitle,
    this.spaceName,
    this.reason,
  });

  /// The thread (the session).
  final ThreadId threadId;

  /// The matching message to jump to, or null when only the title matched.
  final MessageId? messageId;

  /// The channel, for a development session the project forum.
  final ChannelId channelId;

  /// The channel's name.
  final String channelName;

  /// The thread title, or null for the main thread of a text channel.
  final String? threadTitle;

  /// The space, or null for a direct message.
  final String? spaceName;

  /// When the matching message or the thread was created.
  final DateTime at;

  /// The assistant's sentence why, or null for a full-text hit.
  final String? reason;

  /// The excerpt.
  final List<SearchSegment> segments;

  /// Whether the assistant picked this session.
  bool get picked => reason != null;
}

/// Why an answer is full text only.
enum AssistFallback {
  /// No assistant agent is configured.
  notConfigured,

  /// The assistant agent is not connected.
  unavailable,

  /// The assistant did not answer in time.
  timeout,
}

/// The answer to one question.
final class AssistAnswer {
  /// Creates the answer.
  const AssistAnswer({
    required this.hits,
    required this.terms,
    this.fallback,
    this.costMicros,
    this.currency,
  });

  /// The sessions: the picks first, then full-text hits.
  final List<AssistHit> hits;

  /// The search terms of the question; sent back with the next one.
  final List<String> terms;

  /// Why the answer is full text only, or null when the assistant answered.
  final AssistFallback? fallback;

  /// The estimate of the assistant's model call in millionths of [currency], or null.
  final int? costMicros;

  /// The currency of [costMicros].
  final String? currency;
}

/// Why a question failed, as far as the popup treats it differently.
enum AssistFailureKind {
  /// `400 validation_failed` on `/q` with `empty`: no search term.
  queryEmpty,

  /// `400 validation_failed` on `/q` with `too_long`.
  queryTooLong,

  /// No connection: the request did not reach the server.
  offline,

  /// Any other failure (also `503 search_timeout`).
  failed,
}

/// The failure of an [AssistPort] call.
final class AssistFailure implements Exception {
  /// Creates the failure of [kind].
  const AssistFailure(this.kind);

  /// Why the question failed.
  final AssistFailureKind kind;

  @override
  String toString() => 'AssistFailure(${kind.name})';
}

/// The search assistant (operation `assistSearch`).
abstract interface class AssistPort {
  /// The answer to [q] after the earlier questions [history] (oldest first).
  ///
  /// Throws an [AssistFailure].
  Future<AssistAnswer> ask(String q, List<AssistTurn> history);
}

/// The [AssistPort] over the generated client of the signed-in instance.
///
/// Example: `ApiAssistPort(services.auth.api).ask('the takeover of the tool', const [])`.
final class ApiAssistPort implements AssistPort {
  /// Creates the port over [api].
  const ApiAssistPort(this.api);

  /// The REST client.
  final rest.ApiClient api;

  @override
  Future<AssistAnswer> ask(String q, List<AssistTurn> history) async {
    final body = rest.SearchAssistQuery((b) {
      b.q = q;
      if (history.isNotEmpty) {
        b.history = ListBuilder([
          for (final t in history)
            rest.SearchAssistTurn(
              (tb) => tb
                ..q = t.q
                ..terms = t.terms.isEmpty ? null : ListBuilder(t.terms)
                ..threadIds = t.threadIds.isEmpty
                    ? null
                    : ListBuilder([for (final id in t.threadIds) id.value]),
            ),
        ]);
      }
    });
    try {
      final response = await api.getSearchApi().assistSearch(
        searchAssistQuery: body,
      );
      return assistAnswerFromRest(response.data!);
    } on Object catch (e) {
      throw assistFailureOf(e);
    }
  }
}

/// The [AssistFailure] of [error], an error of the generated client.
AssistFailure assistFailureOf(Object error) {
  if (error is AssistFailure) return error;
  final problem = rest.ApiProblem.of(error);
  if (problem != null) {
    if (problem.code == 'validation_failed') {
      for (final e in problem.errors) {
        if (e.pointer != '/q') continue;
        if (e.code == 'empty') {
          return const AssistFailure(AssistFailureKind.queryEmpty);
        }
        if (e.code == 'too_long') {
          return const AssistFailure(AssistFailureKind.queryTooLong);
        }
      }
    }
    return const AssistFailure(AssistFailureKind.failed);
  }
  if (error is DioException &&
      error.response == null &&
      (error.type == DioExceptionType.connectionError ||
          error.type == DioExceptionType.connectionTimeout ||
          error.type == DioExceptionType.sendTimeout ||
          error.type == DioExceptionType.receiveTimeout)) {
    return const AssistFailure(AssistFailureKind.offline);
  }
  return const AssistFailure(AssistFailureKind.failed);
}

/// The answer of the popup for the server's [result]. A hit with a source this version does not
/// know is shown as a full-text hit.
AssistAnswer assistAnswerFromRest(rest.SearchAssistResult result) {
  final reason = result.fallbackReason;
  return AssistAnswer(
    hits: [
      for (final h in result.hits)
        AssistHit(
          threadId: ThreadId.parse(h.threadId),
          messageId: h.messageId == null ? null : MessageId.parse(h.messageId!),
          channelId: ChannelId.parse(h.channelId),
          channelName: h.channelName,
          threadTitle: h.threadTitle,
          spaceName: h.spaceName,
          at: h.at,
          reason: h.source_ == rest.SearchAssistHitSource_Enum.assistant
              ? h.reason
              : null,
          segments: [
            for (final s in h.segments)
              SearchSegment(text: s.text, match: s.match),
          ],
        ),
    ],
    terms: result.terms.toList(),
    fallback: switch (reason) {
      rest.SearchAssistResultFallbackReasonEnum.assistantNotConfigured =>
        AssistFallback.notConfigured,
      rest.SearchAssistResultFallbackReasonEnum.assistantUnavailable =>
        AssistFallback.unavailable,
      rest.SearchAssistResultFallbackReasonEnum.assistantTimeout =>
        AssistFallback.timeout,
      _ =>
        result.mode == rest.SearchAssistResultModeEnum.assisted
            ? null
            : AssistFallback.unavailable,
    },
    costMicros: result.costEstimate?.amountMicros,
    currency: result.costEstimate?.currency,
  );
}

/// Where selecting [hit] leads: the matching message in its thread, else the thread.
String assistTargetOf(AssistHit hit) {
  final message = hit.messageId;
  final thread = hit.threadId.value;
  return message == null ? '/t/$thread' : '/t/$thread?m=${message.value}';
}
