// SPDX-License-Identifier: Apache-2.0

/// A person's detail: owner flag, state, credentials, devices and sessions; disable, enable and
/// a new enrollment code (docs/spec/01-product.md 5.2 A1; 08 3.2, 5.2, 5.4, 10, 11; 07 SR7).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../access/who_has_access.dart' show AdminViewAs;
import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/self_repository.dart';
import '../self/confirm_device_dialog.dart';
import '../self/device_tile.dart';
import '../self/my_sessions_view.dart' show authMethodText;
import '../self/self_feedback.dart';
import 'credentials_section.dart';
import 'enrollment_code_view.dart';
import 'people_list.dart' show personStatusText;

/// The detail of the person [userId].
///
/// Every change asks once ([ConfirmChange]) and then runs through [repository]; the step-up
/// interceptor asks for the step-up where the server answers `401 step_up_required` (05:
/// `user.disable`, `user.enable`, `user.update`, `identity.unlink`, `passkey.revoke`,
/// `device.approve`, `device.revoke` have step-up; `session.revoke` has none). Disabling says
/// that every session ends (08 11); enabling explains `409 i1_violation` (M4-28 resolves
/// it). A section whose list the server refuses shows the reason instead (fixed requirement
/// 9). [onBack] returns to the list; [onViewAs] adds "View as this person", which the host
/// starts (the console's stored view, brief M6-04 Design note 3).
class PersonDetail extends StatefulWidget {
  /// Creates the detail.
  const PersonDetail({
    super.key,
    required this.repository,
    required this.userId,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onBack,
    this.onChanged,
    this.onViewAs,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The person.
  final String userId;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Returns to the list, or null for no button.
  final VoidCallback? onBack;

  /// Runs after a change.
  final VoidCallback? onChanged;

  /// Starts the view as this person in the host, or null for no button. The detail only hands
  /// the person over: whether the caller may view is the server's answer to `viewAsUser`.
  final AdminViewAs? onViewAs;

  @override
  State<PersonDetail> createState() => _PersonDetailState();
}

/// A loaded part of the page: its value or the server's problem.
final class _Part<T> {
  const _Part.value(T this.value) : problem = null;

  const _Part.problem(this.problem) : value = null;

  final T? value;
  final ApiProblem? problem;
}

class _PersonDetailState extends State<PersonDetail>
    with AdminActionState<PersonDetail> {
  _Part<AdminPerson>? _person;
  _Part<AdminCredentials>? _credentials;
  _Part<List<SelfDevice>>? _devices;
  _Part<List<SelfSession>>? _sessions;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(PersonDetail old) {
    super.didUpdateWidget(old);
    if (old.userId != widget.userId) _load();
  }

  Future<_Part<T>> _part<T>(Future<T> Function() load) async {
    try {
      return _Part.value(await load());
    } on Object catch (e) {
      return _Part.problem(loadProblem(e));
    }
  }

  Future<void> _load() async {
    final r = widget.repository;
    final id = widget.userId;
    final results = await Future.wait([
      _part(() => r.person(id)),
      _part(() => r.credentials(id)),
      _part(() => r.devicesOf(id)),
      _part(() => r.sessionsOf(id)),
    ]);
    if (!mounted) return;
    setState(() {
      _person = results[0] as _Part<AdminPerson>;
      _credentials = results[1] as _Part<AdminCredentials>;
      _devices = results[2] as _Part<List<SelfDevice>>;
      _sessions = results[3] as _Part<List<SelfSession>>;
    });
  }

  Future<void> _change(
    String key,
    Future<SelfNotice?> Function() change,
  ) async {
    final ok = await runChange(key, change);
    if (ok) widget.onChanged?.call();
    if (mounted) await _load();
  }

  Future<void> _disable(AdminPerson p) async {
    final l = AppLocalizations.of(context);
    final reason = await ConfirmChange.show(
      context,
      title: l.adminDisableTitle(p.displayName),
      body: l.adminDisableBody(p.displayName),
      confirmLabel: l.adminPersonDisable,
      destructive: true,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    await _change('disable', () async {
      await widget.repository.disablePerson(
        p.id,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: l.stepUpPurposeDisablePerson(p.displayName),
      );
      return SelfNotice(l.adminPersonDisabled(p.displayName));
    });
  }

  Future<void> _enable(AdminPerson p) async {
    final l = AppLocalizations.of(context);
    final reason = await ConfirmChange.show(
      context,
      title: l.adminEnableTitle(p.displayName),
      body: l.adminEnableBody(p.displayName),
      confirmLabel: l.adminPersonEnable,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    await _change('enable', () async {
      await widget.repository.enablePerson(
        p.id,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: l.stepUpPurposeEnablePerson(p.displayName),
      );
      return SelfNotice(l.adminPersonEnabled(p.displayName));
    });
  }

  Future<void> _newCode(AdminPerson p) async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.adminNewCodeTitle(p.displayName),
      body: l.adminNewCodeBody(p.displayName),
      confirmLabel: l.adminPersonNewCode,
    );
    if (confirmed == null || !mounted) return;
    EnrollmentCodeInfo? code;
    final ok = await runChange('code', () async {
      code = await widget.repository.newEnrollmentCode(
        p.id,
        purpose: l.stepUpPurposeNewCode(p.displayName),
      );
      return null;
    });
    final shown = code;
    // 08 3.2: shown once; the dialog holds the only copy.
    code = null;
    if (!ok || shown == null || !mounted) return;
    await EnrollmentCodeView.show(
      context,
      name: p.displayName,
      code: shown,
      timeZone: widget.timeZone,
      clock: widget.clock,
    );
  }

  Future<void> _unlink(AdminPerson p, AdminIdentity identity) async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.adminUnlinkTitle(identity.subject),
      body: l.adminUnlinkBody(p.displayName),
      confirmLabel: l.adminUnlink,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    await _change('identity.${identity.id}', () async {
      await widget.repository.unlinkIdentity(
        p.id,
        identity.id,
        purpose: l.stepUpPurposeUnlinkIdentity(p.displayName),
      );
      return SelfNotice(l.adminUnlinked(identity.subject));
    });
  }

  Future<void> _revokePasskey(AdminPerson p, AdminPasskey passkey) async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.adminRevokePasskeyTitle(passkey.name),
      body: l.adminRevokePasskeyBody(p.displayName),
      confirmLabel: l.adminRevoke,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    await _change('passkey.${passkey.id}', () async {
      await widget.repository.revokePasskey(
        p.id,
        passkey.id,
        purpose: l.stepUpPurposeRevokePasskey(p.displayName),
      );
      return SelfNotice(l.adminPasskeyRevoked(passkey.name));
    });
  }

