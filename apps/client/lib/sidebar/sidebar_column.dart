// SPDX-License-Identifier: Apache-2.0

/// The sidebar column of the desktop main view (01 5.1 D1; components.md 7; brief M4-19
/// Goal): head, tree of spaces and channels, foot.
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart' show AdminLocation, AdminSection;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../admin/admin_route.dart' show adminPathOf;
import '../app/providers/session.dart';
import '../app/router.dart' show NewChatLocation;
import '../forum/new_chat_view.dart' show openNewChat;
import '../new_project/new_project_dialog.dart' show showNewProjectDialog;
import '../new_project/project_sync_dialog.dart' show showProjectSyncDialog;
import '../notifications_settings/providers.dart' show isOwnerProvider;
import '../onboarding/onboarding_controller.dart';
import '../running/providers.dart' show activeRunCountProvider;
import '../switcher/quick_switcher.dart' show showQuickSwitcher;
import '../sync/sidebar_ops.dart';
import '../telemetry/telemetry_providers.dart';
import '../terminal/hosts_group.dart';
import '../viz/live_page.dart' show livePath;
import 'providers.dart';
import 'sidebar_footer.dart';
import 'sidebar_header.dart';
import 'sidebar_live_group.dart';
import 'sidebar_model.dart';
import 'sidebar_order.dart';
import 'space_tree.dart';

/// The largest share of the window height the group "Hosts" takes; it scrolls beyond it.
const double hostsHeightShare = 0.3;

/// The sidebar as it looks for given data, without providers (widget and golden tests use it
/// directly).
class SidebarView extends StatelessWidget {
  /// Creates the view.
  const SidebarView({
    super.key,
    required this.productName,
    required this.nodes,
    required this.onToggle,
    required this.onMove,
    required this.onOpenChannel,
    required this.onSettings,
    required this.onLogout,
    this.logo,
    this.ownName,
    this.selectedChannel,
    this.onJumpTo,
    this.onRunningNow,
    this.runningCount = 0,
    this.onLive,
    this.onStatistics,
    this.showFooter = true,
    this.hosts,
    this.onStatus,
    this.selectedThread,
    this.onOpenThread,
    this.onToggleGroup,
    this.onShowAllPosts,
    this.onAdd,
    this.onNewProject,
    this.onAddFolderForums,
    this.onHold,
    this.holdMode = SidebarHoldMode.pointer,
    this.selectedNewChat,
    this.onOpenNewChat,
  });

  /// `branding.product_name` of the instance.
  final String productName;

  /// The logo bytes, or null.
  final Uint8List? logo;

  /// The tree.
  final List<SidebarSpaceNode> nodes;

  /// The display name of the signed-in person, or null.
  final String? ownName;

  /// The selected channel, or null.
  final ChannelId? selectedChannel;

  /// Collapses or expands a space.
  final void Function(SidebarSpaceNode node, bool expanded) onToggle;

  /// Moves a space to a new index.
  final void Function(SpaceId space, int to) onMove;

  /// Opens a channel.
  final ValueChanged<ChannelId> onOpenChannel;

  /// Opens the settings.
  final VoidCallback onSettings;

  /// Logs out.
  final VoidCallback onLogout;

  /// Opens the quick switcher (01 D6), or null.
  final VoidCallback? onJumpTo;

  /// Opens "Running now" (01 D10), or null.
  final VoidCallback? onRunningNow;

  /// The count of active runs of the entry "Running now".
  final int runningCount;

  /// Opens "Live" (D12); with [onStatistics] it shows the group "Live and statistics", null
  /// hides it (the phone).
  final VoidCallback? onLive;

  /// Opens "Statistics" (A7).
  final VoidCallback? onStatistics;

  /// Whether the foot with the person, Settings and Log out shows; the phone has them in its
  /// Me tab (brief M5-02 Design note 2).
  final bool showFooter;

  /// The group "Hosts" below the spaces (01 D11, M4-36), or null (the phone has no terminal
  /// yet).
  final Widget? hosts;

  /// Opens the status page from the foot (M7-21), or null for no entry.
  final VoidCallback? onStatus;

  /// The thread the route shows, or null.
  final ThreadId? selectedThread;

  /// Opens a thread row; null shows no thread groups (tests of the tree without them).
  final ValueChanged<ThreadId>? onOpenThread;

  /// Collapses or expands the thread group of a channel (01 6.8 item 7).
  final void Function(ChannelId channel, bool expanded)? onToggleGroup;

  /// Lists every open post of a forum's group (true, "Show more") or the newest ones (false,
  /// "Show less"); null shows no such row (01 6.8 item 3, M4-46).
  final void Function(ChannelId forum, bool inFull)? onShowAllPosts;

