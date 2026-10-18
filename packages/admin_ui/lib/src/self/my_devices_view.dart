// SPDX-License-Identifier: Apache-2.0

/// The own devices: confirm a waiting one with its code, add one by its pairing code, revoke
/// one (docs/spec/08-identity.md 5.2, 5.3, 10; 01 5.1 D8, 5.6 E4 and E5, 6.7).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/self_repository.dart';
import 'add_device_dialog.dart';
import 'confirm_device_dialog.dart';
import 'device_tile.dart';
import 'revoke_confirm.dart';
import 'self_feedback.dart';

/// The list of the person's devices with "Add device", "Enter code" and "Revoke".
///
/// Every action goes through [repository]; the step-up of `approveDevice`, `approvePairing`
/// and `revokeDevice` is asked by the step-up interceptor when, and only when, the server
/// answers `401 step_up_required`, with the purpose text this view passes. Revoking asks once
/// ([RevokeConfirmDialog]). Revoking this device calls [onThisDeviceRevoked] (the app logs
/// out and wipes, 08 10). A `403` hides the refused action of that device and shows its
/// reason. After every action the list loads again and [onChanged] runs.
///
/// Example:
/// ```dart
/// MyDevicesView(repository: repository, timeZone: 'Europe/Berlin');
/// ```
class MyDevicesView extends StatefulWidget {
  /// Creates the view.
  const MyDevicesView({
    super.key,
    required this.repository,
    required this.timeZone,
    this.clock = DateTime.now,
    this.onThisDeviceRevoked,
    this.onChanged,
    this.reload,
  });

  /// The devices and their actions.
  final SelfRepository repository;

  /// The time zone of the person (`users.time_zone`).
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after this device was revoked.
  final VoidCallback? onThisDeviceRevoked;

  /// Runs after an action changed something.
  final VoidCallback? onChanged;

  /// Loads the list again when it notifies (another view changed something).
  final Listenable? reload;

  @override
  State<MyDevicesView> createState() => _MyDevicesViewState();
}

class _MyDevicesViewState extends State<MyDevicesView> {
  List<SelfDevice>? _devices;
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
  void didUpdateWidget(MyDevicesView old) {
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
      final devices = await widget.repository.devices();
      if (!mounted) return;
      setState(() {
        _devices = devices;
        _loadFailed = false;
      });
    } on Object {
      if (!mounted) return;
      setState(() => _loadFailed = true);
    }
  }

  /// Runs [action] (which returns the success notice), then loads again; a failure shows its
  /// text and, for a `403`, hides [deniedKey].
  Future<void> _run(
    String deniedKey,
    Future<SelfNotice?> Function() action, {
    String? notFound,
  }) async {
    final l = AppLocalizations.of(context);
    setState(() => _busy = true);
    try {
      final notice = await action();
      if (!mounted) return;
      setState(() => _notice = notice);
      widget.onChanged?.call();
    } on Object catch (e) {
      final failure = selfFailure(l, e, notFound: notFound);
      if (!mounted) return;
      setState(() {
        if (failure == null) return;
        if (failure.denied) _denied.add(deniedKey);
        _notice = SelfNotice(failure.text, tone: DsTone.danger);
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (mounted) await _load();
  }

  Future<void> _confirm(SelfDevice device) async {
    final l = AppLocalizations.of(context);
    final code = await ConfirmDeviceDialog.show(context, device.name);
    if (code == null || !mounted) return;
    await _run('${device.id}.confirm', () async {
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

  Future<void> _add() async {
    final l = AppLocalizations.of(context);
    final code = await AddDeviceDialog.show(context);
    if (code == null || !mounted) return;
    await _run('add', () async {
      final device = await widget.repository.approvePairing(
        code,
        purpose: l.stepUpPurposeAddDevice,
      );
      return SelfNotice(l.devicesAdded(device.name));
    }, notFound: l.devicesPairingNotFound);
  }

  Future<void> _revoke(SelfDevice device) async {
    final l = AppLocalizations.of(context);
    final reason = await RevokeConfirmDialog.show(
      context,
      title: l.devicesRevokeTitle(device.name),
      body: device.isThisDevice ? l.devicesRevokeThisBody : l.devicesRevokeBody,
      confirmLabel: l.devicesRevoke,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    await _run('${device.id}.revoke', () async {
      await widget.repository.revokeDevice(
        device.id,
        reason: reason.isEmpty ? l.devicesRevokeDefaultReason : reason,
        purpose: l.stepUpPurposeRevokeDevice(device.name),
      );
      if (device.isThisDevice) {
        // 08 10: a revoked own device ends in E7 with its data deleted.
        widget.onThisDeviceRevoked?.call();
        return null;
      }
      return SelfNotice(l.devicesRevoked(device.name));
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final devices = _devices;
    final now = widget.clock();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  l.devicesTitle,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            if (!_denied.contains('add'))
              FilledButton(
                key: const ValueKey('devices.add'),
                onPressed: _busy ? null : _add,
                child: Text(l.devicesAdd),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        if (_notice case final notice?) selfNoticeBanner(notice),
        if (_loadFailed && devices == null)
          DsBanner(
            key: const ValueKey('devices.loadFailed'),
            tone: DsTone.warning,
            text: l.selfLoadFailed,
            actionLabel: l.selfRetry,
            onAction: _load,
          )
        else if (devices != null)
          for (final device in devices) ...[
            DeviceTile(
              device: device,
              now: now,
              timeZone: widget.timeZone,
              onConfirm:
                  _busy ||
                      device.isThisDevice ||
                      device.state != SelfDeviceState.pending ||
                      _denied.contains('${device.id}.confirm')
                  ? null
                  : () => _confirm(device),
              onRevoke:
                  _busy ||
                      device.state == SelfDeviceState.revoked ||
                      _denied.contains('${device.id}.revoke')
                  ? null
                  : () => _revoke(device),
            ),
            Divider(height: 1, color: colors.borderSubtle),
          ],
      ],
    );
  }
}
