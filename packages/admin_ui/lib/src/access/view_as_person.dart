// SPDX-License-Identifier: Apache-2.0

/// "View as person" as a read-only snapshot (docs/spec/01-product.md 5.2 A4; 07 9.2, SR6;
/// brief M4-27 Design note 2 and fixed requirement 17).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../grants/grant_labels.dart';
import '../self/self_feedback.dart';

/// The spaces and channels [userId] sees, from `viewAsUser`: owners only, with step-up and an
/// audit entry on the server (05 `access.view_as`). The step-up comes from the interceptor
/// before the answer; a cancelled step-up shows nothing.
///
/// The answer is shown in a clear frame with "Viewing as `<name>` (read-only). Nothing is sent.
/// Logged." and the end of the view (`expires_at`); nothing in the frame is a control. Direct
/// messages of the person never appear, because the server leaves them out (SR6).
///
/// Example:
/// ```dart
/// ViewAsPerson(repository: admin, userId: sam.id, name: 'Sam', onExit: close);
/// ```
class ViewAsPerson extends StatefulWidget {
  /// Creates the view.
  const ViewAsPerson({
    super.key,
    required this.repository,
    required this.userId,
    required this.name,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onExit,
  });

  /// The grant calls.
  final AccessRepository repository;

  /// The person.
  final String userId;

  /// The display name of the person, for the step-up purpose.
  final String name;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Leaves the view, or null for no "Exit" button.
  final VoidCallback? onExit;

  @override
  State<ViewAsPerson> createState() => _ViewAsPersonState();
}

class _ViewAsPersonState extends State<ViewAsPerson> {
  ViewAsResult? _snapshot;
  SelfFailure? _failure;

  @override
  void initState() {
    super.initState();
    // After the first frame: the step-up dialog may open over this view (05 2.4).
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _load(AppLocalizations.of(context));
    });
  }

  Future<void> _load(AppLocalizations l) async {
    try {
      final snapshot = await widget.repository.viewAs(
        widget.userId,
        purpose: l.stepUpPurposeViewAs(widget.name),
      );
      if (mounted) setState(() => _snapshot = snapshot);
    } on Object catch (e) {
      final failure = adminFailure(l, e);
      if (mounted) setState(() => _failure = failure);
    }
  }

  String _until(BuildContext context, DateTime end) {
    final format = L10nFormat(Localizations.localeOf(context));
    String zone(String timeZone) =>
        '${format.relativeDay(end, now: widget.clock(), timeZone: timeZone)} '
        '${format.time(end, timeZone: timeZone)}';
    try {
      return zone(widget.timeZone);
    } on ArgumentError {
      return zone('UTC');
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final snapshot = _snapshot;
    final failure = _failure;
    if (failure != null) {
      return DsBanner(
        key: const ValueKey('viewAs.failed'),
        tone: DsTone.warning,
        text: failure.text,
      );
    }
    if (snapshot == null) return const SizedBox(key: ValueKey('viewAs.wait'));
    final spaces = {for (final s in snapshot.spaces) s.id: s.name};
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    Widget row(String key, String name, ViewAsItem item) => Padding(
      key: ValueKey(key),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(flex: 2, child: Text(name, style: body)),
          Expanded(
            flex: 3,
            child: Text(
              levelWithCapabilities(l, item.level, item.capabilities),
              style: secondary,
            ),
          ),
        ],
      ),
    );
    return Container(
      decoration: BoxDecoration(
        border: Border.all(color: colors.accentBg, width: 2),
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            color: colors.accentSubtle,
            padding: const EdgeInsets.all(DsSpace.s12),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Semantics(
                        liveRegion: true,
                        child: Text(
                          l.adminViewAsBanner(snapshot.user.displayName),
                          key: const ValueKey('viewAs.banner'),
                          style: dsTextStyle(DsType.bodyStrong)
                              .copyWith(color: colors.textPrimary),
                        ),
                      ),
                      Text(
                        l.adminViewAsUntil(_until(context, snapshot.expiresAt)),
                        key: const ValueKey('viewAs.until'),
                        style: secondary,
                      ),
                    ],
                  ),
                ),
                if (widget.onExit case final exit?)
                  TextButton(
                    key: const ValueKey('viewAs.exit'),
                    onPressed: exit,
                    child: Text(l.adminViewAsExit),
                  ),
              ],
            ),
          ),
          // The snapshot itself: text only, so nothing in it can write (07 9.2).
          Padding(
            key: const ValueKey('viewAs.frame'),
            padding: const EdgeInsets.all(DsSpace.s12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (snapshot.spaces.isEmpty && snapshot.channels.isEmpty)
                  Text(l.adminViewAsNothing, style: secondary),
                if (snapshot.spaces.isNotEmpty) ...[
                  Text(l.adminViewAsSpaces, style: secondary),
                  for (final s in snapshot.spaces)
                    row('viewAs.space.${s.id}', s.name, s),
                ],
                if (snapshot.channels.isNotEmpty) ...[
                  const SizedBox(height: DsSpace.s8),
                  Text(l.adminViewAsChannels, style: secondary),
                  for (final c in snapshot.channels)
                    row('viewAs.channel.${c.id}', switch (spaces[c.spaceId]) {
                      final space? => '$space/${c.name}',
                      null => c.name,
                    }, c),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}
