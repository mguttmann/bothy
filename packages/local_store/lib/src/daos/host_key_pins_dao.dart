// SPDX-License-Identifier: Apache-2.0

/// Access to `host_key_pins`: the pinned Noise keys of the hosts on this device
/// (docs/spec/15-client-architecture.md 6.1 and 23.3; brief M4-36 Design note 2). The app's
/// `HostKeyPinStore` of `terminal_ui` reads and writes through it.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../models.dart';

/// Data access object of the pinned host keys.
///
/// A pin is a trust anchor: nothing here deletes one. The cache cleanup and the removal of
/// targets never touch the table, every migration keeps its rows, and only the wipe of all
/// local data deletes it with the database (15 6.1, 6.2).
///
/// Example:
/// ```dart
/// await db.hostKeyPinsDao.write(StoredHostKeyPin(hostId: host, noisePublicKey: key,
///     keyGeneration: 1, keyFingerprint: fingerprint, firstSeenAt: clock.now()));
/// final pin = await db.hostKeyPinsDao.read(host);
/// ```
final class HostKeyPinsDao {
  /// Creates the DAO on a database; use `LocalDatabase.hostKeyPinsDao`.
  HostKeyPinsDao(this._db);

  final LocalDatabase _db;

  /// The pin of [hostId], or null when this device has none.
  Future<StoredHostKeyPin?> read(HostId hostId) async {
    final row = await (_db.select(
      _db.hostKeyPins,
    )..where((t) => t.hostId.equals(hostId.value))).getSingleOrNull();
    return row == null ? null : _pin(row);
  }

  /// Stores [pin], replacing the pin of the same host.
  Future<void> write(StoredHostKeyPin pin) => _db
      .into(_db.hostKeyPins)
      .insertOnConflictUpdate(
        HostKeyPinsCompanion.insert(
          hostId: pin.hostId.value,
          noisePublicKey: pin.noisePublicKey,
          keyGeneration: pin.keyGeneration,
          keyFingerprint: pin.keyFingerprint,
          firstSeenAt: pin.firstSeenAt.toUtc().millisecondsSinceEpoch,
        ),
      );

  /// Every pin of this device, ordered by host ID.
  Future<List<StoredHostKeyPin>> all() async {
    final rows = await (_db.select(
      _db.hostKeyPins,
    )..orderBy([(t) => OrderingTerm.asc(t.hostId)])).get();
    return [for (final r in rows) _pin(r)];
  }

  static StoredHostKeyPin _pin(HostKeyPinRow r) => StoredHostKeyPin(
    hostId: HostId.parse(r.hostId),
    noisePublicKey: r.noisePublicKey,
    keyGeneration: r.keyGeneration,
    keyFingerprint: r.keyFingerprint,
    firstSeenAt: DateTime.fromMillisecondsSinceEpoch(
      r.firstSeenAt,
      isUtc: true,
    ),
  );
}
