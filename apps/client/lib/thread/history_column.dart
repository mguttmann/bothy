// SPDX-License-Identifier: Apache-2.0

/// The history column (brief M4-20 Design notes 1 to 6; 01 5.1 D1 and D2, 6, 7): the header,
/// the messages of the local database with the outbox after them, paging and the jump to a
/// message, focus and read state, the message actions, the composer, and "No access anymore"
/// when the thread or its channel is removed.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../agents/commands/builtin_commands.dart';
import '../agents/commands/command_loader.dart';
import '../agents/commands/thread_command_runner.dart';
import '../agents/component_actions.dart';
import '../agents/followups.dart';
import '../agents/live_box_slot.dart';
import '../agents/message_card.dart';
import '../agents/providers.dart';
import '../agents/run_line.dart';
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../composer/thread_composer.dart';
import '../files/attachment_tile.dart';
import '../files/providers.dart';
import '../files/upload_executor.dart';
import '../forum/forum_model.dart' show timeLabel;
import '../forum/providers.dart'
    show
        channelColumnHiddenProvider,
        overviewPinnedProvider,
        toggleChannelColumn;
import '../keyboard/app_shortcuts.dart';
import '../keyboard/intents.dart';
import '../menus/thread_menu.dart';
import '../running/providers.dart' show mayStop;
import '../search/thread_search.dart';
import '../session_column/session_column.dart';
import '../sidebar/chat_seen_marker.dart';
import '../sidebar/providers.dart';
import '../telemetry/telemetry_providers.dart';
import 'delete_confirm.dart';
import 'history_model.dart';
import 'message_list.dart';
import 'message_menu.dart';
import 'message_tile.dart';
import 'move_thread_dialog.dart';
import 'outbox_rows.dart';
import 'providers.dart';
import 'reaction_picker.dart';
import 'revisions_dialog.dart';
import 'thread_actions.dart';
import 'thread_header.dart';
import 'title_editor.dart';
import 'versions_dialog.dart';

/// The history of [threadId]; [jumpTo] brings a message into view and highlights it
/// (`/t/{thread_id}?m={message_id}`, 15 11).
class HistoryColumn extends ConsumerStatefulWidget {
  /// Creates the column.
  const HistoryColumn({
    super.key,
    required this.threadId,
    this.jumpTo,
    this.besideMain = false,
  });

  /// The thread.
  final ThreadId threadId;

  /// The message to jump to, or null.
  final MessageId? jumpTo;

  /// Whether the column is part of D2 (a thread of a text channel beside the main history),
  /// where the session column is collapsed until the person shows it.
  final bool besideMain;

  @override
  ConsumerState<HistoryColumn> createState() => HistoryColumnState();
}

/// State of [HistoryColumn]: paging, the jump, the read position written last, the message
/// being edited and the removal.
class HistoryColumnState extends ConsumerState<HistoryColumn> {
  final GlobalKey<ThreadComposerState> _composer = GlobalKey();
  late final VisibleThreads _visible;
  bool _loadingOlder = false;
  bool _threadRequested = false;
  bool _jumpLoaded = false;
  bool _newestVisible = true;
  ThreadSeq? _markedSeq;
  MessageId? _editing;
  String? _editingEntry;

  /// Esc was the last key in the history: a Tab now focuses the input (01 7 "Esc then Tab").
  bool _escArmed = false;

  /// The interactions of the cards of this thread (M4-21).
  ComponentActions? _cards;

  /// The channel whose commands were loaded (M4-21: once per opened channel, 15 6.1).
  ChannelId? _commandsLoaded;
  ChannelId? _startersLoaded;

  /// The thread as last stored, so a removal that deleted it is still recognized.
  Thread? _lastThread;

  /// The space of [_lastThread], kept for the same reason.
  String? _lastSpace;

  /// The texts of the waiting messages when access was removed (01 6.6), or null while the
  /// thread is visible.
  List<(String, String)>? _removed;

  String get _thread => widget.threadId.value;

