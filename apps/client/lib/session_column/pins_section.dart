// SPDX-License-Identifier: Apache-2.0

/// The section "Pinned (n)" of the session column (01 5.1 D1; 01 3.3: pinning is moderation):
/// the pinned messages of the open thread; a row opens the message in the history.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../thread/message_tile.dart' show excerpt;
import '../thread/providers.dart';

/// The pinned messages of [threadId]; opening the section reads `listPins` once, so pins
/// outside the loaded history show too.
class PinsSection extends ConsumerStatefulWidget {
  /// Creates the section.
  const PinsSection({super.key, required this.threadId});

  /// The open thread.
  final ThreadId threadId;

  @override
  ConsumerState<PinsSection> createState() => _PinsSectionState();
}

class _PinsSectionState extends ConsumerState<PinsSection> {
  @override
  void initState() {
    super.initState();
    // Offline or refused: the section shows the stored pins.
    unawaited(
      ref
          .read(threadReadsProvider)
          .loadPins(widget.threadId)
          .catchError((Object _) {}),
    );
  }

  @override
  void didUpdateWidget(PinsSection old) {
    super.didUpdateWidget(old);
    if (old.threadId != widget.threadId) {
      unawaited(
        ref
            .read(threadReadsProvider)
            .loadPins(widget.threadId)
            .catchError((Object _) {}),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final threadId = widget.threadId;
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final pinned =
        ref.watch(pinnedMessagesProvider(threadId.value)).value ?? const [];
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    return Column(
      key: const ValueKey('session.pinned'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.sessionPinned(pinned.length),
            style: dsTextStyle(DsType.label)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s6),
        if (pinned.isEmpty)
          Text(
            l.sessionPinnedEmpty,
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          ),
        for (final m in pinned)
          DsTappable(
            key: ValueKey('session.pin.${m.id.value}'),
            onTap: () => context.go('/t/${threadId.value}?m=${m.id.value}'),
            label: excerpt(m.contentMd),
            excludeChildSemantics: true,
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    dsIsolate(principals[m.authorId]?.displayName ?? ''),
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: colors.textSecondary),
                  ),
                  Text(
                    excerpt(m.contentMd),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.textPrimary),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}
