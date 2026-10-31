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

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/router.dart';
import '../app/update/update_notice.dart';
import '../running/providers.dart' show activeRunCountProvider;
import '../settings/settings_shell.dart' show SettingsSection;
import '../sidebar/providers.dart';
import 'android/android_controls.dart';
import 'phone_back_scope.dart';
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
/// take the whole height, with the input at the bottom above the safe area (01 5.3). An
/// Android build (M5-03, 01 5.4) has Material pages, system back through [AndroidBackScope]
/// and the [AndroidTopAppBar] on deeper pages instead of the iOS back button.
class PhoneShell extends ConsumerStatefulWidget {
  /// Creates the shell.
  const PhoneShell({
    super.key,
    required this.location,
    this.external = false,
    this.pageBuilder,
  });

  /// The location the router shows.
  final Uri location;

  /// Whether [location] came from outside the app ([ExternalLink]).
  final bool external;

  /// Builds the content of a page instead of [PhonePageView] (tests of the navigation).
  @visibleForTesting
  final Widget Function(PhonePage page)? pageBuilder;

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
  /// thread); also of the thread below a run graph opened from its live box.
  ThreadChannel? _channelOf(Uri location) {
    if (_tab != PhoneTab.spaces) return null;
    final thread = switch (parseLocation(location)) {
      ThreadLocation() => location.toString(),
      RunGraphLocation(fromThread: true, :final threadId) =>
        '/t/${threadId.value}',
      _ => null,
    };
    if (thread == null) return null;
    final selection = ref.watch(routeSelectionProvider(thread)).value;
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
    // The controls follow the build's platform (D-M5-03-6), the routes the target platform.
    final android = ref.watch(appPortsProvider).platform == 'android';
    final root = pages.length == 1;
    final navigator = _navigators[_tab]!;
    final stack = Navigator(
      key: navigator,
      pages: [
        for (final (i, page) in pages.indexed)
          _page(page, i == 0 ? null : pages[i - 1], iOS: iOS, android: android),
      ],
      onDidRemovePage: _popped,
    );
    return Scaffold(
      key: const ValueKey('phone.shell'),
      backgroundColor: context.dsColors.surfaceCanvas,
      body: SafeArea(
        bottom: !root,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const PhoneConnectionBanner(),
            const RecommendedUpdateBanner(),
            Expanded(
              child: android
                  ? AndroidBackScope(navigator: navigator, child: stack)
                  : stack,
            ),
          ],
        ),
      ),
      bottomNavigationBar: root
          ? PhoneTabBar(active: _tab, onSelect: _select)
          : null,
    );
  }

  Page<Object?> _page(
    PhonePage page,
    PhonePage? below, {
    required bool iOS,
    required bool android,
  }) {
    final child = _PageFrame(
      page: page,
      below: below,
      android: android,
      content: widget.pageBuilder?.call(page) ?? PhonePageView(page: page),
    );
    final key = _pageKey(page);
    // The Cupertino route brings the edge swipe back on iOS (01 5.3); Android (M5-03) has
    // the Material route and system back.
    return iOS
        ? CupertinoPage<void>(key: key, child: child)
        : MaterialPage<void>(key: key, child: child);
  }
}

/// One page with the back bar above it when a page lies below: the iOS back button, or the
/// top app bar of an Android build ([android]).
class _PageFrame extends StatelessWidget {
  const _PageFrame({
    required this.page,
    required this.below,
    required this.android,
    required this.content,
  });

  final PhonePage page;
  final PhonePage? below;
  final bool android;
  final Widget content;

  @override
  Widget build(BuildContext context) {
    final under = below;
    return Material(
      key: ValueKey('phone.frame.${page.kind.name}'),
      color: context.dsColors.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // A thread draws the back button in its one-line header (brief M5-17).
          if (under != null && page.kind == PhonePageKind.thread)
            Expanded(
              child: PhoneHeaderBackFrame(
                page: page,
                below: under,
                android: android,
                child: content,
              ),
            )
          else ...[
            if (under != null)
              android
                  ? PhoneTopAppBar(page: page, below: under)
                  : PhoneBackBar(below: under),
            Expanded(child: content),
          ],
        ],
      ),
    );
  }
}

