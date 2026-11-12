// SPDX-License-Identifier: Apache-2.0

/// The pages of the phone's stacks (docs/spec/01-product.md 5.3 M1 to M4 and M7; brief M5-02
/// Design note 2; the new chat view of 01 6.8 item 8, brief M5-15): each composes the views and controllers of M4-19 to M4-25 unchanged; only
/// the layout around them is the phone's.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../app/providers/session.dart';
import '../app/providers/target.dart';
import '../app/router.dart';
import '../app/target_resolver.dart';
import '../forum/new_chat_view.dart';
import '../forum/post_list_column.dart';
import '../mentions/mentions_view.dart';
import '../new_agent/builtin_agents_entry.dart';
import '../onboarding/onboarding_controller.dart';
import '../running/run_graph_screen.dart';
import '../running/running_now_view.dart';
import '../search/search_view.dart';
import '../settings/settings_shell.dart';
import '../sidebar/providers.dart';
import '../sidebar/sidebar_column.dart';
import '../status/status_route.dart';
import '../thread/history_column.dart';
import '../thread/providers.dart' show threadReadsProvider;
import '../viz/live_page.dart';
import 'me/emergency_entry.dart';
import 'me/logout_confirm.dart';
import 'phone_navigation.dart';
import 'providers.dart';
import 'terminal/hosts_screen.dart';
import 'terminal/terminal_session_screen.dart';

/// The content of one [page] of a stack.
class PhonePageView extends StatelessWidget {
  /// Creates the view.
  const PhonePageView({super.key, required this.page});

  /// The page.
  final PhonePage page;

  @override
  Widget build(BuildContext context) {
    final location = Uri.parse(page.path);
    final parsed = parseLocation(location);
    return switch (page.kind) {
      PhonePageKind.spaces => const SidebarColumn(phone: true),
      PhonePageKind.mentions => const MentionsView(),
      PhonePageKind.running => const RunningNowView(),
      PhonePageKind.runGraph when parsed is RunGraphLocation => RunGraphScreen(
        key: ValueKey('phone.runGraph.${parsed.runId.value}'),
        location: parsed,
      ),
      PhonePageKind.me => const PhoneMePage(),
      PhonePageKind.live => const LivePage(phone: true),
      PhonePageKind.channel when parsed is ChannelLocation => PhoneChannelPage(
        channelId: parsed.channelId,
      ),
      PhonePageKind.thread when parsed is ThreadLocation => PhoneThreadPage(
        location: location,
      ),
      // The view and controller of M4-41 unchanged, as a page (brief M5-15 Design note 2).
      PhonePageKind.newChat when parsed is NewChatLocation => NewChatView(
        key: ValueKey('phone.newChat.${parsed.channelId.value}'),
        forum: parsed.channelId,
        sentEntry: parsed.sentEntry,
      ),
      PhonePageKind.search when parsed is ViewLocation => SearchView(
        key: ValueKey('searchView.${parsed.query}'),
        initialQuery: parsed.query ?? '',
      ),
      PhonePageKind.settingsSection when parsed is SectionLocation =>
        PhoneSettingsSectionPage(segment: parsed.segments.firstOrNull),
      PhonePageKind.status => const AppStatusPage(notFound: PhoneNotFound()),
      PhonePageKind.hosts => const HostsScreen(),
      PhonePageKind.terminal
          when parsed is HostLocation && parsed.hostId != null =>
        TerminalSessionScreen(
          key: ValueKey('phone.terminal.${parsed.hostId!.value}'),
          hostId: parsed.hostId!.value,
        ),
      PhonePageKind.builtinAgents => const PhoneBuiltinAgentsPage(
        notFound: PhoneNotFound(),
      ),
      PhonePageKind.emergency => const PhoneEmergencyPage(
        notFound: PhoneNotFound(),
      ),
      PhonePageKind.emergencyPerson
          when parsed is SectionLocation && parsed.segments.length == 2 =>
        PhoneEmergencyPersonPage(
          userId: parsed.segments[1],
          notFound: const PhoneNotFound(),
        ),
      _ => const PhoneNotFound(),
    };
  }
}

/// "Not found or no access" (01 6.7): a path without a target, an invisible target, and the
/// administration, which the phone does not offer beyond the emergency actions (01 4: desktop
/// and web console; R-PR-07).
class PhoneNotFound extends StatelessWidget {
  /// Creates the page.
  const PhoneNotFound({super.key});

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(DsSpace.s16),
      child: Text(
        AppLocalizations.of(context).accessNotFoundOrNoAccess,
        key: const ValueKey('target.notFound'),
        textAlign: TextAlign.center,
        style: dsTextStyle(DsType.body)
            .copyWith(color: context.dsColors.textSecondary),
      ),
    ),
  );
}

