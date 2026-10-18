// SPDX-License-Identifier: Apache-2.0

/// The composer at the bottom of the history (brief M4-20 Design note 3; components.md 9): the
/// `DsComposer` of M4-07 with the placeholder "Message to `target`", Enter or Shift+Enter by
/// `composer.enter_sends`, the typing line, mention suggestions, templates, the prepared
/// reply, the refusal over the limit, the `overwrote_newer` notice, the offline hint and the
/// note of a read-only channel.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../settings/settings_providers.dart';
import '../thread/providers.dart';
import '../thread/thread_actions.dart';
import '../thread/typing_line.dart';
import 'composer_controller.dart';
import 'draft_sync.dart';
import 'templates_menu.dart';

/// The composer of [threadId] in [channelId], named [targetName] in the placeholder.
class ThreadComposer extends ConsumerStatefulWidget {
  /// Creates the composer.
  const ThreadComposer({
    super.key,
    required this.threadId,
    required this.channelId,
    required this.targetName,
    required this.onEditLast,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel.
  final ChannelId channelId;

  /// The channel or thread name of the placeholder (a foreign name).
  final String targetName;

  /// Arrow up in the empty field: edit the last own message (01 7).
  final VoidCallback onEditLast;

  @override
  ConsumerState<ThreadComposer> createState() => ThreadComposerState();
}

/// State of [ThreadComposer]; the history calls [startReply] for the action Reply.
class ThreadComposerState extends ConsumerState<ThreadComposer> {
  ComposerController? _composer;
  bool _overwritten = false;

  @override
  void initState() {
    super.initState();
    final engine = ref.read(syncEngineProvider);
    final db = ref.read(localDatabaseProvider);
    if (engine == null || db == null) return;
    _composer = ComposerController(
      thread: widget.threadId,
      channel: widget.channelId,
      actions: ThreadActions(engine),
      drafts: DraftWriter(
        outbox: engine.outbox,
        dao: db.outboxDao,
        thread: widget.threadId,
      ),
      editing: ref.read(draftEditingProvider),
      typing: engine.typing,
    )..addListener(_changed);
    final composer = _composer!;
    final thread = widget.threadId.value;
    unawaited(composer.restoreUnsent());
    ref.listenManual(
      threadDraftProvider(widget.channelId.value, thread),
      (_, next) => unawaited(composer.applyStored(next.value)),
      fireImmediately: true,
    );
    ref.listenManual(syncNoticesProvider, (_, next) {
      final notice = next.value;
      // 04 14: this device overwrote a newer draft of another device.
      if (notice is DraftOverwritten &&
          notice.draft.threadId == widget.threadId &&
          mounted) {
        setState(() => _overwritten = true);
      }
    });
  }

  void _changed() => setState(() {});

  @override
  void dispose() {
    _composer?.removeListener(_changed);
    _composer?.dispose();
    super.dispose();
  }

  /// Prepares a reply to [message] of [authorName] and puts the focus into the field.
  void startReply(Message message, String authorName) {
    _composer?.startReply(message, authorName);
  }

  Future<void> _template(TemplateChoice choice) async {
    final composer = _composer;
    if (composer == null) return;
    switch (choice) {
      case InsertTemplate(:final template):
        composer.insert(template.contentMd);
      case SaveTemplate():
        final name = await askTemplateName(context);
        final engine = ref.read(syncEngineProvider);
        if (name == null || engine == null) return;
        await ThreadActions(engine).saveTemplate(name, composer.text.text);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final composer = _composer;
    if (composer == null) return const SizedBox.shrink();
    final thread = widget.threadId.value;
    final channel = widget.channelId.value;
    final config = ref.watch(appConfigProvider);
    final me = ref.watch(myPrincipalIdProvider);
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    composer
      ..principals = principals
      ..limits = ref.watch(instanceLimitsProvider).value;
    final phase = ref.watch(connectionStateProvider).value?.phase;
    final offline =
        phase == null ||
        phase == ConnectionPhase.offline ||
        phase == ConnectionPhase.backoff ||
        phase == ConnectionPhase.suspended;
    final permissions = ref.watch(threadPermissionsProvider(channel));
    final typing = typingNames(
      ref.watch(typingInProvider(thread)).value ?? const {},
      principals,
      me: me,
    );
    final error = composer.error;
    final colors = context.dsColors;
    Widget note(Key key, DsTone tone, String text, {VoidCallback? onClose}) =>
        Padding(
          key: key,
          padding: const EdgeInsets.only(bottom: DsSpace.s4),
          child: DsBanner(
            tone: tone,
            text: text,
            liveRegion: true,
            actionLabel: onClose == null ? null : l.threadCancel,
            onAction: onClose,
          ),
        );
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        DsSpace.s4,
        DsSpace.s16,
        DsSpace.s12,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (_overwritten)
            note(
              const ValueKey('composer.draftNotice'),
              DsTone.warning,
              l.composerDraftOverwritten,
              onClose: () => setState(() => _overwritten = false),
            ),
          if (error case ContentTooLong(:final limit))
            note(
              const ValueKey('composer.error'),
              DsTone.danger,
              l.composerTooLong(limit),
            ),
          if (composer.replyTo != null)
            Padding(
              key: const ValueKey('composer.reply'),
              padding: const EdgeInsets.only(bottom: DsSpace.s4),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      l.composerReplyTo(dsIsolate(composer.replyName ?? '')),
                      style: dsTextStyle(DsType.caption)
                          .copyWith(color: colors.textSecondary),
                    ),
                  ),
                  DsButton(
                    key: const ValueKey('composer.cancelReply'),
                    label: l.composerCancelReply,
                    style: DsButtonStyle.ghost,
                    onPressed: composer.cancelReply,
                  ),
                ],
              ),
            ),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              if (!permissions.readOnly)
                TemplatesButton(
                  channelId: widget.channelId,
                  canSave: composer.text.text.trim().isNotEmpty,
                  onChoice: _template,
                ),
              Expanded(
                child: Focus(
                  canRequestFocus: false,
                  skipTraversal: true,
                  onFocusChange: composer.setFocused,
                  child: DsComposer(
                    key: const ValueKey('composer'),
                    targetName: widget.targetName,
                    maxLines: config.composerMaxLines,
                    enterSends: ref.watch(personalSettingsProvider).enterSends,
                    controller: composer.text,
                    onChanged: composer.onChanged,
                    onSend: (text) => unawaited(composer.send(text)),
                    onEditLast: widget.onEditLast,
                    offline: offline,
                    readOnly: permissions.readOnly,
                    typingNames: typing,
                    suggestions: [
                      for (final p in composer.suggestions)
                        DsSuggestion(
                          id: p.id.value,
                          command: '@${p.handle}',
                          description: p.displayName,
                        ),
                    ],
                    onAcceptSuggestion: (s) {
                      final p = principals[PrincipalId.parse(s.id)];
                      if (p != null) composer.acceptSuggestion(p);
                    },
                    onDismissSuggestions: composer.dismissSuggestions,
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
