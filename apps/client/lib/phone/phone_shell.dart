// SPDX-License-Identifier: Apache-2.0

/// The main view of the phone after the login (docs/spec/01-product.md 5.3, 6.6; brief M5-02
/// Design note 2; docs/decisions.d/M5-02.md D-M5-02-3): the connection banner, one navigator
/// per tab whose pages follow the router's path, and the bottom tab bar.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/cupertino.dart' show CupertinoPage;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/session.dart';
import '../app/router.dart';
import '../running/providers.dart' show activeRunCountProvider;
import '../sidebar/providers.dart';
import 'phone_navigation.dart';
import 'phone_pages.dart';
import 'providers.dart';

/// Marks a navigation that came from outside the app (a deep link of the URL scheme or a
/// push): the router passes it as `extra`, and the phone opens it in the Spaces tab with the
/// whole stack below it (15 11; brief M5-02 Design note 2).
final class ExternalLink {
  /// Creates the mark.
  const ExternalLink();
}

/// The phone's main view for [location] (path and query of the router).
///
/// The router builds it for every content path under one page key, so the shell stays while
/// the path changes: it decides the tab of each path ([tabOf]), keeps the last path of every
/// tab (each tab keeps its stack) and shows the pages of the active tab ([phoneStack]) in that
/// tab's navigator. The pages are `CupertinoPage`s on iOS, so the edge swipe goes back; a pop
/// goes to the path of the page below. The tab bar shows on a tab's first page; deeper pages
/// take the whole height, with the input at the bottom above the safe area (01 5.3).
class PhoneShell extends ConsumerStatefulWidget {
  /// Creates the shell.
  const PhoneShell({super.key, required this.location, this.external = false});

  /// The location the router shows.
  final Uri location;

  /// Whether [location] came from outside the app ([ExternalLink]).
  final bool external;

  @override
  ConsumerState<PhoneShell> createState() => PhoneShellState();
}

/// State of [PhoneShell]: the active tab and the last path of each tab.
class PhoneShellState extends ConsumerState<PhoneShell> {
  late PhoneTab _tab;
  final Map<PhoneTab, Uri> _last = {};
  final Map<PhoneTab, GlobalKey<NavigatorState>> _navigators = {
    for (final tab in PhoneTab.values)
      tab: GlobalKey<NavigatorState>(debugLabel: 'phone.${tab.name}'),
  };
  List<PhonePage> _pages = const [];

  /// The active tab.
  PhoneTab get tab => _tab;

  /// The pages the active tab shows, bottom first.
  List<PhonePage> get pages => _pages;

  @override
  void initState() {
    super.initState();
    _tab = PhoneTab.spaces;
    _visit(widget.location, external: widget.external);
  }

  @override
  void didUpdateWidget(PhoneShell oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.location != widget.location || widget.external) {
      _visit(widget.location, external: widget.external);
    }
  }

  void _visit(Uri location, {required bool external}) {
    _tab = tabOf(parseLocation(location), _tab, external: external);
    _last[_tab] = location;
  }

  /// The tab bar: another tab opens at its last path; the active tab goes back to its first
  /// page.
  void _select(PhoneTab tab) {
    final target = tab == _tab ? null : _last[tab];
    setState(() => _tab = tab);
    context.go((target ?? Uri.parse(tab.root)).toString());
  }

  /// A page left the navigator by a pop (the edge swipe, the back button): the router goes to
  /// the page below. A page that left because the path changed is already gone from [_pages].
  void _popped(Page<Object?> page) {
    final pages = _pages;
    if (pages.length < 2 || page.key != _pageKey(pages.last)) return;
    context.go(pages[pages.length - 2].path);
  }

  static ValueKey<String> _pageKey(PhonePage page) =>
      ValueKey('phone.page.${page.kind.name}.${page.path}');

  /// The channel of a thread path in Spaces, once stored (the stack is spaces, channel,
  /// thread).
  ThreadChannel? _channelOf(Uri location) {
    if (_tab != PhoneTab.spaces) return null;
    if (parseLocation(location) is! ThreadLocation) return null;
    final selection = ref
        .watch(routeSelectionProvider(location.toString()))
        .value;
    final channel = selection?.channel;
    if (channel == null) return null;
    final stored = ref.watch(storedChannelOfRouteProvider(channel.value)).value;
    return (channel: channel, mainThread: stored?.mainThreadId);
  }

  @override
  Widget build(BuildContext context) {
    final location = _last[_tab] ?? Uri.parse(_tab.root);
    final pages = phoneStack(_tab, location, channelOf: _channelOf(location));
    _pages = pages;
    final iOS = Theme.of(context).platform == TargetPlatform.iOS;
    final root = pages.length == 1;
    return Scaffold(
      key: const ValueKey('phone.shell'),
      backgroundColor: context.dsColors.surfaceCanvas,
      body: SafeArea(
        bottom: !root,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const PhoneConnectionBanner(),
            Expanded(
              child: Navigator(
                key: _navigators[_tab],
                pages: [
                  for (final (i, page) in pages.indexed)
                    _page(page, i == 0 ? null : pages[i - 1], iOS: iOS),
                ],
                onDidRemovePage: _popped,
              ),
            ),
          ],
        ),
      ),
      bottomNavigationBar: root
          ? PhoneTabBar(active: _tab, onSelect: _select)
          : null,
    );
  }

  Page<Object?> _page(PhonePage page, PhonePage? below, {required bool iOS}) {
    final child = _PageFrame(page: page, below: below);
    final key = _pageKey(page);
    // The Cupertino route brings the edge swipe back on iOS (01 5.3); Android (M5-03) has
    // the Material route and system back.
    return iOS
        ? CupertinoPage<void>(key: key, child: child)
        : MaterialPage<void>(key: key, child: child);
  }
}

