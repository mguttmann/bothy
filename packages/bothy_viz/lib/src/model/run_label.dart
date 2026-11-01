// SPDX-License-Identifier: Apache-2.0

/// The shared label of a run or a chat in the views (docs/spec/25-visualization.md 4 rule 6).
library;

import 'package:api_client/api_client.dart' as api;

import 'names.dart';

/// What a run card, a timeline row and their table rows call a run: the project (the name of the
/// channel or forum) and the chat (the thread title) as the main text, the agent and a short ID
/// only as secondary text, and the chat to open on a tap.
///
/// Project, chat and link come only from the `chat` the server sent (`StatsRunChat`), which it
/// sends only to a person who may read the thread, by the rule of the sidebar; without it the
/// label names no project and no chat and has no link (25 4 rule 5: clients hold no permission
/// logic).
///
/// ```dart
/// final label = VizRunLabel.of(run.chat, agent: 'dev-runner', id: run.runId);
/// label.named; // true when the server named the chat
/// ```
final class VizRunLabel {
  /// Creates a label; [VizRunLabel.of] is the normal way.
  const VizRunLabel({
    required this.agent,
    required this.shortId,
    this.project,
    this.chat,
    this.threadId,
  });

  /// The label of [chat] (absent when the server named none) for the run or row of [agent],
  /// with the short form of [id] as the short ID.
  factory VizRunLabel.of(
    api.StatsRunChat? chat, {
    required String agent,
    required String id,
  }) {
    final title = chat?.threadTitle;
    return VizRunLabel(
      agent: agent,
      shortId: VizNames.short(id),
      project: chat?.channelName,
      chat: title == null || title.isEmpty ? null : title,
      threadId: chat?.threadId,
    );
  }

  /// Name of the channel or forum of the chat; `null` when the server named none.
  final String? project;

  /// Title of the thread; `null` for a main thread or when the server named none.
  final String? chat;

  /// Display name of the agent (the operator of the run).
  final String agent;

  /// The short form of the run or chat ID ([VizNames.short]).
  final String shortId;

  /// The thread a tap opens; `null` when the server named no chat.
  final String? threadId;

  /// Whether the server named the chat: only then the label shows a project and has a link.
  bool get named => project != null;

  @override
  bool operator ==(Object other) =>
      other is VizRunLabel &&
      other.project == project &&
      other.chat == chat &&
      other.agent == agent &&
      other.shortId == shortId &&
      other.threadId == threadId;

  @override
  int get hashCode => Object.hash(project, chat, agent, shortId, threadId);
}