  @override
  void initState() {
    super.initState();
    _visible = ref.read(visibleThreadsProvider.notifier)..add(widget.threadId);
    // `client.thread_open.duration` runs until the first messages show (brief M4-31).
    ref.read(appMeasurementsProvider).threadOpening(_thread);
  }

  @override
  void dispose() {
    _visible.remove(widget.threadId);
    super.dispose();
  }

  ThreadActions? get _actions {
    final engine = ref.read(syncEngineProvider);
    final db = ref.read(localDatabaseProvider);
    return engine == null
        ? null
        : ThreadActions(engine, outboxDao: db?.outboxDao);
  }

  /// Loads the thread of this column, which the local database does not hold (15 11). A
  /// refused or failed read leaves the column empty; the target check of the route shows
  /// "Not found or no access" when the server answers so.
  Future<void> _loadThread() async {
    try {
      await ref.read(threadReadsProvider).loadThread(widget.threadId);
    } on Object {
      // Offline or refused: nothing to show yet.
    }
  }

  // ---- paging, jump and read state (Design notes 1 and 2) ------------------------------

  Future<void> _loadOlder(ContiguousWindow window, Thread? thread) async {
    if (_loadingOlder || !mounted) return;
    final first = window.messages.isEmpty ? null : window.messages.first;
    final limit = ref.read(historyLimitProvider(_thread));
    if (window.messages.length >= limit) {
      // Stored and loaded but cut by the limit: show more without reading.
      ref
          .read(historyLimitProvider(_thread).notifier)
          .atLeast(limit + historyPageSize);
      return;
    }
    final ThreadSeq before;
    if (first != null) {
      if (window.gapBefore == null) return;
      before = first.threadSeq;
    } else {
      final last = thread?.lastMessageSeq;
      if (last == null || last.value == 0) return;
      before = last.next;
    }
    _loadingOlder = true;
    try {
      await ref.read(threadReadsProvider).loadOlder(widget.threadId, before);
      if (!mounted) return;
      ref
          .read(historyLimitProvider(_thread).notifier)
          .atLeast(window.messages.length + historyPageSize);
    } on Object {
      // Offline or refused: the gap row stays and the next scroll tries again.
    } finally {
      _loadingOlder = false;
    }
  }

  Future<void> _jump(ContiguousWindow window) async {
    final target = widget.jumpTo;
    if (target == null || _jumpLoaded) return;
    if (window.messages.any((m) => m.id == target)) {
      _jumpLoaded = true;
      return;
    }
    if (window.messages.isEmpty) return;
    _jumpLoaded = true;
    final reads = ref.read(threadReadsProvider);
    try {
      final seq = await reads.positionOf(target);
      final first = window.messages.first.threadSeq;
      if (seq == null || !mounted || seq >= first) return;
      await reads.loadRange(widget.threadId, seq, ThreadSeq(first.value - 1));
      if (!mounted) return;
      ref
          .read(historyLimitProvider(_thread).notifier)
          .atLeast(window.messages.last.threadSeq.value - seq.value + 1);
    } on Object {
      // The message stays out of view; the history shows the newest messages.
    }
  }

  /// Marks the thread read up to its newest message while that is on screen and the window
  /// is focused, at most once per newest message; the server keeps the maximum (Design
  /// note 2, 04 14).
  void _markRead(ContiguousWindow window, {bool force = false}) {
    if (window.messages.isEmpty || window.gapAfter != null) return;
    final focused = ref.read(windowFocusedProvider).value ?? false;
    if (!force && (!_newestVisible || !focused)) return;
    final newest = window.messages.last.threadSeq;
    final read = ref.read(threadReadStateProvider(_thread))?.lastReadSeq;
    // Esc compares with the stored position only: after "Mark as unread" it marks read again.
    final done = force ? read : _markedSeq ?? read;
    if (done != null && done >= newest) return;
    _markedSeq = newest;
    unawaited(_actions?.markRead(widget.threadId, newest));
  }

  // ---- removal (Design note 6) ---------------------------------------------------------

