// SPDX-License-Identifier: Apache-2.0

/// The queue of waiting devices: person, device, platform, time and origin of the login;
/// approve with the code, reject (docs/spec/01-product.md 5.2 A1; 08 5.2; brief M4-26
/// requirement 15).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/self_repository.dart';
import '../self/device_tile.dart' show activityText, platformText;
import '../self/my_sessions_view.dart' show authMethodText;
import '../self/self_feedback.dart';
import 'approve_device_dialog.dart';

/// The waiting devices of every person ([AdminRepository.waitingDevices]).
///
/// Each row shows the person, the device name, the platform, when the login happened and its
/// origin (`Session.auth_method`). "Key: hardware, attested" of A1 is left out: the API's
/// `Device` has no hardware or attestation field. "Approve" asks for the code
/// ([ApproveDeviceDialog]) and sends `approveDevice`; "Reject" asks once and revokes the
/// device (`revokeDevice`). Both have step-up in 05, asked by the interceptor. A `403` shows
/// the server's reason instead of the queue.
class WaitingDevicesView extends StatefulWidget {
  /// Creates the view.
  const WaitingDevicesView({
    super.key,
    required this.repository,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after a change (the shell reloads its count).
  final VoidCallback? onChanged;

  @override
  State<WaitingDevicesView> createState() => _WaitingDevicesViewState();
}

class _WaitingDevicesViewState extends State<WaitingDevicesView>
    with AdminActionState<WaitingDevicesView> {
  List<WaitingDevice>? _waiting;
  ApiProblem? _problem;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final waiting = await widget.repository.waitingDevices();
      if (!mounted) return;
      setState(() {
        _waiting = waiting;
        _problem = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    }
  }

  Future<void> _change(
    String key,
    Future<SelfNotice?> Function() change,
  ) async {
    final ok = await runChange(key, change);
    if (ok) widget.onChanged?.call();
    if (mounted) await _load();
  }

  Future<void> _approve(WaitingDevice w) async {
    final l = AppLocalizations.of(context);
    final code = await ApproveDeviceDialog.show(
      context,
      personName: w.person.displayName,
      deviceName: w.device.name,
    );
    if (code == null || !mounted) return;
    await _change('${w.device.id}.approve', () async {
      final result = await widget.repository.approveDevice(
        w.device.id,
        code,
        purpose: l.stepUpPurposeApproveDeviceOf(
          w.device.name,
          w.person.displayName,
        ),
      );
      // 08 5.2: a wrong code revokes the waiting device; the answer is still 200.
      return result.state == SelfDeviceState.active
          ? SelfNotice(l.adminApproved(w.device.name, w.person.displayName))
          : SelfNotice(l.devicesConfirmRejected, tone: DsTone.warning);
    });
  }

  Future<void> _reject(WaitingDevice w) async {
    final l = AppLocalizations.of(context);
    final reason = await ConfirmChange.show(
      context,
      title: l.adminRejectTitle(w.device.name),
      body: l.adminRejectBody(w.person.displayName),
      confirmLabel: l.adminReject,
      destructive: true,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    await _change('${w.device.id}.reject', () async {
      await widget.repository.revokeDevice(
        w.device.id,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: l.stepUpPurposeRejectDevice(
          w.device.name,
          w.person.displayName,
        ),
      );
      return SelfNotice(l.adminRejected(w.device.name));
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final waiting = _waiting;
    final now = widget.clock();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.adminWaitingTitle,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s12),
        noticeBanner(),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (waiting != null && waiting.isEmpty)
          Text(
            l.adminWaitingEmpty,
            key: const ValueKey('waiting.empty'),
            style: dsTextStyle(DsType.body)
                .copyWith(color: colors.textSecondary),
          )
        else if (waiting != null)
          for (final w in waiting) ...[
            Padding(
              key: ValueKey('waiting.${w.device.id}'),
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
              child: Wrap(
                spacing: DsSpace.s8,
                runSpacing: DsSpace.s8,
                alignment: WrapAlignment.spaceBetween,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  ConstrainedBox(
                    constraints: const BoxConstraints(minWidth: 280),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          l.adminWaitingDevice(
                            w.person.displayName,
                            w.device.name,
                          ),
                          style: dsTextStyle(DsType.bodyStrong)
                              .copyWith(color: colors.textPrimary),
                        ),
                        Wrap(
                          spacing: DsSpace.s12,
                          children: [
                            for (final detail in [
                              platformText(l, w.device.platform),
                              l.adminWaitingLoggedIn(
                                activityText(
                                  context,
                                  w.device.createdAt,
                                  now: now,
                                  timeZone: widget.timeZone,
                                ),
                              ),
                              switch (w.origin) {
                                final origin? => l.adminWaitingVia(
                                  authMethodText(l, origin),
                                ),
                                null => l.adminWaitingNoOrigin,
                              },
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
                  Wrap(
                    spacing: DsSpace.s8,
                    children: [
                      if (!denied.contains('${w.device.id}.approve'))
                        FilledButton(
                          key: ValueKey('waiting.${w.device.id}.approve'),
                          onPressed: busy ? null : () => _approve(w),
                          child: Text(l.adminApprove),
                        ),
                      if (!denied.contains('${w.device.id}.reject'))
                        OutlinedButton(
                          key: ValueKey('waiting.${w.device.id}.reject'),
                          onPressed: busy ? null : () => _reject(w),
                          child: Text(l.adminReject),
                        ),
                    ],
                  ),
                ],
              ),
            ),
            Divider(height: 1, color: colors.borderSubtle),
          ],
      ],
    );
  }
}
