// SPDX-License-Identifier: Apache-2.0

/// "View versions" of an edited message (01 3.3; `listMessageRevisions`, where the server
/// allows `message.revisions.read`).
library;

import 'package:design_system/design_system.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'thread_reads.dart';

/// Shows the current text of [message] and its earlier versions from [reads].
///
/// Example: `await showVersions(context, reads, message);`
Future<void> showVersions(
  BuildContext context,
  ThreadReads reads,
  Message message,
) => showDialog<void>(
  context: context,
  builder: (context) =>
      VersionsDialog(message: message, versions: reads.revisions(message.id)),
);

/// The dialog: the current version first, then the earlier ones, newest first; a failed read
/// shows its reason (`problemText`).
class VersionsDialog extends StatelessWidget {
  /// Creates the dialog.
  const VersionsDialog({
    super.key,
    required this.message,
    required this.versions,
  });

  /// The edited message.
  final Message message;

  /// The earlier versions as the server returns them.
  final Future<List<Revision>> versions;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    Widget block(Key key, String heading, String text) => Padding(
      key: key,
      padding: const EdgeInsets.only(bottom: DsSpace.s12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(heading, style: dsTextStyle(DsType.label)),
          const SizedBox(height: DsSpace.s4),
          SelectableText(text, style: dsTextStyle(DsType.body)),
        ],
      ),
    );
    return AlertDialog(
      key: const ValueKey('versions.dialog'),
      title: Text(l.messageVersionsTitle),
      content: SizedBox(
        width: DsLayout.listWidth * 1.5,
        child: FutureBuilder<List<Revision>>(
          future: versions,
          builder: (context, snap) {
            if (snap.hasError) {
              final e = snap.error;
              final code = e is DioException ? problemCodeOf(e) : null;
              return Text(
                l.messageVersionsFailed(problemText(l, code ?? 'unavailable')),
              );
            }
            final list = snap.data;
            if (list == null) {
              return const Center(child: CircularProgressIndicator());
            }
            return SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                mainAxisSize: MainAxisSize.min,
                children: [
                  block(
                    const ValueKey('versions.current'),
                    l.messageVersionsCurrent,
                    message.contentMd,
                  ),
                  if (list.isEmpty) Text(l.messageVersionsEmpty),
                  for (final r in list.reversed)
                    block(
                      ValueKey('versions.item.${r.number}'),
                      l.messageVersionsRevision(r.number),
                      r.contentMd,
                    ),
                ],
              ),
            );
          },
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('versions.close'),
          label: l.threadCancel,
          style: DsButtonStyle.ghost,
          onPressed: () => Navigator.of(context).pop(),
        ),
      ],
    );
  }
}

/// The problem code of a failed REST call, or null.
String? problemCodeOf(DioException e) {
  final data = e.response?.data;
  return data is Map ? data['code'] as String? : null;
}
