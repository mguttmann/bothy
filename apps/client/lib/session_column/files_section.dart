// SPDX-License-Identifier: Apache-2.0

/// The section "Files (n)" of the session column (01 5.1 D1; brief M4-22 Scope): the files of
/// the open thread's messages, newest message first, each as the short form of its attachment
/// tile (name, size, state, a small thumbnail for an image from the server).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../files/attachment_tile.dart';
import '../files/providers.dart';

/// The files of [threadId].
class FilesSection extends ConsumerWidget {
  /// Creates the section.
  const FilesSection({super.key, required this.threadId});

  /// The open thread.
  final ThreadId threadId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final files =
        ref.watch(threadFilesProvider(threadId.value)).value ?? const [];
    return Column(
      key: const ValueKey('session.files'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.sessionFiles(files.length),
            style: dsTextStyle(DsType.label)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s6),
        if (files.isEmpty)
          Text(
            l.sessionFilesEmpty,
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          ),
        for (final f in files)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s6),
            child: AttachmentTile(file: f.file, compact: true),
          ),
      ],
    );
  }
}
