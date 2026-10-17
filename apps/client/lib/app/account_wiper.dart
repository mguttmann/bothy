// SPDX-License-Identifier: Apache-2.0

/// The one sign-out path of the app (R-SY-14; brief M4-18 Design note 2; docs/DECISIONS.md
/// D-M4-18-2).
// The named parameters keep public names (stop, keys) while the fields stay private.
// ignore_for_file: prefer_initializing_formals
library;

import 'dart:io';

import '../platform/device_key_signer.dart';
import '../platform/secure_store.dart';

/// Why the account of this device ended; E7 shows the matching text (01 5.6).
enum SignOutCause {
  /// The person logged out, or cancelled E4.
  loggedOut,

  /// Another device of the person revoked this one (`revoked_by_self`).
  revokedBySelf,

  /// The owner revoked this device (`revoked_by_owner`).
  revokedByOwner,

  /// The person was disabled (`user_disabled`); E7 shows no login button.
  userDisabled,

  /// The device was not used for too long (`inactive`).
  inactive,

  /// The confirmation in E4 was rejected (`verification_failed`).
  rejected,

  /// E4 expired (`pending_expired`).
  pendingExpired,

  /// The server ended the session (`4012`, `session.revoked`, `401 session_revoked`).
  sessionRevoked,

  /// The server no longer accepts the refresh token (`auth.session_max_age`).
  sessionExpired;

  /// The cause of a `devices.revoke_reason` code, or [sessionRevoked] for an unknown one.
  static SignOutCause ofRevokeReason(String? reason) => switch (reason) {
    'revoked_by_self' => revokedBySelf,
    'revoked_by_owner' => revokedByOwner,
    'user_disabled' => userDisabled,
    'inactive' => inactive,
    'verification_failed' => rejected,
    'pending_expired' => pendingExpired,
    _ => sessionRevoked,
  };
}

/// Deletes every local datum of one instance's account, for every sign-out cause: logout, E4
/// "Cancel", a rejected or expired device, `4012`, `session.revoked` and `401 session_revoked`
/// (R-SY-14, 04 8.5).
///
/// [wipe] stops the engine ([stop]), closes and deletes the local database ([wipeDatabase]),
/// deletes the instance's folder with the database files and the file cache, deletes both
/// device keys and deletes every secret of the instance (tokens, database key, device ID).
/// Only the server address stays, which is not an instance secret ([AppPreferences]). Each
/// step runs even when an earlier one failed, so a broken database never keeps a key alive.
///
/// Example: `await wiper.wipe(); showSignedOut(cause);`
final class AccountWiper {
  /// Creates the wiper of one instance.
  AccountWiper({
    required this.instanceId,
    required this.folder,
    required Future<void> Function() stop,
    required Future<void> Function() wipeDatabase,
    required DeviceKeyStore keys,
    required InstanceSecrets secrets,
  }) : _stop = stop,
       _wipeDatabase = wipeDatabase,
       _keys = keys,
       _secrets = secrets;


  /// The instance whose data goes.
  final String instanceId;

  /// The folder of the instance: database files and file cache.
  final Directory folder;

  final Future<void> Function() _stop;
  final Future<void> Function() _wipeDatabase;
  final DeviceKeyStore _keys;
  final InstanceSecrets _secrets;
  Future<void>? _running;

  /// Deletes everything; a second call waits for the first and does nothing more.
  Future<void> wipe() => _running ??= _wipe();

  Future<void> _wipe() async {
    final failures = <Object>[];
    Future<void> step(Future<void> Function() body) async {
      try {
        await body();
      } on Object catch (e) {
        failures.add(e);
      }
    }

    await step(_stop);
    await step(_wipeDatabase);
    await step(() async {
      if (await folder.exists()) await folder.delete(recursive: true);
    });
    await step(() => _keys.delete(instanceId));
    await step(_secrets.deleteAll);
    if (failures.isNotEmpty) {
      throw StateError(
        'wipe incomplete: ${failures.map((f) => f.runtimeType).join(', ')}',
      );
    }
  }
}
