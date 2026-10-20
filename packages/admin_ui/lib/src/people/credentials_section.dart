// SPDX-License-Identifier: Apache-2.0

/// The credentials of a person: password, one-time code app, linked logins and passkeys
/// (docs/spec/08-identity.md 2.5, 3.3; 05 `identity.unlink`, `passkey.revoke` with step-up).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/admin_repository.dart';
import '../self/device_tile.dart' show activityText;

/// The overview of [credentials] with "Unlink" per linked login and "Revoke" per passkey.
///
/// [onUnlink] and [onRevokePasskey] are null while a change runs; [denied] holds the keys of
/// controls the server refused (`identity.<id>`, `passkey.<id>`), which the section hides.
class CredentialsSection extends StatelessWidget {
  /// Creates the section.
  const CredentialsSection({
    super.key,
    required this.credentials,
    required this.now,
    required this.timeZone,
    this.onUnlink,
    this.onRevokePasskey,
    this.denied = const {},
  });

  /// The credentials.
  final AdminCredentials credentials;

  /// The current time, for the relative day a passkey was added.
  final DateTime now;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// Unlinks a login, or null.
  final ValueChanged<AdminIdentity>? onUnlink;

  /// Revokes a passkey, or null.
  final ValueChanged<AdminPasskey>? onRevokePasskey;

  /// Refused controls by key.
  final Set<String> denied;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final strong = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    Widget row(Key key, String title, String detail, Widget? action) => Padding(
      key: key,
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: strong),
                Text(detail, style: secondary),
              ],
            ),
          ),
          ?action,
        ],
      ),
    );

    return Column(
      key: const ValueKey('credentials'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.adminCredentialsTitle,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        Text(
          credentials.localPasswordSet
              ? l.adminCredentialsPasswordSet
              : l.adminCredentialsPasswordNone,
          style: secondary,
        ),
        Text(
          credentials.totpEnrolled
              ? l.adminCredentialsTotpSet
              : l.adminCredentialsTotpNone,
          style: secondary,
        ),
        const SizedBox(height: DsSpace.s8),
        Text(l.adminCredentialsIdentities, style: strong),
        if (credentials.identities.isEmpty)
          Text(l.adminCredentialsNone, style: secondary),
        for (final identity in credentials.identities)
          row(
            ValueKey('identity.${identity.id}'),
            identity.subject,
            identity.issuer ?? identity.providerKind,
            onUnlink == null || denied.contains('identity.${identity.id}')
                ? null
                : OutlinedButton(
                    key: ValueKey('identity.${identity.id}.unlink'),
                    onPressed: () => onUnlink!(identity),
                    child: Text(l.adminUnlink),
                  ),
          ),
        const SizedBox(height: DsSpace.s8),
        Text(l.adminCredentialsPasskeys, style: strong),
        if (credentials.passkeys.isEmpty)
          Text(l.adminCredentialsNone, style: secondary),
        for (final passkey in credentials.passkeys)
          row(
            ValueKey('passkey.${passkey.id}'),
            passkey.name,
            l.adminPasskeyAdded(
              activityText(
                context,
                passkey.createdAt,
                now: now,
                timeZone: timeZone,
              ),
            ),
            onRevokePasskey == null || denied.contains('passkey.${passkey.id}')
                ? null
                : OutlinedButton(
                    key: ValueKey('passkey.${passkey.id}.revoke'),
                    onPressed: () => onRevokePasskey!(passkey),
                    child: Text(l.adminRevoke),
                  ),
          ),
      ],
    );
  }
}
