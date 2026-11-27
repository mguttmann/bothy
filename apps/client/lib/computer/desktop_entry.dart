// SPDX-License-Identifier: Apache-2.0

/// The desktop entry of a host in the app's list of hosts (docs/spec/27-computer-use.md 4,
/// 11 items 2 and 3; 01 3.9; brief CU-07 requirement 14): the enablement the host reports,
/// the operating system account and the permissions, the live computer session with Stop, and
/// "Switch enablement off" with the note that switching on is only possible at the machine.
///
/// There is no control that switches an enablement on (27 4 item 1, R-CU-03). The entry exists
/// only for a host that carries the reported `computer` object, which the server sends to
/// active owners only (07 2.15 item 5): a person who is no owner never sees it.
library;

import 'package:api_client/api_client.dart'
    show HostComputer, HostComputerHelperEnum;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import 'computer_chip.dart';
import 'providers.dart';

/// The reported computer state of one host as the entry shows it.
@immutable
final class ComputerDesktopView {
  const ComputerDesktopView._({
    required this.hostId,
    required this.name,
    required this.helper,
    required this.enabled,
    required this.disabledRemotely,
    required this.osAccount,
    required this.capture,
    required this.input,
    required this.locked,
  });

  /// The view of host [hostId] named [name] with its reported [computer] object and the
  /// [disabledGeneration] of a remote switch-off; null without the object (no helper reported,
  /// or a reader who is no owner).
  static ComputerDesktopView? of({
    required String hostId,
    required String name,
    required HostComputer? computer,
    required int? disabledGeneration,
  }) {
    if (computer == null) return null;
    final remote =
        disabledGeneration != null && computer.generation <= disabledGeneration;
    return ComputerDesktopView._(
      hostId: hostId,
      name: name,
      helper: switch (computer.helper) {
        HostComputerHelperEnum.connected => 'connected',
        HostComputerHelperEnum.refused => 'refused',
        _ => 'absent',
      },
      enabled: computer.enabled && !remote,
      disabledRemotely: remote,
      osAccount: computer.osAccount,
      capture: computer.permissions.capture.name,
      input: computer.permissions.input.name,
      locked: computer.locked,
    );
  }

  /// The host.
  final String hostId;

  /// The host's name.
  final String name;

  /// `connected`, `absent` or `refused`.
  final String helper;

  /// Whether control is allowed now: the switch is on and no remote switch-off holds it.
  final bool enabled;

  /// Whether `computer.device.disable` holds the reported generation off (27 4 item 4).
  final bool disabledRemotely;

  /// The operating system account of the helper.
  final String osAccount;

  /// The screen recording permission: `granted`, `denied` or `unknown`.
  final String capture;

  /// The input permission: `granted`, `denied` or `unknown`.
  final String input;

  /// Whether the desktop reports a locked screen.
  final bool locked;
}

/// The entry of [desktop]; [onChanged] runs after a switch-off, so the list reads the host
/// again.
class ComputerDesktopEntry extends ConsumerStatefulWidget {
  /// Creates the entry.
  const ComputerDesktopEntry({
    super.key,
    required this.desktop,
    this.onChanged,
  });

  /// The host's reported state.
  final ComputerDesktopView desktop;

  /// Runs after a successful switch-off.
  final VoidCallback? onChanged;

  @override
  ConsumerState<ComputerDesktopEntry> createState() =>
      _ComputerDesktopEntryState();
}

class _ComputerDesktopEntryState extends ConsumerState<ComputerDesktopEntry> {
  String? _error;
  String? _done;
  bool _busy = false;

  Future<void> _run(
    Future<void> Function() call,
    String Function(String problem) failed, {
    String? done,
  }) async {
    setState(() {
      _busy = true;
      _error = null;
      _done = null;
    });
    try {
      await call();
      if (mounted) setState(() => _done = done);
    } on Object catch (e) {
      if (!mounted) return;
      final l = AppLocalizations.of(context);
      setState(() => _error = failed(computerProblemText(l, e)));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final d = widget.desktop;
    final sessions = ref.watch(computerSessionsProvider);
    final notifier = ref.read(computerSessionsProvider.notifier);
    final live = sessions.liveOn(d.hostId);
    final caption = dsTextStyle(DsType.caption)
        .copyWith(color: c.textSecondary);
    final body = dsTextStyle(DsType.bodySm).copyWith(color: c.textPrimary);
    return Padding(
      key: ValueKey('computer.desktop.${d.hostId}'),
      padding: const EdgeInsets.only(
        left: DsSpace.s24,
        right: DsSpace.s8,
        top: DsSpace.s4,
        bottom: DsSpace.s4,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(
            header: true,
            child: Text(
              l.computerDesktopTitle,
              style: dsTextStyle(DsType.label).copyWith(color: c.textPrimary),
            ),
          ),
          if (d.helper != 'connected')
            Text(l.computerHelperState(d.helper), style: body)
          else ...[
            Text(
              d.disabledRemotely
                  ? l.computerDisabledRemotely
                  : d.enabled
                  ? l.computerEnabled
                  : l.computerNotEnabled,
              key: ValueKey('computer.enablement.${d.hostId}'),
              style: body,
            ),
            Text(l.computerOsAccount(d.osAccount), style: caption),
            Text(
              l.computerPermissions(
                l.computerPermission(d.capture),
                l.computerPermission(d.input),
              ),
              style: caption,
            ),
            if (d.locked) Text(l.computerLocked, style: caption),
          ],
          if (live != null) ...[
            const SizedBox(height: DsSpace.s4),
            Text(l.computerLiveSession, style: body),
            ComputerStatusChip(
              status: live,
              stopping: _busy,
              onStop: () => _run(
                () => notifier.stop(live.sessionId),
                l.computerStopFailed,
              ),
            ),
          ],
          if (d.enabled && d.helper == 'connected')
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s4),
              child: ComputerActionButton(
                key: ValueKey('computer.disable.${d.hostId}'),
                text: l.computerDisable,
                label: l.computerDisableLabel(d.name),
                onPressed: _busy
                    ? null
                    : () => _run(
                        () async {
                          await notifier.disableDevice(d.hostId);
                          widget.onChanged?.call();
                        },
                        l.computerDisableFailed,
                        done: l.computerDisabledDone(d.name),
                      ),
              ),
            ),
          Text(l.computerOnlyLocal, style: caption),
          if (_done case final text?)
            Semantics(liveRegion: true, child: Text(text, style: body)),
          if (_error case final text?)
            Semantics(
              liveRegion: true,
              child: Text(
                text,
                style: dsTextStyle(DsType.caption).copyWith(color: c.dangerFg),
              ),
            ),
        ],
      ),
    );
  }
}