  Future<void> _approve(SelfDevice device) async {
    final l = AppLocalizations.of(context);
    final code = await ConfirmDeviceDialog.show(context, device.name);
    if (code == null || !mounted) return;
    await _change('${device.id}.confirm', () async {
      final result = await widget.repository.approveDevice(
        device.id,
        code,
        purpose: l.stepUpPurposeApproveDevice(device.name),
      );
      // 08 5.2: a wrong code revokes the waiting device; the answer is still 200.
      return result.state == SelfDeviceState.active
          ? SelfNotice(l.devicesConfirmed(device.name))
          : SelfNotice(l.devicesConfirmRejected, tone: DsTone.warning);
    });
  }

  Future<void> _revokeDevice(SelfDevice device) async {
    final l = AppLocalizations.of(context);
    final reason = await ConfirmChange.show(
      context,
      title: l.devicesRevokeTitle(device.name),
      body: l.devicesRevokeBody,
      confirmLabel: l.devicesRevoke,
      destructive: true,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    await _change('${device.id}.revoke', () async {
      await widget.repository.revokeDevice(
        device.id,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: l.stepUpPurposeRevokeDevice(device.name),
      );
      return SelfNotice(l.devicesRevoked(device.name));
    });
  }

  Future<void> _endSession(SelfSession session) async {
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.sessionsEndTitle,
      body: l.adminEndSessionBody,
      confirmLabel: l.sessionsEnd,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    await _change('session.${session.id}', () async {
      await widget.repository.endSession(session.id);
      return SelfNotice(l.adminSessionEnded);
    });
  }

  Widget _heading(String text) => Padding(
    padding: const EdgeInsets.only(top: DsSpace.s24, bottom: DsSpace.s8),
    child: Semantics(
      header: true,
      child: Text(
        text,
        style: dsTextStyle(DsType.heading)
            .copyWith(color: context.dsColors.textPrimary),
      ),
    ),
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final now = widget.clock();
    final part = _person;
    final person = part?.value;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (widget.onBack case final back?)
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              key: const ValueKey('person.back'),
              onPressed: back,
              child: Text(l.adminPersonBack),
            ),
          ),
        if (part?.problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (person != null) ...[
          Row(
            children: [
              DsAvatar(name: person.displayName, origin: DsMessageOrigin.human),
              const SizedBox(width: DsSpace.s12),
              Expanded(
                child: Wrap(
                  spacing: DsSpace.s8,
                  runSpacing: DsSpace.s4,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    Semantics(
                      header: true,
                      child: Text(
                        person.displayName,
                        style: dsTextStyle(DsType.title)
                            .copyWith(color: colors.textPrimary),
                      ),
                    ),
                    Text(
                      '@${person.handle}',
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: colors.textSecondary),
                    ),
                    if (person.isOwner)
                      DsBadge(
                        key: const ValueKey('person.owner'),
                        text: l.adminOwnerBadge,
                        background: colors.accentSubtle,
                        foreground: colors.textPrimary,
                      ),
                    DsBadge(
                      key: const ValueKey('person.status'),
                      text: personStatusText(l, person.status),
                      background: person.status == AdminPersonStatus.active
                          ? colors.accentSubtle
                          : person.status == AdminPersonStatus.pending
                          ? colors.attentionSubtle
                          : colors.dangerSubtle,
                      foreground: colors.textPrimary,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: DsSpace.s12),
          Wrap(
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s8,
            children: [
              if (person.status == AdminPersonStatus.disabled &&
                  !denied.contains('enable'))
                FilledButton(
                  key: const ValueKey('person.enable'),
                  onPressed: busy ? null : () => _enable(person),
                  child: Text(l.adminPersonEnable),
                ),
              if ((person.status == AdminPersonStatus.active ||
                      person.status == AdminPersonStatus.pending) &&
                  !denied.contains('code'))
                OutlinedButton(
                  key: const ValueKey('person.newCode'),
                  onPressed: busy ? null : () => _newCode(person),
                  child: Text(l.adminPersonNewCode),
                ),
              if ((person.status == AdminPersonStatus.active ||
                      person.status == AdminPersonStatus.pending) &&
                  !denied.contains('disable'))
                OutlinedButton(
                  key: const ValueKey('person.disable'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: colors.dangerFg,
                  ),
                  onPressed: busy ? null : () => _disable(person),
                  child: Text(l.adminPersonDisable),
                ),
              if (widget.onViewAs case final viewAs?)
                OutlinedButton(
                  key: const ValueKey('person.viewAs'),
                  onPressed: busy ? null : () => viewAs(person),
                  child: Text(l.adminPersonViewAs),
                ),
            ],
          ),
          const SizedBox(height: DsSpace.s8),
          noticeBanner(),
          const SizedBox(height: DsSpace.s8),
          if (_credentials case final c?)
            if (c.problem case final problem?)
              ProblemBanner(problem: problem)
            else
              CredentialsSection(
                credentials: c.value!,
                now: now,
                timeZone: widget.timeZone,
                denied: denied,
                onUnlink: busy ? null : (i) => _unlink(person, i),
                onRevokePasskey: busy ? null : (k) => _revokePasskey(person, k),
              ),
          _heading(l.devicesTitle),
          if (_devices case final d?)
            if (d.problem case final problem?)
              ProblemBanner(problem: problem)
            else
              for (final device in d.value!) ...[
                DeviceTile(
                  device: device,
                  now: now,
                  timeZone: widget.timeZone,
                  onConfirm:
                      busy ||
                          device.state != SelfDeviceState.pending ||
                          denied.contains('${device.id}.confirm')
                      ? null
                      : () => _approve(device),
                  onRevoke:
                      busy ||
                          device.state == SelfDeviceState.revoked ||
                          denied.contains('${device.id}.revoke')
                      ? null
                      : () => _revokeDevice(device),
                ),
                Divider(height: 1, color: colors.borderSubtle),
              ],
          _heading(l.sessionsTitle),
          if (_sessions case final s?)
            if (s.problem case final problem?)
              ProblemBanner(problem: problem)
            else
              for (final session in s.value!) ...[
                Padding(
                  key: ValueKey('session.${session.id}'),
                  padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
                  child: Row(
                    children: [
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              session.deviceName ?? l.sessionsUnknownDevice,
                              style: dsTextStyle(DsType.bodyStrong)
                                  .copyWith(color: colors.textPrimary),
                            ),
                            Wrap(
                              spacing: DsSpace.s12,
                              children: [
                                for (final detail in [
                                  authMethodText(l, session.authMethod),
                                  l.sessionsLastUsed(
                                    activityText(
                                      context,
                                      session.lastUsedAt,
                                      now: now,
                                      timeZone: widget.timeZone,
                                    ),
                                  ),
                                ])
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
                      if (!denied.contains('session.${session.id}'))
                        OutlinedButton(
                          key: ValueKey('session.${session.id}.end'),
                          onPressed: busy ? null : () => _endSession(session),
                          child: Text(l.sessionsEnd),
                        ),
                    ],
                  ),
                ),
                Divider(height: 1, color: colors.borderSubtle),
              ],
        ],
      ],
    );
  }
}
