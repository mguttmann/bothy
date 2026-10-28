// SPDX-License-Identifier: Apache-2.0

/// The new chat view of a forum in the content column (01 6.8 item 8; components.md 6; brief
/// M4-41 requirement 12): "New chat in `forum`" or "New post in `forum`", the operator line, the
/// title (optional with an operator agent, required without), the tags and the composer with
/// the focus, without a dialog.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart' show OutboxState;
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/router.dart' show NewChatLocation;
import '../settings/settings_providers.dart';
import '../sidebar/providers.dart' show collapsedGroupsProvider;
import '../sync/thread_ops.dart';
import '../thread/providers.dart'
    show draftEditingProvider, instanceLimitsProvider, syncNoticesProvider;
import 'failed_post_notice.dart';
import 'new_chat_controller.dart';
import 'providers.dart';

/// The new chat view of [forum]; with [sentEntry] the view of the sent post that waits for the
/// server (01 6.6: "sending", or the refusal with "Try again").
///
/// Esc leaves the view to the forum overview (D15). On the echo of the sent post the history of
/// the new post opens (`/t/{id}`), and its sidebar row takes the place of the draft row.
class NewChatView extends ConsumerWidget {
  /// Creates the view.
  const NewChatView({super.key, required this.forum, this.sentEntry});

  /// The forum.
  final ChannelId forum;

  /// The outbox entry of the sent post, or null while the person writes.
  final String? sentEntry;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final channel = ref.watch(storedChannelProvider(forum.value)).value;
    if (channel == null) return const SizedBox.expand();
    final operator = channel.operator.effectiveAgentId;
    final names = ref.watch(principalNamesProvider).value ?? const {};
    final header = NewChatHeader(
      forumName: channel.name,
      projectBound: channel.projectBound,
      operatorName: operator == null ? null : names[operator],
      hasOperator: operator != null,
    );
    void leave() => context.go('/c/${forum.value}');
    final entry = sentEntry;
    return CallbackShortcuts(
      // 01 6.8 item 8 and requirement 12: Esc leaves the view.
      bindings: {const SingleActivator(LogicalKeyboardKey.escape): leave},
      child: Material(
        key: const ValueKey('newChat.view'),
        color: context.dsColors.surfaceCanvas,
        child: entry == null
            ? NewChatCompose(
                key: ValueKey('newChat.compose.${forum.value}'),
                forum: forum,
                header: header,
                titleRequired: operator == null,
              )
            : _SentPane(forum: forum, entry: entry, header: header),
      ),
    );
  }
}

/// The head of the view: the forum's icon and "New chat in `forum`" (a forum with a project
/// binding) or "New post in `forum`", and the operator line of the channel header (M4-20):
/// "Operator: `name`" or "No agent answers here".
class NewChatHeader extends StatelessWidget {
  /// Creates the head.
  const NewChatHeader({
    super.key,
    required this.forumName,
    required this.projectBound,
    required this.hasOperator,
    this.operatorName,
  });

  /// The forum's name (a foreign name).
  final String forumName;

  /// Whether the forum has a project binding.
  final bool projectBound;

  /// Whether the forum has an operator agent.
  final bool hasOperator;

  /// The operator's display name, or null while it is not stored.
  final String? operatorName;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final name = dsIsolate(forumName);
    final operator = operatorName;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            DsIcon(
              dsChannelGlyph(DsChannelKind.forum, projectBound: projectBound),
              size: DsIconSizeRole.md,
              color: c.textSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s6),
            Flexible(
              child: Semantics(
                header: true,
                child: Text(
                  projectBound
                      ? l.sidebarNewChatIn(name)
                      : l.sidebarNewPostIn(name),
                  key: const ValueKey('newChat.heading'),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: c.textPrimary),
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: DsSpace.s4),
        Text(
          !hasOperator
              ? l.threadNoOperator
              : l.threadOperator(dsIsolate(operator ?? '')),
          key: const ValueKey('newChat.operator'),
          style: dsTextStyle(DsType.bodySm).copyWith(color: c.textSecondary),
        ),
      ],
    );
  }
}

