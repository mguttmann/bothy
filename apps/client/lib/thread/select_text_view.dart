// SPDX-License-Identifier: Apache-2.0

/// The view of a message with selectable text on the phone (01 6.15 item 5, R-PR-29; brief
/// M4-50): "Select text" in the message menu opens it, so a part of a message can be copied
/// with the system selection while a long press in the history keeps opening the menu.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// Opens the sheet with the Markdown [content] of a message under the system selection: a long
/// press selects a word, the handles extend it and the system menu copies it (as plain text in
/// the structure of the message, the code exactly). [mentionHandles], [resolver] and
/// [onOpenLink] are those of the message in the history, so the text looks the same and links
/// stay active.
///
/// Example: `await showSelectText(context, content: data.content, onOpenLink: open);`
Future<void> showSelectText(
  BuildContext context, {
  required String content,
  required void Function(Uri) onOpenLink,
  Set<String> mentionHandles = const {},
  DsAttachmentResolver? resolver,
}) => showModalBottomSheet<void>(
  context: context,
  useSafeArea: true,
  isScrollControlled: true,
  builder: (sheet) => SelectTextView(
    content: content,
    mentionHandles: mentionHandles,
    resolver: resolver,
    onOpenLink: (uri) {
      // A link leaves the view, so it closes first.
      Navigator.of(sheet).pop();
      onOpenLink(uri);
    },
  ),
);

/// The content of the sheet of [showSelectText]: a title row with its close button over the
/// rendered message below a [SelectionArea].
class SelectTextView extends StatelessWidget {
  /// Creates the view of [content].
  const SelectTextView({
    super.key,
    required this.content,
    required this.onOpenLink,
    this.mentionHandles = const {},
    this.resolver,
  });

  /// The Markdown content of the message.
  final String content;

  /// Opens an activated link.
  final void Function(Uri) onOpenLink;

  /// The resolved mention handles of the message.
  final Set<String> mentionHandles;

  /// Resolves the internal image references of the message to its files.
  final DsAttachmentResolver? resolver;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      key: const ValueKey('selectText.view'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(
            DsSpace.s16,
            DsSpace.s8,
            DsSpace.s4,
            DsSpace.s4,
          ),
          child: Row(
            children: [
              Expanded(
                child: Semantics(
                  header: true,
                  child: Text(
                    l.messageSelectTextTitle,
                    style: dsTextStyle(DsType.heading)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
              DsTappable(
                key: const ValueKey('selectText.close'),
                label: l.messageSelectTextClose,
                excludeChildSemantics: true,
                onTap: () => Navigator.of(context).pop(),
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: DsIcon(
                    DsIcons.actionDismiss,
                    color: colors.textSecondary,
                    decorative: true,
                  ),
                ),
              ),
            ],
          ),
        ),
        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(
              DsSpace.s16,
              DsSpace.s4,
              DsSpace.s16,
              DsSpace.s24,
            ),
            child: SelectionArea(
              child: DsMarkdown(
                content,
                onOpenLink: onOpenLink,
                attachments: resolver,
                mentionHandles: mentionHandles,
                style: dsTextStyle(DsType.body).copyWith(
                  color: colors.textPrimary,
                  fontSize: context.dsDensity.messageFontSize,
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