/// The mark of a page whose target this device does not hold while the server could not be
/// asked ([TargetStatus.unknown], for example a push tapped without a network): the page waits
/// and asks again when the connection is live ([_askAgainWhenLive]).
class PhoneTargetWaiting extends StatelessWidget {
  /// Creates the mark.
  const PhoneTargetWaiting({super.key});

  @override
  Widget build(BuildContext context) => const Center(
    key: ValueKey('target.waiting'),
    child: CircularProgressIndicator(),
  );
}

/// Asks the target check of [path] again when the connection turns live while the check could
/// not ask the server (15 11: a link waits until its target is loaded; `TargetStatus.unknown`
/// "waits and tries again later"); [again] runs with it.
void _askAgainWhenLive(WidgetRef ref, String path, {VoidCallback? again}) {
  ref.listen(connectionStateProvider, (before, now) {
    if (now.value?.phase != ConnectionPhase.live ||
        before?.value?.phase == ConnectionPhase.live) {
      return;
    }
    final status = ref.read(targetStatusProvider(path)).value;
    if (status != TargetStatus.unknown) return;
    ref.invalidate(targetStatusProvider(path));
    again?.call();
  });
}

/// A channel on the phone: the post list of a forum (M2) or the main history of a text
/// channel; "Not found or no access" for an invisible one, and [PhoneTargetWaiting] for one
/// this device does not hold while the server cannot be asked.
class PhoneChannelPage extends ConsumerWidget {
  /// Creates the page.
  const PhoneChannelPage({super.key, required this.channelId});

  /// The channel.
  final ChannelId channelId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final path = '/c/${channelId.value}';
    final status = ref.watch(targetStatusProvider(path)).value;
    if (status == TargetStatus.notFound) return const PhoneNotFound();
    // The channel itself comes with the sync once the connection is live.
    _askAgainWhenLive(ref, path);
    final channel = ref
        .watch(storedChannelOfRouteProvider(channelId.value))
        .value;
    if (channel == null) {
      return status == TargetStatus.unknown
          ? const PhoneTargetWaiting()
          : const SizedBox.expand();
    }
    if (channel.kind == ChannelKind.forum) {
      return PostListColumn(
        key: ValueKey('postList.${channelId.value}'),
        channelId: channelId,
      );
    }
    final main = channel.mainThreadId;
    if (main == null) return const SizedBox.expand();
    return HistoryColumn(
      key: ValueKey('historyColumn.${main.value}.null'),
      threadId: main,
    );
  }
}

/// A thread on the phone (M3, M4, M7): the history with its compact live box, cards and
/// outbox rows; [location] may name a message to jump to (`/t/{id}?m={id}`, 15 11). A thread
/// this device does not hold while the server cannot be asked shows [PhoneTargetWaiting] over
/// the empty history until the thread is stored.
class PhoneThreadPage extends ConsumerWidget {
  /// Creates the page.
  const PhoneThreadPage({super.key, required this.location});

  /// The thread path.
  final Uri location;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final parsed = parseLocation(location);
    if (parsed is! ThreadLocation) return const PhoneNotFound();
    final path = location.toString();
    final thread = parsed.threadId;
    final status = ref.watch(targetStatusProvider(path)).value;
    if (status == TargetStatus.notFound) return const PhoneNotFound();
    // The history loads a thread it does not hold once (15 11); that read failed with the
    // check, so it is asked again with it.
    _askAgainWhenLive(
      ref,
      path,
      again: () => unawaited(
        ref
            .read(threadReadsProvider)
            .loadThread(thread)
            .then<void>((_) {}, onError: (Object _) {}),
      ),
    );
    final waiting =
        status == TargetStatus.unknown &&
        ref.watch(storedThreadProvider(thread.value)).value == null;
    // Always the same tree, so the history keeps its state when the mark goes.
    return Stack(
      fit: StackFit.expand,
      children: [
        HistoryColumn(
          key: ValueKey(
            'historyColumn.${thread.value}.${parsed.messageId?.value}',
          ),
          threadId: thread,
          jumpTo: parsed.messageId,
        ),
        if (waiting) const PhoneTargetWaiting(),
      ],
    );
  }
}