/// The view while the person writes: title, tags and the composer on the forum's draft of a
/// new post ([NewChatController]).
class NewChatCompose extends ConsumerStatefulWidget {
  /// Creates the pane.
  const NewChatCompose({
    super.key,
    required this.forum,
    required this.header,
    required this.titleRequired,
  });

  /// The forum.
  final ChannelId forum;

  /// The head of the view.
  final Widget header;

  /// Whether the forum has no operator agent, so the title is required (09 12).
  final bool titleRequired;

  @override
  ConsumerState<NewChatCompose> createState() => NewChatComposeState();
}

/// State of [NewChatCompose]; exposes the controller to tests.
class NewChatComposeState extends ConsumerState<NewChatCompose> {
  NewChatController? _chat;

  /// The controller, or null without a session.
  @visibleForTesting
  NewChatController? get chat => _chat;

  final FocusNode _fieldGroup = FocusNode(
    debugLabel: 'newChat.composer',
    canRequestFocus: false,
    skipTraversal: true,
  );

  bool _hadStoredDraft = false;

  /// This device overwrote a newer draft of another device (04 14).
  bool _overwritten = false;

  @override
  void initState() {
    super.initState();
    final engine = ref.read(syncEngineProvider);
    final db = ref.read(localDatabaseProvider);
    if (engine == null || db == null) return;
    final forum = widget.forum;
    final chat = _chat = NewChatController(
      forum: forum,
      outbox: engine.outbox,
      dao: db.outboxDao,
      editing: ref.read(draftEditingProvider),
      deviceId: ref.read(signedInServicesProvider)?.auth.session?.deviceId,
      dropStoredDraft: () async {
        final stored = await db.personalDao.watchDraft(channelId: forum).first;
        if (stored != null) await db.personalDao.deleteDraft(stored.id);
      },
    )..titleRequired = widget.titleRequired;
    chat.composer.limits = ref.read(instanceLimitsProvider).value;
    chat.addListener(_changed);
    final preset = ref.read(newChatPresetProvider.notifier).take(forum);
    if (preset != null) chat.preset(preset.$1, preset.$2);
    unawaited(chat.composer.restoreUnsent());
    ref.listenManual(channelDraftProvider(forum.value), (_, next) {
      final draft = next.value;
      if (draft != null && draft.contentMd.trim().isNotEmpty) {
        _hadStoredDraft = true;
      }
      unawaited(chat.composer.applyStored(draft));
    }, fireImmediately: true);
    ref.listenManual(syncNoticesProvider, (_, next) {
      final notice = next.value;
      // 04 14: this device overwrote a newer draft of the new post of another device.
      if (notice is DraftOverwritten &&
          notice.draft.threadId == null &&
          notice.draft.channelId == forum &&
          mounted) {
        setState(() => _overwritten = true);
      }
    });
    // The input has the focus when the view opens (01 6.8 item 8).
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) focusInput();
    });
  }

  @override
  void didUpdateWidget(NewChatCompose oldWidget) {
    super.didUpdateWidget(oldWidget);
    _chat?.titleRequired = widget.titleRequired;
  }

  void _changed() => setState(() {});

  /// Puts the focus into the text field of the composer; returns whether there is one.
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

  Future<void> _send(String text) async {
    final chat = _chat;
    if (chat == null) return;
    final entry = await chat.send(text);
    if (entry == null || !mounted) return;
    // The view of the sent post: its row is selected and the history opens on the echo.
    context.go(NewChatLocation.pathOf(widget.forum, entry: entry));
  }

  @override
  void dispose() {
    final chat = _chat;
    if (chat != null) {
      chat.removeListener(_changed);
      // 01 6.8 item 8: an empty draft is discarded when the person leaves the view.
      unawaited(chat.leave(hadStoredDraft: _hadStoredDraft));
      chat.dispose();
    }
    _fieldGroup.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final chat = _chat;
    final forum = widget.forum.value;
    final channel = ref.watch(storedChannelProvider(forum)).value;
    final tags = ref.watch(forumTagsProvider(forum)).value ?? const [];
    final phase = ref.watch(connectionStateProvider).value?.phase;
    final offline =
        phase == null ||
        phase == ConnectionPhase.offline ||
        phase == ConnectionPhase.backoff ||
        phase == ConnectionPhase.suspended;
    final required = widget.titleRequired;
    final missingTitle =
        chat != null &&
        required &&
        chat.title.text.trim().isEmpty &&
        chat.composer.text.text.trim().isNotEmpty;
    return Padding(
      padding: const EdgeInsets.all(DsSpace.s12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          widget.header,
          const SizedBox(height: DsSpace.s12),
          TextField(
            key: const ValueKey('newChat.title'),
            controller: chat?.title,
            enabled: chat != null,
            textInputAction: TextInputAction.next,
            decoration: InputDecoration(
              labelText: required ? l.newPostTitle : l.newChatTitleOptional,
              helperText: required ? null : l.newChatTitleAgentHint,
              helperMaxLines: 2,
            ),
          ),
          if (tags.isNotEmpty && chat != null) ...[
            const SizedBox(height: DsSpace.s12),
            Text(l.newPostTags, style: dsTextStyle(DsType.label)),
            const SizedBox(height: DsSpace.s6),
            Wrap(
              spacing: DsSpace.s6,
              runSpacing: DsSpace.s6,
              children: [
                for (final t in tags)
                  FilterChip(
                    key: ValueKey('newChat.tag.${t.id.value}'),
                    label: Text(dsIsolate(t.name)),
                    selected: chat.tags.contains(t.id),
                    // Whether the person may set a restricted tag is the server's decision
                    // (15 1); a refusal shows as every outbox failure (01 6.6).
                    onSelected: (_) => chat.toggleTag(
                      t.id,
                      order: [for (final x in tags) x.id],
                    ),
                  ),
              ],
            ),
          ],
          const Spacer(),
          if (_overwritten)
            Padding(
              padding: const EdgeInsets.only(bottom: DsSpace.s4),
              child: DsBanner(
                key: const ValueKey('newChat.draftNotice'),
                tone: DsTone.warning,
                text: l.composerDraftOverwritten,
                liveRegion: true,
                actionLabel: l.threadCancel,
                onAction: () => setState(() => _overwritten = false),
              ),
            ),
          if (missingTitle)
            Padding(
              padding: const EdgeInsets.only(bottom: DsSpace.s6),
              child: Text(
                l.newChatTitleMissing,
                key: const ValueKey('newChat.titleMissing'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: c.textSecondary),
              ),
            ),
          if (chat != null)
            Focus(
              focusNode: _fieldGroup,
              child: DsComposer(
                key: const ValueKey('newChat.composer'),
                targetName: channel?.name ?? '',
                maxLines: ref.watch(appConfigProvider).composerMaxLines,
                enterSends: ref.watch(personalSettingsProvider).enterSends,
                controller: chat.composer.text,
                onChanged: chat.composer.onChanged,
                onSend: (text) => unawaited(_send(text)),
                offline: offline,
                sendEnabled: chat.canSend,
              ),
            ),
        ],
      ),
    );
  }
}

