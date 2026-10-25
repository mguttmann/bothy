// SPDX-License-Identifier: Apache-2.0

/// The pins of the terminal in the encrypted local database (docs/spec/15-client-architecture.md
/// 6.1, 23.1 and 23.3; brief M4-36 Design note 2): the port `HostKeyPinStore` of `terminal_ui`
/// over the table `host_key_pins` of `local_store`.
library;

import 'package:domain/domain.dart' show HostId;
import 'package:local_store/local_store.dart'
    show HostKeyPinsDao, StoredHostKeyPin;
import 'package:terminal_ui/terminal_ui.dart' show HostKeyPin, HostKeyPinStore;

/// [HostKeyPinStore] over [HostKeyPinsDao].
///
/// The table survives every migration and every cache cleanup; only the wipe of all local
/// data on logout or revocation deletes it (15 6.1, 6.2).
///
/// Example:
/// ```dart
/// final pins = LocalHostKeyPinStore(database.hostKeyPinsDao);
/// final pin = await pins.read(hostId);
/// ```
final class LocalHostKeyPinStore implements HostKeyPinStore {
  /// Creates the store over [dao].
  LocalHostKeyPinStore(this._dao);

  final HostKeyPinsDao _dao;

  @override
  Future<HostKeyPin?> read(String hostId) async {
    final pin = await _dao.read(HostId.parse(hostId));
    if (pin == null) return null;
    return HostKeyPin(
      hostId: pin.hostId.value,
      noisePublicKey: pin.noisePublicKey,
      keyGeneration: pin.keyGeneration,
      fingerprint: pin.keyFingerprint,
      firstSeen: pin.firstSeenAt,
    );
  }

  @override
  Future<void> write(HostKeyPin pin) => _dao.write(
    StoredHostKeyPin(
      hostId: HostId.parse(pin.hostId),
      noisePublicKey: pin.noisePublicKey,
      keyGeneration: pin.keyGeneration,
      keyFingerprint: pin.fingerprint,
      firstSeenAt: pin.firstSeen.toUtc(),
    ),
  );
}