/// The back action to [below] for a page that shows it in its own header (a thread, brief
/// M5-17): the same name and the same pop as [PhoneBackBar]. Until the page's header claims it
/// (the thread is loading or not found) the bar of the frame shows, so there is always a way
/// back.
class PhoneHeaderBackFrame extends ConsumerStatefulWidget {
  /// Creates the frame of [page] over [below]; [android] chooses the top app bar as the bar.
  const PhoneHeaderBackFrame({
    super.key,
    required this.page,
    required this.below,
    required this.android,
    required this.child,
  });

  /// The page.
  final PhonePage page;

  /// The page the back action returns to.
  final PhonePage below;

  /// Whether the bar is the Android top app bar.
  final bool android;

  /// The page's content.
  final Widget child;

  @override
  ConsumerState<PhoneHeaderBackFrame> createState() =>
      PhoneHeaderBackFrameState();
}

/// State of [PhoneHeaderBackFrame]: the claims of the header buttons below.
class PhoneHeaderBackFrameState extends ConsumerState<PhoneHeaderBackFrame> {
  final PhoneBackClaims _claims = PhoneBackClaims();

  @override
  void dispose() {
    _claims.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ListenableBuilder(
          listenable: _claims,
          builder: (context, _) => _claims.count > 0
              ? const SizedBox.shrink()
              : widget.android
              ? PhoneTopAppBar(page: widget.page, below: widget.below)
              : PhoneBackBar(below: widget.below),
        ),
        Expanded(
          child: PhoneBackScope(
            label: phonePageName(ref, l, widget.below) ?? l.phoneBack,
            onBack: _back,
            claims: _claims,
            child: widget.child,
          ),
        ),
      ],
    );
  }

  void _back() => unawaited(Navigator.of(context).maybePop());
}

/// The name of [page] as a back button or a title shows it: the tab's name, a channel's name
/// once stored, the name of a settings section or of the search; null for a page that names
/// itself in its own header (a thread) or has no name.
String? phonePageName(WidgetRef ref, AppLocalizations l, PhonePage page) {
  final parsed = parseLocation(Uri.parse(page.path));
  return switch (page.kind) {
    PhonePageKind.spaces => l.phoneTabSpaces,
    PhonePageKind.mentions => l.phoneTabMentions,
    PhonePageKind.running => l.phoneTabRunning,
    PhonePageKind.runGraph => l.runGraphTitle,
    PhonePageKind.me => l.phoneTabMe,
    PhonePageKind.search => l.searchTitle,
    PhonePageKind.channel when parsed is ChannelLocation =>
      ref
          .watch(storedChannelOfRouteProvider(parsed.channelId.value))
          .value
          ?.name,
    PhonePageKind.settingsSection when parsed is SectionLocation =>
      SettingsSection.of(parsed.segments.firstOrNull)?.label(l),
    PhonePageKind.emergency => l.emergencyTitle,
    PhonePageKind.status => l.statusTitle,
    PhonePageKind.hosts => l.hostsGroupTitle,
    _ => null,
  };
}

/// The back button of a page, named after the page below (the iOS convention).
class PhoneBackBar extends ConsumerWidget {
  /// Creates the bar.
  const PhoneBackBar({super.key, required this.below});

  /// The page the button goes back to.
  final PhonePage below;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    return PhoneBackButton(
      label: phonePageName(ref, l, below) ?? l.phoneBack,
      onBack: () => unawaited(Navigator.of(context).maybePop()),
    );
  }
}

/// The top app bar of [page] on Android (01 5.4): the up arrow back to [below] and the page's
/// own name as the title.
class PhoneTopAppBar extends ConsumerWidget {
  /// Creates the bar.
  const PhoneTopAppBar({super.key, required this.page, required this.below});

  /// The page the bar belongs to.
  final PhonePage page;

  /// The page the up arrow goes back to.
  final PhonePage below;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    return AndroidTopAppBar(
      backLabel: l.phoneBackTo(phonePageName(ref, l, below) ?? l.phoneBack),
      title: phonePageName(ref, l, page),
      onBack: () => unawaited(Navigator.of(context).maybePop()),
    );
  }
}