  void _onNotice(SyncNotice? notice) {
    if (notice is! TargetsRemoved || _removed != null) return;
    final channel = _lastThread?.channelId.value;
    final space = _lastSpace;
    final hit = notice.targets.any(
      (t) =>
          t.id == _thread ||
          (channel != null && t.id == channel) ||
          (space != null && t.id == space),
    );
    if (!hit) return;
    final waiting = ref.read(threadOutboxProvider(_thread)).value ?? const [];
    setState(
      () => _removed = [
        for (final v in waiting)
          if (v.entry.kind == MessageOps.createMessage)
            (v.id, v.payload['content_md'] as String? ?? ''),
      ],
    );
  }

  // ---- actions (Design note 4) ---------------------------------------------------------

  Future<void> _onAction(
    MessageAction action,
    Message message,
    Thread thread,
    Map<PrincipalId, Principal> principals,
  ) async {
    final actions = _actions;
    if (actions == null) return;
    final l = AppLocalizations.of(context);
    switch (action) {
      case MessageAction.reply:
        _composer.currentState?.startReply(
          message,
          principals[message.authorId]?.displayName ?? '',
        );
      case MessageAction.startThread:
        await actions.startThread(thread.channelId, message);
      case MessageAction.react:
        final existing = await ref.read(
          messageReactionsProvider(message.id.value).future,
        );
        if (!mounted) return;
        final choice = await pickReaction(context, existing);
        if (choice != null) {
          await actions.react(message, choice.emoji, remove: choice.remove);
        }
      case MessageAction.pin || MessageAction.unpin:
        await actions.pin(message, pinned: action == MessageAction.pin);
      case MessageAction.edit:
        setState(() => _editing = message.id);
      case MessageAction.versions:
        await showVersions(context, ref.read(threadReadsProvider), message);
      case MessageAction.delete:
        if (await confirmDelete(context)) await actions.delete(message);
      case MessageAction.markUnread:
        // The view does not mark the thread read again until a newer message arrives
        // (Design note 2: at most once per newest message).
        final newest = thread.lastMessageSeq;
        final marked = _markedSeq;
        if (marked == null || marked < newest) _markedSeq = newest;
        await actions.markUnread(
          message,
          ref.read(threadReadStateProvider(_thread)),
        );
      case MessageAction.copyText:
        await Clipboard.setData(ClipboardData(text: message.contentMd));
        _copied(l);
      case MessageAction.copyLink:
        final scheme = ref.read(appPortsProvider).branding.urlScheme;
        await Clipboard.setData(
          ClipboardData(
            text: '$scheme://t/${thread.id.value}?m=${message.id.value}',
          ),
        );
        _copied(l);
      case MessageAction.copySelection || MessageAction.selectText:
        // The tile runs them: they read what it shows, not the stored message (01 6.15).
        break;
    }
  }

  void _copied(AppLocalizations l) {
    if (!mounted) return;
    ScaffoldMessenger.maybeOf(context)
        ?.showSnackBar(SnackBar(content: Text(l.messageCopied)));
  }

  void _editLast(ContiguousWindow window) {
    final last = lastOwnMessage(window, ref.read(myPrincipalIdProvider));
    if (last != null) setState(() => _editing = last.id);
  }

  void _openLink(Uri uri) {
    // Internal paths of 15 11 open in the app; everything else in the system browser. The
    // renderer hands over only links it classified as active (SEC-OUT-1).
    if (uri.scheme.isEmpty) {
      context.go(uri.toString());
    } else {
      unawaited(ref.read(appPortsProvider).browser.open(uri));
    }
  }

  Future<void> _move(Thread thread, Channel? channel) async {
    final operator =
        thread.operatorAgentId ?? channel?.operator.effectiveAgentId;
    final targets = moveTargets(
      ref.read(visibleChannelsProvider),
      from: thread.channelId,
      operator: operator,
    );
    final choice = await showMoveThread(context, targets);
    if (choice != null) {
      await _actions?.move(thread, choice.target, choice.reason);
    }
  }

