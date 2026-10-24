// SPDX-License-Identifier: Apache-2.0

/// The frame of the session column (01 5.1 D1: "Session" with its sections; brief M4-20
/// Scope): the agent session (M4-21: agent card, backend, model, session, project, context,
/// thread cost, runs, earlier sessions) and the sections "Pinned" and "Files" (M4-22) of the
/// open thread.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'agent_section.dart';
import 'files_section.dart';
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
        else ...[
          AgentSection(threadId: thread),
          const SizedBox(height: DsSpace.s16),
          PinsSection(threadId: thread),
          const SizedBox(height: DsSpace.s16),
          FilesSection(threadId: thread),
        ],
      ],
    );
  }
}
