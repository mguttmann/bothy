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

import '../agents/agent_typing.dart';
import '../agents/commands/autocomplete_client.dart';
import '../agents/commands/builtin_commands.dart';
import '../agents/commands/command_invoke.dart';
import '../agents/commands/command_options.dart';
import '../agents/commands/command_suggestions.dart';
import '../agents/followups.dart';
import '../agents/providers.dart';
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../files/attach_controller.dart';
import '../files/attachment_mapping.dart';
import '../files/attachment_tile.dart';
import '../files/providers.dart';
import '../settings/settings_providers.dart';
import '../sync/agent_ops.dart';
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
    this.operatorId,
    this.builtins = const [],
    this.onBuiltin,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel.
  final ChannelId channelId;

  /// The channel or thread name of the placeholder (a foreign name).
  final String targetName;

  /// Arrow up in the empty field: edit the last own message (01 7).
  final VoidCallback onEditLast;

  /// The operator agent of the thread, whose commands come first (01 D5), or null.
  final PrincipalId? operatorId;

  /// The built-ins of 09 10.1 the server's answers allow here (M4-21).
  final List<BuiltinCommand> builtins;

  /// Runs a built-in with the text after its name; null offers none.
  final void Function(BuiltinCommand command, String argument)? onBuiltin;

  @override
  ConsumerState<ThreadComposer> createState() => ThreadComposerState();
}

/// State of [ThreadComposer]; the history calls [startReply] for the action Reply.
class ThreadComposerState extends ConsumerState<ThreadComposer> {
  ComposerController? _composer;
  AttachController? _attach;
  VoiceController? _voice;
  AutocompleteClient? _autocomplete;
  bool _overwritten = false;

  /// The slash suggestions were dismissed with Escape for the text as it is now.
  String? _slashDismissed;

