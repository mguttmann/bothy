// SPDX-License-Identifier: Apache-2.0

/// The seen mark of a chat on screen (docs/spec/01-product.md 6.17 item 3; brief M4-52 Design
/// note 3): while the history of a chat shows, on the desktop or the phone, its newest run and
/// the state of it are the person's seen state on this device, so the sidebar shows no status
/// dot for them.
library;

import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../agents/providers.dart' show threadRunsProvider;
import 'providers.dart' show chatSeenWriterProvider;
import 'sidebar_status.dart';

/// Writes the seen mark of [threadId] while it is in the tree: once when it appears and again on
/// every change of the chat's newest run or its status, so a change the person watches never
/// shows a dot after they leave.
///
/// ```dart
/// ChatSeenMarker(threadId: thread, child: history)
/// ```
class ChatSeenMarker extends ConsumerStatefulWidget {
  /// Creates the marker of [threadId] around [child].
  const ChatSeenMarker({
    super.key,
    required this.threadId,
    required this.child,
  });

  /// The chat on screen.
  final ThreadId threadId;

  /// What shows the chat.
  final Widget child;

  @override
  ConsumerState<ChatSeenMarker> createState() => _ChatSeenMarkerState();
}

class _ChatSeenMarkerState extends ConsumerState<ChatSeenMarker> {
  ProviderSubscription<AsyncValue<List<RunSummary>>>? _runs;

  /// The run and status written last, so a run step that changes nothing writes nothing.
  (RunId, Object?)? _written;

  @override
  void initState() {
    super.initState();
    _runs = ref.listenManual(
      threadRunsProvider(widget.threadId.value),
      (_, next) => _write(next.value),
      fireImmediately: true,
    );
  }

  void _write(List<RunSummary>? runs) {
    if (runs == null || runs.isEmpty) return;
    final newest = newestRuns(runs)[widget.threadId];
    if (newest == null) return;
    final key = (newest.id, chatStatusOf(newest.state) ?? newest.state);
    if (key == _written) return;
    _written = key;
    ref.read(chatSeenWriterProvider)?.call(widget.threadId, newest);
  }

  @override
  void dispose() {
    _runs?.close();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
