// SPDX-License-Identifier: Apache-2.0

/// The emergency view of one person on the phone: revoke a device, revoke a grant, disable the
/// person (docs/spec/01-product.md 3.7, 5.2 A1 and A3; 05 `device.revoke`, `grant.revoke`,
/// `user.disable`; 07 2.6, 9.5, SR8; 08 10, 11, 14.3; R-PR-07; brief M5-09 Design notes 1 to
/// 3).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:auth/auth.dart' show StepUpCancelled;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/self_repository.dart';
import '../grants/grant_labels.dart';
import '../people/people_list.dart' show personStatusText;
import '../self/device_tile.dart';
import '../self/self_feedback.dart';
import 'emergency_sheet.dart';

/// The text of a failed emergency action [error]: [adminFailure], and for `409 last_owner`
/// (07 SR8) the explanation of what to do instead; null when the person cancelled the
/// step-up. The client never prevents disabling the last owner itself: it explains the
/// server's answer (brief M5-09 requirement 13).
SelfFailure? emergencyFailure(AppLocalizations l, Object error) {
  final failure = adminFailure(l, error);
  if (failure != null &&
      error is SelfActionFailed &&
      error.problem?.code == 'last_owner') {
    return SelfFailure('${failure.text}. ${l.emergencyLastOwner}');
  }
  return failure;
}

