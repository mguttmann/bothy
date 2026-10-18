// SPDX-License-Identifier: Apache-2.0

/// What the history column of the shell shows for a location (01 5.1 D1 and D2; brief M4-20
/// Design note 1): a thread, the main history of a text channel, or both side by side when a
/// thread of a text channel is open; "Not found or no access" for an invisible target
/// (01 6.7).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/target.dart';
import '../app/router.dart';
import '../app/target_resolver.dart';
import '../sidebar/providers.dart';
import 'history_column.dart';

/// The history area of [location] (path and query of the router).
class HistoryArea extends ConsumerWidget {
  /// Creates the area.
  const HistoryArea({super.key, required this.location});

  /// The location the router shows.
  final Uri location;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(targetStatusProvider(location.toString())).value;
    if (status == TargetStatus.notFound) {
      return Center(
        child: Text(
          AppLocalizations.of(context).accessNotFoundOrNoAccess,
          key: const ValueKey('target.notFound'),
          style: dsTextStyle(DsType.body)
              .copyWith(color: context.dsColors.textSecondary),
        ),
      );
    }
    Widget history(ThreadId thread, {MessageId? jumpTo}) => HistoryColumn(
      key: ValueKey('historyColumn.${thread.value}.${jumpTo?.value}'),
      threadId: thread,
      jumpTo: jumpTo,
    );
    switch (parseLocation(location)) {
      case ThreadLocation(:final threadId, :final messageId):
        final thread = ref.watch(storedThreadProvider(threadId.value)).value;
        // Without a stored thread the column stays: it waits for the thread, or shows "No
        // access anymore" after a removal deleted it (01 6.6).
        if (thread == null) return history(threadId, jumpTo: messageId);
        final channel = ref
            .watch(storedChannelOfRouteProvider(thread.channelId.value))
            .value;
        final main = channel?.mainThreadId;
        // D2: a thread of a text channel opens to the right of the main history.
        if (channel?.kind == ChannelKind.text &&
            thread.kind != ThreadKind.main &&
            main != null) {
          return Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Expanded(child: history(main)),
              VerticalDivider(
                width: DsBorderWidth.hairline,
                thickness: DsBorderWidth.hairline,
                color: context.dsColors.borderSubtle,
              ),
              Expanded(child: history(threadId, jumpTo: messageId)),
            ],
          );
        }
        return history(threadId, jumpTo: messageId);
      case ChannelLocation(:final channelId):
        final channel = ref
            .watch(storedChannelOfRouteProvider(channelId.value))
            .value;
        final main = channel?.mainThreadId;
        if (channel == null ||
            channel.kind == ChannelKind.forum ||
            main == null) {
          return const SizedBox.expand();
        }
        return history(main);
      default:
        return const SizedBox.expand();
    }
  }
}
