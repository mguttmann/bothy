// SPDX-License-Identifier: Apache-2.0

/// The own sessions: device, method and last use; end one after one confirmation
/// (docs/spec/08-identity.md 7, 10: `session.revoke` needs no step-up; R-SY-14).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/self_repository.dart';
import 'device_tile.dart' show activityText;
import 'revoke_confirm.dart';
import 'self_feedback.dart';

/// The name of the login method [method].
String authMethodText(AppLocalizations l, SelfAuthMethod method) =>
    switch (method) {
      SelfAuthMethod.oidc => l.sessionsMethodOidc,
      SelfAuthMethod.trustedHeader => l.sessionsMethodTrustedHeader,
      SelfAuthMethod.passkey => l.sessionsMethodPasskey,
      SelfAuthMethod.passwordTotp => l.sessionsMethodPasswordTotp,
      SelfAuthMethod.enrollmentCode => l.sessionsMethodEnrollmentCode,
      SelfAuthMethod.devicePairing => l.sessionsMethodDevicePairing,
      SelfAuthMethod.other => l.sessionsMethodOther,
    };

/// The list of the person's sessions with "End session".
///
/// Ending a session asks once ([RevokeConfirmDialog]) and never for a step-up (05: the action
/// `session.revoke` has none, and the view adds none). Ending the current session calls
/// [onCurrentSessionEnded] (the app logs out and wipes, R-SY-14). A `403` hides the action of
/// that session and shows its reason.
///
/// Example:
/// ```dart
/// MySessionsView(repository: repository, timeZone: 'Europe/Berlin');
/// ```
class MySessionsView extends StatefulWidget {
  /// Creates the view.
  const MySessionsView({
    super.key,
    required this.repository,
    required this.timeZone,
    this.clock = DateTime.now,
    this.onCurrentSessionEnded,
    this.onChanged,
    this.reload,
  });

  /// The sessions and their action.
  final SelfRepository repository;

  /// The time zone of the person (`users.time_zone`).
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after the current session was ended.
  final VoidCallback? onCurrentSessionEnded;

  /// Runs after a session was ended.
  final VoidCallback? onChanged;

  /// Loads the list again when it notifies.
  final Listenable? reload;

  @override
  State<MySessionsView> createState() => _MySessionsViewState();
}

class _MySessionsViewState extends State<MySessionsView> {
  List<SelfSession>? _sessions;
  bool _loadFailed = false;
  bool _busy = false;
  SelfNotice? _notice;
  final Set<String> _denied = {};

  @override
  void initState() {
    super.initState();
    widget.reload?.addListener(_load);
    _load();
  }

  @override
  void didUpdateWidget(MySessionsView old) {
    super.didUpdateWidget(old);
    if (old.reload != widget.reload) {
      old.reload?.removeListener(_load);
      widget.reload?.addListener(_load);
    }
  }

  @override
  void dispose() {
    widget.reload?.removeListener(_load);
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final sessions = await widget.repository.sessions();
      if (!mounted) return;
      setState(() {
        _sessions = sessions;
        _loadFailed = false;
      });
    } on Object {
      if (!mounted) return;
      setState(() => _loadFailed = true);
    }
  }

  Future<void> _end(SelfSession session) async {
    final l = AppLocalizations.of(context);
    final confirmed = await RevokeConfirmDialog.show(
      context,
      title: l.sessionsEndTitle,
      body: session.current ? l.sessionsEndCurrentBody : l.sessionsEndBody,
      confirmLabel: l.sessionsEnd,
    );
    if (confirmed == null || !mounted) return;
    setState(() => _busy = true);
    try {
      await widget.repository.revokeSession(session.id);
      if (session.current) {
        // R-SY-14: the own session ended, so the app logs out and deletes its data.
        widget.onCurrentSessionEnded?.call();
        return;
      }
      if (!mounted) return;
      setState(() => _notice = SelfNotice(l.sessionsEnded));
      widget.onChanged?.call();
    } on Object catch (e) {
      final failure = selfFailure(l, e);
      if (!mounted || failure == null) return;
      setState(() {
        if (failure.denied) _denied.add(session.id);
        _notice = SelfNotice(failure.text, tone: DsTone.danger);
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final sessions = _sessions;
    final now = widget.clock();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.sessionsTitle,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        if (_notice case final notice?) selfNoticeBanner(notice),
        if (_loadFailed && sessions == null)
          DsBanner(
            key: const ValueKey('sessions.loadFailed'),
            tone: DsTone.warning,
            text: l.selfLoadFailed,
            actionLabel: l.selfRetry,
            onAction: _load,
          )
        else if (sessions != null)
          for (final session in sessions) ...[
            _SessionTile(
              session: session,
              details: [
                authMethodText(l, session.authMethod),
                l.sessionsLastUsed(
                  activityText(
                    context,
                    session.lastUsedAt,
                    now: now,
                    timeZone: widget.timeZone,
                  ),
                ),
              ],
              onEnd: _busy || _denied.contains(session.id)
                  ? null
                  : () => _end(session),
            ),
            Divider(height: 1, color: colors.borderSubtle),
          ],
      ],
    );
  }
}

class _SessionTile extends StatelessWidget {
  const _SessionTile({
    required this.session,
    required this.details,
    required this.onEnd,
  });

  final SelfSession session;
  final List<String> details;
  final VoidCallback? onEnd;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Padding(
      key: ValueKey('session.${session.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s8,
        crossAxisAlignment: WrapCrossAlignment.center,
        alignment: WrapAlignment.spaceBetween,
        children: [
          ConstrainedBox(
            constraints: const BoxConstraints(minWidth: 240),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Wrap(
                  spacing: DsSpace.s8,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    Text(
                      session.deviceName ?? l.sessionsUnknownDevice,
                      style: dsTextStyle(DsType.bodyStrong)
                          .copyWith(color: colors.textPrimary),
                    ),
                    if (session.current)
                      DsBadge(
                        key: const ValueKey('session.current'),
                        text: l.sessionsCurrent,
                        background: colors.accentSubtle,
                        foreground: colors.textPrimary,
                      ),
                  ],
                ),
                Wrap(
                  spacing: DsSpace.s12,
                  children: [
                    for (final detail in details)
                      Text(
                        detail,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: colors.textSecondary),
                      ),
                  ],
                ),
              ],
            ),
          ),
          if (onEnd != null)
            OutlinedButton(
              key: ValueKey('session.${session.id}.end'),
              onPressed: onEnd,
              child: Text(l.sessionsEnd),
            ),
        ],
      ),
    );
  }
}
