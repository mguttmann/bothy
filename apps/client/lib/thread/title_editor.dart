// SPDX-License-Identifier: Apache-2.0

/// The title editor of the thread header (01 6.7: editing makes the title manual;
/// `setThreadTitle`).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// Asks for a new title, starting from [current]; returns it trimmed, or null when the person
/// cancels. [maxChars] is `thread_title_max_chars` of the instance (D-W-115), or null.
///
/// Example: `final title = await editTitle(context, thread.title, maxChars: 200);`
Future<String?> editTitle(
  BuildContext context,
  String current, {
  int? maxChars,
}) => showDialog<String>(
  context: context,
  builder: (context) => TitleEditor(current: current, maxChars: maxChars),
);

/// The dialog of [editTitle].
class TitleEditor extends StatefulWidget {
  /// Creates the editor.
  const TitleEditor({super.key, required this.current, this.maxChars});

  /// The title now.
  final String current;

  /// The longest title, or null.
  final int? maxChars;

  @override
  State<TitleEditor> createState() => _TitleEditorState();
}

class _TitleEditorState extends State<TitleEditor> {
  late final TextEditingController _title = TextEditingController(
    text: widget.current,
  )..addListener(() => setState(() {}));

  @override
  void dispose() {
    _title.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final text = _title.text.trim();
    final ok = text.isNotEmpty && text != widget.current;
    return AlertDialog(
      key: const ValueKey('title.dialog'),
      title: Text(l.threadEditTitle),
      content: SizedBox(
        width: DsLayout.listWidth * 1.5,
        child: TextField(
          key: const ValueKey('title.field'),
          controller: _title,
          autofocus: true,
          maxLength: widget.maxChars,
          decoration: InputDecoration(labelText: l.threadTitleLabel),
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('title.cancel'),
          label: l.threadCancel,
          style: DsButtonStyle.ghost,
          onPressed: () => Navigator.of(context).pop(),
        ),
        DsButton(
          key: const ValueKey('title.save'),
          label: l.threadSave,
          style: DsButtonStyle.primary,
          onPressed: ok ? () => Navigator.of(context).pop(text) : null,
        ),
      ],
    );
  }
}