/// The settings sections the phone offers: everything of D8 but the keyboard shortcuts, plus
/// Security with the app lock (brief M5-08).
const List<SettingsSection> phoneSettingsSections = [
  SettingsSection.appearance,
  SettingsSection.notifications,
  SettingsSection.language,
  SettingsSection.devices,
  SettingsSection.privacy,
  SettingsSection.security,
  SettingsSection.about,
];

/// The Me tab: the signed-in person, the settings sections (devices and sessions among them,
/// M4-25), "Live" (M10, M6-16), the status page (M7-21), "Built-in agents" for an owner with
/// built-in agents ([BuiltinAgentsEntry], A1-09), "Emergency" for an owner or a manager of a
/// space ([EmergencyEntry], M5-09) and "Log out" (E7), which asks once ([confirmLogout]).
class PhoneMePage extends ConsumerWidget {
  /// Creates the page.
  const PhoneMePage({super.key});

  /// "Log out": asks once, naming the outbox entries that still wait, then logs out.
  static Future<void> _logOut(BuildContext context, WidgetRef ref) async {
    // Read before the dialog: the page may be gone when it closes.
    final control = ref.read(onboardingControllerProvider.notifier);
    final waiting = ref.read(outboxWaitingProvider).value ?? 0;
    if (!await confirmLogout(context, waiting: waiting)) return;
    await control.logout();
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final name = ref.watch(ownPrincipalProvider).value?.displayName;
    return ListView(
      key: const ValueKey('phone.me'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      children: [
        Padding(
          padding: const EdgeInsets.all(DsSpace.s16),
          child: Semantics(
            header: true,
            child: Text(
              name == null ? l.phoneTabMe : dsIsolate(name),
              key: const ValueKey('phone.me.name'),
              style: dsTextStyle(DsType.heading)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
        ),
        for (final section in phoneSettingsSections)
          ListTile(
            key: ValueKey('phone.me.${section.segment}'),
            title: Text(section.label(l)),
            trailing: DsIcon(
              DsIcons.actionExpand,
              size: DsIconSizeRole.sm,
              color: colors.textSecondary,
              decorative: true,
            ),
            onTap: () => context.go(section.path),
          ),
        ListTile(
          key: const ValueKey('phone.me.live'),
          title: Text(l.meLiveEntry),
          subtitle: Text(l.meLiveEntryHint),
          trailing: DsIcon(
            DsIcons.actionExpand,
            size: DsIconSizeRole.sm,
            color: colors.textSecondary,
            decorative: true,
          ),
          onTap: () => context.go(livePath),
        ),
        ListTile(
          key: const ValueKey('phone.me.hosts'),
          title: Text(l.meHostsEntry),
          subtitle: Text(l.meHostsEntryHint),
          trailing: DsIcon(
            DsIcons.actionExpand,
            size: DsIconSizeRole.sm,
            color: colors.textSecondary,
            decorative: true,
          ),
          onTap: () => context.go('/hosts'),
        ),
        ListTile(
          key: const ValueKey('phone.me.status'),
          title: Text(l.statusNavEntry),
          trailing: DsIcon(
            DsIcons.actionExpand,
            size: DsIconSizeRole.sm,
            color: colors.textSecondary,
            decorative: true,
          ),
          onTap: () => context.go(statusPath),
        ),
        const BuiltinAgentsEntry(),
        const EmergencyEntry(),
        // The gap is padding of the button, not an item of its own: an empty spacer would be
        // an empty stop and an extra item of the list for screen readers (brief M5-10).
        Padding(
          padding: const EdgeInsets.fromLTRB(
            DsSpace.s16,
            DsSpace.s16,
            DsSpace.s16,
            0,
          ),
          child: DsButton(
            key: const ValueKey('shell.logout'),
            label: l.onboardingLogOut,
            style: DsButtonStyle.danger,
            expand: true,
            onPressed: () => unawaited(_logOut(context, ref)),
          ),
        ),
      ],
    );
  }
}

/// One settings section on the phone, the same body as on the desktop (M4-25).
class PhoneSettingsSectionPage extends StatelessWidget {
  /// Creates the page.
  const PhoneSettingsSectionPage({super.key, required this.segment});

  /// The path segment of the section.
  final String? segment;

  @override
  Widget build(BuildContext context) {
    final section = SettingsSection.of(segment);
    if (section == null) return const PhoneNotFound();
    return SingleChildScrollView(
      key: ValueKey('phone.settings.${section.segment}'),
      padding: const EdgeInsets.all(DsSpace.s16),
      child: sectionBody(section),
    );
  }
}