/// The view of a sent post that waits for the server: "Sending" (01 6.6), or the refusal with
/// "Try again", "Edit" and "Discard". When the server created the post its history opens.
class _SentPane extends ConsumerStatefulWidget {
  const _SentPane({
    required this.forum,
    required this.entry,
    required this.header,
  });

  final ChannelId forum;
  final String entry;
  final Widget header;

  @override
  ConsumerState<_SentPane> createState() => _SentPaneState();
}

class _SentPaneState extends ConsumerState<_SentPane> {
  StreamSubscription<String>? _created;
  bool _left = false;
  bool _checking = false;

  @override
  void initState() {
    super.initState();
    final created = ref.read(createdPostsProvider);
    _created = created.changes.listen((id) {
      if (id == widget.entry) _open(created.threadOf(id));
    });
    WidgetsBinding.instance.addPostFrameCallback(
      (_) => _open(created.threadOf(widget.entry)),
    );
  }

  /// Opens the history of [thread] once it is known (brief M4-41 requirement 14).
  void _open(ThreadId? thread) {
    if (thread == null || _left || !mounted) return;
    _left = true;
    context.go('/t/${thread.value}');
  }

  /// The entry is missing from the forum's waiting posts. The outbox's stream follows a write
  /// a moment later, so right after the send that list can predate the entry; the outbox
  /// decides: a waiting or refused entry keeps the view, and an entry that was confirmed or is
  /// gone without this run seeing its answer (another run sent it) leaves to the overview,
  /// which shows the post.
  Future<void> _checkLeft() async {
    if (_checking || _left) return;
    _checking = true;
    final entry = await ref
        .read(localDatabaseProvider)
        ?.outboxDao
        .byId(widget.entry);
    _checking = false;
    if (!mounted || _left) return;
    final thread = ref.read(createdPostsProvider).threadOf(widget.entry);
    if (thread != null) return _open(thread);
    if (entry != null && entry.state != OutboxState.confirmed) return;
    _left = true;
    context.go('/c/${widget.forum.value}');
  }

