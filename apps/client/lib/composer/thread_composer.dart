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
import '../files/attach_controller.dart';
import '../files/attachment_mapping.dart';
import '../files/attachment_tile.dart';
import '../files/providers.dart';
import '../settings/settings_providers.dart';
import '../telemetry/telemetry_providers.dart';
import '../thread/providers.dart';
import '../thread/thread_actions.dart';
import '../thread/typing_line.dart';
import '../voice/voice_button.dart';
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
  AttachController? _attach;
  VoiceController? _voice;
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
      deviceId: ref.read(signedInServicesProvider)?.auth.session?.deviceId,
      measurements: ref.read(appMeasurementsProvider),
    )..addListener(_changed);
    final composer = _composer!;
    // M4-22: attachments and voice messages of this composer.
    final ports = ref.read(filePortsProvider);
    final attach = _attach = AttachController(
      thread: widget.threadId,
      channel: widget.channelId,
      outbox: engine.outbox,
      progress: ref.read(uploadProgressProvider),
      newKey: ref.read(attachmentKeysProvider),
    )..addListener(_changed);
    composer.attach = attach;
    _voice = VoiceController(
      recorder: ports.recorder,
      bitRate: ref.read(appConfigProvider).voiceBitrate,
      scratch: () async => ref.read(outgoingFolderProvider),
      onSend: (file) async {
        await sendVoice(engine.outbox, attach, widget.threadId, file);
      },
    )..addListener(_changed);
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
    _attach?.removeListener(_changed);
    _attach?.dispose();
    _voice?.removeListener(_changed);
    _voice?.dispose();
    super.dispose();
  }

  Future<void> _pick() async {
    final files = await ref.read(filePortsProvider).picker.pickFiles();
    await _attach?.add(files);
  }

  /// The chips of the attached files with the state of their upload entries (12 2).
  Widget _chips(AttachController attach) {
    final outbox =
        ref.watch(threadOutboxProvider(widget.threadId.value)).value ??
        const <OutboxEntryView>[];
    final progress = ref.watch(uploadProgressProvider);
    final l = AppLocalizations.of(context);
    return Padding(
      key: const ValueKey('composer.attachments'),
      padding: const EdgeInsets.only(bottom: DsSpace.s4),
      child: Semantics(
        container: true,
        label: l.composerAttachments(attach.attachments.length),
        child: ListenableBuilder(
          listenable: progress,
          builder: (context, _) => Wrap(
            spacing: DsSpace.s6,
            runSpacing: DsSpace.s6,
            children: [
              for (final a in attach.attachments)
                () {
                  OutboxEntryView? view;
                  for (final v in outbox) {
                    if (v.id == a.entryId) view = v;
                  }
                  final code = view?.failureCode;
                  return UploadChip(
                    attachmentKey: a.key,
                    name: a.name,
                    sizeBytes: a.sizeBytes,
                    // A confirmed upload left the outbox: the server checks the file now.
                    state: view == null
                        ? AttachmentState.checking
                        : stateOfUpload(view),
                    percent: ((progress.fractionOf(a.key) ?? 0) * 100).round(),
                    reason: code == null
                        ? null
                        : uploadFailureText(
                            l,
                            code,
                            reason: view?.failureReason,
                          ),
                    onRemove: () => unawaited(attach.remove(a.key)),
                    onRetry: () => unawaited(attach.retry(a.key)),
                  );
                }(),
            ],
          ),
        ),
      ),
    );
  }

  /// [child] with the drop area and the paste key of [attach] (brief M4-22 Scope).
  Widget _withFiles(AttachController? attach, Widget child) {
    if (attach == null) return child;
    return AttachDropTarget(
      controller: attach,
      child: AttachPasteShortcut(
        controller: attach,
        paste: ref.read(filePortsProvider).paste,
        scratch: () async => ref.read(outgoingFolderProvider),
        child: child,
      ),
    );
  }

  String? _refusalText(AppLocalizations l, AttachRefusal? refusal) {
    final locale = Localizations.localeOf(context).languageCode;
    return switch (refusal) {
      null => null,
      FileTooLarge(:final name, :final limitBytes) => l.composerFileTooLarge(
        dsIsolate(name),
        fileSizeLabel(l, limitBytes, locale: locale),
      ),
      TooManyFiles(:final limit) => l.composerTooManyFiles(limit),
      FileUnreadable(:final name) => l.composerFileUnreadable(dsIsolate(name)),
    };
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
    final limits = ref.watch(instanceLimitsProvider).value;
    composer
      ..principals = principals
      ..limits = limits;
    final attach = _attach;
    final voice = _voice;
    final maxVoice = limits?.voiceMaxDurationMs;
    voice?.limit = maxVoice == null ? null : Duration(milliseconds: maxVoice);
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
          if (_refusalText(l, attach?.refusal) case final refused?)
            note(
              const ValueKey('composer.attachError'),
              DsTone.danger,
              refused,
            ),
          if (attach != null && attach.dragging)
            note(
              const ValueKey('composer.dropHint'),
              DsTone.info,
              l.composerDropFiles,
            ),
          if (attach != null && attach.attachments.isNotEmpty) _chips(attach),
          if (voice != null) VoiceRecordingBar(controller: voice),
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
                child: _withFiles(
                  attach,
                  Focus(
                    canRequestFocus: false,
                    skipTraversal: true,
                    onFocusChange: composer.setFocused,
                    child: DsComposer(
                      key: const ValueKey('composer'),
                      targetName: widget.targetName,
                      maxLines: config.composerMaxLines,
                      enterSends: ref
                          .watch(personalSettingsProvider)
                          .enterSends,
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
                      // M4-22: the attachment button, the microphone, and sending a message
                      // of attachments only.
                      onAttach: attach == null || permissions.readOnly
                          ? null
                          : () => unawaited(_pick()),
                      onVoice: voice == null || permissions.readOnly
                          ? null
                          : () => unawaited(voice.start()),
                      hasAttachments: attach?.attachments.isNotEmpty ?? false,
                    ),
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
