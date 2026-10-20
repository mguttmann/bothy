// SPDX-License-Identifier: Apache-2.0

/// Disable, enable, re-pair, retire and continue pairing (docs/spec/01-product.md 5.2 A8;
/// 24 3.1, 3.3, 3.4; brief M4-37 fixed requirement 17).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/hosts_repository.dart';

/// An action of [HostStateActions].
enum HostAction {
  /// Open the pairing wizard of a host in `pairing` or `confirming`.
  pair,

  /// Issue a new pairing code: from `active` or `disabled` the keys are dropped (24 3.3).
  repair,

  /// Disable an active host.
  disable,

  /// Enable a host an owner disabled (not after `duplicate_connection`, 24 3.4).
  enable,

  /// Retire an active or disabled host for good.
  retire,
}

/// The actions the life cycle of 24 3.1 draws from the state of [host]; a retired host has
/// none. These follow the state the server returned, not a permission: whether the caller may
/// act is the server's answer to the call. [onAction] runs the chosen action; [disabled] turns
/// the buttons off while a change runs.
class HostStateActions extends StatelessWidget {
  /// Creates the actions.
  const HostStateActions({
    super.key,
    required this.host,
    required this.onAction,
    this.disabled = false,
  });

  /// The host.
  final AdminHost host;

  /// Runs an action.
  final ValueChanged<HostAction> onAction;

  /// Whether the buttons are off.
  final bool disabled;

  /// The actions of [host] in the order they are shown.
  static List<HostAction> of(AdminHost host) => switch (host.state) {
    HostLifecycle.pairing ||
    HostLifecycle.confirming => [HostAction.pair, HostAction.repair],
    HostLifecycle.active => [
      HostAction.repair,
      HostAction.disable,
      HostAction.retire,
    ],
    HostLifecycle.disabled => [
      if (host.ownerDisabled) HostAction.enable,
      HostAction.repair,
      HostAction.retire,
    ],
    HostLifecycle.retired || HostLifecycle.unknown => const [],
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    String label(HostAction a) => switch (a) {
      HostAction.pair => l.hostPair,
      HostAction.repair => l.hostRepair,
      HostAction.disable => l.hostDisable,
      HostAction.enable => l.hostEnable,
      HostAction.retire => l.hostRetire,
    };
    return Wrap(
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s8,
      children: [
        for (final a in of(host))
          a == HostAction.pair || a == HostAction.enable
              ? FilledButton(
                  key: ValueKey('host.${a.name}'),
                  onPressed: disabled ? null : () => onAction(a),
                  child: Text(label(a)),
                )
              : OutlinedButton(
                  key: ValueKey('host.${a.name}'),
                  style: a == HostAction.retire || a == HostAction.disable
                      ? OutlinedButton.styleFrom(
                          foregroundColor: colors.dangerFg,
                        )
                      : null,
                  onPressed: disabled ? null : () => onAction(a),
                  child: Text(label(a)),
                ),
      ],
    );
  }
}
