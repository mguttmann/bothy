// SPDX-License-Identifier: Apache-2.0

/// Computer control on the host page of the console and the desktop app (docs/spec/
/// 27-computer-use.md 4, 11 items 2 and 3; 01 3.9; brief CU-07 requirement 14): the enablement
/// the host reports, the operating system account and the permissions, the signed-in owner's
/// live computer session on the host with Stop, and "Switch enablement off" with the note that
/// switching on is only possible at the machine.
///
/// No control switches an enablement on (27 4 item 1, R-CU-03). The page shows the section only
/// for a host that carries the reported object, which the server sends to active owners only
/// (07 2.15 item 5). Metadata only: no image is requested or shown (R-CL-18).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/hosts_repository.dart';
import '../data/self_repository.dart' show SelfActionFailed;

/// The computer section of [host]; [onChanged] gets the host after a switch-off.
class HostComputerSection extends StatefulWidget {
  /// Creates the section.
  const HostComputerSection({
    super.key,
    required this.repository,
    required this.host,
    required this.onChanged,
  });

  /// The calls of the hosts.
  final HostsRepository repository;

  /// The host; its [AdminHost.computer] is set.
  final AdminHost host;

  /// Runs with the host the switch-off returned.
  final ValueChanged<AdminHost> onChanged;

  @override
  State<HostComputerSection> createState() => _HostComputerSectionState();
}

class _HostComputerSectionState extends State<HostComputerSection> {
  AdminComputerSession? _live;
  String? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _loadLive();
  }

  @override
  void didUpdateWidget(HostComputerSection old) {
    super.didUpdateWidget(old);
    if (old.host.id != widget.host.id) {
      _live = null;
      _error = null;
      _loadLive();
    }
  }

  Future<void> _loadLive() async {
    final hostId = widget.host.id;
    try {
      final live = await widget.repository.liveComputerSession(hostId);
      if (mounted && widget.host.id == hostId) setState(() => _live = live);
    } on Object {
      // Without the list the section shows the reported state only; Stop stays in the thread.
      if (mounted && widget.host.id == hostId) setState(() => _live = null);
    }
  }

  Future<void> _run(
    Future<void> Function() call,
    String Function(String problem) failed,
  ) async {
    final l = AppLocalizations.of(context);
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await call();
    } on Object catch (e) {
      if (mounted) {
        setState(() => _error = failed(_problemText(l, e)));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  static String _problemText(AppLocalizations l, Object e) {
    final problem = e is SelfActionFailed ? e.problem : null;
    return problem == null
        ? l.selfOffline
        : problemText(l, problem.code, reason: problem.reason);
  }

  String _sessionText(AppLocalizations l, AdminComputerSession s) =>
      switch (s.state) {
        'opening' => l.computerChipOpening(s.desktop),
        'paused' => l.computerChipLocked(s.desktop),
        _ => switch (s.frontApp) {
          final app? => l.computerChipActive(s.desktop, app, s.actionCount),
          null => l.computerChipActiveNoApp(s.desktop, s.actionCount),
        },
      };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final h = widget.host;
    final computer = h.computer!;
    final body = dsTextStyle(DsType.body).copyWith(color: c.textPrimary);
    final caption = dsTextStyle(DsType.bodySm).copyWith(color: c.textSecondary);
    final remote = h.computerDisabledRemotely;
    final connected = computer.helper == 'connected';
    final enabled = connected && computer.enabled && !remote;
    final live = _live;
    return Column(
      key: const ValueKey('host.computer'),
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (!connected)
          Text(l.computerHelperState(computer.helper), style: body)
        else ...[
          Text(
            remote
                ? l.computerDisabledRemotely
                : computer.enabled
                ? l.computerEnabled
                : l.computerNotEnabled,
            key: const ValueKey('host.computer.enablement'),
            style: body,
          ),
          Text(l.computerOsAccount(computer.osAccount), style: caption),
          Text(
            l.computerPermissions(
              l.computerPermission(computer.capture),
              l.computerPermission(computer.input),
            ),
            style: caption,
          ),
          if (computer.locked) Text(l.computerLocked, style: caption),
        ],
        if (live != null) ...[
          const SizedBox(height: DsSpace.s8),
          Text(l.computerLiveSession, style: body),
          Row(
            children: [
              Expanded(
                child: Semantics(
                  liveRegion: true,
                  child: Text(_sessionText(l, live), style: caption),
                ),
              ),
              Semantics(
                button: true,
                label: l.computerStopLabel(live.desktop),
                excludeSemantics: true,
                child: TextButton(
                  key: const ValueKey('host.computer.stop'),
                  onPressed: _busy
                      ? null
                      : () => _run(() async {
                          await widget.repository.stopComputerSession(live.id);
                          await _loadLive();
                        }, l.computerStopFailed),
                  child: Text(l.computerStop),
                ),
              ),
            ],
          ),
        ],
        if (enabled)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: Semantics(
              button: true,
              label: l.computerDisableLabel(h.name),
              excludeSemantics: true,
              child: OutlinedButton(
                key: const ValueKey('host.computer.disable'),
                onPressed: _busy
                    ? null
                    : () => _run(() async {
                        final changed = await widget.repository.disableComputer(
                          h.id,
                        );
                        widget.onChanged(changed);
                        await _loadLive();
                      }, l.computerDisableFailed),
                child: Text(l.computerDisable),
              ),
            ),
          ),
        Padding(
          padding: const EdgeInsets.only(top: DsSpace.s4),
          child: Text(l.computerOnlyLocal, style: caption),
        ),
        if (_error case final text?)
          Semantics(
            liveRegion: true,
            child: Text(
              text,
              key: const ValueKey('host.computer.error'),
              style: dsTextStyle(DsType.bodySm).copyWith(color: c.dangerFg),
            ),
          ),
      ],
    );
  }
}
