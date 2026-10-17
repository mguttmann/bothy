// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

/// A thread inside an administration page (an audit entry, a run). The console has no chat
/// (ADR 0028) and `getDiscovery` names no deep link scheme of the apps, so the thread shows its
/// ID with a copy action and the note to open it in the app (docs/DECISIONS.md D-M6-02-7).
class ThreadLink extends StatefulWidget {
  /// Creates the link of [threadId].
  const ThreadLink({super.key, required this.threadId});

  /// The thread ID.
  final String threadId;

  @override
  State<ThreadLink> createState() => _ThreadLinkState();
}

class _ThreadLinkState extends State<ThreadLink> {
  bool _copied = false;

  Future<void> _copy() async {
    await Clipboard.setData(ClipboardData(text: widget.threadId));
    if (mounted) setState(() => _copied = true);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Wrap(
      crossAxisAlignment: WrapCrossAlignment.center,
      spacing: DsSpace.s8,
      children: [
        Text(l.consoleThreadInApp(widget.threadId)),
        TextButton(onPressed: _copy, child: Text(l.consoleCopyThreadId)),
        if (_copied) Semantics(liveRegion: true, child: Text(l.consoleCopied)),
      ],
    );
  }
}