  /// A built-in that is not available here and was sent; the text stays (09 10.1).
  BuiltinCommand? _blocked;

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
    final attach = _attach = ref.read(
      composerAttachmentsProvider(
        widget.threadId.value,
        widget.channelId.value,
      ),
    )!..addListener(_changed);
    composer.attach = attach;
    _voice = VoiceController(
      recorder: ports.recorder,
      bitRate: ref.read(appConfigProvider).voiceBitrate,
      scratch: () async => ref.read(outgoingFolderProvider),
      onSend: (file) async {
        await sendVoice(engine.outbox, attach, widget.threadId, file);
      },
    )..addListener(_changed);
    // M4-21: the option suggestions of the agent, throttled by `ui.autocomplete_debounce`.
    _autocomplete = AutocompleteClient(
      debounce: ref.read(appConfigProvider).autocompleteDebounce,
      fetch: ref.read(agentReadsProvider).autocomplete,
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

  void _changed() => setState(() {
    if (_blocked != null &&
        !(_composer?.text.text.startsWith('/${_blocked!.name}') ?? false)) {
      _blocked = null;
    }
  });

  @override
  void dispose() {
    _fieldGroup.dispose();
    _composer?.removeListener(_changed);
    _composer?.dispose();
    // The attachments outlive the composer (composerAttachmentsProvider).
    _attach?.removeListener(_changed);
    _voice?.removeListener(_changed);
    _voice?.dispose();
    _autocomplete?.removeListener(_changed);
    _autocomplete?.dispose();
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

  /// The group around the composer's field and buttons (M4-24: "Focus the input").
  final FocusNode _fieldGroup = FocusNode(
    debugLabel: 'composer.group',
    canRequestFocus: false,
    skipTraversal: true,
  );

  /// Puts the focus into the text field (01 7: Cmd+L or Ctrl+L, Esc then Tab); returns
  /// whether there is a field (a read-only channel has none).
  ///
  /// The field is the focusable node below the composer that belongs to an [EditableText];
  /// the buttons beside it are not.
  bool focusInput() {
    for (final node in _fieldGroup.descendants) {
      final context = node.context;
      if (context == null || !node.canRequestFocus) continue;
      if (context.widget is EditableText ||
          context.findAncestorWidgetOfExactType<EditableText>() != null) {
        node.requestFocus();
        return true;
      }
    }
    return false;
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

  /// The slash state of the field (01 D5; brief M4-21 Design note 4): the suggestions while
  /// the first word starting with "/" is typed, the chosen command with its options after
  /// it, and the option suggestions of the agent for an option with `autocomplete`.
  _Slash _slash(
    ComposerController composer,
    List<Command> commands,
    Map<PrincipalId, Principal> principals,
    AppLocalizations l,
  ) {
    final value = composer.text.value;
    final text = value.text;
    final cursor = value.selection.isValid
        ? value.selection.baseOffset
        : text.length;
    final query = slashQuery(text, cursor);
    if (query != null) {
      final items = _slashDismissed == text
          ? const <CommandChoiceItem>[]
          : commandSuggestions(
              query,
              builtins: widget.builtins,
              commands: commands,
              operatorId: widget.operatorId,
              principals: principals,
            );
      return _Slash(
        suggestions: [for (final i in items) suggestionOf(l, i, principals)],
      );
    }
    final parsed = slashCommandOf(text);
    if (parsed == null) return const _Slash();
    final builtin = BuiltinCommand.byName(parsed.token);
    if (builtin == BuiltinCommand.title && widget.builtins.contains(builtin)) {
      // `/title <text>`: the title is its one required option (09 10.1).
      return _Slash(
        command: DsComposerCommand(
          name: '/${builtin!.name}',
          options: [
            DsCommandOption(
              name: builtin.name,
              required: true,
              value: parsed.rest.trim(),
            ),
          ],
        ),
      );
    }
    final command = commandOfToken(
      parsed.token,
      commands: commands,
      operatorId: widget.operatorId,
      principals: principals,
    );
    if (command == null) return const _Slash();
    final active = activeOption(command, parsed.rest);
    return _Slash(
      command: composerCommand(command, parsed.rest),
      registered: command,
      rest: parsed.rest,
      autocomplete: active != null && active.option.autocomplete
          ? (
              channel: widget.channelId,
              command: command.id,
              thread: widget.threadId,
              option: active.option.name,
              value: active.value,
              options: [
                for (final o in invokeOptions(command, parsed.rest))
                  if (o.name != active.option.name) o,
              ],
            )
          : null,
      activeOption: active?.option,
    );
  }

  void _acceptSlash(
    ComposerController composer,
    _Slash slash,
    DsSuggestion s,
    Map<PrincipalId, Principal> principals,
  ) {
    final String next;
    if (s.id.startsWith('choice.')) {
      final command = slash.registered;
      final option = slash.activeOption;
      if (command == null || option == null) return;
      final value = s.id.substring('choice.'.length);
      next =
          '/${slashCommandOf(composer.text.text)!.token} '
          '${withOptionValue(command, slash.rest, option, value)}';
      _autocomplete?.clear();
    } else if (s.id.startsWith('builtin.') || s.id.startsWith('command.')) {
      next = '${s.command} ';
    } else {
      final p = principals[PrincipalId.parse(s.id)];
      if (p != null) composer.acceptSuggestion(p);
      return;
    }
    composer.text.value = TextEditingValue(
      text: next,
      selection: TextSelection.collapsed(offset: next.length),
    );
    composer.onChanged(next);
  }

  void _send(
    ComposerController composer,
    String text,
    List<Command> commands,
    Map<PrincipalId, Principal> principals,
  ) {
    final outcome = resolveCommand(
      text,
      builtins: widget.builtins,
      commands: commands,
      operatorId: widget.operatorId,
      principals: principals,
    );
    switch (outcome) {
      case SendBuiltin(:final command, :final argument):
        // Built-ins never go through invokeCommand (09 10.1).
        widget.onBuiltin?.call(command, argument);
        composer.clearAfterCommand();
      case SendRegistered(:final command, :final options):
        final engine = ref.read(syncEngineProvider);
        if (engine == null) return;
        unawaited(
          AgentOps.enqueueCommand(
            engine,
            channel: widget.channelId,
            command: command.id,
            thread: widget.threadId,
            options: options,
          ),
        );
        _autocomplete?.clear();
        composer.clearAfterCommand();
      case BlockedBuiltin(:final command):
        setState(() => _blocked = command);
      case SendMessage():
        unawaited(composer.send(text));
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
    // M4-21: the follow-up form of the placeholder, the agent's line from the run state
    // (D-L9) and the slash commands of the channel.
    final runs = ref.watch(threadRunsProvider(thread)).value ?? const [];
    final followup = followupAgentName(runs, principals);
    final working = workingAgentNames(runs, principals);
    final commands =
        ref.watch(channelCommandsProvider(channel)).value ?? const [];
    final slash = _slash(composer, commands, principals, l);
    final autocomplete = _autocomplete;
    final ask = slash.autocomplete;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || autocomplete == null) return;
      if (ask == null) {
        autocomplete.clear();
      } else {
        autocomplete.query(ask);
      }
    });
    final choices = ask == null
        ? const <DsSuggestion>[]
        : [
            for (final c in autocomplete?.choices ?? const <CommandChoice>[])
              DsSuggestion(
                id: 'choice.${c.value}',
                command: c.name,
                description: c.value == c.name ? null : c.value,
              ),
          ];
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
          if (_blocked case final blocked?)
            note(
              const ValueKey('composer.commandBlocked'),
              DsTone.warning,
              l.commandNotAvailable('/${blocked.name}'),
              onClose: () => setState(() => _blocked = null),
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
          for (final name in working)
            Padding(
              key: ValueKey('composer.agentWorking.$name'),
              padding: const EdgeInsets.only(bottom: DsSpace.s4),
              child: Semantics(
                liveRegion: true,
                child: Text(
                  l.agentWorking(dsIsolate(name)),
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: colors.textSecondary),
                ),
              ),
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
                child: _withFiles(
                  attach,
                  Focus(
                    focusNode: _fieldGroup,
                    canRequestFocus: false,
                    skipTraversal: true,
                    onFocusChange: composer.setFocused,
                    child: DsComposer(
                      key: const ValueKey('composer'),
                      targetName: widget.targetName,
                      followupAgentName: followup,
                      command: slash.command,
                      maxLines: config.composerMaxLines,
                      enterSends: ref
                          .watch(personalSettingsProvider)
                          .enterSends,
                      controller: composer.text,
                      onChanged: composer.onChanged,
                      onSend: (text) =>
                          _send(composer, text, commands, principals),
                      onEditLast: widget.onEditLast,
                      offline: offline,
                      readOnly: permissions.readOnly,
                      typingNames: typing,
                      suggestions: composer.suggestions.isNotEmpty
                          ? [
                              for (final p in composer.suggestions)
                                DsSuggestion(
                                  id: p.id.value,
                                  command: '@${p.handle}',
                                  description: p.displayName,
                                ),
                            ]
                          : slash.suggestions.isNotEmpty
                          ? slash.suggestions
                          : choices,
                      onAcceptSuggestion: (s) =>
                          _acceptSlash(composer, slash, s, principals),
                      onDismissSuggestions: () {
                        composer.dismissSuggestions();
                        autocomplete?.clear();
                        setState(() => _slashDismissed = composer.text.text);
                      },
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

/// The slash state of the composer in one build.
final class _Slash {
  const _Slash({
    this.suggestions = const [],
    this.command,
    this.registered,
    this.rest = '',
    this.autocomplete,
    this.activeOption,
  });

  /// The command suggestions while the first word is typed.
  final List<DsSuggestion> suggestions;

  /// The chosen command with its options, or null.
  final DsComposerCommand? command;

  /// The chosen registered command, or null.
  final Command? registered;

  /// The text after the command's name.
  final String rest;

  /// The request for the option suggestions of the agent, or null.
  final AutocompleteRequest? autocomplete;

  /// The option being typed, or null.
  final CommandOption? activeOption;
}
