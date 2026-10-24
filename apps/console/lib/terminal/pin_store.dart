// SPDX-License-Identifier: Apache-2.0

/// The pinned host keys of the console in IndexedDB of the origin
/// (docs/spec/15-client-architecture.md 23.1 and 23.3; ADR 0070 item 3; brief M6-18 Design note 3).
library;

import 'package:terminal_ui/terminal_ui.dart' show HostKeyPin, HostKeyPinStore;

import 'ports.dart';

/// [HostKeyPinStore] of the console: one record per host (public key, key generation,
/// fingerprint, first seen) in the IndexedDB database of the origin, so the console pins per
/// browser profile (15 23.3).
///
/// [clear] deletes the whole database; the console calls it at logout and when the server ends
/// the session, before it shows E7 (R-CL-15). Deleting costs no protection: in the browser the
/// pin only notices a key change an honest server reports, because the server delivers the
/// console's code itself (24 5.1).
///
/// A record that cannot be read as a pin counts as no pin: the controller then pins the key
/// again after the next handshake, which is what a first contact does.
///
/// Example:
/// ```dart
/// final pins = IndexedDbHostKeyPinStore(ports.pins);
/// final pin = await pins.read(hostId);
/// await pins.clear();
/// ```
final class IndexedDbHostKeyPinStore implements HostKeyPinStore {
  /// Creates the store over [database].
  IndexedDbHostKeyPinStore(this.database);

  /// The IndexedDB seam (a fake in tests, memory outside the browser).
  final PinDatabase database;

  @override
  Future<HostKeyPin?> read(String hostId) async {
    final record = await database.read(hostId);
    if (record == null) return null;
    final key = record['noise_public_key'];
    // IndexedDB hands numbers back as JavaScript numbers, so an integer may arrive as a double.
    final generation = record['key_generation'];
    final fingerprint = record['fingerprint'];
    final firstSeen = record['first_seen'];
    if (key is! String ||
        generation is! num ||
        fingerprint is! String ||
        firstSeen is! String) {
      return null;
    }
    final seen = DateTime.tryParse(firstSeen);
    if (seen == null) return null;
    return HostKeyPin(
      hostId: hostId,
      noisePublicKey: key,
      keyGeneration: generation.toInt(),
      fingerprint: fingerprint,
      firstSeen: seen,
    );
  }

  @override
  Future<void> write(HostKeyPin pin) => database.write(pin.hostId, {
    'host_id': pin.hostId,
    'noise_public_key': pin.noisePublicKey,
    'key_generation': pin.keyGeneration,
    'fingerprint': pin.fingerprint,
    'first_seen': pin.firstSeen.toUtc().toIso8601String(),
  });

  /// Deletes every pin of this browser profile (the whole database of the origin).
  ///
  /// Throws what the database throws; the caller decides whether E7 waits for it.
  Future<void> clear() => database.delete();
}