  @override
  void dispose() {
    unawaited(_created?.cancel());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final forum = widget.forum.value;
    final pending = ref.watch(pendingPostsProvider(forum)).value;
    final failed = [
      for (final f
          in ref.watch(failedPostsProvider(forum)).value ??
              const <FailedPost>[])
        if (f.entryId == widget.entry) f,
    ];
    // The entry is not among the waiting posts: it may have left the outbox, or the list may
    // still predate the send; the outbox decides (_checkLeft).
    if (pending != null &&
        !pending.any((p) => p.entryId == widget.entry) &&
        ref.read(createdPostsProvider).threadOf(widget.entry) == null &&
        !_left) {
      WidgetsBinding.instance.addPostFrameCallback(
        (_) => unawaited(_checkLeft()),
      );
    }
    final engine = ref.read(syncEngineProvider);
    return Padding(
      padding: const EdgeInsets.all(DsSpace.s12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          widget.header,
          const SizedBox(height: DsSpace.s12),
          if (failed.isEmpty)
            Text(
              l.messageSending,
              key: const ValueKey('newChat.sending'),
              style: dsTextStyle(DsType.body)
                  .copyWith(color: context.dsColors.textSecondary),
            )
          else
            FailedPostNotice(
              post: failed.first,
              onRetry: () {
                if (engine != null) {
                  unawaited(engine.outbox.retry(widget.entry));
                }
              },
              onDiscard: () {
                if (engine != null) {
                  unawaited(engine.outbox.discard(widget.entry));
                }
                context.go('/c/$forum');
              },
              onEdit: () => unawaited(
                editRefusedPost(context, ref, widget.forum, failed.first),
              ),
            ),
        ],
      ),
    );
  }
}

/// "Edit" of a refused post (01 6.6; D-M4-41-3): its title and tags go to the new chat view of
/// [forum], its text into the person's draft of a new post there (ahead of a draft that already
/// has text), the refused entry leaves the outbox and the view opens.
Future<void> editRefusedPost(
  BuildContext context,
  WidgetRef ref,
  ChannelId forum,
  FailedPost post,
) async {
  final engine = ref.read(syncEngineProvider);
  final db = ref.read(localDatabaseProvider);
  if (engine == null || db == null) return;
  final stored = await db.personalDao.watchDraft(channelId: forum).first;
  final current =
      ref.read(unsentChannelDraftProvider(forum.value)).value ??
      stored?.contentMd ??
      '';
  final text = current.trim().isEmpty
      ? post.contentMd
      : '${post.contentMd}\n\n$current';
  await engine.outbox.enqueue(
    ThreadOps.channelDraftRequest(
      forum: forum,
      contentMd: text,
      baseVersion: stored?.version,
    ),
  );
  ref.read(newChatPresetProvider.notifier).set(forum, post.title, post.tagIds);
  await engine.outbox.discard(post.entryId);
  if (context.mounted) context.go(NewChatLocation.pathOf(forum));
}

/// Opens the new chat view of [forum] (01 6.8 item 8, brief M4-41): "+", the context menu entry
/// and Cmd+N or Ctrl+N. A collapsed thread group of the forum expands first, so its draft row
/// shows and is selected (D-M4-41-4).
void openNewChat(BuildContext context, WidgetRef ref, ChannelId forum) {
  final db = ref.read(localDatabaseProvider);
  final collapsed = ref.read(collapsedGroupsProvider).value ?? const {};
  if (db != null && collapsed.contains(forum)) {
    unawaited(db.forumViewDao.setGroupCollapsed(forum, collapsed: false));
  }
  context.go(NewChatLocation.pathOf(forum));
}
