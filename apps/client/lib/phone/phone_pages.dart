// SPDX-License-Identifier: Apache-2.0

/// The pages of the phone's stacks (docs/spec/01-product.md 5.3 M1 to M4 and M7; brief M5-02
/// Design note 2): each composes the views and controllers of M4-19 to M4-25 unchanged; only
/// the layout around them is the phone's.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/target.dart';
import '../app/router.dart';
import '../app/target_resolver.dart';
import '../forum/post_list_column.dart';
import '../mentions/mentions_view.dart';
import '../onboarding/onboarding_controller.dart';
import '../running/run_graph_screen.dart';
import '../running/running_now_view.dart';
import '../search/search_view.dart';
import '../settings/settings_shell.dart';
import '../sidebar/providers.dart';
import '../sidebar/sidebar_column.dart';
import '../thread/history_column.dart';
import 'me/emergency_entry.dart';
import 'phone_navigation.dart';

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
      PhonePageKind.channel when parsed is ChannelLocation => PhoneChannelPage(
        channelId: parsed.channelId,
      ),
      PhonePageKind.thread when parsed is ThreadLocation => PhoneThreadPage(
        location: location,
      ),
      PhonePageKind.search when parsed is ViewLocation => SearchView(
        key: ValueKey('searchView.${parsed.query}'),
        initialQuery: parsed.query ?? '',
      ),
      PhonePageKind.settingsSection when parsed is SectionLocation =>
        PhoneSettingsSectionPage(segment: parsed.segments.firstOrNull),
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

/// A channel on the phone: the post list of a forum (M2) or the main history of a text
/// channel; "Not found or no access" for an invisible one.
class PhoneChannelPage extends ConsumerWidget {
  /// Creates the page.
  const PhoneChannelPage({super.key, required this.channelId});

  /// The channel.
  final ChannelId channelId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref
        .watch(targetStatusProvider('/c/${channelId.value}'))
        .value;
    if (status == TargetStatus.notFound) return const PhoneNotFound();
    final channel = ref
        .watch(storedChannelOfRouteProvider(channelId.value))
        .value;
    if (channel == null) return const SizedBox.expand();
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
/// outbox rows; [location] may name a message to jump to (`/t/{id}?m={id}`, 15 11).
class PhoneThreadPage extends ConsumerWidget {
  /// Creates the page.
  const PhoneThreadPage({super.key, required this.location});

  /// The thread path.
  final Uri location;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final parsed = parseLocation(location);
    if (parsed is! ThreadLocation) return const PhoneNotFound();
    final status = ref.watch(targetStatusProvider(location.toString())).value;
    if (status == TargetStatus.notFound) return const PhoneNotFound();
    return HistoryColumn(
      key: ValueKey(
        'historyColumn.${parsed.threadId.value}.${parsed.messageId?.value}',
      ),
      threadId: parsed.threadId,
      jumpTo: parsed.messageId,
    );
  }
}

/// The settings sections the phone offers: everything of D8 but the keyboard shortcuts.
const List<SettingsSection> phoneSettingsSections = [
  SettingsSection.appearance,
  SettingsSection.notifications,
  SettingsSection.language,
  SettingsSection.devices,
  SettingsSection.privacy,
  SettingsSection.about,
];

/// The Me tab: the signed-in person, the settings sections (devices and sessions among them,
/// M4-25), "Emergency" for an owner or a manager of a space ([EmergencyEntry], M5-09) and
/// "Log out" (E7).
class PhoneMePage extends ConsumerWidget {
  /// Creates the page.
  const PhoneMePage({super.key});

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
        const EmergencyEntry(),
        const SizedBox(height: DsSpace.s16),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: DsSpace.s16),
          child: DsButton(
            key: const ValueKey('shell.logout'),
            label: l.onboardingLogOut,
            style: DsButtonStyle.danger,
            expand: true,
            onPressed: () => unawaited(
              ref.read(onboardingControllerProvider.notifier).logout(),
            ),
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