/// One page with the back bar above it when a page lies below.
class _PageFrame extends StatelessWidget {
  const _PageFrame({required this.page, required this.below});

  final PhonePage page;
  final PhonePage? below;

  @override
  Widget build(BuildContext context) {
    final under = below;
    return Material(
      key: ValueKey('phone.frame.${page.kind.name}'),
      color: context.dsColors.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (under != null) PhoneBackBar(below: under),
          Expanded(child: PhonePageView(page: page)),
        ],
      ),
    );
  }
}

/// The back button of a page, named after the page below (the iOS convention), with the
/// hit area `density.target-min`.
class PhoneBackBar extends ConsumerWidget {
  /// Creates the bar.
  const PhoneBackBar({super.key, required this.below});

  /// The page the button goes back to.
  final PhonePage below;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final parsed = parseLocation(Uri.parse(below.path));
    final label = switch (below.kind) {
      PhonePageKind.spaces => l.phoneTabSpaces,
      PhonePageKind.mentions => l.phoneTabMentions,
      PhonePageKind.running => l.phoneTabRunning,
      PhonePageKind.me => l.phoneTabMe,
      PhonePageKind.channel when parsed is ChannelLocation =>
        ref
                .watch(storedChannelOfRouteProvider(parsed.channelId.value))
                .value
                ?.name ??
            l.phoneBack,
      _ => l.phoneBack,
    };
    return Align(
      alignment: Alignment.centerLeft,
      child: DsTappable(
        key: const ValueKey('phone.back'),
        onTap: () => unawaited(Navigator.of(context).maybePop()),
        label: l.phoneBackTo(label),
        excludeChildSemantics: true,
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: DsSpace.s8,
            vertical: DsSpace.s4,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              DsIcon(
                DsIcons.actionBack,
                size: DsIconSizeRole.md,
                color: context.dsAccent.textLink,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s4),
              Flexible(
                child: Text(
                  dsIsolate(label),
                  overflow: TextOverflow.ellipsis,
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: context.dsAccent.textLink),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The banner above the pages (01 6.6, 5.3 M7): "Offline" with the number of entries waiting
/// in the outbox while the engine has no connection, "Update required" after `4007` (15 15).
class PhoneConnectionBanner extends ConsumerWidget {
  /// Creates the banner.
  const PhoneConnectionBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final phase = ref.watch(connectionStateProvider).value?.phase;
    return switch (phase) {
      ConnectionPhase.updateRequired => DsBanner(
        key: const ValueKey('shell.updateRequired'),
        tone: DsTone.danger,
        text: l.shellUpdateRequired,
        liveRegion: true,
      ),
      ConnectionPhase.offline ||
      ConnectionPhase.backoff ||
      ConnectionPhase.suspended => DsBanner(
        key: const ValueKey('shell.offline'),
        tone: DsTone.info,
        text: l.phoneOfflineWaiting(
          ref.watch(outboxWaitingProvider).value ?? 0,
        ),
        liveRegion: true,
      ),
      _ => const SizedBox.shrink(),
    };
  }
}

/// The bottom tab bar: Spaces, Mentions, Running (with the count of active runs) and Me
/// (01 5.3 M1).
class PhoneTabBar extends ConsumerWidget {
  /// Creates the bar.
  const PhoneTabBar({super.key, required this.active, required this.onSelect});

  /// The active tab.
  final PhoneTab active;

  /// Selects a tab.
  final ValueChanged<PhoneTab> onSelect;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final running = ref.watch(activeRunCountProvider);
    Widget icon(DsGlyph glyph, {bool filled = false}) => DsIcon(
      glyph,
      size: DsIconSizeRole.md,
      color: filled ? context.dsAccent.textLink : colors.textSecondary,
      decorative: true,
    );
    NavigationDestination destination(
      PhoneTab tab,
      DsGlyph glyph,
      String label, {
      int count = 0,
    }) {
      Widget mark(bool filled) => count == 0
          ? icon(glyph, filled: filled)
          : Badge.count(
              count: count,
              child: icon(glyph, filled: filled),
            );
      return NavigationDestination(
        key: ValueKey('phone.tab.${tab.name}'),
        icon: mark(false),
        selectedIcon: mark(true),
        label: label,
      );
    }

    return NavigationBar(
      key: const ValueKey('phone.tabBar'),
      backgroundColor: colors.surfaceRaised,
      selectedIndex: active.index,
      onDestinationSelected: (i) => onSelect(PhoneTab.values[i]),
      destinations: [
        destination(PhoneTab.spaces, DsIcons.channelForum, l.phoneTabSpaces),
        destination(
          PhoneTab.mentions,
          DsIcons.actionNotifications,
          l.phoneTabMentions,
        ),
        destination(
          PhoneTab.running,
          DsIcons.runRunning,
          l.phoneTabRunning,
          count: running,
        ),
        destination(PhoneTab.me, DsIcons.person, l.phoneTabMe),
      ],
    );
  }
}
