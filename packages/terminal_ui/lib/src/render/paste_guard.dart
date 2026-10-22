// SPDX-License-Identifier: Apache-2.0

/// Pasting into the terminal (docs/spec/15-client-architecture.md 23.4): text with a line break
/// is shown first and pasted only after "Paste", because every line break can start a command.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

/// Whether [text] needs the confirmation: it holds a line feed or a carriage return.
bool needsPasteConfirmation(String text) =>
    text.contains('\n') || text.contains('\r');

/// The number of lines of [text] as the confirmation counts them (CR LF, LF and CR end a
/// line; a trailing break adds no empty line).
int pasteLineCount(String text) {
  final lines = text
      .replaceAll('\r\n', '\n')
      .replaceAll('\r', '\n')
      .split('\n');
  if (lines.length > 1 && lines.last.isEmpty) lines.removeLast();
  return lines.length;
}

/// Asks before pasting [text] with line breaks; returns whether to paste. Text without a line
/// break returns true at once without a dialog.
///
/// ```dart
/// if (await confirmPaste(context, text)) terminal.paste(text);
/// ```
Future<bool> confirmPaste(BuildContext context, String text) async {
  if (!needsPasteConfirmation(text)) return true;
  final answer = await showDialog<bool>(
    context: context,
    builder: (_) => PasteConfirmDialog(text: text),
  );
  return answer ?? false;
}

/// The dialog "Paste with line breaks?" with the text to paste.
class PasteConfirmDialog extends StatelessWidget {
  /// Creates the dialog for [text].
  const PasteConfirmDialog({super.key, required this.text});

  /// The text from the clipboard.
  final String text;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    return AlertDialog(
      title: Text(l.terminalPasteTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(
          maxWidth: DsLayout.columnMin * 3,
          maxHeight: DsLayout.columnMin * 2,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l.terminalPasteBody(pasteLineCount(text))),
            const SizedBox(height: DsSpace.s8),
            Flexible(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: c.surfaceSunken,
                  borderRadius: BorderRadius.circular(DsRadius.md),
                ),
                child: SingleChildScrollView(
                  padding: const EdgeInsets.all(DsSpace.s8),
                  child: Text(
                    text,
                    style: dsTextStyle(DsType.codeSm)
                        .copyWith(color: c.textPrimary),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          autofocus: true,
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l.terminalCancel),
        ),
        FilledButton(
          onPressed: () => Navigator.of(context).pop(true),
          child: Text(l.terminalPasteConfirm),
        ),
      ],
    );
  }
}
