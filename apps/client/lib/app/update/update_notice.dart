// SPDX-License-Identifier: Apache-2.0

/// The update notices of the main view (docs/spec/18-release.md 9, 15 15; brief M7-13
/// requirements 14 to 16): "Update required" after `4007` with the update path, and "Update
/// recommended" below `client.recommended_version.<platform>`, dismissible once per version.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart'
    show
        ClientVersionAllowed,
        ClientVersionDecision,
        ClientVersionUpdateRecommended;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../platform/secure_store.dart';
import '../providers/ports.dart';
import '../providers/session.dart';

part 'update_notice.g.dart';

/// The version gate's decision for the signed-in instance (18 9 item 2): the decision of the
/// start, then every new one before a reconnect; allowed while no instance is signed in.
@Riverpod(keepAlive: true)
Stream<ClientVersionDecision> clientVersion(Ref ref) async* {
  final versions = ref.watch(signedInServicesProvider)?.versions;
  if (versions == null) {
    yield const ClientVersionAllowed();
    return;
  }
  yield versions.current;
  yield* versions.changes;
}

/// The recommended version whose notice the person dismissed, kept in the app preferences.
@Riverpod(keepAlive: true)
class UpdateNoticeDismissal extends _$UpdateNoticeDismissal {
  @override
  Future<String?> build() =>
      AppPreferences(ref.watch(appPortsProvider).secureBackend)
          .dismissedUpdateNotice();

  /// Hides the notice for [version] until the instance recommends another one.
  Future<void> dismiss(String version) async {
    state = AsyncData(version);
    await AppPreferences(ref.read(appPortsProvider).secureBackend)
        .setDismissedUpdateNotice(version);
  }
}

/// The version the notice "Update recommended" names, or null when none shows: the app is
/// below `client.recommended_version.<platform>` and the person has not dismissed that version.
@Riverpod(keepAlive: true)
String? recommendedUpdate(Ref ref) {
  final decision = ref.watch(clientVersionProvider).value;
  if (decision is! ClientVersionUpdateRecommended) return null;
  final dismissal = ref.watch(updateNoticeDismissalProvider);
  // Until the preference is read nothing shows, so a dismissed notice never flashes.
  if (!dismissal.hasValue) return null;
  return dismissal.value == decision.recommendedVersion
      ? null
      : decision.recommendedVersion;
}

/// Opens the update path of the build through the ports; does nothing without one.
Future<void> openUpdatePath(WidgetRef ref) async {
  final ports = ref.read(appPortsProvider);
  final path = ports.updatePath;
  if (path.available) await ports.updateLauncher.open(path);
}

/// "Update required" after `4007` (04 6.4, 15 15) with the action that opens the update path
/// of the build, when it has one.
class UpdateRequiredBanner extends ConsumerWidget {
  /// Creates the banner.
  const UpdateRequiredBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final available = ref.watch(appPortsProvider).updatePath.available;
    return UpdateRequiredBannerView(
      onUpdate: available ? () => unawaited(openUpdatePath(ref)) : null,
    );
  }
}

/// The look of [UpdateRequiredBanner]: the text and, with [onUpdate], the action "Get the
/// update".
class UpdateRequiredBannerView extends StatelessWidget {
  /// Creates the banner.
  const UpdateRequiredBannerView({super.key, this.onUpdate});

  /// Opens the update path, or null when the build has none.
  final VoidCallback? onUpdate;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return DsBanner(
      key: const ValueKey('shell.updateRequired'),
      tone: DsTone.danger,
      text: l.shellUpdateRequired,
      liveRegion: true,
      actionLabel: onUpdate == null ? null : l.updateGet,
      onAction: onUpdate,
    );
  }
}

/// "Version X is available" while the app is below `client.recommended_version.<platform>`
/// (brief M7-13 requirement 14): the app keeps working; the action opens the update path, the
/// close button hides the notice for this version.
class RecommendedUpdateBanner extends ConsumerWidget {
  /// Creates the banner.
  const RecommendedUpdateBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final version = ref.watch(recommendedUpdateProvider);
    if (version == null) return const SizedBox.shrink();
    final available = ref.watch(appPortsProvider).updatePath.available;
    return RecommendedUpdateBannerView(
      version: version,
      onUpdate: available ? () => unawaited(openUpdatePath(ref)) : null,
      onDismiss: () => unawaited(
        ref.read(updateNoticeDismissalProvider.notifier).dismiss(version),
      ),
    );
  }
}

/// The look of [RecommendedUpdateBanner]: the text with [version], the action "Get the
/// update" with [onUpdate] and a close button.
class RecommendedUpdateBannerView extends StatelessWidget {
  /// Creates the banner.
  const RecommendedUpdateBannerView({
    super.key,
    required this.version,
    required this.onDismiss,
    this.onUpdate,
  });

  /// The recommended version.
  final String version;

  /// Opens the update path, or null when the build has none.
  final VoidCallback? onUpdate;

  /// Hides the notice for [version].
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Row(
      key: const ValueKey('shell.updateRecommended'),
      children: [
        Expanded(
          child: DsBanner(
            tone: DsTone.info,
            text: l.updateRecommended(version),
            liveRegion: true,
            actionLabel: onUpdate == null ? null : l.updateGet,
            onAction: onUpdate,
          ),
        ),
        IconButton(
          key: const ValueKey('shell.updateRecommended.dismiss'),
          tooltip: l.updateDismiss,
          onPressed: onDismiss,
          icon: DsIcon(
            DsIcons.actionDismiss,
            size: DsIconSizeRole.sm,
            color: context.dsColors.textSecondary,
            decorative: true,
          ),
        ),
      ],
    );
  }
}