  /// The new chat row the route selects, or null (brief M4-41).
  final SidebarNewChatSelection? selectedNewChat;

  /// Opens a new chat row of a forum: its draft (entry null) or a sent post; null for none.
  final void Function(ChannelId forum, String? entry)? onOpenNewChat;

  /// The "+" action of a forum row (01 6.8 item 8), or null for none.
  final ValueChanged<ChannelId>? onAdd;

  /// The action "New project" of a space row and its menu (01 6.9 item 1), or null for none.
  final ValueChanged<Space>? onNewProject;

  /// The space menu entry "Add forums for existing folders" (01 6.10 item 1), or null for none.
  final ValueChanged<Space>? onAddFolderForums;

  /// Called with true while the person is in the sidebar ([holdMode]) and with false when the
  /// person left it (01 6.8 item 4: rows that no longer qualify stay until then).
  final ValueChanged<bool>? onHold;

  /// What counts as "in the sidebar" for [onHold]: the pointer and the keyboard focus on the
  /// desktop, a finger, a scroll and the screen reader on a touch screen (brief M5-15 Design
  /// note 1).
  final SidebarHoldMode holdMode;

  @override
  Widget build(BuildContext context) {
    final column = _column(context);
    final hold = onHold;
    if (hold == null) return column;
    return switch (holdMode) {
      SidebarHoldMode.pointer => _HoldRegion(onHold: hold, child: column),
      SidebarHoldMode.touch => _TouchHoldRegion(onHold: hold, child: column),
    };
  }

  // The view paints its brand surface itself (D-DS-02): every `text.on-sidebar*` and
  // `category.*.on-sidebar` role is paired only with `surface.sidebar*`, so the view must
  // never stand on the host's surface. The phone's Spaces page has no column around it; on
  // `surface.canvas` the light theme's sidebar texts were nearly invisible
  // (FIX-light-theme-contrast).
  Widget _column(BuildContext context) => ColoredBox(
    color: context.dsColors.surfaceSidebar,
    child: _content(context),
  );

  Widget _content(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      SidebarHeader(
        productName: productName,
        logo: logo,
        onJumpTo: onJumpTo,
        onRunningNow: onRunningNow,
        runningCount: runningCount,
      ),
      if (onLive != null && onStatistics != null)
        SidebarLiveGroup(onLive: onLive!, onStatistics: onStatistics!),
      Expanded(
        child: SpaceTree(
          nodes: nodes,
          selectedChannel: selectedChannel,
          selectedThread: selectedThread,
          onToggle: onToggle,
          onMove: onMove,
          onOpenChannel: onOpenChannel,
          onOpenThread: onOpenThread,
          onToggleGroup: onToggleGroup,
          onShowAllPosts: onShowAllPosts,
          onAdd: onAdd,
          onNewProject: onNewProject,
          onAddFolderForums: onAddFolderForums,
          selectedNewChat: selectedNewChat,
          onOpenNewChat: onOpenNewChat,
        ),
      ),
      // The group scrolls within a share of the column, so the spaces keep their room.
      if (hosts case final group?)
        ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(context).height * hostsHeightShare,
          ),
          child: SingleChildScrollView(child: group),
        ),
      if (showFooter)
        SidebarFooter(
          name: ownName,
          onSettings: onSettings,
          onLogout: onLogout,
          onStatus: onStatus,
        ),
    ],
  );
}

/// What holds the thread rows of the sidebar in place (01 6.8 item 4): a row that no longer
/// qualifies leaves only once the person is no longer in the sidebar.
enum SidebarHoldMode {
  /// The desktop (brief M4-40 Design note 2): the pointer is over the sidebar or the keyboard
  /// focus is in it.
  pointer,

  /// A touch screen, M1 of the phone (brief M5-15 Design note 1; D-M5-15-1): a finger is on the
  /// list, the list scrolls, or a screen reader runs while the list is the page on screen,
  /// because the app learns no position of the screen reader focus but on the focused node
  /// itself, and on M1 that focus is in the list or one step from it.
  touch,
}

/// Reports whether the pointer is over [child] or the keyboard focus is inside it (brief M4-40
/// Design note 2): true when either starts, false when both have left.
class _HoldRegion extends StatefulWidget {
  const _HoldRegion({required this.onHold, required this.child});

  final ValueChanged<bool> onHold;
  final Widget child;

  @override
  State<_HoldRegion> createState() => _HoldRegionState();
}

class _HoldRegionState extends State<_HoldRegion> {
  bool _pointer = false;
  bool _focus = false;
  bool _held = false;

