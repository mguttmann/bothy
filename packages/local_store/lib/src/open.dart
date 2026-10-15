// SPDX-License-Identifier: Apache-2.0

/// Opening the encrypted local database (docs/spec/15-client-architektur.md 6.3, R-CL-08,
/// T-DV-01): a 256-bit key from the [DatabaseKeySource] port, SQLite3MultipleCiphers with its
/// default cipher ChaCha20-Poly1305, and the database in its own isolate (15 7.1).
library;

import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift/isolate.dart' show DriftRemoteException;
import 'package:drift/native.dart';
import 'package:sqlite3/sqlite3.dart';

import 'database.dart';
import 'models.dart';

/// Length of the database key in bytes (256 bits, 15 6.3).
const int databaseKeyBytes = 32;

/// The cipher SQLite3MultipleCiphers uses by default and the store expects
/// (docs/DECISIONS.md D-M4-12-1).
const String expectedCipher = 'chacha20';

/// Port to the key of the local database. The app creates 32 random bytes at the first start
/// and keeps them in secure storage (M4-18 for desktop, M5 for mobile); this package never
/// stores the key.
abstract interface class DatabaseKeySource {
  /// Returns the 32-byte key of the database. Throws whatever the secure storage throws.
  Future<Uint8List> obtainKey();
}

/// Opens the encrypted database [file] with the key of [keys].
///
/// With [inBackground] (the default) the database runs in its own isolate
/// (`NativeDatabase.createInBackground`, 15 7.1); tests may open it in the calling isolate.
/// The first statement on the connection sets the key in the raw form `x'<64 hex>'`, which
/// SQLite3MultipleCiphers takes without a key derivation; then the opener checks that the key
/// opens the file, that the cipher is `chacha20` and that FTS5 is compiled in. The returned
/// database is open and migrated.
///
/// Throws [LocalStoreKeyError] when the key does not open the file and [LocalStoreSetupError]
/// when the key does not have 32 bytes or the SQLite library lacks encryption or FTS5. No
/// message contains the key.
///
/// Example: `final db = await openLocalStore(file: File(path), keys: secureKeys);`
Future<LocalDatabase> openLocalStore({
  required File file,
  required DatabaseKeySource keys,
  bool inBackground = true,
  DateTime Function()? clock,
}) async {
  final key = await keys.obtainKey();
  if (key.length != databaseKeyBytes) {
    throw const LocalStoreSetupError('the database key must have 32 bytes');
  }
  // A copy for the setup callback: it is sent to the database isolate, while the caller's list
  // is cleared below once the connection is open.
  final keyCopy = Uint8List.fromList(key);
  void setup(Database database) => _setUp(database, keyCopy);
  final QueryExecutor executor = inBackground
      ? NativeDatabase.createInBackground(file, setup: setup)
      : NativeDatabase(file, setup: setup);
  final db = LocalDatabase(executor, file: file, clock: clock);
  try {
    await db.customSelect('SELECT 1').get();
  } on Object catch (error) {
    await db.close();
    throw _unwrap(error);
  } finally {
    keyCopy.fillRange(0, keyCopy.length, 0);
  }
  return db;
}

/// Opens an empty in-memory database in the calling isolate for tests, with the same feature
/// checks as [openLocalStore] but without a key; [clock] replaces the time source.
LocalDatabase openInMemoryForTests({DateTime Function()? clock}) =>
    LocalDatabase(
      NativeDatabase.memory(setup: _checkFeatures),
      clock: clock,
    );

/// Runs on the database connection before drift uses it.
void _setUp(Database database, Uint8List key) {
  // The hex form exists only in this call (Design note 2): built, used once, not stored.
  final hex = StringBuffer();
  for (final byte in key) {
    hex.write(byte.toRadixString(16).padLeft(2, '0'));
  }
  database.execute('PRAGMA key = "x\'$hex\'"');
  try {
    database.select('SELECT count(*) FROM sqlite_master');
  } on SqliteException {
    throw const LocalStoreKeyError();
  }
  final cipher = database.select('PRAGMA cipher');
  if (cipher.isEmpty || cipher.first.values.first != expectedCipher) {
    throw const LocalStoreSetupError(
      'SQLite3MultipleCiphers with the cipher chacha20 is required '
      '(hooks: user_defines: sqlite3: source: sqlite3mc)',
    );
  }
  _checkFeatures(database);
  database.execute('PRAGMA journal_mode = WAL');
}

/// Fails when FTS5 is not compiled in (14 6, Design note 5).
void _checkFeatures(Database database) {
  final fts5 = database.select(
    "SELECT sqlite_compileoption_used('ENABLE_FTS5') AS on_",
  );
  if (fts5.first['on_'] != 1) {
    throw const LocalStoreSetupError('the SQLite library lacks FTS5');
  }
}

/// Returns the error of the setup callback that drift wraps when it ran in another isolate.
Object _unwrap(Object error) {
  var current = error;
  while (current is DriftRemoteException) {
    current = current.remoteCause;
  }
  return current;
}
