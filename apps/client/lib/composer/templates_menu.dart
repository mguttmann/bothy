// SPDX-License-Identifier: Apache-2.0

/// The templates menu of the composer (01 3.3: personal templates and those of the channel;
/// `listMyTemplates`, `listTemplates`, `createPersonalTemplate`).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../thread/providers.dart';
import '../thread/thread_reads.dart';

/// What the person chose in the menu.
sealed class TemplateChoice {
  const TemplateChoice();
}

/// Insert [template].
final class InsertTemplate extends TemplateChoice {
  /// Creates the choice.
  const InsertTemplate(this.template);

  /// The template.
  final TemplateItem template;
}

/// Save the current text as a personal template.
final class SaveTemplate extends TemplateChoice {
  /// Creates the choice.
  const SaveTemplate();
}

/// The button "Templates" that opens the menu of the templates of [channelId]; [canSave]
/// offers "Save as template" for the current text.
class TemplatesButton extends ConsumerWidget {
  /// Creates the button.
  const TemplatesButton({
    super.key,
    required this.channelId,
    required this.canSave,
    required this.onChoice,
  });

  /// The channel of the thread.
  final ChannelId channelId;

  /// Whether the field holds text to save.
  final bool canSave;

  /// Called with the choice.
  final ValueChanged<TemplateChoice> onChoice;

  Future<void> _open(BuildContext context, WidgetRef ref) async {
    ref.invalidate(templateListsProvider(channelId.value));
    final choice = await showDialog<TemplateChoice>(
      context: context,
      builder: (context) =>
          TemplatesDialog(channelId: channelId, canSave: canSave),
    );
    if (choice != null) onChoice(choice);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    return Tooltip(
      message: l.composerTemplates,
      excludeFromSemantics: true,
      child: DsTappable(
        key: const ValueKey('composer.templates'),
        onTap: () => _open(context, ref),
        label: l.composerTemplates,
        excludeChildSemantics: true,
        child: Padding(
          padding: const EdgeInsets.all(DsSpace.s6),
          child: DsIcon(
            DsIcons.actionNew,
            size: DsIconSizeRole.md,
            color: context.dsColors.textSecondary,
            decorative: true,
          ),
        ),
      ),
    );
  }
}

/// The menu: the personal templates, the channel's templates and "Save as template".
class TemplatesDialog extends ConsumerWidget {
  /// Creates the menu.
  const TemplatesDialog({
    super.key,
    required this.channelId,
    required this.canSave,
  });

  /// The channel.
  final ChannelId channelId;

  /// Whether "Save as template" is offered.
  final bool canSave;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final lists = ref.watch(templateListsProvider(channelId.value));
    Widget group(String heading, List<TemplateItem> items) => Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Padding(
          padding: const EdgeInsets.only(top: DsSpace.s8, bottom: DsSpace.s4),
          child: Text(heading, style: dsTextStyle(DsType.label)),
        ),
        if (items.isEmpty) Text(l.composerTemplatesEmpty),
        for (final t in items)
          ListTile(
            key: ValueKey('template.${t.id}'),
            dense: true,
            title: Text(dsIsolate(t.name)),
            subtitle: Text(
              t.contentMd,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            onTap: () => Navigator.of(context).pop(InsertTemplate(t)),
          ),
      ],
    );
    return AlertDialog(
      key: const ValueKey('templates.dialog'),
      title: Text(l.composerTemplates),
      content: SizedBox(
        width: DsLayout.listWidth * 1.5,
        child: switch (lists) {
          AsyncData(:final value) => SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                group(l.composerTemplatesPersonal, value.personal),
                group(l.composerTemplatesChannel, value.channel),
              ],
            ),
          ),
          AsyncError() => Text(l.composerTemplatesEmpty),
          _ => const Center(child: CircularProgressIndicator()),
        },
      ),
      actions: [
        DsButton(
          key: const ValueKey('templates.close'),
          label: l.threadCancel,
          style: DsButtonStyle.ghost,
          onPressed: () => Navigator.of(context).pop(),
        ),
        if (canSave)
          DsButton(
            key: const ValueKey('template.save'),
            label: l.composerTemplatesSave,
            style: DsButtonStyle.secondary,
            onPressed: () => Navigator.of(context).pop(const SaveTemplate()),
          ),
      ],
    );
  }
}

/// Asks for the name of a new personal template; returns it trimmed, or null.
Future<String?> askTemplateName(BuildContext context) => showDialog<String>(
  context: context,
  builder: (context) => const _TemplateName(),
);

class _TemplateName extends StatefulWidget {
  const _TemplateName();

  @override
  State<_TemplateName> createState() => _TemplateNameState();
}

class _TemplateNameState extends State<_TemplateName> {
  final TextEditingController _name = TextEditingController();

  @override
  void initState() {
    super.initState();
    _name.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final name = _name.text.trim();
    return AlertDialog(
      key: const ValueKey('template.nameDialog'),
      title: Text(l.composerTemplatesSave),
      content: TextField(
        key: const ValueKey('template.name'),
        controller: _name,
        autofocus: true,
        decoration: InputDecoration(labelText: l.composerTemplatesName),
      ),
      actions: [
        DsButton(
          key: const ValueKey('template.cancel'),
          label: l.threadCancel,
          style: DsButtonStyle.ghost,
          onPressed: () => Navigator.of(context).pop(),
        ),
        DsButton(
          key: const ValueKey('template.saveConfirm'),
          label: l.threadSave,
          style: DsButtonStyle.primary,
          onPressed: name.isEmpty
              ? null
              : () => Navigator.of(context).pop(name),
        ),
      ],
    );
  }
}
