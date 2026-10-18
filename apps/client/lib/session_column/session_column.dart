// SPDX-License-Identifier: Apache-2.0

/// The frame of the session column (01 5.1 D1: "Session" with its sections; brief M4-20
/// Scope): the section "Pinned" of the open thread. M4-21 adds the agent session and M4-22
/// the files.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'pins_section.dart';

/// The session column of [threadId], or a hint while no thread is open.
class SessionColumn extends StatelessWidget {
  /// Creates the column.
  const SessionColumn({super.key, required this.threadId});

  /// The open thread, or null.
  final ThreadId? threadId;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final thread = threadId;
    return ListView(
      key: const ValueKey('session.column'),
      padding: const EdgeInsets.all(DsSpace.s16),
      children: [
        Semantics(
          header: true,
          child: Text(
            l.shellColumnSession,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s12),
        if (thread == null)
          Text(
            l.sessionNoThread,
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          )
        else
          PinsSection(threadId: thread),
      ],
    );
  }
}