  Future<void> _title(Thread thread) async {
    final limits = ref.read(instanceLimitsProvider).value;
    final title = await editTitle(
      context,
      thread.title,
      maxChars: limits?.threadTitleMaxChars,
    );
    if (title != null) await _actions?.setTitle(thread, title);
  }

  Future<void> _compare(String id, String mine, String theirs) async {
    final choice = await showConflict(context, mine: mine, theirs: theirs);
    final actions = _actions;
    if (choice == null || actions == null) return;
    switch (choice) {
      case ConflictChoice.keepMine:
        await actions.keepMine(id, mine);
      case ConflictChoice.takeTheirs:
        await actions.discard(id);
    }
  }

  // ---- agents (M4-21) -----------------------------------------------------------------

  /// Loads the commands of [channel] once while the column shows it, so the slash
  /// suggestions work offline (brief M4-21 Scope).
  void _loadCommands(ChannelId channel) {
    if (_commandsLoaded == channel) return;
    _commandsLoaded = channel;
    final db = ref.read(localDatabaseProvider);
    if (db == null) return;
    unawaited(
      loadChannelCommands(db, ref.read(agentReadsProvider).commands, channel),
    );
  }

  /// Loads the newest threads of [channel] once while its main history shows, so the starter
  /// lines also appear for threads created before this device's first sync.
  void _loadStarters(ChannelId channel) {
    if (_startersLoaded == channel) return;
    _startersLoaded = channel;
    unawaited(
      ref
          .read(threadReadsProvider)
          .loadChannelThreads(channel)
          .then<void>(
            (_) {},
            // Offline or refused: the lines come with the next thread events.
            onError: (Object _) {},
          ),
    );
  }

  ComponentActions? get _cardActions {
    final engine = ref.read(syncEngineProvider);
    if (engine == null) return null;
    final held = _cards;
    if (held != null && held.engine == engine) return held;
    return _cards = ComponentActions(engine);
  }

  /// The session column of the thread as a sheet (the phone, 01 5.3).
  Future<void> _openSession() => showModalBottomSheet<void>(
    context: context,
    useSafeArea: true,
    isScrollControlled: true,
    builder: (_) => SessionColumn(threadId: widget.threadId),
  );

  // ---- keyboard (M4-24) ---------------------------------------------------------------