  void _update({bool? pointer, bool? focus}) {
    _pointer = pointer ?? _pointer;
    _focus = focus ?? _focus;
    final held = _pointer || _focus;
    if (held == _held) return;
    _held = held;
    widget.onHold(held);
  }

  @override
  void dispose() {
    // A sidebar that leaves the screen holds nothing any more; reported after the frame that
    // removes it, because a provider must not change while widgets unmount.
    if (_held) {
      final report = widget.onHold;
      scheduleMicrotask(() => report(false));
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => MouseRegion(
    onEnter: (_) => _update(pointer: true),
    onExit: (_) => _update(pointer: false),
    child: Focus(
      canRequestFocus: false,
      skipTraversal: true,
      includeSemantics: false,
      onFocusChange: (focused) => _update(focus: focused),
      child: widget.child,
    ),
  );
}

/// Reports whether a finger is on [child], [child] scrolls, or a screen reader runs while
/// [child] is on the page on screen (brief M5-15 Design note 1): true when one starts, false
/// when none holds any more.
class _TouchHoldRegion extends StatefulWidget {
  const _TouchHoldRegion({required this.onHold, required this.child});

  final ValueChanged<bool> onHold;
  final Widget child;

  @override
  State<_TouchHoldRegion> createState() => _TouchHoldRegionState();
}

class _TouchHoldRegionState extends State<_TouchHoldRegion> {
  /// The pointers that are down on the list now.
  final Set<int> _fingers = {};

  /// The scrollables below that scroll now (the list, a nested one).
  int _scrolls = 0;

  bool _reader = false;
  bool _held = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // A screen reader in the list holds while M1 is the page on screen; a page on top of it
    // (M2, M3, the new chat page) ends the hold, so M1 is current when the person returns.
    final reader =
        MediaQuery.accessibleNavigationOf(context) &&
        (ModalRoute.isCurrentOf(context) ?? true);
    if (reader == _reader) return;
    _reader = reader;
    // Dependencies change during a build; the provider must not change in it.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _report();
    });
  }

  void _report() {
    final held = _fingers.isNotEmpty || _scrolls > 0 || _reader;
    if (held == _held) return;
    _held = held;
    widget.onHold(held);
  }

  void _down(PointerDownEvent e) {
    _fingers.add(e.pointer);
    _report();
  }

  void _up(PointerEvent e) {
    _fingers.remove(e.pointer);
    _report();
  }

  bool _onScroll(ScrollNotification n) {
    if (n is ScrollStartNotification) {
      _scrolls++;
      _report();
    } else if (n is ScrollEndNotification && _scrolls > 0) {
      _scrolls--;
      _report();
    }
    return false;
  }

  @override
  void dispose() {
    // As [_HoldRegionState.dispose]: a list that leaves holds nothing any more.
    if (_held) {
      final report = widget.onHold;
      scheduleMicrotask(() => report(false));
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Listener(
    behavior: HitTestBehavior.translucent,
    onPointerDown: _down,
    onPointerUp: _up,
    onPointerCancel: _up,
    child: NotificationListener<ScrollNotification>(
      onNotification: _onScroll,
      child: widget.child,
    ),
  );
}

/// The sidebar of the signed-in instance over the providers (R-CL-03).
///
/// The desktop shows the group "Live and statistics" (D12) below the head. A click or Enter on
/// a channel goes to `/c/<id>`; collapse is kept on this device (`space_view_states`; 01 6.7,
/// R-PR-24), a drag writes the personal order through the outbox ([SidebarWriter]);
/// [selectedChannel] follows the route.
/// As M1 of the phone ([phone], 01 5.3) it has no foot (the Me tab has it) and the search
/// field opens Search instead of the quick switcher.
class SidebarColumn extends ConsumerWidget {
  /// Creates the column.
  const SidebarColumn({
    super.key,
    this.selectedChannel,
    this.selectedThread,
    this.selectedHost,
    this.phone = false,
    this.newChat,
  });

  /// The new chat row the route selects (the new chat view of a forum, M4-41), or null.
  final SidebarNewChatSelection? newChat;

  /// The channel of the route, or null.
  final ChannelId? selectedChannel;

  /// The thread of the route, or null.
  final ThreadId? selectedThread;

  /// The host of the route (`/hosts/<id>`), or null.
  final String? selectedHost;

  /// Whether the column is the Spaces page of the phone.
  final bool phone;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final instance = ref.watch(currentInstanceProvider);
    final nodes = ref.watch(sidebarTreeProvider);
    final hold = ref.read(sidebarHoldProvider.notifier);
    // `client.cold_start.duration` ends with the first sidebar that shows spaces from the
    // local database (15 19, brief M4-31); later calls do nothing.
    if (nodes.isNotEmpty) ref.read(appMeasurementsProvider).sidebarShown();
    SidebarWriter? writer() {
      final engine = ref.read(syncEngineProvider);
      final db = ref.read(localDatabaseProvider);
      return engine == null || db == null ? null : SidebarWriter(engine, db);
    }

    final owner = !phone && (ref.watch(isOwnerProvider).value ?? false);
    return SidebarView(
      productName: instance?.productName ?? '',
      logo: instance?.logo,
      nodes: nodes,
      ownName: ref.watch(ownPrincipalProvider).value?.displayName,
      selectedChannel: selectedChannel,
      // 01 6.7, R-PR-24: the collapse of a space stays on this device; no outbox entry, so no
      // other device of the person follows.
      onToggle: (node, expanded) => unawaited(
        ref
            .read(localDatabaseProvider)
            ?.forumViewDao
            .setSpaceCollapsed(node.entry.space.id, collapsed: !expanded),
      ),
      onMove: (space, to) => unawaited(
        writer()?.move(moveSpace([for (final n in nodes) n.entry], space, to)),
      ),
      onOpenChannel: (channel) => context.go('/c/${channel.value}'),
      // The desktop and M1 of the phone show the thread groups (01 6.8, D-SB-13): a thread
      // row opens the history, on the phone as M3 on top of M1.
      selectedThread: selectedThread,
      onOpenThread: (thread) => context.go('/t/${thread.value}'),
      onToggleGroup: (channel, expanded) => unawaited(
        ref
            .read(localDatabaseProvider)
            ?.forumViewDao
            .setGroupCollapsed(channel, collapsed: !expanded),
      ),
      // 01 6.8 item 3 (M4-46): "Show more" and "Show less" of a forum group, kept on this
      // device.
      onShowAllPosts: (forum, inFull) => unawaited(
        ref
            .read(localDatabaseProvider)
            ?.forumViewDao
            .setPostListShownInFull(forum, inFull: inFull),
      ),
      // 01 6.8 item 8: "+" opens the new chat view of the forum, its draft when one exists; a
      // collapsed group expands, so the draft row shows and is selected. On the phone the view
      // is a page on top of M1 (brief M5-15 Design note 2).
      onAdd: (channel) => openNewChat(context, ref, channel),
      // 01 6.9 item 1: owners start a new project from a space row; the desktop only
      // (D-M4-43-12).
      onNewProject: owner
          ? (space) => unawaited(_newProject(context, ref, space))
          : null,
      // 01 6.10 item 1: owners add forums for the folders of a runner, the desktop only
      // (D-M4-44-7).
      onAddFolderForums: owner
          ? (space) => unawaited(showProjectSyncDialog(context, ref, space))
          : null,
      selectedNewChat: newChat,
      onOpenNewChat: (forum, entry) =>
          context.go(NewChatLocation.pathOf(forum, entry: entry)),
      onHold: (held) => hold.set(held: held),
      // No pointer on touch (brief M5-15 Design note 1): a finger, a scroll or the screen
      // reader holds the rows of M1.
      holdMode: phone ? SidebarHoldMode.touch : SidebarHoldMode.pointer,
      onSettings: () => context.go('/settings'),
      onStatus: () => context.go('/status'),
      onJumpTo: phone
          ? () => context.go('/search')
          : () => unawaited(showQuickSwitcher(context)),
      showFooter: !phone,
      // The terminal of the phone is M5's; the desktop shows the group below the spaces.
      hosts: phone ? null : HostsGroup(selectedHost: selectedHost),
      onRunningNow: () => context.go('/running'),
      runningCount: ref.watch(activeRunCountProvider),
      onLive: phone ? null : () => context.go(livePath),
      onStatistics: phone
          ? null
          : () => context.go(
              adminPathOf(const AdminLocation(AdminSection.statistics)),
            ),
      onLogout: () =>
          unawaited(ref.read(onboardingControllerProvider.notifier).logout()),
    );
  }
}

/// Opens the new project dialog of [space] and, when every step is done, the new forum or,
/// for an idea, its new chat (01 6.9 item 7).
Future<void> _newProject(
  BuildContext context,
  WidgetRef ref,
  Space space,
) async {
  final result = await showNewProjectDialog(context, ref, space);
  if (result == null || !context.mounted) return;
  final entry = result.sentEntry;
  context.go(
    entry == null
        ? '/c/${result.forum.value}'
        : NewChatLocation.pathOf(result.forum, entry: entry),
  );
}
