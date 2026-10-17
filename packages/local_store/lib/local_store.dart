// SPDX-License-Identifier: Apache-2.0

/// The encrypted local database of the apps (docs/spec/15-client-architecture.md 6, ADR 0029,
/// R-CL-08): drift on SQLite3MultipleCiphers, schema version 1 with the mirror tables of 15 6.1
/// and the local-only tables, data access objects that hand out the domain types of
/// `package:domain`, removal of targets, the cache cleanup and offline search. No Flutter
/// (15 3.1 rule 1).
library;

export 'src/cleanup.dart';
export 'src/daos/commands_dao.dart';
export 'src/daos/file_cache_dao.dart';
export 'src/daos/instance_dao.dart';
export 'src/daos/messages_dao.dart';
export 'src/daos/outbox_dao.dart';
export 'src/daos/personal_dao.dart';
export 'src/daos/principals_dao.dart';
export 'src/daos/runs_dao.dart';
export 'src/daos/search_dao.dart';
export 'src/daos/share_inbox_dao.dart';
export 'src/daos/structure_dao.dart';
export 'src/daos/sync_state_dao.dart';
export 'src/daos/threads_dao.dart';
export 'src/database.dart' show LocalDatabase;
export 'src/migration_rules.dart';
export 'src/models.dart';
export 'src/open.dart';
export 'src/removal.dart';