  /// Esc in the history marks the thread read; Esc then Tab focuses the input (01 7). Keys
  /// that the focused item handles itself (the composer's suggestions, the inline editor)
  /// never arrive here.
  KeyEventResult _onKey(KeyEvent event, ContiguousWindow window) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;
    final keys = HardwareKeyboard.instance;
    final plain =
        !keys.isMetaPressed &&
        !keys.isControlPressed &&
        !keys.isAltPressed &&
        !keys.isShiftPressed;
    final key = event.logicalKey;
    final armed = _escArmed;
    _escArmed = false;
    if (plain && key == LogicalKeyboardKey.escape) {
      _markRead(window, force: true);
      _escArmed = true;
      return KeyEventResult.handled;
    }
    if (plain && armed && key == LogicalKeyboardKey.tab) {
      return _focusInput() ? KeyEventResult.handled : KeyEventResult.ignored;
    }
    return KeyEventResult.ignored;
  }

  bool _focusInput() => _composer.currentState?.focusInput() ?? false;

  // ---- build -------------------------------------------------------------------------

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final stored = ref.watch(storedThreadProvider(_thread));
    final thread = stored.value;
    ref.listen(syncNoticesProvider, (_, next) => _onNotice(next.value));
    if (thread != null) _lastThread = thread;
    final removed = _removed;
    if (removed != null) return _NoAccess(waiting: removed);
    if (thread == null) {
      // 15 11: a thread this device never stored is loaded through REST, once per column.
      if (stored.hasValue && !_threadRequested) {
        _threadRequested = true;
        unawaited(_loadThread());
      }
      return const SizedBox.expand();
    }
    final channelId = thread.channelId.value;
    final channel = ref.watch(storedChannelOfRouteProvider(channelId)).value;
    _loadCommands(thread.channelId);
    _lastSpace = channel?.spaceId?.value ?? _lastSpace;
    final config = ref.watch(appConfigProvider);
    final windowValue = ref.watch(threadWindowProvider(_thread));
    final window = windowValue.value ?? const ContiguousWindow(messages: []);
    if (window.messages.isNotEmpty) {
      ref.read(appMeasurementsProvider).threadShown(_thread);
    }
    final outbox = ref.watch(threadOutboxProvider(_thread)).value ?? const [];
    // The upload entries by attachment key, for the files of a waiting message (M4-22).
    final uploads = {
      for (final v in outbox)
        if (v.entry.kind == UploadOps.upload)
          v.payload['attachment_key']! as String: v,
    };
    final progress = ref.watch(uploadProgressProvider);
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    final me = ref.watch(myPrincipalIdProvider);
    final permissions = ref.watch(threadPermissionsProvider(channelId));
    // Kept alive while the column shows: the read state decides "Mark as unread" and the
    // read position written next (Design note 2).
    ref.watch(threadReadStateProvider(_thread));
    // The move targets are listed from the sidebar's channels; kept while Move is offered.
    if (permissions.canManage) ref.watch(visibleChannelsProvider);
    final main = thread.kind == ThreadKind.main;
    if (main) _loadStarters(thread.channelId);
    final textLayout = widget.besideMain;
    // M4-42: a thread beside the main history of its text channel, or a forum post beside the
    // pinned overview, offers to hide that channel column (01 D1, D2).
    final channelColumn =
        !main &&
        (textLayout ||
            (channel?.kind == ChannelKind.forum &&
                (ref.watch(overviewPinnedProvider).value ?? false)));
    final channelHidden =
        channelColumn &&
        (ref.watch(channelColumnHiddenProvider).value ?? false);
    final sessionHidden =
        ref.watch(sessionColumnHiddenProvider(textThread: textLayout)).value ??
        textLayout;
    final starters = main
        ? ref.watch(threadsOfChannelProvider(channelId)).value ?? const []
        : const <Thread>[];
    ref.listen(windowFocusedProvider, (_, _) => _markRead(window));
    final items = buildHistory(
      window: window,
      outbox: outbox,
      groupWindow: config.messageGroupWindow,
      threads: starters,
    );
    final ports = ref.watch(appPortsProvider);
    final phone = ports.formFactor.isPhone;
    final now = ports.clock.now();
    final format = L10nFormat(Localizations.localeOf(context));
    String time(DateTime at) => timeLabel(at, now: now, format: format);
    final operatorId =
        thread.operatorAgentId ?? channel?.operator.effectiveAgentId;
    // M4-21: the built-ins the server's answers allow here (09 10.1).
    final access = ref.watch(channelAccessProvider(channelId)).value;
    final activeRun = newestActiveRun(
      ref.watch(threadRunsProvider(_thread)).value ?? const [],
    );
    final canEditTitle =
        permissions.canWrite &&
        (thread.createdBy == me || permissions.canManage);
    final builtins = availableBuiltins(
      builtinAvailability(
        thread: thread,
        activeRun: activeRun,
        canStop: activeRun != null && mayStop(activeRun, access, me),
        triggers: access?.level == AccessLevel.trigger && !permissions.readOnly,
        canEditTitle: canEditTitle && !main,
        canWrite: permissions.canWrite,
      ),
    );
    final header = ThreadHeaderData(
      title: main ? channel?.name ?? '' : thread.title,
      automaticTitle: thread.titleSource == TitleSource.auto,
      operatorName: operatorId == null
          ? null
          : principals[operatorId]?.displayName ?? operatorId.value,
      mainHistory: main,
      textChannel: main && channel?.kind == ChannelKind.text,
      done: thread.state == ThreadState.closed,
      canEditTitle: canEditTitle,
      canSetDone: permissions.canWrite,
      canMove: permissions.canManage,
    );
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      unawaited(_jump(window));
      _markRead(window);
      // A thread never opened on this device has no loaded range yet: its newest page.
      if (windowValue.hasValue &&
          window.messages.isEmpty &&
          thread.lastMessageSeq.value > 0) {
        unawaited(_loadOlder(window, thread));
      }
    });
    final jumpKey = widget.jumpTo == null
        ? null
        : 'message.${widget.jumpTo!.value}';

    Widget row(BuildContext context, HistoryItem item) => switch (item) {
      GapItem(:final before) => _GapRow(before: before),
      // The anchor of a live box (M4-21): the collapsed line once the run ended.
      MessageItem(:final message) when message.kind == MessageKind.run =>
        Padding(
          key: ValueKey(item.key),
          padding: const EdgeInsets.fromLTRB(
            DsSize.avatarMd + DsSpace.s12 + DsSpace.s8,
            DsSpace.s2,
            DsSpace.s8,
            DsSpace.s2,
          ),
          child: RunLine(message: message),
        ),
      MessageItem(:final message, :final continuation, :final started) =>
        _MessageRow(
          key: ValueKey(item.key),
          message: message,
          continuation: continuation,
          started: started,
          principals: principals,
          me: me,
          time: time(message.createdAt),
          collapseLines: config.messageCollapseLines,
          highlighted: item.key == jumpKey,
          actions: messageActions(
            message,
            me: me,
            permissions: permissions,
            mainHistory: main && channel?.kind == ChannelKind.text,
            startedThread: started.isNotEmpty,
          ),
          editing: _editing == message.id
              ? InlineEditor(
                  initial: message.contentMd,
                  onSave: (text) {
                    setState(() => _editing = null);
                    unawaited(_actions?.edit(message, text));
                  },
                  onCancel: () => setState(() => _editing = null),
                )
              : null,
          onAction: (a) => _onAction(a, message, thread, principals),
          onOpenLink: _openLink,
          onOpenThread: (t) => context.go('/t/${t.id.value}'),
          card: message.kind == MessageKind.card && message.deletedAt == null
              ? MessageCard(
                  message: message,
                  actions: _cardActions,
                  onOpenLink: _openLink,
                )
              : null,
        ),
      PendingItem(:final view) => KeyedSubtree(
        key: ValueKey(item.key),
        child: _editingEntry == view.id
            ? InlineEditor(
                initial: view.payload['content_md'] as String? ?? '',
                onSave: (text) {
                  setState(() => _editingEntry = null);
                  unawaited(_actions?.editEntry(view.id, text));
                },
                onCancel: () => setState(() => _editingEntry = null),
              )
            : PendingRow(
                view: view,
                authorName: me == null ? '' : principals[me]?.displayName ?? '',
                timeLabel: time(view.entry.createdAt),
                collapseLines: config.messageCollapseLines,
                onResend: () => unawaited(_actions?.retry(view.id)),
                onEdit: () => setState(() => _editingEntry = view.id),
                onDiscard: () => unawaited(_actions?.discard(view.id)),
                onOpenLink: _openLink,
                uploads: uploads,
                progress: progress,
                onRetryUpload: (id) => unawaited(_actions?.retry(id)),
              ),
      ),
      FailedWriteItem(:final view) => KeyedSubtree(
        key: ValueKey(item.key),
        child: FailedWriteRow(
          view: view,
          onCompare: (mine, theirs) =>
              unawaited(_compare(view.id, mine, theirs)),
          onResend: () => unawaited(_actions?.retry(view.id)),
          onDiscard: () => unawaited(_actions?.discard(view.id)),
        ),
      ),
    };

    // M4-24: the thread's actions for the shortcut layer (01 7); the thread beside the main
    // history (D2) wins over it when neither holds the focus.
    final keyboard = <ShortcutAction, KeyboardHandler>{
      ShortcutAction.markRead: KeyboardHandler(
        () => _markRead(window, force: true),
      ),
      ShortcutAction.focusInput: KeyboardHandler(
        _focusInput,
        enabled: !permissions.readOnly,
      ),
      ShortcutAction.searchInThread: KeyboardHandler(
        () => unawaited(
          showThreadSearch(
            context,
            channelId: thread.channelId,
            threadId: thread.id,
          ),
        ),
      ),
      ShortcutAction.toggleDone: KeyboardHandler(
        () => unawaited(
          _actions?.setDone(thread, reopen: thread.state == ThreadState.closed),
        ),
        enabled: !main && header.canSetDone,
      ),
    };
    // 01 6.17: while the history shows, the chat's agent status is seen on this device.
    return ChatSeenMarker(
      threadId: widget.threadId,
      child: KeyboardActions(
        priority: main ? 0 : 1,
        handlers: keyboard,
        child: Focus(
          canRequestFocus: false,
          skipTraversal: true,
          onKeyEvent: (_, event) => _onKey(event, window),
          child: Column(
            key: ValueKey('history.$_thread'),
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              ThreadHeader(
                data: header,
                // 01 5.3 M3: the phone has no session column; its actions sit in one menu and
                // the session (pins, agent session, costs) opens as a sheet.
                compact: phone,
                onEditTitle: () => unawaited(_title(thread)),
                onDone: () => unawaited(
                  _actions?.setDone(
                    thread,
                    reopen: thread.state == ThreadState.closed,
                  ),
                ),
                onMove: () => unawaited(_move(thread, channel)),
                onPin: phone
                    ? () => unawaited(_openSession())
                    : () => unawaited(
                        showSessionColumn(
                          ref.read(localDatabaseProvider),
                          textThread: textLayout,
                        ),
                      ),
                // M4-45 (01 D1, D2): kept on this device per layout.
                onColumn: () => unawaited(
                  toggleSessionColumn(
                    ref.read(localDatabaseProvider),
                    textThread: textLayout,
                  ),
                ),
                sessionColumnHidden: sessionHidden,
                // M4-42 (01 D1, D2): the channel column beside the thread can be hidden.
                onChannelColumn: channelColumn
                    ? () => unawaited(
                        toggleChannelColumn(ref.read(localDatabaseProvider)),
                      )
                    : null,
                channelColumnHidden: channelHidden,
                // The level of the thread, or of the channel for its main history (M4-33).
                menu: channel == null
                    ? null
                    : TargetMenuButton(
                        channel: channel,
                        thread: main ? null : thread.id,
                      ),
                // FIX-draft-conflict-banner: on the phone the menu of M4-33 joins the header's
                // one overflow menu instead of a second button beside it.
                compactMenu: channel == null
                    ? null
                    : (actions) => TargetMenuButton(
                        channel: channel,
                        thread: main ? null : thread.id,
                        actions: actions,
                      ),
              ),
              // M4-21: the live box of the active run, pinned over the history (01 6.2).
              LiveBoxSlot(
                threadId: widget.threadId,
                channelId: thread.channelId,
                keyboardPriority: main ? 0 : 1,
              ),
              Expanded(
                child: items.isEmpty
                    ? Center(
                        key: const ValueKey('history.empty'),
                        child: Text(
                          l.threadEmpty,
                          style: dsTextStyle(DsType.body)
                              .copyWith(color: context.dsColors.textSecondary),
                        ),
                      )
                    : MessageList(
                        items: items,
                        itemBuilder: row,
                        jumpTo: jumpKey,
                        onNearTop: () => unawaited(_loadOlder(window, thread)),
                        onNewestVisible: (visible) {
                          _newestVisible = visible;
                          if (visible) _markRead(window);
                        },
                      ),
              ),
              ThreadComposer(
                key: _composer,
                threadId: widget.threadId,
                channelId: thread.channelId,
                targetName: main ? channel?.name ?? '' : thread.title,
                onEditLast: () => _editLast(window),
                operatorId: operatorId,
                builtins: builtins,
                onBuiltin: (command, argument) =>
                    runThreadBuiltin(context, ref, thread, command, argument),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// A message row with its reactions and the quote of a reply, read per message.
class _MessageRow extends ConsumerWidget {
  const _MessageRow({
    super.key,
    required this.message,
    required this.continuation,
    required this.started,
    required this.principals,
    required this.me,
    required this.time,
    required this.collapseLines,
    required this.highlighted,
    required this.actions,
    required this.editing,
    required this.onAction,
    required this.onOpenLink,
    required this.onOpenThread,
    this.card,
  });

  final Message message;
  final bool continuation;
  final List<Thread> started;
  final Map<PrincipalId, Principal> principals;
  final PrincipalId? me;
  final String time;
  final int collapseLines;
  final bool highlighted;
  final List<MessageAction> actions;
  final Widget? editing;
  final ValueChanged<MessageAction> onAction;
  final void Function(Uri) onOpenLink;
  final ValueChanged<Thread> onOpenThread;
  final Widget? card;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final reactions =
        ref.watch(messageReactionsProvider(message.id.value)).value ?? const [];
    final replyId = message.replyToMessageId;
    final replied = replyId == null
        ? null
        : ref.watch(storedMessageProvider(replyId.value)).value;
    // The files of the message (M4-22): tiles below the content, and the resolver of the
    // internal image references in it (SEC-OUT-2).
    final files =
        ref.watch(messageFilesProvider(message.id.value)).value ?? const [];
    return MessageTile(
      data: messageData(
        message,
        principals: principals,
        me: me,
        timeLabel: time,
        reactions: reactions,
        replied: replied,
      ),
      continuation: continuation,
      collapseLines: collapseLines,
      actions: actions,
      onAction: onAction,
      onOpenLink: onOpenLink,
      highlighted: highlighted,
      editing: editing,
      attachments: files.isEmpty || message.deletedAt != null
          ? null
          : MessageAttachmentsView(
              messageId: message.id,
              voice: message.kind == MessageKind.voice,
            ),
      resolver: files.isEmpty ? null : attachmentResolver(files),
      components: card,
      longPressMenu: ref.watch(appPortsProvider).formFactor.isPhone,
      started: [
        for (final t in started)
          ThreadStarterLine(thread: t, onOpen: () => onOpenThread(t)),
      ],
    );
  }
}

/// The loading row of a gap (04 4: never messages out of order).
class _GapRow extends StatelessWidget {
  const _GapRow({required this.before});

  final bool before;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Padding(
      key: ValueKey(before ? 'history.gapBefore' : 'history.gapAfter'),
      padding: const EdgeInsets.all(DsSpace.s12),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          SizedBox.square(
            dimension: DsIconSize.sizeSm,
            child: CircularProgressIndicator(
              strokeWidth: DsBorderWidth.focus,
              color: colors.textSecondary,
              value: MediaQuery.of(context).disableAnimations ? 0 : null,
            ),
          ),
          const SizedBox(width: DsSpace.s8),
          Text(
            before ? l.threadLoadingOlder : l.threadLoadingNewer,
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          ),
        ],
      ),
    );
  }
}

/// "No access anymore" with the waiting messages as copyable text (01 6.6; 04 8.3 item 4).
class _NoAccess extends StatelessWidget {
  const _NoAccess({required this.waiting});

  final List<(String, String)> waiting;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Padding(
      key: const ValueKey('thread.noAccess'),
      padding: const EdgeInsets.all(DsSpace.s24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          DsBanner(
            tone: DsTone.warning,
            text: l.threadNoAccess,
            liveRegion: true,
          ),
          if (waiting.isNotEmpty) ...[
            const SizedBox(height: DsSpace.s16),
            Text(l.threadNoAccessWaiting, style: dsTextStyle(DsType.label)),
            const SizedBox(height: DsSpace.s8),
            Expanded(
              child: ListView(
                children: [
                  for (final (id, text) in waiting)
                    Padding(
                      padding: const EdgeInsets.only(bottom: DsSpace.s8),
                      child: SelectableText(
                        text,
                        key: ValueKey('thread.noAccess.text.$id'),
                        style: dsTextStyle(DsType.body)
                            .copyWith(color: colors.textPrimary),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}