/// The back button: the chevron and [label], with the hit area `density.target-min`.
class PhoneBackButton extends StatelessWidget {
  /// Creates the button.
  const PhoneBackButton({super.key, required this.label, required this.onBack});

  /// The name of the page below.
  final String label;

  /// Goes back.
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final link = context.dsAccent.textLink;
    return Align(
      alignment: Alignment.centerLeft,
      child: DsTappable(
        key: const ValueKey('phone.back'),
        onTap: onBack,
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
                color: link,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s4),
              Flexible(
                child: Text(
                  dsIsolate(label),
                  overflow: TextOverflow.ellipsis,
                  style: dsTextStyle(DsType.body).copyWith(color: link),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The banner above the pages (01 6.6, 5.3 M7) over the providers: the engine's connection
/// state and the outbox entries waiting.
class PhoneConnectionBanner extends ConsumerWidget {
  /// Creates the banner.
  const PhoneConnectionBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) => PhoneBanner(
    phase: ref.watch(connectionStateProvider).value?.phase,
    waiting: ref.watch(outboxWaitingProvider).value ?? 0,
  );
}

/// "Offline" with the number of outbox entries [waiting] while the engine has no connection
/// ([phase] offline, backoff or suspended), "Update required" after `4007` (15 15), nothing
/// otherwise.
class PhoneBanner extends StatelessWidget {
  /// Creates the banner.
  const PhoneBanner({super.key, required this.phase, required this.waiting});

  /// The connection phase, or null before the engine reports one.
  final ConnectionPhase? phase;

  /// The outbox entries that are not final.
  final int waiting;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return switch (phase) {
      ConnectionPhase.updateRequired => const UpdateRequiredBanner(),
      ConnectionPhase.offline ||
      ConnectionPhase.backoff ||
      ConnectionPhase.suspended => DsBanner(
        key: const ValueKey('shell.offline'),
        tone: DsTone.info,
        text: l.phoneOfflineWaiting(waiting),
        liveRegion: true,
      ),
      _ => const SizedBox.shrink(),
    };
  }
}

/// The tab bar over the providers: the count of active runs on Running.
class PhoneTabBar extends ConsumerWidget {
  /// Creates the bar.
  const PhoneTabBar({super.key, required this.active, required this.onSelect});

  /// The active tab.
  final PhoneTab active;

  /// Selects a tab.
  final ValueChanged<PhoneTab> onSelect;

  @override
  Widget build(BuildContext context, WidgetRef ref) => PhoneTabBarView(
    active: active,
    running: ref.watch(activeRunCountProvider),
    onSelect: onSelect,
  );
}

/// The bottom tab bar: Spaces, Mentions, Running (with the count [running] of active runs)
/// and Me (01 5.3 M1).
class PhoneTabBarView extends StatelessWidget {
  /// Creates the bar.
  const PhoneTabBarView({
    super.key,
    required this.active,
    required this.onSelect,
    this.running = 0,
  });

  /// The active tab.
  final PhoneTab active;

  /// Selects a tab.
  final ValueChanged<PhoneTab> onSelect;

  /// The number of active runs.
  final int running;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    Widget icon(DsGlyph glyph, {required bool selected}) => DsIcon(
      glyph,
      size: DsIconSizeRole.md,
      color: selected ? context.dsAccent.textLink : colors.textSecondary,
      decorative: true,
    );
    NavigationDestination destination(
      PhoneTab tab,
      DsGlyph glyph,
      String label, {
      int count = 0,
    }) {
      Widget mark({required bool selected}) => count == 0
          ? icon(glyph, selected: selected)
          // The badge says what it counts: a screen reader never reads a bare number
          // before the tab's name (brief M5-10).
          : Semantics(
              label: l.phoneTabRunningCount(count),
              excludeSemantics: true,
              child: Badge.count(
                count: count,
                child: icon(glyph, selected: selected),
              ),
            );
      return NavigationDestination(
        key: ValueKey('phone.tab.${tab.name}'),
        icon: mark(selected: false),
        selectedIcon: mark(selected: true),
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
          DsIcons.stepWorkflow,
          l.phoneTabRunning,
          count: running,
        ),
        destination(PhoneTab.me, DsIcons.person, l.phoneTabMe),
      ],
    );
  }
}
