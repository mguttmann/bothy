// SPDX-License-Identifier: Apache-2.0

/// The one confirmation before "Log out" of the phone's Me tab (docs/spec/01-product.md 5.6 E7;
/// docs/decisions.d/AUD-client-phone-01.md): a logout deletes the local data with the outbox
/// and both device keys (04 8.5, R-SY-14), so a tap by mistake would lose the changes that still
/// wait to be sent and ask for a new login of the device.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// Asks once whether the person wants to log out and names what the logout deletes; with
/// [waiting] outbox entries above zero it says that they are lost. Returns true when the person
/// confirms; "Cancel" and a tap outside return false.
///
/// Example: `if (await confirmLogout(context, waiting: 2)) await controller.logout();`
Future<bool> confirmLogout(
  BuildContext context, {
  required int waiting,
}) async =>
    await showDialog<bool>(
      context: context,
      builder: (context) {
        final l = AppLocalizations.of(context);
        return AlertDialog(
          key: const ValueKey('logout.dialog'),
          title: Text(l.phoneLogoutTitle),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(l.phoneLogoutBody),
              if (waiting > 0)
                Padding(
                  padding: const EdgeInsets.only(top: DsSpace.s12),
                  child: Text(
                    l.phoneLogoutWaiting(waiting),
                    key: const ValueKey('logout.waiting'),
                  ),
                ),
            ],
          ),
          actions: [
            DsButton(
              key: const ValueKey('logout.cancel'),
              label: l.onboardingCancel,
              style: DsButtonStyle.ghost,
              onPressed: () => Navigator.of(context).pop(false),
            ),
            DsButton(
              key: const ValueKey('logout.confirm'),
              label: l.onboardingLogOut,
              style: DsButtonStyle.danger,
              onPressed: () => Navigator.of(context).pop(true),
            ),
          ],
        );
      },
    ) ??
    false;