/// The person [userId] with the emergency actions the server's answers allow.
///
/// [owner] is the caller's owner flag from the server's self information (`getMe`,
/// `is_owner`): only an owner sees the person's devices with "Revoke" (`device.list`,
/// `device.revoke`: self or owner) and "Disable person" (`user.disable`: owner). Everyone the
/// entry admits sees the grants of the person that the server lists for them
/// ([AccessRepository.grantsOf]; a manager gets only those within their `manage` scope, 07 2.6) with "Revoke".
/// No grant is read to decide a control (15 1 principle 1): a refused list or action shows the
/// server's reason, and a refused action hides its button.
///
/// Every action asks once in an [EmergencyConfirmSheet] that says it takes effect at once
/// (07 9.5) and then runs through [repository], the data source of the administration
/// (M4-26, M4-27). The step-up interceptor of `auth` answers `401 step_up_required` with the
/// device key and repeats the request with the same `Idempotency-Key`; a cancelled step-up
/// sends nothing more and says that nothing changed.
///
/// Example:
/// ```dart
/// PersonEmergencyView(repository: admin, userId: samId, owner: true);
/// ```
class PersonEmergencyView extends StatefulWidget {
  /// Creates the view.
  const PersonEmergencyView({
    super.key,
    required this.repository,
    required this.userId,
    required this.owner,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The person.
  final String userId;

  /// Whether the caller is an owner (`getMe`, `is_owner`).
  final bool owner;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<PersonEmergencyView> createState() => _PersonEmergencyViewState();
}

/// A loaded part of the page: its value or the server's problem.
final class _Part<T> {
  const _Part.value(T this.value) : problem = null;

  const _Part.problem(this.problem) : value = null;

  final T? value;
  final ApiProblem? problem;
}

class _PersonEmergencyViewState extends State<PersonEmergencyView>
    with AdminActionState<PersonEmergencyView> {
  _Part<AdminPerson>? _person;
  _Part<List<SelfDevice>>? _devices;
  _Part<List<AdminGrant>>? _grants;
  List<AdminTarget> _targets = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(PersonEmergencyView old) {
    super.didUpdateWidget(old);
    if (old.userId != widget.userId || old.owner != widget.owner) _load();
  }

  Future<_Part<T>> _part<T>(Future<T> Function() load) async {
    try {
      return _Part.value(await load());
    } on Object catch (e) {
      return _Part.problem(loadProblem(e));
    }
  }

  Future<List<AdminTarget>> _orEmpty(
    Future<List<AdminTarget>> Function() load,
  ) async {
    try {
      return await load();
    } on Object {
      // A refused name list leaves the IDs as they are.
      return const [];
    }
  }

  Future<void> _load() async {
    final r = widget.repository;
    final id = widget.userId;
    final person = _part(() => r.person(id));
    final devices = widget.owner ? _part(() => r.devicesOf(id)) : null;
    final grants = _part(() => r.grantsOf(AdminSubjectKind.user, id));
    final targets = _orEmpty(r.targets);
    final hosts = _orEmpty(
      () async => [for (final h in await r.hosts()) h.target],
    );
    final loaded = (
      person: await person,
      devices: await devices,
      grants: await grants,
      targets: [...await targets, ...await hosts],
    );
    if (!mounted) return;
    setState(() {
      _person = loaded.person;
      _devices = loaded.devices;
      _grants = loaded.grants;
      _targets = loaded.targets;
    });
  }

  /// Runs [change] under [key] like [runChange], with the SR8 explanation of
  /// [emergencyFailure] and the notice of a cancelled step-up; reloads the page afterwards.
  Future<void> _run(String key, Future<SelfNotice> Function() change) async {
    final l = AppLocalizations.of(context);
    setState(() => busy = true);
    try {
      final result = await change();
      if (mounted) setState(() => notice = result);
    } on StepUpCancelled {
      if (mounted) setState(() => notice = SelfNotice(l.emergencyCancelled));
    } on Object catch (e) {
      final failure = emergencyFailure(l, e);
      if (mounted && failure != null) {
        setState(() {
          if (failure.denied) denied.add(key);
          notice = SelfNotice(failure.text, tone: DsTone.danger);
        });
      }
    } finally {
      if (mounted) setState(() => busy = false);
    }
    if (mounted) await _load();
  }

  String _targetName(AppLocalizations l, AdminGrant g) {
    final kind = g.targetKind;
    if (kind == null) return g.targetId ?? '';
    if (kind == AdminTargetKind.instance) {
      return targetLabel(l, AdminTarget.instance);
    }
    final known = _targets
        .where((t) => t.kind == kind && t.id == g.targetId)
        .firstOrNull;
    return targetLabel(
      l,
      known ?? AdminTarget(kind, id: g.targetId, name: g.targetId ?? ''),
    );
  }

  Future<void> _revokeDevice(AdminPerson p, SelfDevice device) async {
    final l = AppLocalizations.of(context);
    final reason = await EmergencyConfirmSheet.show(
      context,
      title: l.devicesRevokeTitle(device.name),
      body: l.emergencyRevokeDeviceBody(device.name, p.displayName),
      stepUp: true,
      confirmLabel: l.devicesRevoke,
    );
    if (reason == null || !mounted) return;
    await _run('${device.id}.revoke', () async {
      await widget.repository.revokeDevice(
        device.id,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: l.stepUpPurposeRevokeDevice(device.name),
      );
      return SelfNotice(l.devicesRevoked(device.name));
    });
  }

  Future<void> _revokeGrant(AdminPerson p, AdminGrant grant) async {
    final l = AppLocalizations.of(context);
    final reason = await EmergencyConfirmSheet.show(
      context,
      title: l.adminRevokeGrantTitle(p.displayName),
      body: l.emergencyRevokeGrantBody(p.displayName, _targetName(l, grant)),
      stepUp: true,
      confirmLabel: l.adminRevoke,
      reasonRequired: true,
    );
    if (reason == null || !mounted) return;
    await _run('grant.${grant.id}.revoke', () async {
      await widget.repository.revokeGrant(
        grant.id,
        reason: reason,
        purpose: l.stepUpPurposeRevokeGrant(p.displayName),
      );
      return SelfNotice(l.adminGrantRevoked(p.displayName));
    });
  }

  Future<void> _disable(AdminPerson p) async {
    final l = AppLocalizations.of(context);
    final reason = await EmergencyConfirmSheet.show(
      context,
      title: l.adminDisableTitle(p.displayName),
      body: l.emergencyDisableBody(p.displayName),
      stepUp: true,
      confirmLabel: l.adminPersonDisable,
    );
    if (reason == null || !mounted) return;
    await _run('disable', () async {
      await widget.repository.disablePerson(
        p.id,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: l.stepUpPurposeDisablePerson(p.displayName),
      );
      return SelfNotice(l.adminPersonDisabled(p.displayName));
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

  Widget _empty(String key, String text) => Padding(
    padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
    child: Text(
      text,
      key: ValueKey(key),
      style: dsTextStyle(DsType.body)
          .copyWith(color: context.dsColors.textSecondary),
    ),
  );

  Widget _grantRow(AdminPerson p, AdminGrant g) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final level = g.level;
    final expires = g.expiresAt;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Padding(
      key: ValueKey('emergency.grant.${g.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            _targetName(l, g),
            style: dsTextStyle(DsType.bodyStrong)
                .copyWith(color: colors.textPrimary),
          ),
          Text(
            [
              if (level != null)
                levelWithCapabilities(l, level, g.capabilities)
              else
                g.capabilities.map((c) => capabilityLabel(l, c)).join(', '),
              if (expires != null)
                l.adminGrantsUntil(
                  lastDayLabel(
                    context,
                    expires,
                    now: widget.clock(),
                    timeZone: widget.timeZone,
                  ),
                ),
            ].join(', '),
            style: secondary,
          ),
          Text(l.adminGrantsReason(g.reason), style: secondary),
          if (!denied.contains('grant.${g.id}.revoke')) ...[
            const SizedBox(height: DsSpace.s8),
            Align(
              alignment: AlignmentDirectional.centerEnd,
              child: DsButton(
                key: ValueKey('emergency.grant.${g.id}.revoke'),
                label: l.adminRevoke,
                onPressed: busy ? null : () => _revokeGrant(p, g),
              ),
            ),
          ],
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final now = widget.clock();
    final part = _person;
    final person = part?.value;
    final devices = _devices;
    final grants = _grants;
    return ListView(
      key: const ValueKey('emergency.person'),
      padding: const EdgeInsets.all(DsSpace.s16),
      children: [
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
                        dsIsolate(person.displayName),
                        style: dsTextStyle(DsType.title)
                            .copyWith(color: colors.textPrimary),
                      ),
                    ),
                    Text(
                      '@${person.handle}',
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: colors.textSecondary),
                    ),
                    DsBadge(
                      key: const ValueKey('emergency.person.status'),
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
          noticeBanner(),
          if (widget.owner &&
              (person.status == AdminPersonStatus.active ||
                  person.status == AdminPersonStatus.pending) &&
              !denied.contains('disable')) ...[
            const SizedBox(height: DsSpace.s8),
            DsButton(
              key: const ValueKey('emergency.disable'),
              label: l.adminPersonDisable,
              style: DsButtonStyle.danger,
              expand: true,
              onPressed: busy ? null : () => _disable(person),
            ),
          ],
          if (devices != null) ...[
            _heading(l.devicesTitle),
            if (devices.problem case final problem?)
              ProblemBanner(problem: problem)
            else if (devices.value!
                .where((d) => d.state != SelfDeviceState.revoked)
                .isEmpty)
              _empty('emergency.devices.empty', l.emergencyDevicesEmpty)
            else
              for (final device in devices.value!)
                if (device.state != SelfDeviceState.revoked) ...[
                  DeviceTile(
                    key: ValueKey('emergency.device.${device.id}'),
                    device: device,
                    now: now,
                    timeZone: widget.timeZone,
                    onRevoke: busy || denied.contains('${device.id}.revoke')
                        ? null
                        : () => _revokeDevice(person, device),
                  ),
                  Divider(height: 1, color: colors.borderSubtle),
                ],
          ],
          _heading(l.adminGrantsTitle),
          if (grants?.problem case final problem?)
            ProblemBanner(problem: problem)
          else if (grants?.value case final list?)
            if (list.isEmpty)
              _empty('emergency.grants.empty', l.emergencyGrantsEmpty)
            else
              for (final g in list) ...[
                _grantRow(person, g),
                Divider(height: 1, color: colors.borderSubtle),
              ],
        ],
      ],
    );
  }
}
